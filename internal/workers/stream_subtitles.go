package workers

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/tasks"
)

const noticeSubtitlesNotDrawn = "Los vídeos han salido sin subtítulos: no se pudieron preparar en este PC."

type StreamSubtitleSource interface {
	StreamSubtitles(ctx context.Context, req streamclips.SubtitleRequest) streamclips.SubtitleTrack
}

func (w *StreamRenderWorker) UseSubtitles(source StreamSubtitleSource) {
	w.subtitles = source
}

type streamSubtitleJob struct {
	jobID   uuid.UUID
	plan    streamclips.EditPlan
	workDir string
}

type streamRenderSubtitles struct {
	filters     map[string]string
	fingerprint string
	warnings    []string
}

func (w *StreamRenderWorker) streamSubtitles(ctx context.Context, in streamSubtitleJob) streamRenderSubtitles {
	if w.subtitles == nil {
		return streamRenderSubtitles{}
	}
	videos := in.plan.SubtitleVideos()
	track := w.subtitles.StreamSubtitles(ctx, streamclips.SubtitleRequest{JobID: in.jobID, Stretches: streamclips.SubtitleStretches(videos)})
	if track.Notice != "" {
		return streamRenderSubtitles{warnings: []string{track.Notice}}
	}
	scripts, err := streamclips.NewSubtitleScripts(videos, track)
	var filters map[string]string
	if err == nil {
		filters, err = scripts.WriteFilters(filepath.Join(in.workDir, "subtitles"))
	}
	if err != nil {
		logWorkerError(in.jobID, tasks.TypeRenderStreamClip, fmt.Errorf("subtitles left out: %w", err))
		return streamRenderSubtitles{warnings: []string{noticeSubtitlesNotDrawn}}
	}
	return streamRenderSubtitles{filters: filters, fingerprint: scripts.Fingerprint}
}
