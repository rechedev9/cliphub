package workers

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/subtitles"
	"github.com/rechedev9/cliphub/internal/tasks"
)

// SubtitleSource gives a render the transcript of its demo's team voices; it never waits for a transcription.
type SubtitleSource interface {
	Subtitles(ctx context.Context, req subtitles.Request) (*subtitles.Track, error)
}

// UseSubtitles gives the render the demo transcripts it burns as subtitles.
func (w *RenderWorker) UseSubtitles(source SubtitleSource) {
	w.subtitles = source
}

// renderSubtitles reads the transcript a Short, a summary or an approved Full Demo burns. Subtitles never
// block a render: without a transcript the video goes without them.
func (w *RenderWorker) renderSubtitles(ctx context.Context, j job.Job, edit renderplan.EditRequest) *subtitles.Track {
	req := subtitles.Request{JobID: j.ID, Target: j.TargetSteamID, Hidden: edit.SubtitlesHidden}
	on := edit.Subtitles
	if edit.FullDemo != nil {
		options := edit.FullDemo.Document.Options.Subtitles
		on = options != nil && options.Enabled
		if on {
			req.Hidden = options.Hidden
		}
	}
	if !on || w.subtitles == nil {
		return nil
	}
	track, err := w.subtitles.Subtitles(ctx, req)
	if err != nil {
		logWorkerError(j.ID, tasks.TypeRenderVariant, fmt.Errorf("subtitles left out: %w", err))
		return nil
	}
	return track
}

// writeShortSubtitles writes the subtitle track of a Short or summary; a Full Demo carries its own in
// the execution.
func writeShortSubtitles(in shortDataInput) string {
	if in.extras.Subtitles == nil || in.edit.FullDemo != nil {
		return ""
	}
	path := filepath.Join(in.workDir, "subtitles.json")
	if err := writeJSONFile(path, in.extras.Subtitles); err != nil {
		logWorkerError(in.job.ID, tasks.TypeRenderVariant, fmt.Errorf("subtitles left out: %w", err))
		return ""
	}
	return path
}
