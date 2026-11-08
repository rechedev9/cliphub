package workers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/rechedev9/cliphub/internal/streamclips"
)

// errSourceNotMP4 marks a source Studio's player cannot open and FFmpeg could not repair.
var errSourceNotMP4 = errors.New("stream source is MPEG-TS, not MP4")

// sourceIsMPEGTS reports a VOD that yt-dlp saved as MPEG-TS inside the .mp4,
// which happens when it cannot reach FFmpeg. Chromium does not play MPEG-TS.
func sourceIsMPEGTS(probe streamclips.SourceProbe) bool {
	return probe.Container == "mpegts"
}

// remuxSourceToMP4 rewrites path in place as MP4 without re-encoding.
func (w *AcquireWorker) remuxSourceToMP4(ctx context.Context, path string) error {
	if w.cfg.FFmpegPath == "" {
		return fmt.Errorf("%w and ffmpeg is not configured", errSourceNotMP4)
	}
	remuxed := strings.TrimSuffix(path, ".mp4") + ".remux.mp4"
	_, err := w.runner.Run(ctx, w.cfg.FFmpegPath,
		"-hide_banner", "-nostdin", "-loglevel", "error", "-y",
		"-i", path,
		"-map", "0:v", "-map", "0:a?",
		"-c", "copy",
		"-movflags", "+faststart",
		remuxed,
	)
	if err != nil {
		_ = os.Remove(remuxed)
		return fmt.Errorf("%w: remux failed: %w", errSourceNotMP4, err)
	}
	if err := os.Rename(remuxed, path); err != nil {
		_ = os.Remove(remuxed)
		return fmt.Errorf("replace stream source with remuxed MP4: %w", err)
	}
	return nil
}
