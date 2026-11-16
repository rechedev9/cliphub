package workers

import (
	"context"
	"io"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/streamclips"
)

// progressCommandRunner streams a command's stdout while it runs.
type progressCommandRunner interface {
	RunProgress(ctx context.Context, exe string, progress io.Writer, args ...string) ([]byte, error)
}

type streamClipProgress struct {
	jobID     uuid.UUID
	plan      streamclips.EditPlan
	index     int
	placement streamclips.ClipPlacement
	args      []string
}

// runClipWithProgress runs one clip's FFmpeg render and posts the render's
// progress on cfg.Progress while it encodes.
func (w *StreamRenderWorker) runClipWithProgress(ctx context.Context, cfg StreamRenderWorkerConfig, r streamClipProgress) error {
	clips := len(r.plan.Clips)
	report := func(fraction float64) {
		progress := streamclips.NewRenderProgress(r.plan.Variant, r.index, clips, fraction)
		// The percent weighs every moment; the label counts videos, where a group is one.
		progress.Clip, progress.Clips = r.placement.Video, r.placement.Videos
		cfg.Progress.Set(r.jobID, progress)
	}
	report(0)
	runner, ok := w.runner.(progressCommandRunner)
	if !ok || cfg.Progress == nil {
		_, err := w.runner.Run(ctx, cfg.FFmpegPath, r.args...)
		return err
	}
	duration := r.plan.Clips[r.index].OutputDurationSeconds()
	args := append([]string{"-progress", "pipe:1", "-nostats"}, r.args...)
	_, err := runner.RunProgress(ctx, cfg.FFmpegPath, editor.NewFFmpegProgressWriter(duration, report), args...)
	return err
}
