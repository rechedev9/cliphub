package workers

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/vodplays"
)

type killFeedRequest struct {
	ffmpeg  string
	source  string
	workDir string
	plan    streamclips.EditPlan
	clip    streamclips.ClipRange
	probe   streamclips.SourceProbe
}

// clipKillFeed rebuilds the streamer's notices seen during the clip and
// scales them to the output, with times relative to the clip start.
func clipKillFeed(ctx context.Context, req killFeedRequest) ([]streamclips.KillFeedNotice, error) {
	if !req.plan.KillFeedEnabled() {
		return nil, nil
	}
	scale, err := streamclips.KillFeedScale(req.plan, req.probe.Width, req.probe.Height)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(req.workDir, "killfeed", req.clip.ID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	sprites, err := vodplays.ExtractNotices(ctx, vodplays.NoticeOptions{
		FFmpeg: req.ffmpeg, Input: req.source, WorkDir: dir,
		StartSeconds: req.clip.StartSeconds, EndSeconds: req.clip.EndSeconds,
	})
	if err != nil {
		return nil, err
	}
	duration := req.clip.EndSeconds - req.clip.StartSeconds
	notices := make([]streamclips.KillFeedNotice, 0, len(sprites))
	for i, sprite := range sprites {
		scaled, err := vodplays.ScaleNotice(sprite, scale, filepath.Join(dir, fmt.Sprintf("scaled-%03d.png", i+1)))
		if err != nil {
			return nil, err
		}
		notices = append(notices, streamclips.KillFeedNotice{
			Path: scaled.Path, Width: scaled.Width, Height: scaled.Height,
			AppearSeconds:    scaled.AppearSeconds - req.clip.StartSeconds,
			DisappearSeconds: math.Min(scaled.DisappearSeconds-req.clip.StartSeconds, duration),
		})
	}
	return notices, nil
}
