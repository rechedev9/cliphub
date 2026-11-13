package httpapi

import (
	"crypto/subtle"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/gamestate"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/voiceplugin"
)

// codes of the plugin routes.
const (
	codePluginUnavailable = "plugin_unavailable"
	codeRecordingAttached = "recording_attached"
	codeNoCS2             = "cs2_not_found"
)

// maxGameStateBytes bounds one CS2 post; Studio asks only for the provider, map and round.
const maxGameStateBytes = 64 << 10

func (h *Handlers) pluginRoute(w http.ResponseWriter) bool {
	if !h.voiceReady(w) {
		return false
	}
	if !h.pluginReady() {
		writeCodedError(w, http.StatusServiceUnavailable, codePluginUnavailable, "the TeamSpeak plugin needs Studio's data folder and FFmpeg")
		return false
	}
	return true
}

// voiceRecordingView is a recording as the Studio UI and the MCP read it.
type voiceRecordingView struct {
	library.VoiceRecording
	// Status is attached once its tracks are in a match, else pending.
	Status     string `json:"status"`
	DurationMS int64  `json:"duration_ms"`
}

func recordingView(rec library.VoiceRecording) voiceRecordingView {
	v := voiceRecordingView{VoiceRecording: rec, Status: "pending", DurationMS: rec.EndedAt.Sub(rec.StartedAt).Milliseconds()}
	if rec.MatchID != nil {
		v.Status = "attached"
	}
	return v
}

// ListVoiceRecordings handles GET /api/library/voice/recordings: the plugin's recordings, newest first.
func (h *Handlers) ListVoiceRecordings(w http.ResponseWriter, r *http.Request) {
	if !h.pluginRoute(w) {
		return
	}
	recs, err := h.libraryRepo.ListVoiceRecordings(r.Context())
	if err != nil {
		internalError(w, "list voice recordings", err)
		return
	}
	out := make([]voiceRecordingView, 0, len(recs))
	for _, rec := range recs {
		out = append(out, recordingView(rec))
	}
	writeJSON(w, http.StatusOK, struct {
		Recordings []voiceRecordingView `json:"recordings"`
	}{out})
}

func (h *Handlers) voiceRecording(w http.ResponseWriter, r *http.Request) (library.VoiceRecording, bool) {
	if !h.pluginRoute(w) {
		return library.VoiceRecording{}, false
	}
	id := chi.URLParam(r, "id")
	if !voiceplugin.ValidID(id) {
		writeError(w, http.StatusBadRequest, "id must be a recording id")
		return library.VoiceRecording{}, false
	}
	rec, err := h.libraryRepo.GetVoiceRecording(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get voice recording", err)
		return library.VoiceRecording{}, false
	}
	return rec, true
}

// AttachVoiceRecording handles POST /api/library/voice/recordings/{id}/attach: the recording's tracks
// go to that match, tied to the players its teams remember, and are placed on the demo.
func (h *Handlers) AttachVoiceRecording(w http.ResponseWriter, r *http.Request) {
	rec, ok := h.voiceRecording(w, r)
	if !ok {
		return
	}
	var req struct {
		MatchID uuid.UUID `json:"match_id"`
	}
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil || req.MatchID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "send match_id")
		return
	}
	m, err := h.libraryRepo.GetMatch(r.Context(), req.MatchID)
	if err != nil {
		writeLibraryError(w, "get match", err)
		return
	}
	// A scan attaches under the same lock, so read the recording again once it is held.
	p := h.voice.plugin
	p.scanMu.Lock()
	defer p.scanMu.Unlock()
	if rec, err = h.libraryRepo.GetVoiceRecording(r.Context(), rec.ID); err != nil {
		writeLibraryError(w, "get voice recording", err)
		return
	}
	if rec.MatchID != nil {
		if *rec.MatchID == m.ID {
			writeJSON(w, http.StatusOK, recordingView(rec))
			return
		}
		if _, err := h.libraryRepo.GetMatchVoice(r.Context(), *rec.MatchID); !errors.Is(err, library.ErrNotFound) {
			writeCodedError(w, http.StatusConflict, codeRecordingAttached, "the recording is in another match's voice; remove that voice first")
			return
		}
	}
	out, err := h.attachRecording(r.Context(), attachRequest{rec: rec, match: m, note: "Asociada a mano."})
	switch {
	case errors.Is(err, errVoicePresent):
		writeCodedError(w, http.StatusConflict, codeVoicePresent, "the match already has team voice; remove it first")
	case err != nil:
		internalError(w, "attach voice recording", err)
	default:
		writeJSON(w, http.StatusOK, recordingView(out))
	}
}

// DeleteVoiceRecording handles DELETE /api/library/voice/recordings/{id}: its folder goes; a match keeps
// the tracks already added to it.
func (h *Handlers) DeleteVoiceRecording(w http.ResponseWriter, r *http.Request) {
	rec, ok := h.voiceRecording(w, r)
	if !ok {
		return
	}
	dir, err := h.voice.deps.Plugin.Dir.Recording(rec.ID)
	if err != nil {
		internalError(w, "voice recording folder", err)
		return
	}
	if err := h.libraryRepo.DeleteVoiceRecording(r.Context(), rec.ID); err != nil {
		writeLibraryError(w, "delete voice recording", err)
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		log.Printf("httpapi: remove TeamSpeak recording %s: %v", rec.ID, err)
	}
	w.WriteHeader(http.StatusNoContent)
}

type voiceSpeakersRequest struct {
	Speakers []struct {
		Track     int    `json:"track"`
		SteamID64 string `json:"steamid64"`
	} `json:"speakers"`
}

// PutLibraryMatchVoiceSpeakers handles PUT /api/library/matches/{id}/voice/speakers: who speaks on each
// track, which the team keeps for the plugin's next recordings.
func (h *Handlers) PutLibraryMatchVoiceSpeakers(w http.ResponseWriter, r *http.Request) {
	m, ok := h.voiceRecord(w, r)
	if !ok {
		return
	}
	var req voiceSpeakersRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil || len(req.Speakers) == 0 || len(req.Speakers) > library.MaxVoiceTracks {
		writeError(w, http.StatusBadRequest, "send speakers: a track and a steamid64 each")
		return
	}
	v, err := h.libraryRepo.GetMatchVoice(r.Context(), m.ID)
	if errors.Is(err, library.ErrNotFound) {
		writeCodedError(w, http.StatusNotFound, codeNoVoice, "this match has no team voice")
		return
	}
	if err != nil {
		internalError(w, "get match voice", err)
		return
	}
	has := map[int]bool{}
	for _, t := range v.Tracks {
		has[t.Track] = true
	}
	for _, s := range req.Speakers {
		if !has[s.Track] || (s.SteamID64 != "" && !matchHasPlayer(m, s.SteamID64)) {
			writeError(w, http.StatusBadRequest, "each speaker needs a track of this match and one of its players, or an empty steamid64")
			return
		}
	}
	for _, s := range req.Speakers {
		steamID := s.SteamID64
		if _, err := h.libraryRepo.UpdateVoiceTrack(r.Context(), library.VoiceTrackPatch{MatchID: m.ID, Track: s.Track, SteamID64: &steamID}); err != nil {
			writeLibraryError(w, "update voice track", err)
			return
		}
		h.rememberVoiceSpeaker(r.Context(), speakerChoice{match: m, track: s.Track, steamID64: steamID})
	}
	h.writeVoice(w, r, voiceReply{matchID: m.ID, status: http.StatusOK})
}

// voicePluginStatus is what Ajustes shows about the plugin and CS2.
type voicePluginStatus struct {
	Inbox      string         `json:"inbox"`
	WebOrigin  string         `json:"web_origin"`
	CS2Found   bool           `json:"cs2_found"`
	GameState  string         `json:"game_state"`
	ConfigPath string         `json:"config_path,omitempty"`
	Live       gamestate.Live `json:"live"`
	Pending    int            `json:"pending"`
}

func (h *Handlers) pluginStatus(r *http.Request) (voicePluginStatus, error) {
	deps := h.voice.deps.Plugin
	st := voicePluginStatus{Inbox: deps.Dir.Inbox(), WebOrigin: deps.WebOrigin, GameState: "missing", Live: h.voice.plugin.game.State(time.Now())}
	cfg, err := h.gameStateConfig()
	switch {
	case errors.Is(err, gamestate.ErrNoCS2):
	case err != nil:
		return st, err
	default:
		st.CS2Found, st.ConfigPath = true, cfg.Path
		if cfg.Present {
			st.GameState = "stale"
		}
		if cfg.Current {
			st.GameState = "installed"
		}
	}
	recs, err := h.libraryRepo.ListVoiceRecordings(r.Context())
	if err != nil {
		return st, err
	}
	for _, rec := range recs {
		if rec.MatchID == nil {
			st.Pending++
		}
	}
	return st, nil
}

// GetVoicePlugin handles GET /api/library/voice/plugin.
func (h *Handlers) GetVoicePlugin(w http.ResponseWriter, r *http.Request) {
	if !h.pluginRoute(w) {
		return
	}
	st, err := h.pluginStatus(r)
	if err != nil {
		internalError(w, "voice plugin status", err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// SetupVoicePlugin handles POST /api/library/voice/plugin/setup: CS2 learns to tell Studio when a match is live.
func (h *Handlers) SetupVoicePlugin(w http.ResponseWriter, r *http.Request) {
	if !h.pluginRoute(w) {
		return
	}
	if h.voice.deps.Plugin.WebOrigin == "" {
		writeCodedError(w, http.StatusServiceUnavailable, codePluginUnavailable, "Studio started without its web address")
		return
	}
	if _, err := h.installGameState(); err != nil {
		if errors.Is(err, gamestate.ErrNoCS2) {
			writeCodedError(w, http.StatusNotFound, codeNoCS2, "CS2 is not installed or was not found on this PC")
			return
		}
		internalError(w, "install the CS2 game state config", err)
		return
	}
	h.GetVoicePlugin(w, r)
}

// PostGameState handles POST /api/voice-plugin/gamestate: CS2's posts, through the web server's guard.
func (h *Handlers) PostGameState(w http.ResponseWriter, r *http.Request) {
	if !h.pluginRoute(w) {
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxGameStateBytes))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "game state too large")
		return
	}
	p, err := gamestate.Parse(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "game state must be JSON")
		return
	}
	tok, err := h.voice.deps.Plugin.Dir.Token()
	if err != nil {
		internalError(w, "game state token", err)
		return
	}
	if subtle.ConstantTimeCompare([]byte(p.Auth.Token), []byte(tok)) != 1 {
		writeError(w, http.StatusUnauthorized, "game state token mismatch")
		return
	}
	h.voice.plugin.game.Observe(p, time.Now())
	w.WriteHeader(http.StatusNoContent)
}

// GetVoicePluginLive handles GET /api/voice-plugin/live: whether a match is live, as plain text for the plugin.
func (h *Handlers) GetVoicePluginLive(w http.ResponseWriter, _ *http.Request) {
	if !h.pluginRoute(w) {
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, h.voice.plugin.game.State(time.Now()).Text())
}
