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

// nvencCQOffset maps an x264 CRF to the NVENC CQ of the same SSIM on CS2
// captures. At CQ 16 to 18 NVENC spends about 35 Mbps for no visible gain.
const nvencCQOffset = 4

func appendVideoEncodeArgs(command []string, short ShortEdit) []string {
	crf := videoCRFForCommand(short.VideoCRF)
	switch strings.TrimSpace(short.VideoEncoder) {
	case VideoEncoderNVENC:
		return append(command,
			"-c:v", "h264_nvenc",
			"-preset", "p5",
			"-rc", "vbr",
			"-b:v", "0",
			"-cq", fmt.Sprintf("%d", min(crf+nvencCQOffset, 51)),
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
