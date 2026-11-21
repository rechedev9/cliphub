package httpapi

import (
	"context"
	"errors"
	"log"
	"math"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/teamvoice"
	"github.com/rechedev9/cliphub/internal/vodplays"
)

const (
	streamVoiceAvailable  = "available"
	streamVoiceMixedTrack = "mixed_track"
	streamVoiceNoAudio    = "no_audio"
	codeMixedTrack        = "mixed_track"
	codeStreamNotReady    = "stream_not_ready"
	streamSpanPadMS       = 1000
	streamSpanGridMS      = 10_000
	maxStreamSpans        = 500
	maxStreamVideoRunes   = 200
)

var (
	errMixedTrack     = errors.New("the stream has only a mixed audio track")
	errNoStreamAudio  = errors.New("the stream has no audio")
	errStreamNotReady = errors.New("the stream source is not on this PC yet")
)

type streamSpan struct {
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds"`
}

type streamTranscript struct {
	streamclips.VoiceTranscript
	Progress     float64                     `json:"progress,omitempty"`
	Availability string                      `json:"availability"`
	Lines        []streamclips.VoiceLine     `json:"lines"`
	Transcriber  teamvoice.TranscriberStatus `json:"transcriber"`
	Subtitles    streamclips.Subtitles       `json:"subtitles"`
}

func streamVoiceTrack(probe streamclips.SourceProbe) (int, string) {
	if probe.AudioCodec == "" && probe.AudioStreams == 0 {
		return 0, streamVoiceNoAudio
	}
	track, mic := vodplays.VoiceTrack(probe.AudioTitles, probe.AudioStreams)
	if !mic {
		return 0, streamVoiceMixedTrack
	}
	return track, streamVoiceAvailable
}

func streamVoiceSpans(spans []streamSpan, durationSeconds float64) ([]library.VoiceSpan, bool) {
	if len(spans) == 0 || len(spans) > maxStreamSpans {
		return nil, false
	}
	out := make([]library.VoiceSpan, 0, len(spans))
	for _, s := range spans {
		if math.IsNaN(s.StartSeconds) || math.IsInf(s.EndSeconds, 0) || s.StartSeconds < 0 || s.EndSeconds <= s.StartSeconds {
			return nil, false
		}
		start := max(0, int64(s.StartSeconds*1000)-streamSpanPadMS) / streamSpanGridMS * streamSpanGridMS
		end := (int64(s.EndSeconds*1000) + streamSpanPadMS + streamSpanGridMS - 1) / streamSpanGridMS * streamSpanGridMS
		if durationSeconds > 0 {
			end = min(end, int64(math.Ceil(durationSeconds*1000)))
		}
		out = append(out, library.VoiceSpan{StartMS: start, EndMS: end})
	}
	out = streamclips.MergeVoiceSpans(out)
	return out, len(out) > 0
}

func (h *Handlers) streamVoiceRepo(w http.ResponseWriter) (store.StreamVoiceRepository, bool) {
	repo, ok := h.streamRepo.(store.StreamVoiceRepository)
	if !ok {
		writeCodedError(w, http.StatusNotImplemented, codeVoiceUnavailable, "stream transcripts are not available in this Studio")
	}
	return repo, ok
}

func (h *Handlers) streamTranscriptOf(ctx context.Context, repo store.StreamVoiceRepository, j streamclips.Job) (streamTranscript, error) {
	t, err := repo.GetStreamVoice(ctx, j.ID)
	if err != nil {
		return streamTranscript{}, err
	}
	lines, err := repo.ListStreamVoiceLines(ctx, j.ID)
	if err != nil {
		return streamTranscript{}, err
	}
	subtitles, err := repo.GetStreamSubtitles(ctx, j.ID)
	if err != nil {
		return streamTranscript{}, err
	}
	t.Spans, t.Pending = append([]library.VoiceSpan{}, t.Spans...), append([]library.VoiceSpan{}, t.Pending...)
	_, availability := streamVoiceTrack(j.Probe)
	out := streamTranscript{VoiceTranscript: t, Availability: availability, Lines: lines, Transcriber: h.transcriberStatus(), Subtitles: subtitles}
	if busyTranscript(t.State) && h.voice != nil {
		out.Progress = h.voice.progressOf(j.ID)
	}
	return out, nil
}

func (h *Handlers) writeStreamTranscript(w http.ResponseWriter, r *http.Request, j streamclips.Job, status int) {
	repo, ok := h.streamVoiceRepo(w)
	if !ok {
		return
	}
	out, err := h.streamTranscriptOf(r.Context(), repo, j)
	if err != nil {
		internalError(w, "get stream transcript", err)
		return
	}
	writeJSON(w, status, out)
}

func (h *Handlers) GetStreamTranscript(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadStreamJob(w, r)
	if !ok {
		return
	}
	h.writeStreamTranscript(w, r, j, http.StatusOK)
}

type streamTranscribeRequest struct {
	Spans []streamSpan `json:"spans"`
	Force bool         `json:"force"`
}

func (h *Handlers) TranscribeStream(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadStreamJob(w, r)
	if !ok {
		return
	}
	repo, ok := h.streamVoiceRepo(w)
	if !ok {
		return
	}
	var req streamTranscribeRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "body must be a JSON object with spans and force")
		return
	}
	spans, ok := streamVoiceSpans(req.Spans, j.Probe.DurationSeconds)
	if !ok {
		writeError(w, http.StatusBadRequest, "spans takes 1 to 500 stretches, each with start_seconds before end_seconds")
		return
	}
	queued, err := h.queueStreamTranscript(r.Context(), streamTranscribe{repo: repo, job: j, spans: spans, force: req.Force})
	switch {
	case errors.Is(err, errNoTranscriber):
		writeCodedError(w, http.StatusConflict, codeNoTranscriber, "local transcription is not installed; install it first")
	case errors.Is(err, errMixedTrack):
		writeCodedError(w, http.StatusConflict, codeMixedTrack, "subtitles are not available yet for a stream with only a mixed audio track")
	case errors.Is(err, errNoStreamAudio):
		writeCodedError(w, http.StatusConflict, codeNoVoice, "this stream has no audio")
	case errors.Is(err, errStreamNotReady):
		writeCodedError(w, http.StatusConflict, codeStreamNotReady, "the video of this stream is not on this PC yet")
	case errors.Is(err, errQueueFull):
		writeCodedError(w, http.StatusServiceUnavailable, codeQueueFull, "too many transcriptions are waiting; try again later")
	case err != nil:
		internalError(w, "queue stream transcript", err)
	case queued:
		h.writeStreamTranscript(w, r, j, http.StatusAccepted)
	default:
		h.writeStreamTranscript(w, r, j, http.StatusOK)
	}
}

type streamTranscribe struct {
	repo  store.StreamVoiceRepository
	job   streamclips.Job
	spans []library.VoiceSpan
	force bool
}

func (h *Handlers) queueStreamTranscript(ctx context.Context, in streamTranscribe) (bool, error) {
	if h.transcriberStatus().State != teamvoice.TranscriberInstalled {
		return false, errNoTranscriber
	}
	if !streamSourceIsReady(in.job.Status) {
		return false, errStreamNotReady
	}
	switch _, availability := streamVoiceTrack(in.job.Probe); availability {
	case streamVoiceNoAudio:
		return false, errNoStreamAudio
	case streamVoiceMixedTrack:
		return false, errMixedTrack
	}
	h.voice.streamMu.Lock()
	defer h.voice.streamMu.Unlock()
	cur, err := in.repo.GetStreamVoice(ctx, in.job.ID)
	if err != nil {
		return false, err
	}
	todo := in.spans
	if !in.force {
		todo = streamclips.MissingVoiceSpans(in.spans, slices.Concat(cur.Spans, cur.Pending))
	}
	if len(todo) == 0 {
		return false, nil
	}
	next, busy := cur, busyTranscript(cur.State)
	next.Pending, next.Error, next.Model = streamclips.MergeVoiceSpans(slices.Concat(cur.Pending, todo)), "", teamvoice.Pinned.ModelName
	if !busy {
		next.State = library.TranscriptQueued
	}
	if err := in.repo.SetStreamVoice(ctx, streamclips.VoiceUpdate{JobID: in.job.ID, Transcript: next}); err != nil {
		return false, err
	}
	if !busy && !h.enqueueTranscript(voiceJob{matchID: in.job.ID, stream: true}) {
		if err := in.repo.SetStreamVoice(ctx, streamclips.VoiceUpdate{JobID: in.job.ID, Transcript: cur}); err != nil {
			log.Printf("httpapi: restore the transcript state of stream %s: %v", in.job.ID, err)
		}
		return false, errQueueFull
	}
	return true, nil
}

func (h *Handlers) PatchStreamTranscriptLine(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadStreamJob(w, r)
	if !ok {
		return
	}
	repo, ok := h.streamVoiceRepo(w)
	if !ok {
		return
	}
	lineID, ok := libraryUUIDParam(w, r, "line")
	if !ok {
		return
	}
	var req demoLinePatch
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil || (req.Text == nil && req.Discarded == nil) {
		writeError(w, http.StatusBadRequest, "send text, discarded or both")
		return
	}
	patch := streamclips.VoiceLinePatch{JobID: j.ID, ID: lineID, Discarded: req.Discarded}
	if req.Text != nil {
		text := strings.Join(strings.Fields(*req.Text), " ")
		if text == "" || len([]rune(text)) > library.MaxVoiceLineRunes {
			writeError(w, http.StatusBadRequest, "a phrase takes 1 to 2000 characters")
			return
		}
		patch.Text = &text
	}
	line, err := repo.UpdateStreamVoiceLine(r.Context(), patch)
	if errors.Is(err, streamclips.ErrVoiceLineNotFound) {
		writeError(w, http.StatusNotFound, "that phrase is not in this stream's transcript")
		return
	}
	if err != nil {
		internalError(w, "correct stream transcript line", err)
		return
	}
	writeJSON(w, http.StatusOK, line)
}

type streamSubtitlesPatch struct {
	Shorts  *bool      `json:"shorts"`
	Summary *bool      `json:"summary"`
	Video   string     `json:"video"`
	LineID  *uuid.UUID `json:"line_id"`
	Hidden  *bool      `json:"hidden"`
}

func (p streamSubtitlesPatch) valid() bool {
	if p.Video == "" {
		return p.LineID == nil && p.Hidden == nil && (p.Shorts != nil || p.Summary != nil)
	}
	return p.LineID != nil && p.Hidden != nil && len([]rune(p.Video)) <= maxStreamVideoRunes
}

func (p streamSubtitlesPatch) apply(s streamclips.Subtitles) streamclips.Subtitles {
	if p.Shorts != nil {
		s.Shorts = *p.Shorts
	}
	if p.Summary != nil {
		s.Summary = *p.Summary
	}
	if p.Video == "" {
		return s
	}
	ids := slices.DeleteFunc(slices.Clone(s.Hidden[p.Video]), func(id uuid.UUID) bool { return id == *p.LineID })
	if *p.Hidden {
		ids = append(ids, *p.LineID)
	}
	s.Hidden[p.Video] = ids
	return s
}

func (h *Handlers) PatchStreamSubtitles(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadStreamJob(w, r)
	if !ok {
		return
	}
	repo, ok := h.streamVoiceRepo(w)
	if !ok {
		return
	}
	var req streamSubtitlesPatch
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil || !req.valid() {
		writeError(w, http.StatusBadRequest, "send shorts or summary to turn subtitles on or off, or video, line_id and hidden to hide a phrase in one video")
		return
	}
	h.streamPlanMu.Lock()
	defer h.streamPlanMu.Unlock()
	cur, err := repo.GetStreamSubtitles(r.Context(), j.ID)
	if err != nil {
		internalError(w, "get stream subtitles", err)
		return
	}
	next := req.apply(cur)
	if err := repo.SetStreamSubtitles(r.Context(), next); err != nil {
		internalError(w, "save stream subtitles", err)
		return
	}
	saved, err := repo.GetStreamSubtitles(r.Context(), j.ID)
	if err != nil {
		internalError(w, "get stream subtitles", err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}
