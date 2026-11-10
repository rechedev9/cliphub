package httpapi

import (
	"cmp"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/teamvoice"
)

// StartVoice runs the transcription queue until ctx ends, and queues again the matches
// a previous run left queued or transcribing.
func (h *Handlers) StartVoice(ctx context.Context) {
	if h.voice == nil || h.libraryRepo == nil {
		return
	}
	h.voice.mu.Lock()
	h.voice.installCtx = ctx
	h.voice.mu.Unlock()
	h.startVoicePlugin(ctx)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case job := <-h.voice.queue:
				h.transcribeMatch(ctx, job)
			}
		}
	}()
	pending, err := h.libraryRepo.ListVoicesByTranscript(ctx, []string{library.TranscriptQueued, library.TranscriptRunning})
	if err != nil {
		log.Printf("httpapi: list pending transcripts: %v", err)
		return
	}
	for _, id := range pending {
		v, err := h.libraryRepo.GetMatchVoice(ctx, id)
		if err != nil {
			continue
		}
		h.enqueueTranscript(voiceJob{matchID: id, language: v.Transcript.Language})
	}
}

// enqueueTranscript queues a match without blocking; a full queue leaves it queued for the next start.
func (h *Handlers) enqueueTranscript(job voiceJob) bool {
	select {
	case h.voice.queue <- job:
		return true
	default:
		return false
	}
}

type voiceTranscribeRequest struct {
	Language string `json:"language"`
}

// TranscribeLibraryMatchVoice handles POST /api/library/matches/{id}/voice/transcribe.
func (h *Handlers) TranscribeLibraryMatchVoice(w http.ResponseWriter, r *http.Request) {
	m, ok := h.voiceMatch(w, r)
	if !ok {
		return
	}
	var req voiceTranscribeRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "body must be a JSON object with language")
		return
	}
	req.Language = cmp.Or(req.Language, "es")
	if !slices.Contains(teamvoice.Languages, req.Language) {
		writeError(w, http.StatusBadRequest, "language must be es, en, pt, fr, de or auto")
		return
	}
	if t := h.voice.deps.Transcriber; t == nil || t.Status().State != teamvoice.TranscriberInstalled {
		writeCodedError(w, http.StatusConflict, codeNoTranscriber, "local transcription is not installed; install it first")
		return
	}
	v, err := h.libraryRepo.GetMatchVoice(r.Context(), m.ID)
	if errors.Is(err, library.ErrNotFound) {
		writeCodedError(w, http.StatusNotFound, codeNoVoice, "this match has no team voice; import a recording first")
		return
	}
	if err != nil {
		internalError(w, "get match voice", err)
		return
	}
	if v.Transcript.State == library.TranscriptQueued || v.Transcript.State == library.TranscriptRunning {
		h.writeVoice(w, r, voiceReply{matchID: m.ID, status: http.StatusAccepted})
		return
	}
	if err := h.libraryRepo.SetVoiceTranscript(r.Context(), library.VoiceTranscriptUpdate{MatchID: m.ID,
		Transcript: library.VoiceTranscript{State: library.TranscriptQueued, Language: req.Language, Model: v.Transcript.Model}}); err != nil {
		internalError(w, "queue transcript", err)
		return
	}
	if !h.enqueueTranscript(voiceJob{matchID: m.ID, language: req.Language}) {
		if err := h.libraryRepo.SetVoiceTranscript(r.Context(), library.VoiceTranscriptUpdate{MatchID: m.ID, Transcript: v.Transcript}); err != nil {
			log.Printf("httpapi: restore transcript state of match %s: %v", m.ID, err)
		}
		writeCodedError(w, http.StatusServiceUnavailable, codeQueueFull, "too many transcriptions are waiting; try again later")
		return
	}
	h.writeVoice(w, r, voiceReply{matchID: m.ID, status: http.StatusAccepted})
}

// transcribeMatch transcribes every track of a match and replaces its lines; a failure keeps the old ones.
func (h *Handlers) transcribeMatch(ctx context.Context, job voiceJob) {
	v, err := h.libraryRepo.GetMatchVoice(ctx, job.matchID)
	if err != nil {
		return
	}
	model := teamvoice.Pinned.ModelName
	state := library.VoiceTranscript{State: library.TranscriptRunning, Language: job.language, Model: model}
	if err := h.libraryRepo.SetVoiceTranscript(ctx, library.VoiceTranscriptUpdate{MatchID: job.matchID, Transcript: state}); err != nil {
		log.Printf("httpapi: start transcript of match %s: %v", job.matchID, err)
		return
	}
	h.voice.setProgress(job.matchID, 0)
	defer h.voice.setProgress(job.matchID, -1)
	started := time.Now()
	lines := []library.VoiceLine{}
	for i, t := range v.Tracks {
		path, err := h.voice.trackPath(t)
		if err == nil {
			var got []teamvoice.Line
			share := 1 / float64(len(v.Tracks))
			got, err = h.voice.deps.Transcriber.Transcribe(ctx, teamvoice.TranscribeJob{Audio: path, Language: job.language, Speech: t.Speech,
				Progress: func(p float64) { h.voice.setProgress(job.matchID, (float64(i)+p)*share) }})
			for _, l := range got {
				lines = append(lines, library.VoiceLine{MatchID: job.matchID, Track: t.Track, StartMS: l.StartMS, EndMS: l.EndMS, Text: l.Text})
			}
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("httpapi: transcribe track %d of match %s: %v", t.Track, job.matchID, err)
			state.State, state.Error = library.TranscriptFailed, "La transcripción falló en este PC; studio.log tiene el detalle."
			if err := h.libraryRepo.SetVoiceTranscript(context.WithoutCancel(ctx), library.VoiceTranscriptUpdate{MatchID: job.matchID, Transcript: state}); err != nil {
				log.Printf("httpapi: mark transcript of match %s failed: %v", job.matchID, err)
			}
			return
		}
	}
	state.State = library.TranscriptReady
	if err := h.libraryRepo.SetVoiceTranscript(ctx, library.VoiceTranscriptUpdate{MatchID: job.matchID, Transcript: state, Lines: &lines}); err != nil {
		log.Printf("httpapi: store transcript of match %s: %v", job.matchID, err)
		return
	}
	log.Printf("httpapi: transcribed match %s: %d lines from %d tracks in %s", job.matchID, len(lines), len(v.Tracks), time.Since(started).Round(time.Second))
}

// GetVoiceTranscriber handles GET /api/library/voice/transcriber.
func (h *Handlers) GetVoiceTranscriber(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.transcriberStatus())
}

func (h *Handlers) transcriberStatus() teamvoice.TranscriberStatus {
	if h.voice == nil || h.voice.deps.Transcriber == nil {
		return teamvoice.TranscriberStatus{State: teamvoice.TranscriberUnsupported, Version: teamvoice.Pinned.Version, Model: teamvoice.Pinned.ModelName}
	}
	return h.voice.deps.Transcriber.Status()
}

// InstallVoiceTranscriber handles POST /api/library/voice/transcriber/install: fetches the pinned
// whisper.cpp build and model in the background, about 200 MB.
func (h *Handlers) InstallVoiceTranscriber(w http.ResponseWriter, _ *http.Request) {
	if h.voice == nil || h.voice.deps.Transcriber == nil {
		writeCodedError(w, http.StatusServiceUnavailable, codeVoiceUnavailable, "local transcription is not available in this Studio")
		return
	}
	if err := h.voice.deps.Transcriber.Install(h.voice.lifetime()); err != nil {
		writeCodedError(w, http.StatusConflict, "unsupported", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, h.transcriberStatus())
}
