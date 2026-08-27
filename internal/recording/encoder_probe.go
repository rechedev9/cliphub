package recording

import (
	"fmt"
	"os/exec"
	"strings"
)

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