package recording

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// encoderTrialTimeout bounds the trial encode of CheckEncoderUsable.
const encoderTrialTimeout = 20 * time.Second

// EncoderFFmpegCodec maps a public stream encoder name to the ffmpeg encoder
// id emitted by ffmpegSettingsCommand. It must stay in step with the codec
// strings in that function.
func EncoderFFmpegCodec(encoder string) string {
	switch encoder {
	case EncoderNVENC:
		return "h264_nvenc"
	case EncoderAMF:
		return "h264_amf"
	case EncoderQSV:
		return "h264_qsv"
	default:
		return "libx264"
	}
}

// HLAEStreamFFmpeg resolves the ffmpeg binary HLAE's mirv_streams will invoke
// for the given HLAE.exe: the bundled ffmpeg\bin\ffmpeg.exe when present, then
// configured, the ffmpeg zv-recorder writes into ffmpeg\ffmpeg.ini when Studio
// passes one, then the Path declared in that ini, and finally the PATH ffmpeg
// that ensureHLAEFFmpegConfig would write into a fresh ini. Capture-encoder
// probes must use this binary, not whatever ffmpeg the caller itself resolved,
// because the two can differ per install.
func HLAEStreamFFmpeg(hlaeExe, configured string) string {
	dir := filepath.Join(filepath.Dir(hlaeExe), "ffmpeg")
	bundled := filepath.Join(dir, "bin", "ffmpeg.exe")
	if _, err := os.Stat(bundled); err == nil {
		return bundled
	}
	// zv-recorder only writes the configured ffmpeg into an existing ffmpeg
	// folder; without one HLAE never sees it, so the probe must not either.
	if _, err := os.Stat(dir); configured != "" && err == nil {
		return configured
	}
	if data, err := os.ReadFile(filepath.Join(dir, "ffmpeg.ini")); err == nil {
		if path := HLAEFFmpegINIPath(data); path != "" {
			return path
		}
	}
	return FindFFmpeg()
}

// CheckEncoderSupported verifies that the ffmpeg binary HLAE's mirv_streams
// will use can encode with the requested stream encoder. Software x264 needs no
// probe (ffmpegSettingsCommand only ever emits -c:v libx264 there); hardware
// encoders must appear in `ffmpeg -encoders`. ffmpegPath is the resolved binary
// (recording.FindFFmpeg / whatever zr-recorder forwards through HLAE's
// ffmpeg.ini), not the encoder's feature flag on the host GPU.
func CheckEncoderSupported(ffmpegPath, encoder string) error {
	if encoder == "" || encoder == EncoderLibx264 {
		return nil
	}
	if !ValidEncoder(encoder) {
		return fmt.Errorf("unknown capture encoder %q", encoder)
	}
	if ffmpegPath == "" {
		return fmt.Errorf("cannot verify capture encoder %q: ffmpeg not found", encoder)
	}
	want := EncoderFFmpegCodec(encoder)
	if want == "libx264" {
		return nil
	}
	// #nosec G204 -- ffmpegPath is a locally resolved tool path and args are fixed.
	out, err := exec.Command(ffmpegPath, "-hide_banner", "-encoders").Output()
	if err != nil {
		return fmt.Errorf("query encoders from %q: %w", ffmpegPath, err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, want) {
			return nil
		}
	}
	return fmt.Errorf("capture encoder %q requires %s, which %q does not provide (check the resolved HLAE ffmpeg and its ffmpeg.ini)", encoder, want, ffmpegPath)
}

// CheckEncoderUsable verifies that ffmpegPath can really encode with the
// encoder on this machine. A build lists h264_nvenc whatever the GPU driver
// is, and an older driver then refuses to open it, so a hardware encoder must
// also get through a two-frame trial encode.
func CheckEncoderUsable(ffmpegPath, encoder string) error {
	if err := CheckEncoderSupported(ffmpegPath, encoder); err != nil {
		return err
	}
	codec := EncoderFFmpegCodec(encoder)
	if codec == "libx264" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), encoderTrialTimeout)
	defer cancel()
	// #nosec G204 -- ffmpegPath is a locally resolved tool path and args are fixed.
	out, err := exec.CommandContext(ctx, ffmpegPath, "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=black:s=256x256:r=30", "-frames:v", "2", "-c:v", codec, "-f", "null", "-").CombinedOutput()
	if err != nil {
		reason, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
		return fmt.Errorf("%s from %q does not open on this machine: %w: %s", codec, ffmpegPath, err, strings.TrimSpace(reason))
	}
	return nil
}
