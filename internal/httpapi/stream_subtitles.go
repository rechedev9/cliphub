package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/vodplays"
)

const (
	streamSubtitleWait = 5 * time.Minute
	streamSubtitlePoll = 100 * time.Millisecond

	noticeShortsWithoutSubtitles  = "Los vídeos han salido sin subtítulos: "
	noticeSummaryWithoutSubtitles = "El resumen ha salido sin subtítulos: "
	reasonNoTranscriber           = "la transcripción no está instalada en este PC."
	reasonMixedTrack              = "este stream solo tiene una pista con la voz y el juego mezclados."
	reasonNoStreamAudio           = "este stream no tiene audio."
	reasonTranscriptFailed        = "la transcripción falló en este PC."
	reasonShortsTranscriptLate    = "la transcripción no terminó a tiempo. Expórtalos de nuevo cuando termine."
	reasonSummaryTranscriptLate   = "la transcripción no terminó a tiempo. Créalo de nuevo cuando termine."
	reasonSubtitlesNotDrawn       = "no se pudieron preparar en este PC."
)

var errStreamTranscriptLate = errors.New("the stream transcript did not finish in time")

func (h *Handlers) StreamSubtitles(ctx context.Context, req streamclips.SubtitleRequest) streamclips.SubtitleTrack {
	repo, ok := h.streamRepo.(store.StreamVoiceRepository)
	if !ok {
		return streamclips.SubtitleTrack{}
	}
	choice, err := repo.GetStreamSubtitles(ctx, req.JobID)
	if err != nil {
		log.Printf("httpapi: read the subtitles of stream %s: %v", req.JobID, err)
		return streamclips.SubtitleTrack{}
	}
	if !subtitlesOn(choice, req.Summary) || len(req.Stretches) == 0 {
		return streamclips.SubtitleTrack{}
	}
	lines, err := h.awaitStreamFrases(ctx, repo, req)
	if err != nil {
		log.Printf("httpapi: stream %s renders without subtitles: %v", req.JobID, err)
		return streamclips.SubtitleTrack{Notice: streamSubtitleNotice(req.Summary, err)}
	}
	return streamclips.SubtitleTrack{Lines: lines, Hidden: choice.Hidden}
}

func streamSubtitleNotice(summary bool, err error) string {
	opening, late := noticeShortsWithoutSubtitles, reasonShortsTranscriptLate
	if summary {
		opening, late = noticeSummaryWithoutSubtitles, reasonSummaryTranscriptLate
	}
	switch {
	case errors.Is(err, errNoTranscriber):
		return opening + reasonNoTranscriber
	case errors.Is(err, errMixedTrack):
		return opening + reasonMixedTrack
	case errors.Is(err, errNoStreamAudio):
		return opening + reasonNoStreamAudio
	case errors.Is(err, errStreamTranscriptLate):
		return opening + late
	}
	return opening + reasonTranscriptFailed
}

func streamSpansOf(stretches []library.VoiceSpan, durationSeconds float64) ([]library.VoiceSpan, bool) {
	spans := make([]streamSpan, 0, len(stretches))
	for _, s := range stretches {
		spans = append(spans, streamSpan{StartSeconds: float64(s.StartMS) / 1000, EndSeconds: float64(s.EndMS) / 1000})
	}
	return streamVoiceSpans(spans, durationSeconds)
}

func (h *Handlers) awaitStreamFrases(ctx context.Context, repo store.StreamVoiceRepository, req streamclips.SubtitleRequest) ([]streamclips.VoiceLine, error) {
	j, err := h.streamRepo.Get(ctx, req.JobID)
	if err != nil {
		return nil, err
	}
	spans, ok := streamSpansOf(req.Stretches, j.Probe.DurationSeconds)
	if !ok {
		return nil, errors.New("the video has no stretch to transcribe")
	}
	if _, err := h.queueStreamTranscript(ctx, streamTranscribe{repo: repo, job: j, spans: spans}); err != nil {
		return nil, err
	}
	limit := h.voice.streamWait
	if limit <= 0 {
		limit = streamSubtitleWait
	}
	deadline := time.NewTimer(limit)
	defer deadline.Stop()
	tick := time.NewTicker(streamSubtitlePoll)
	defer tick.Stop()
	for {
		t, err := repo.GetStreamVoice(ctx, req.JobID)
		if err != nil {
			return nil, err
		}
		if len(streamclips.MissingVoiceSpans(spans, t.Spans)) == 0 {
			return repo.ListStreamVoiceLines(ctx, req.JobID)
		}
		if !busyTranscript(t.State) {
			return nil, errors.New("the stream transcript failed")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline.C:
			return nil, errStreamTranscriptLate
		case <-tick.C:
		}
	}
}

func subtitlesOn(choice streamclips.Subtitles, summary bool) bool {
	if summary {
		return choice.Summary
	}
	return choice.Shorts
}

type streamSubtitlesStale struct {
	Shorts  bool `json:"shorts"`
	Summary bool `json:"summary"`
}

type streamSubtitleView struct {
	job     streamclips.Job
	summary bool
	videos  []streamclips.SubtitleVideo
}

func (h *Handlers) streamSubtitleFingerprint(ctx context.Context, view streamSubtitleView) string {
	repo, ok := h.streamRepo.(store.StreamVoiceRepository)
	if !ok {
		return ""
	}
	choice, err := repo.GetStreamSubtitles(ctx, view.job.ID)
	if err != nil || !subtitlesOn(choice, view.summary) {
		return ""
	}
	spans, ok := streamSpansOf(streamclips.SubtitleStretches(view.videos), view.job.Probe.DurationSeconds)
	if !ok {
		return ""
	}
	t, err := repo.GetStreamVoice(ctx, view.job.ID)
	if err != nil || len(streamclips.MissingVoiceSpans(spans, t.Spans)) > 0 {
		return ""
	}
	lines, err := repo.ListStreamVoiceLines(ctx, view.job.ID)
	if err != nil {
		return ""
	}
	scripts, err := streamclips.NewSubtitleScripts(view.videos, streamclips.SubtitleTrack{Lines: lines, Hidden: choice.Hidden})
	if err != nil {
		return ""
	}
	return scripts.Fingerprint
}

func (h *Handlers) publishedStreamSubtitlesChanged(ctx context.Context, j streamclips.Job, result streamclips.RenderResult) bool {
	plan, saved, err := h.savedStreamEditPlan(j)
	if err != nil || !saved {
		return false
	}
	plan = streamclips.NormalizeEditPlan(plan)
	if plan.Variant != result.Variant {
		return false
	}
	return result.SubtitlesFingerprint != h.streamSubtitleFingerprint(ctx, streamSubtitleView{job: j, videos: plan.SubtitleVideos()})
}

func (h *Handlers) streamSubtitlesStaleOf(ctx context.Context, j streamclips.Job) streamSubtitlesStale {
	var stale streamSubtitlesStale
	if plan, saved, err := h.savedStreamEditPlan(j); err == nil && saved {
		state, exists, err := h.readStreamRenderState(j.ID, streamclips.NormalizeEditPlan(plan).Variant)
		if err == nil && exists && state.HasPublishedRender() {
			if result, ok, err := h.readPublishedStreamResult(state); err == nil && ok {
				stale.Shorts = h.publishedStreamSubtitlesChanged(ctx, j, result)
			}
		}
	}
	rc, err := h.storage.Open(streamclips.SummaryStateKey(j.ID))
	if err != nil {
		return stale
	}
	defer rc.Close()
	var summary streamSummary
	if err := json.NewDecoder(rc).Decode(&summary); err != nil || summary.Status != playScanDone {
		return stale
	}
	scan, err := h.currentStreamPlayScan(j.ID)
	if err != nil {
		return stale
	}
	video := summarySubtitleVideo(chosenPlays(scan.Plays, summary.PlayIDs))
	stale.Summary = summary.SubtitlesFingerprint != h.streamSubtitleFingerprint(ctx, streamSubtitleView{job: j, summary: true, videos: []streamclips.SubtitleVideo{video}})
	return stale
}

func summarySubtitleVideo(plays []vodplays.Play) streamclips.SubtitleVideo {
	video := streamclips.SubtitleVideo{ID: streamclips.SummaryVideo, Width: vodplays.SummaryWidth, Height: vodplays.SummaryHeight}
	for _, p := range vodplays.SummarySegments(plays) {
		video.Clips = append(video.Clips, streamclips.ClipRange{ID: p.ID, StartSeconds: p.StartSeconds, EndSeconds: p.EndSeconds})
	}
	return video
}

type summarySubtitles struct {
	filters     map[string]string
	fingerprint string
	notice      string
}

type summarySubtitleJob struct {
	id      uuid.UUID
	plays   []vodplays.Play
	workDir string
}

func (h *Handlers) summarySubtitles(in summarySubtitleJob) summarySubtitles {
	id, workDir := in.id, in.workDir
	videos := []streamclips.SubtitleVideo{summarySubtitleVideo(in.plays)}
	track := h.StreamSubtitles(context.Background(), streamclips.SubtitleRequest{JobID: id, Summary: true, Stretches: streamclips.SubtitleStretches(videos)})
	if track.Notice != "" {
		return summarySubtitles{notice: track.Notice}
	}
	scripts, err := streamclips.NewSubtitleScripts(videos, track)
	var filters map[string]string
	if err == nil {
		filters, err = scripts.WriteFilters(filepath.Join(workDir, "subtitles"))
	}
	if err != nil {
		log.Printf("httpapi: stream %s summary renders without subtitles: %v", id, err)
		return summarySubtitles{notice: noticeSummaryWithoutSubtitles + reasonSubtitlesNotDrawn}
	}
	return summarySubtitles{filters: filters, fingerprint: scripts.Fingerprint}
}
