package httpapi

import (
	"errors"
	"net/http"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/voicecomms"
)

// jobSubtitles is what the producer's Subtítulos panel reads for a job: the demo transcript with only
// the phrases of the player's team, and Availability in the words of the Full Demo voice check.
type jobSubtitles struct {
	demoTranscript
	Availability string `json:"availability"`
}

// Availabilities the demo voice check of a Full Demo plan does not give.
const (
	voiceNotRequested = "not_requested"
	voiceExtracting   = "extracting"
	voiceDemoMissing  = "demo_missing"
	voiceNoTeamVoice  = "no_team_voice"
	voiceDecodeFailed = "decode_failed"
)

func subtitleAvailability(status demoVoiceStatus, team string) string {
	switch status.State {
	case demoVoiceReady:
		if len(status.Tracks) == 0 {
			return voicecomms.NoPackets
		}
		for _, t := range status.Tracks {
			if t.TeamKey == team {
				return voicecomms.Available
			}
		}
		return voiceNoTeamVoice
	case demoVoiceExtracting:
		return voiceExtracting
	case demoVoiceFailed:
		return voiceDecodeFailed
	case demoVoiceUnreadable:
		return voiceDemoMissing
	}
	return voiceNotRequested
}

// GetJobSubtitles handles GET /api/jobs/{id}/subtitles.
func (h *Handlers) GetJobSubtitles(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJobMeta(w, r)
	if !ok {
		return
	}
	if h.voice == nil || h.libraryRepo == nil {
		writeError(w, http.StatusNotFound, "team voice is not available in this Studio")
		return
	}
	demo, err := h.libraryRepo.DemoForJob(r.Context(), j.ID)
	if errors.Is(err, library.ErrNotFound) || (err == nil && demo.MatchID == nil) {
		writeCodedError(w, http.StatusConflict, codeNoVoice, "this job's demo is not an analysed library demo")
		return
	}
	if err != nil {
		internalError(w, "find the job's demo", err)
		return
	}
	m, err := h.libraryRepo.GetMatch(r.Context(), *demo.MatchID)
	if err != nil {
		writeLibraryError(w, "get the job's match", err)
		return
	}
	t, err := h.demoTranscriptOf(r.Context(), m)
	if err != nil {
		internalError(w, "get demo transcript", err)
		return
	}
	team := teamOf(m, j.TargetSteamID)
	lines := t.Lines[:0]
	for _, l := range t.Lines {
		if l.TeamKey == team {
			lines = append(lines, l)
		}
	}
	t.Lines = lines
	writeJSON(w, http.StatusOK, jobSubtitles{demoTranscript: t, Availability: subtitleAvailability(h.demoVoiceStatusOf(r.Context(), m), team)})
}

// teamOf is the team key of a match player; empty when they did not play it.
func teamOf(m library.Match, steamID string) string {
	for _, p := range m.Players {
		if p.SteamID64 == steamID {
			return p.TeamKey
		}
	}
	return ""
}
