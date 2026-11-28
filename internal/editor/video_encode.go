package editor

import (
	"fmt"
	"strings"
	"sync"

	"github.com/rechedev9/cliphub/internal/recording"
)

const (
	VideoEncoderAuto = "auto"
	VideoEncoderAMF  = "amf-h264"
	VideoEncoderQSV  = "qsv-h264"
)

var hardwareEncoderCache = struct {
	mu      sync.Mutex
	entries map[string]string
}{
	entries: map[string]string{},
}

var hardwareEncoderCandidates = []struct {
	recordingEncoder string
	editorEncoder    string
}{
	{recordingEncoder: recording.EncoderNVENC, editorEncoder: VideoEncoderNVENC},
	{recordingEncoder: recording.EncoderAMF, editorEncoder: VideoEncoderAMF},
	{recordingEncoder: recording.EncoderQSV, editorEncoder: VideoEncoderQSV},
}

func DetectHardwareEncoder(ffmpegPath string) string {
	resolved := strings.TrimSpace(ffmpegPath)
	if resolved == "" {
		resolved = recording.FindFFmpeg()
	}
	if resolved == "" {
		return ""
	}
	hardwareEncoderCache.mu.Lock()
	if cached, ok := hardwareEncoderCache.entries[resolved]; ok {
		hardwareEncoderCache.mu.Unlock()
		return cached
	}
	hardwareEncoderCache.mu.Unlock()

	detected := probeHardwareEncoder(resolved)

	hardwareEncoderCache.mu.Lock()
	hardwareEncoderCache.entries[resolved] = detected
	hardwareEncoderCache.mu.Unlock()
	return detected
}

func probeHardwareEncoder(ffmpegPath string) string {
	for _, candidate := range hardwareEncoderCandidates {
		if err := recording.CheckEncoderUsable(ffmpegPath, candidate.recordingEncoder); err == nil {
			return candidate.editorEncoder
		}
	}
	return ""
}

func validateVideoEncoder(encoder string) error {
	switch strings.TrimSpace(encoder) {
	case "", VideoEncoderAuto, VideoEncoderNVENC, VideoEncoderAMF, VideoEncoderQSV:
		return nil
	default:
		return fmt.Errorf(
			"unsupported video encoder %q (supported: %q, %q, %q, %q or libx264 default)",
			encoder, VideoEncoderAuto, VideoEncoderNVENC, VideoEncoderAMF, VideoEncoderQSV,
		)
	}
}

// nvencCQOffset maps an x264 CRF to the NVENC CQ of the same SSIM on CS2
// captures. At CQ 16 to 18 NVENC spends about 35 Mbps for no visible gain.
const nvencCQOffset = 4

func appendVideoEncodeArgs(command []string, short ShortEdit) []string {
	crf := videoCRFForCommand(short.VideoCRF)
	encoder := strings.TrimSpace(short.VideoEncoder)
	if encoder == VideoEncoderAuto {
		ffmpegPath := ""
		if len(command) > 0 {
			ffmpegPath = command[0]
		}
		encoder = DetectHardwareEncoder(ffmpegPath)
	}
	switch encoder {
	case VideoEncoderNVENC:
		return append(command,
			"-c:v", "h264_nvenc",
			"-preset", "p5",
			"-rc", "vbr",
			"-b:v", "0",
			"-cq", fmt.Sprintf("%d", min(crf+nvencCQOffset, 51)),
			"-pix_fmt", "yuv420p",
		)
	case VideoEncoderAMF:
		qp := fmt.Sprintf("%d", min(crf+nvencCQOffset, 51))
		return append(command,
			"-c:v", "h264_amf",
			"-quality", "balanced",
			"-rc", "cqp",
			"-qp_i", qp,
			"-qp_p", qp,
			"-pix_fmt", "yuv420p",
		)
	case VideoEncoderQSV:
		return append(command,
			"-c:v", "h264_qsv",
			"-preset", "medium",
			"-global_quality", fmt.Sprintf("%d", min(crf+nvencCQOffset, 51)),
			"-pix_fmt", "yuv420p",
		)
	default:
		return append(command,
			"-c:v", "libx264",
			"-preset", videoPresetForCommand(short.VideoPreset),
			"-crf", fmt.Sprintf("%d", crf),
		)
	}
}
