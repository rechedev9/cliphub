package workers

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/subtitles"
	"github.com/rechedev9/cliphub/internal/tasks"
)

// SubtitleSource gives a render the transcript of its demo's team voices, waiting for one that runs.
type SubtitleSource interface {
	Subtitles(ctx context.Context, req subtitles.Request) (*subtitles.Track, error)
}

// UseSubtitles gives the render the demo transcripts it burns as subtitles.
func (w *RenderWorker) UseSubtitles(source SubtitleSource) {
	w.subtitles = source
}

// fullDemoSubtitles reads the transcript an approved Full Demo burns; without one the video goes without subtitles.
func (w *RenderWorker) fullDemoSubtitles(ctx context.Context, j job.Job, options *recapplan.SubtitleOptions) *subtitles.Track {
	if options == nil || !options.Enabled || w.subtitles == nil {
		return nil
	}
	track, err := w.subtitles.Subtitles(ctx, subtitles.Request{JobID: j.ID, Target: j.TargetSteamID, English: options.English, Hidden: options.Hidden})
	if err != nil {
		logWorkerError(j.ID, tasks.TypeRenderVariant, fmt.Errorf("subtitles left out: %w", err))
		return nil
	}
	return track
}

// writeShortSubtitles writes the subtitle track of a Short or summary. Subtitles never block a
// render: without a transcript the video goes without them.
func (w *RenderWorker) writeShortSubtitles(ctx context.Context, in shortDataInput) string {
	edit := in.edit
	if !edit.Subtitles || edit.FullDemo != nil || w.subtitles == nil {
		return ""
	}
	track, err := w.subtitles.Subtitles(ctx, subtitles.Request{JobID: in.job.ID, Target: in.job.TargetSteamID, English: edit.SubtitlesEnglish, Hidden: edit.SubtitlesHidden})
	if err != nil {
		logWorkerError(in.job.ID, tasks.TypeRenderVariant, fmt.Errorf("subtitles left out: %w", err))
		return ""
	}
	path := filepath.Join(in.workDir, "subtitles.json")
	if err := writeJSONFile(path, track); err != nil {
		logWorkerError(in.job.ID, tasks.TypeRenderVariant, fmt.Errorf("subtitles left out: %w", err))
		return ""
	}
	return path
}
