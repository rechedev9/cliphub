package httpapi

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/teamvoice"
)

// demoTranscript is the transcript of the voices a demo recorded, as the producer and the MCP read it.
type demoTranscript struct {
	library.DemoVoiceTranscript
	Progress    float64                     `json:"progress,omitempty"`
	Voice       string                      `json:"voice"`
	Lines       []library.DemoVoiceLine     `json:"lines"`
	Transcriber teamvoice.TranscriberStatus `json:"transcriber"`
	Translator  teamvoice.TranscriberStatus `json:"translator"`
}

func busyTranscript(state string) bool {
	return state == library.TranscriptQueued || state == library.TranscriptRunning
}

// demoVoiceState is a match's stored transcript state; none stored reads as none.
func (h *Handlers) demoVoiceState(ctx context.Context, matchID uuid.UUID) (library.DemoVoiceTranscript, error) {
	t, err := h.libraryRepo.GetDemoVoice(ctx, matchID)
	if errors.Is(err, library.ErrNotFound) {
		return library.DemoVoiceTranscript{MatchID: matchID, State: library.TranscriptNone, English: library.TranscriptNone}, nil
	}
	return t, err
}

func (h *Handlers) demoTranscriptOf(ctx context.Context, m library.Match) (demoTranscript, error) {
	t, err := h.demoVoiceState(ctx, m.ID)
	if err != nil {
		return demoTranscript{}, err
	}
	lines, err := h.libraryRepo.ListDemoVoiceLines(ctx, library.DemoVoiceLineFilter{MatchIDs: []uuid.UUID{m.ID}})
	if err != nil {
		return demoTranscript{}, err
	}
	out := demoTranscript{DemoVoiceTranscript: t, Voice: h.demoVoiceStatusOf(ctx, m).State, Lines: lines,
		Transcriber: h.transcriberStatus(), Translator: h.translatorStatus()}
	if busyTranscript(t.State) || busyTranscript(t.English) {
		out.Progress = h.voice.progressOf(m.DemoID)
	}
	return out, nil
}

func (h *Handlers) writeDemoTranscript(w http.ResponseWriter, r *http.Request, m library.Match, status int) {
	out, err := h.demoTranscriptOf(r.Context(), m)
	if err != nil {
		internalError(w, "get demo transcript", err)
		return
	}
	writeJSON(w, status, out)
}

// GetLibraryMatchDemoTranscript handles GET /api/library/matches/{id}/voice/demo/transcript.
func (h *Handlers) GetLibraryMatchDemoTranscript(w http.ResponseWriter, r *http.Request) {
	m, ok := h.voiceMatch(w, r)
	if !ok {
		return
	}
	h.writeDemoTranscript(w, r, m, http.StatusOK)
}

type demoTranscribeRequest struct {
	Force   bool `json:"force"`
	English bool `json:"english"`
}

// TranscribeLibraryMatchDemoVoice handles POST /api/library/matches/{id}/voice/demo/transcript: queues
// the transcription of the demo's voices, and their English with english. A done one is reused unless force.
func (h *Handlers) TranscribeLibraryMatchDemoVoice(w http.ResponseWriter, r *http.Request) {
	m, ok := h.voiceMatch(w, r)
	if !ok {
		return
	}
	var req demoTranscribeRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "body must be a JSON object with force and english")
		return
	}
	queued, err := h.queueDemoTranscript(r.Context(), m, req)
	switch {
	case errors.Is(err, errNoTranscriber):
		writeCodedError(w, http.StatusConflict, codeNoTranscriber, "local transcription is not installed; install it first")
	case errors.Is(err, errNoTranslator):
		writeCodedError(w, http.StatusConflict, codeNoTranslator, "the English translation is not installed; install it first")
	case errors.Is(err, errNoDemoVoice):
		writeCodedError(w, http.StatusNotFound, codeNoVoice, "this demo recorded no team voice")
	case errors.Is(err, errNoDemoFile):
		writeCodedError(w, http.StatusConflict, codeNoDemoFile, "the demo file of this match is not on this PC")
	case errors.Is(err, errQueueFull):
		writeCodedError(w, http.StatusServiceUnavailable, codeQueueFull, "too many transcriptions are waiting; try again later")
	case err != nil:
		internalError(w, "queue demo transcript", err)
	case queued:
		h.writeDemoTranscript(w, r, m, http.StatusAccepted)
	default:
		h.writeDemoTranscript(w, r, m, http.StatusOK)
	}
}

// Why a demo transcript cannot be queued.
var (
	errNoTranscriber = errors.New("local transcription is not installed")
	errNoTranslator  = errors.New("the English translation is not installed")
	errNoDemoVoice   = errors.New("the demo recorded no team voice")
	errNoDemoFile    = errors.New("the demo file is not on this PC")
	errQueueFull     = errors.New("the transcription queue is full")
)

// queueDemoTranscript queues what req asks that is not done yet, extracting the demo's voices
// first when needed; queued also covers one already waiting or running.
func (h *Handlers) queueDemoTranscript(ctx context.Context, m library.Match, req demoTranscribeRequest) (bool, error) {
	if h.transcriberStatus().State != teamvoice.TranscriberInstalled {
		return false, errNoTranscriber
	}
	if req.English && h.translatorStatus().State != teamvoice.TranscriberInstalled {
		return false, errNoTranslator
	}
	switch voice := h.demoVoiceStatusOf(ctx, m); {
	case voice.State == demoVoiceReady && len(voice.Tracks) == 0:
		return false, errNoDemoVoice
	case voice.State != demoVoiceReady && voice.State != demoVoiceExtracting && !h.startDemoVoice(ctx, m):
		return false, errNoDemoFile
	}
	cur, err := h.demoVoiceState(ctx, m.ID)
	if err != nil || busyTranscript(cur.State) || busyTranscript(cur.English) {
		return err == nil, err
	}
	next := cur
	if req.Force || cur.State != library.TranscriptReady {
		next.State, next.Error, next.Model = library.TranscriptQueued, "", teamvoice.Pinned.ModelName
		next.English, next.EnglishError = library.TranscriptNone, ""
	}
	if req.English && (next.State == library.TranscriptQueued || cur.English != library.TranscriptReady) {
		next.English, next.EnglishError = library.TranscriptQueued, ""
	}
	if next == cur {
		return false, nil
	}
	if err := h.libraryRepo.SetDemoVoice(ctx, library.DemoVoiceUpdate{MatchID: m.ID, Transcript: next}); err != nil {
		return false, err
	}
	if !h.enqueueTranscript(voiceJob{matchID: m.ID, demo: true}) {
		if err := h.libraryRepo.SetDemoVoice(ctx, library.DemoVoiceUpdate{MatchID: m.ID, Transcript: cur}); err != nil {
			log.Printf("httpapi: restore demo transcript state of match %s: %v", m.ID, err)
		}
		return false, errQueueFull
	}
	return true, nil
}

type demoLinePatch struct {
	Text    *string `json:"text"`
	English *string `json:"english"`
}

// PatchLibraryMatchDemoTranscriptLine handles PATCH /api/library/matches/{id}/voice/demo/transcript/lines/{line}:
// corrects a phrase for every later video of the demo.
func (h *Handlers) PatchLibraryMatchDemoTranscriptLine(w http.ResponseWriter, r *http.Request) {
	m, ok := h.voiceMatch(w, r)
	if !ok {
		return
	}
	lineID, ok := libraryUUIDParam(w, r, "line")
	if !ok {
		return
	}
	var req demoLinePatch
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil || (req.Text == nil && req.English == nil) {
		writeError(w, http.StatusBadRequest, "send text or english")
		return
	}
	patch := library.DemoVoiceLinePatch{MatchID: m.ID, ID: lineID}
	for _, field := range []struct {
		in  *string
		out **string
	}{{req.Text, &patch.Text}, {req.English, &patch.English}} {
		if field.in == nil {
			continue
		}
		text := strings.Join(strings.Fields(*field.in), " ")
		if text == "" || len([]rune(text)) > library.MaxVoiceLineRunes {
			writeError(w, http.StatusBadRequest, "a phrase takes 1 to 2000 characters")
			return
		}
		*field.out = &text
	}
	line, err := h.libraryRepo.UpdateDemoVoiceLine(r.Context(), patch)
	if err != nil {
		writeLibraryError(w, "correct demo transcript line", err)
		return
	}
	writeJSON(w, http.StatusOK, line)
}

// requeueDemoTranscripts queues again the demos a previous run left queued or transcribing.
func (h *Handlers) requeueDemoTranscripts(ctx context.Context) {
	pending, err := h.libraryRepo.ListDemoVoices(ctx, []string{library.TranscriptQueued, library.TranscriptRunning})
	if err != nil {
		log.Printf("httpapi: list pending demo transcripts: %v", err)
		return
	}
	for _, t := range pending {
		h.enqueueTranscript(voiceJob{matchID: t.MatchID, demo: true})
	}
}

// transcribeDemo transcribes, then translates, what the match's transcript state asks for.
func (h *Handlers) transcribeDemo(ctx context.Context, matchID uuid.UUID) {
	m, err := h.libraryRepo.GetMatch(ctx, matchID)
	if err != nil {
		return
	}
	state, err := h.demoVoiceState(ctx, matchID)
	if err != nil {
		log.Printf("httpapi: read demo transcript of match %s: %v", matchID, err)
		return
	}
	h.voice.setProgress(m.DemoID, 0)
	defer h.voice.setProgress(m.DemoID, -1)
	idx, err := h.awaitDemoVoice(ctx, m)
	if err == nil && busyTranscript(state.State) {
		state, err = h.transcribeDemoTracks(ctx, demoTracks{match: m, index: idx, state: state})
	}
	if err == nil && busyTranscript(state.English) {
		state, err = h.translateDemoTracks(ctx, demoTracks{match: m, index: idx, state: state})
	}
	if err == nil || ctx.Err() != nil {
		return
	}
	log.Printf("httpapi: transcribe the demo voices of match %s: %v", matchID, err)
	if busyTranscript(state.State) {
		state.State, state.Error = library.TranscriptFailed, "La transcripción falló en este PC; studio.log tiene el detalle."
	}
	if busyTranscript(state.English) {
		state.English, state.EnglishError = library.TranscriptFailed, "La traducción falló en este PC; studio.log tiene el detalle."
	}
	if err := h.libraryRepo.SetDemoVoice(context.WithoutCancel(ctx), library.DemoVoiceUpdate{MatchID: matchID, Transcript: state}); err != nil {
		log.Printf("httpapi: mark demo transcript of match %s failed: %v", matchID, err)
	}
}

// awaitDemoVoice waits for a running extraction of the demo's voices and reads its index.
func (h *Handlers) awaitDemoVoice(ctx context.Context, m library.Match) (demoVoiceIndex, error) {
	for {
		h.voice.mu.Lock()
		job := h.voice.demoJobs[m.DemoID]
		h.voice.mu.Unlock()
		if job != demoVoiceExtracting {
			break
		}
		select {
		case <-ctx.Done():
			return demoVoiceIndex{}, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	idx, ok := h.readDemoVoice(m.DemoID)
	if !ok {
		return demoVoiceIndex{}, errors.New("the demo voices were not extracted")
	}
	return idx, nil
}

type demoTracks struct {
	match library.Match
	index demoVoiceIndex
	state library.DemoVoiceTranscript
}

func (h *Handlers) demoTrackPath(demoID uuid.UUID, t demoVoiceTrack) (string, error) {
	rel := filepath.FromSlash(t.File)
	if !filepath.IsLocal(rel) {
		return "", errors.New("a demo voice track left its folder")
	}
	return filepath.Join(h.voice.demoVoiceDir(demoID), rel), nil
}

// transcribeDemoTracks transcribes each player's track and replaces the demo's phrases, placed on its rounds.
func (h *Handlers) transcribeDemoTracks(ctx context.Context, in demoTracks) (library.DemoVoiceTranscript, error) {
	state, m := in.state, in.match
	state.State = library.TranscriptRunning
	if err := h.libraryRepo.SetDemoVoice(ctx, library.DemoVoiceUpdate{MatchID: m.ID, Transcript: state}); err != nil {
		return state, err
	}
	started := time.Now()
	timeline := teamvoice.Timeline{}
	if doc, err := h.readLibraryDocument(m.DemoID); err == nil {
		timeline = teamvoice.TimelineOf(doc)
	}
	lines := []library.DemoVoiceLine{}
	share := 1 / float64(max(1, len(in.index.Tracks)))
	for i, t := range in.index.Tracks {
		path, err := h.demoTrackPath(m.DemoID, t)
		if err != nil {
			return state, err
		}
		got, err := h.voice.deps.Transcriber.Transcribe(ctx, teamvoice.TranscribeJob{Audio: path,
			Progress: func(p float64) { h.voice.setProgress(m.DemoID, (float64(i)+p)*share) }})
		if err != nil {
			return state, err
		}
		for _, l := range got {
			at := teamvoice.Locate(teamvoice.LocateInput{Timeline: timeline, AudioMS: l.StartMS, SteamID64: t.SteamID64})
			lines = append(lines, library.DemoVoiceLine{ID: uuid.New(), SteamID64: t.SteamID64, Name: t.Name, TeamKey: t.TeamKey,
				StartMS: l.StartMS, EndMS: l.EndMS, Tick: at.Tick, Round: at.Round, Text: l.Text})
		}
	}
	state.State, state.Error = library.TranscriptReady, ""
	if err := h.libraryRepo.SetDemoVoice(ctx, library.DemoVoiceUpdate{MatchID: m.ID, Transcript: state, Lines: &lines}); err != nil {
		return state, err
	}
	log.Printf("httpapi: transcribed the demo voices of match %s: %d phrases from %d players in %s", m.ID, len(lines), len(in.index.Tracks), time.Since(started).Round(time.Second))
	return state, nil
}

// translateDemoTracks translates each player's phrases by their own stretch of audio.
func (h *Handlers) translateDemoTracks(ctx context.Context, in demoTracks) (library.DemoVoiceTranscript, error) {
	state, m := in.state, in.match
	if h.voice.deps.Translator == nil {
		return state, errors.New("no translator")
	}
	state.English = library.TranscriptRunning
	if err := h.libraryRepo.SetDemoVoice(ctx, library.DemoVoiceUpdate{MatchID: m.ID, Transcript: state}); err != nil {
		return state, err
	}
	lines, err := h.libraryRepo.ListDemoVoiceLines(ctx, library.DemoVoiceLineFilter{MatchIDs: []uuid.UUID{m.ID}})
	if err != nil {
		return state, err
	}
	english := map[uuid.UUID]string{}
	share := 1 / float64(max(1, len(in.index.Tracks)))
	for i, t := range in.index.Tracks {
		var ids []uuid.UUID
		var spans []library.VoiceSpan
		for _, l := range lines {
			if l.SteamID64 == t.SteamID64 {
				ids, spans = append(ids, l.ID), append(spans, library.VoiceSpan{StartMS: l.StartMS, EndMS: l.EndMS})
			}
		}
		if len(spans) == 0 {
			continue
		}
		path, err := h.demoTrackPath(m.DemoID, t)
		if err != nil {
			return state, err
		}
		got, err := h.voice.deps.Translator.Translate(ctx, teamvoice.TranslateJob{Audio: path, Spans: spans,
			Progress: func(p float64) { h.voice.setProgress(m.DemoID, (float64(i)+p)*share) }})
		if err != nil {
			return state, err
		}
		for j, text := range got {
			if j < len(ids) {
				english[ids[j]] = text
			}
		}
	}
	state.English, state.EnglishError = library.TranscriptReady, ""
	return state, h.libraryRepo.SetDemoVoice(ctx, library.DemoVoiceUpdate{MatchID: m.ID, Transcript: state, English: english})
}

// GetVoiceTranslator handles GET /api/library/voice/translator.
func (h *Handlers) GetVoiceTranslator(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.translatorStatus())
}

func (h *Handlers) translatorStatus() teamvoice.TranscriberStatus {
	if h.voice == nil || h.voice.deps.Translator == nil {
		return teamvoice.TranscriberStatus{State: teamvoice.TranscriberUnsupported, Version: teamvoice.PinnedTranslator.Version, Model: teamvoice.PinnedTranslator.ModelName}
	}
	return h.voice.deps.Translator.Status()
}

// InstallVoiceTranslator handles POST /api/library/voice/translator/install: fetches the pinned
// whisper.cpp build and medium model in the background, about 550 MB.
func (h *Handlers) InstallVoiceTranslator(w http.ResponseWriter, _ *http.Request) {
	if h.voice == nil || h.voice.deps.Translator == nil {
		writeCodedError(w, http.StatusServiceUnavailable, codeVoiceUnavailable, "the English translation is not available in this Studio")
		return
	}
	if err := h.voice.deps.Translator.Install(h.voice.lifetime()); err != nil {
		writeCodedError(w, http.StatusConflict, "unsupported", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, h.translatorStatus())
}
