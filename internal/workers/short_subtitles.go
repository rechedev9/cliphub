package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/subtitles"
	"github.com/rechedev9/cliphub/internal/tasks"
)

const (
	subtitleWaitMax      = 30 * time.Minute
	subtitlePollInterval = 2 * time.Second
	subtitleWaitPercent  = 1
)

const (
	subtitlesFailedWarning  = "Salió sin subtítulos: la transcripción de las voces falló. Transcríbelas de nuevo en Subtítulos y vuelve a crear el vídeo."
	subtitlesMissingWarning = "Salió sin subtítulos: no se pudieron transcribir las voces de la demo."
)

type SubtitleSource interface {
	Subtitles(ctx context.Context, req subtitles.Request) (subtitles.Lookup, error)
}

type subtitleReader struct {
	source       SubtitleSource
	waitMax      time.Duration
	pollInterval time.Duration
}

func (r subtitleReader) pollCountMax() int {
	return int(r.waitMax/r.pollInterval) + 1
}

func (w *RenderWorker) UseSubtitles(source SubtitleSource) {
	w.subtitles = subtitleReader{source: source, waitMax: subtitleWaitMax, pollInterval: subtitlePollInterval}
}

func subtitleRequest(j job.Job, edit renderplan.EditRequest) (subtitles.Request, bool) {
	req := subtitles.Request{JobID: j.ID, Target: j.TargetSteamID, Hidden: edit.SubtitlesHidden}
	if edit.FullDemo == nil {
		return req, edit.Subtitles
	}
	options := edit.FullDemo.Document.Options.Subtitles
	if options == nil || !options.Enabled {
		return req, false
	}
	req.Hidden = options.Hidden
	return req, true
}

func (w *RenderWorker) readSubtitles(ctx context.Context, j job.Job, edit renderplan.EditRequest) *subtitles.Track {
	req, on := subtitleRequest(j, edit)
	if !on || w.subtitles.source == nil {
		return nil
	}
	lookup, err := w.subtitles.source.Subtitles(ctx, req)
	if err != nil {
		logWorkerError(j.ID, tasks.TypeRenderVariant, fmt.Errorf("subtitles left out: %w", err))
		return nil
	}
	track, _ := lookup.Track()
	return track
}

func (w *RenderWorker) awaitSubtitles(ctx context.Context, j job.Job, edit renderplan.EditRequest) (*subtitles.Track, string) {
	req, on := subtitleRequest(j, edit)
	if !on || w.subtitles.source == nil {
		return nil, ""
	}
	for poll := range w.subtitles.pollCountMax() {
		if poll > 0 {
			select {
			case <-ctx.Done():
				return nil, ""
			case <-time.After(w.subtitles.pollInterval):
			}
		}
		lookup, err := w.subtitles.source.Subtitles(ctx, req)
		if err != nil {
			logWorkerError(j.ID, tasks.TypeRenderVariant, fmt.Errorf("subtitles left out: %w", err))
			return nil, subtitlesMissing(err)
		}
		if track, ready := lookup.Track(); ready {
			return track, ""
		}
		w.reportSubtitleWait(j.ID, lookup.Progress())
	}
	logWorkerError(j.ID, tasks.TypeRenderVariant, fmt.Errorf("subtitles left out: the transcription did not finish in %s", w.subtitles.waitMax))
	return nil, fmt.Sprintf("Salió sin subtítulos: la transcripción de las voces no terminó en %d min. Cuando termine, vuelve a crear el vídeo.", int(w.subtitles.waitMax.Minutes()))
}

func subtitlesMissing(err error) string {
	if errors.Is(err, subtitles.ErrTranscriptFailed) {
		return subtitlesFailedWarning
	}
	return subtitlesMissingWarning
}

func (w *RenderWorker) reportSubtitleWait(id uuid.UUID, progress float64) {
	stage := fmt.Sprintf("Esperando la transcripción de las voces (%d %%)", int(math.Round(progress*100)))
	body, err := json.Marshal(editor.EditorProgress{Schema: editor.EditorProgressSchema, Stage: stage, Percent: subtitleWaitPercent})
	if err == nil {
		err = w.storage.Put(artifacts.RenderProgressKey(id), bytes.NewReader(body))
	}
	if err != nil {
		logWorkerError(id, tasks.TypeRenderVariant, fmt.Errorf("report the subtitle wait: %w", err))
	}
}

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
