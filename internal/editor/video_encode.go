package editor

import (
	"fmt"
	"strings"
)

func validateVideoEncoder(encoder string) error {
	switch strings.TrimSpace(encoder) {
	case "", VideoEncoderNVENC:
		return nil
	default:
		return fmt.Errorf("unsupported video encoder %q (supported: %q or libx264 default)", encoder, VideoEncoderNVENC)
	}
}

func appendVideoEncodeArgs(command []string, short ShortEdit) []string {
	crf := videoCRFForCommand(short.VideoCRF)
	switch strings.TrimSpace(short.VideoEncoder) {
	case VideoEncoderNVENC:
		return append(command,
			"-c:v", "h264_nvenc",
			"-preset", "p5",
			"-rc", "vbr",
			"-b:v", "0",
			"-cq", fmt.Sprintf("%d", crf),
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
