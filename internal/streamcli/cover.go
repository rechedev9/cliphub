package streamcli

import (
	"context"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
)

type streamCoverGenerator interface {
	Generate(ctx context.Context, ffmpeg, videoPath, coverPath string, atSeconds float64) error
}

type ffmpegStreamCoverGenerator struct{}

func (ffmpegStreamCoverGenerator) Generate(ctx context.Context, ffmpeg, videoPath, coverPath string, atSeconds float64) error {
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", videoPath,
		"-ss", strconv.FormatFloat(atSeconds, 'f', 3, 64),
		"-frames:v", "1", "-q:v", "2", coverPath,
	}
	// #nosec G204 -- ffmpeg is a configured local executable and arguments are
	// passed directly without a shell.
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	detail := strings.TrimSpace(string(out))
	if len(detail) > 4096 {
		detail = detail[:4096] + "..."
	}
	if detail == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, detail)
}

// streamCoverTimestamp picks a stable, non-black frame from the rendered clip.
// The first third of the clip avoids both the opening fade and the tail.
func streamCoverTimestamp(renderedDuration float64) float64 {
	return clampStreamCoverTimestamp(renderedDuration*0.35, renderedDuration)
}

func clampStreamCoverTimestamp(at, duration float64) float64 {
	if math.IsNaN(at) || math.IsInf(at, 0) || at < 0 {
		at = 0
	}
	if duration > 0 {
		lastFrame := math.Max(0, duration-0.05)
		at = math.Min(at, lastFrame)
	}
	return at
}
