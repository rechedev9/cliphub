package httpapi

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/teamvoice"
)

// libraryVoiceRepository is the team voice part of LibraryRepository.
type libraryVoiceRepository interface {
	GetMatchVoice(ctx context.Context, matchID uuid.UUID) (library.MatchVoice, error)
	AddVoiceTracks(ctx context.Context, add library.VoiceAdd) (library.MatchVoice, error)
	SetVoiceSync(ctx context.Context, matchID uuid.UUID, sync library.VoiceSync) (library.MatchVoice, error)
	UpdateVoiceTrack(ctx context.Context, patch library.VoiceTrackPatch) (library.MatchVoice, error)
	SetVoiceTranscript(ctx context.Context, update library.VoiceTranscriptUpdate) error
	ListVoiceLines(ctx context.Context, filter library.VoiceLineFilter) ([]library.VoiceLine, error)
	ListVoicesByTranscript(ctx context.Context, states []string) ([]uuid.UUID, error)
	ListVoiceSummaries(ctx context.Context) ([]library.VoiceSummary, error)
	DeleteMatchVoice(ctx context.Context, matchID uuid.UUID) error
}

// VoiceMedia reads and encodes recordings; teamvoice.Media runs FFmpeg for it.
type VoiceMedia interface {
	Probe(ctx context.Context, path string) (teamvoice.Probe, error)
	Encode(ctx context.Context, job teamvoice.EncodeJob) error
	Speech(ctx context.Context, path string) ([]library.VoiceSpan, int64, error)
}

// VoiceTranscriber is local speech to text; teamvoice.Transcriber runs whisper.cpp for it.
type VoiceTranscriber interface {
	Status() teamvoice.TranscriberStatus
	Install(ctx context.Context) error
	Transcribe(ctx context.Context, job teamvoice.TranscribeJob) ([]teamvoice.Line, error)
}

// VoiceDeps enables team voice: Dir holds each match's tracks under the profile's data folder.
type VoiceDeps struct {
	Dir         string
	Media       VoiceMedia
	Transcriber VoiceTranscriber
	DemoVoice   DemoVoiceExtractor
}

// WithVoice enables the team voice routes of the library.
func WithVoice(deps VoiceDeps) Option {
	return func(h *Handlers) {
		h.voice = &voiceService{deps: deps, queue: make(chan voiceJob, 64), progress: map[uuid.UUID]float64{}, demoJobs: map[uuid.UUID]string{}}
	}
}

type voiceJob struct {
	matchID  uuid.UUID
	language string
}

type voiceService struct {
	deps     VoiceDeps
	queue    chan voiceJob
	mu       sync.Mutex
	progress map[uuid.UUID]float64
	// demoJobs is the extraction state of each demo's in-game voice while it runs or after it failed.
	demoJobs map[uuid.UUID]string
	// installCtx outlives the request that asked for an install or a demo voice extraction.
	installCtx context.Context
}

// lifetime is the context background voice work runs under: Studio's, once StartVoice ran.
func (v *voiceService) lifetime() context.Context {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.installCtx == nil {
		return context.Background()
	}
	return v.installCtx
}

func (v *voiceService) setProgress(id uuid.UUID, share float64) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if share < 0 {
		delete(v.progress, id)
		return
	}
	v.progress[id] = share
}

func (v *voiceService) progressOf(id uuid.UUID) float64 {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.progress[id]
}

// matchDir is where a match's tracks live; a key never leaves it.
func (v *voiceService) matchDir(matchID uuid.UUID) string {
	return filepath.Join(v.deps.Dir, matchID.String())
}

func (v *voiceService) trackPath(t library.VoiceTrack) (string, error) {
	name := filepath.FromSlash(t.StoredKey)
	if name == "" || filepath.IsAbs(name) || strings.Contains(t.StoredKey, "..") {
		return "", errors.New("voice track has no stored file")
	}
	return filepath.Join(v.deps.Dir, name), nil
}

// codes of the voice routes.
const (
	codeVoiceUnavailable = "voice_unavailable"
	codeNoVoice          = "no_voice"
	codeNotAudio         = "not_audio"
	codeTooManyTracks    = "too_many_tracks"
	codeNoTranscriber    = "transcriber_not_installed"
	codeQueueFull        = "queue_full"
	codeNoDemoFile       = "no_demo_file"
)

func (h *Handlers) voiceReady(w http.ResponseWriter) bool {
	if !h.libraryReady(w) {
		return false
	}
	if h.voice == nil || h.voice.deps.Dir == "" || h.voice.deps.Media == nil {
		writeCodedError(w, http.StatusServiceUnavailable, codeVoiceUnavailable, "team voice needs FFmpeg, which this Studio did not start with")
		return false
	}
	return true
}

// voiceMatch reads the {id} match of a voice route.
func (h *Handlers) voiceMatch(w http.ResponseWriter, r *http.Request) (library.Match, bool) {
	if !h.voiceReady(w) {
		return library.Match{}, false
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return library.Match{}, false
	}
	m, err := h.libraryRepo.GetMatch(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get match", err)
		return library.Match{}, false
	}
	return m, true
}

// voiceStatus is a match's voice as the Replay and the MCP read it.
type voiceStatus struct {
	MatchID     uuid.UUID                   `json:"match_id"`
	Present     bool                        `json:"present"`
	Tracks      []library.VoiceTrack        `json:"tracks"`
	DurationMS  int64                       `json:"duration_ms"`
	SpeechMS    int64                       `json:"speech_ms"`
	Sync        library.VoiceSync           `json:"sync"`
	Transcript  voiceTranscriptStatus       `json:"transcript"`
	Transcriber teamvoice.TranscriberStatus `json:"transcriber"`
}

type voiceTranscriptStatus struct {
	library.VoiceTranscript
	Progress float64 `json:"progress,omitempty"`
}

func (h *Handlers) voiceStatusOf(matchID uuid.UUID, v *library.MatchVoice) voiceStatus {
	out := voiceStatus{MatchID: matchID, Tracks: []library.VoiceTrack{},
		Sync:       library.VoiceSync{State: library.VoiceUnsynced},
		Transcript: voiceTranscriptStatus{VoiceTranscript: library.VoiceTranscript{State: library.TranscriptNone}}}
	if t := h.voice.deps.Transcriber; t != nil {
		out.Transcriber = t.Status()
	} else {
		out.Transcriber = teamvoice.TranscriberStatus{State: teamvoice.TranscriberUnsupported}
	}
	if v == nil {
		return out
	}
	out.Present, out.Tracks, out.DurationMS, out.Sync = true, v.Tracks, v.DurationMS(), v.Sync
	spans := make([][]library.VoiceSpan, 0, len(v.Tracks))
	for _, t := range v.Tracks {
		spans = append(spans, t.Speech)
	}
	out.SpeechMS = teamvoice.SpeechMS(teamvoice.UnionSpans(spans...))
	out.Transcript.VoiceTranscript = v.Transcript
	if v.Transcript.State == library.TranscriptRunning {
		out.Transcript.Progress = h.voice.progressOf(matchID)
	}
	return out
}

// voiceReply is the match whose voice answers a request, and the status to answer with.
type voiceReply struct {
	matchID uuid.UUID
	status  int
}

// writeVoice answers with a match's voice, or its absence.
func (h *Handlers) writeVoice(w http.ResponseWriter, r *http.Request, reply voiceReply) {
	matchID, status := reply.matchID, reply.status
	v, err := h.libraryRepo.GetMatchVoice(r.Context(), matchID)
	switch {
	case errors.Is(err, library.ErrNotFound):
		writeJSON(w, status, h.voiceStatusOf(matchID, nil))
	case err != nil:
		internalError(w, "get match voice", err)
	default:
		writeJSON(w, status, h.voiceStatusOf(matchID, &v))
	}
}

// GetLibraryMatchVoice handles GET /api/library/matches/{id}/voice.
func (h *Handlers) GetLibraryMatchVoice(w http.ResponseWriter, r *http.Request) {
	m, ok := h.voiceMatch(w, r)
	if !ok {
		return
	}
	h.writeVoice(w, r, voiceReply{matchID: m.ID, status: http.StatusOK})
}

// DeleteLibraryMatchVoice handles DELETE /api/library/matches/{id}/voice: the tracks, sync and transcript go.
func (h *Handlers) DeleteLibraryMatchVoice(w http.ResponseWriter, r *http.Request) {
	m, ok := h.voiceMatch(w, r)
	if !ok {
		return
	}
	if err := h.libraryRepo.DeleteMatchVoice(r.Context(), m.ID); err != nil {
		internalError(w, "delete match voice", err)
		return
	}
	h.discardMatchVoice(m.ID)
	w.WriteHeader(http.StatusNoContent)
}

// discardMatchVoice removes a match's voice files; the rows go with the match or by DeleteMatchVoice.
func (h *Handlers) discardMatchVoice(matchID uuid.UUID) {
	if h.voice == nil || h.voice.deps.Dir == "" {
		return
	}
	if err := os.RemoveAll(h.voice.matchDir(matchID)); err != nil {
		log.Printf("httpapi: remove voice files of match %s: %v", matchID, err)
	}
}

type voiceSyncRequest struct {
	OffsetMS *int64 `json:"offset_ms"`
	Auto     bool   `json:"auto"`
}

// PatchLibraryMatchVoice handles PATCH /api/library/matches/{id}/voice: a confirmed offset, or auto to estimate again.
func (h *Handlers) PatchLibraryMatchVoice(w http.ResponseWriter, r *http.Request) {
	m, ok := h.voiceMatch(w, r)
	if !ok {
		return
	}
	var req voiceSyncRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil || (req.OffsetMS == nil) == !req.Auto {
		writeError(w, http.StatusBadRequest, "send offset_ms, or auto true")
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
	sync := v.Sync
	if req.Auto {
		res, err := h.estimateVoiceSync(r.Context(), voiceSyncSubject{match: m, tracks: v.Tracks})
		if err != nil {
			internalError(w, "estimate voice sync", err)
			return
		}
		sync = res.Sync
	} else {
		if *req.OffsetMS < -library.MaxVoiceOffsetMS || *req.OffsetMS > library.MaxVoiceOffsetMS {
			writeError(w, http.StatusBadRequest, "offset_ms must be within six hours")
			return
		}
		sync.State, sync.Method, sync.OffsetMS, sync.Confidence = library.VoiceManual, library.VoiceMethodManual, *req.OffsetMS, 1
		sync.Note = ""
	}
	if _, err := h.libraryRepo.SetVoiceSync(r.Context(), m.ID, sync); err != nil {
		writeLibraryError(w, "set voice sync", err)
		return
	}
	h.writeVoice(w, r, voiceReply{matchID: m.ID, status: http.StatusOK})
}

type voiceTrackRequest struct {
	Label     *string `json:"label"`
	SteamID64 *string `json:"steamid64"`
}

// PatchLibraryMatchVoiceTrack handles PATCH /api/library/matches/{id}/voice/tracks/{track}: who speaks on it.
func (h *Handlers) PatchLibraryMatchVoiceTrack(w http.ResponseWriter, r *http.Request) {
	m, ok := h.voiceMatch(w, r)
	if !ok {
		return
	}
	track, err := strconv.Atoi(chi.URLParam(r, "track"))
	if err != nil || track < 1 {
		writeError(w, http.StatusBadRequest, "track must be a track number")
		return
	}
	var req voiceTrackRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil || (req.Label == nil && req.SteamID64 == nil) {
		writeError(w, http.StatusBadRequest, "send label or steamid64")
		return
	}
	patch := library.VoiceTrackPatch{MatchID: m.ID, Track: track}
	if req.Label != nil {
		label := library.CleanName(*req.Label)
		if len([]rune(label)) > library.MaxVoiceLabelRunes {
			writeError(w, http.StatusBadRequest, "label takes at most 60 characters")
			return
		}
		patch.Label = &label
	}
	if req.SteamID64 != nil {
		if *req.SteamID64 != "" && !matchHasPlayer(m, *req.SteamID64) {
			writeError(w, http.StatusBadRequest, "steamid64 must be a player of this match, or empty")
			return
		}
		patch.SteamID64 = req.SteamID64
	}
	if _, err := h.libraryRepo.UpdateVoiceTrack(r.Context(), patch); err != nil {
		writeLibraryError(w, "update voice track", err)
		return
	}
	h.writeVoice(w, r, voiceReply{matchID: m.ID, status: http.StatusOK})
}

func matchHasPlayer(m library.Match, steamID string) bool {
	for _, p := range m.Players {
		if p.SteamID64 == steamID {
			return true
		}
	}
	return false
}

// GetLibraryMatchVoiceAudio handles GET /api/library/matches/{id}/voice/tracks/{track}/audio with ranges.
func (h *Handlers) GetLibraryMatchVoiceAudio(w http.ResponseWriter, r *http.Request) {
	m, ok := h.voiceMatch(w, r)
	if !ok {
		return
	}
	track, err := strconv.Atoi(chi.URLParam(r, "track"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "track must be a track number")
		return
	}
	v, err := h.libraryRepo.GetMatchVoice(r.Context(), m.ID)
	if err != nil {
		writeLibraryError(w, "get match voice", err)
		return
	}
	for _, t := range v.Tracks {
		if t.Track != track {
			continue
		}
		path, err := h.voice.trackPath(t)
		if err != nil {
			internalError(w, "voice track path", err)
			return
		}
		f, err := os.Open(path)
		if err != nil {
			writeLibraryError(w, "open voice track", library.ErrNotFound)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "audio/ogg")
		w.Header().Set("Cache-Control", "private, max-age=3600")
		http.ServeContent(w, r, "", time.Time{}, f)
		return
	}
	writeError(w, http.StatusNotFound, "no such voice track")
}
