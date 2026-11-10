package httpapi

import (
	"cmp"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/teamvoice"
)

// voiceCommsLine is one transcribed phrase placed on the demo.
type voiceCommsLine struct {
	Track     int    `json:"track"`
	Label     string `json:"label"`
	SteamID64 string `json:"steamid64,omitempty"`
	StartMS   int64  `json:"start_ms"`
	EndMS     int64  `json:"end_ms"`
	Tick      int    `json:"tick"`
	Round     int    `json:"round"`
	RoundMS   int64  `json:"round_ms"`
	Phase     string `json:"phase,omitempty"`
	Text      string `json:"text"`
}

type voiceTrackSpeech struct {
	Track     int                 `json:"track"`
	Label     string              `json:"label"`
	SteamID64 string              `json:"steamid64,omitempty"`
	Speech    []library.VoiceSpan `json:"speech"`
}

// voiceComms is a match's comms: talk time by round and phase, the transcript and each track's speech.
type voiceComms struct {
	MatchID    uuid.UUID               `json:"match_id"`
	Sync       library.VoiceSync       `json:"sync"`
	Transcript library.VoiceTranscript `json:"transcript"`
	Tracks     []voiceTrackSpeech      `json:"tracks"`
	Rounds     []teamvoice.RoundTalk   `json:"rounds"`
	Totals     []teamvoice.SpeakerTalk `json:"totals"`
	Lines      []voiceCommsLine        `json:"lines"`
}

// GetLibraryMatchVoiceComms handles GET /api/library/matches/{id}/voice/comms, with ?round= for one round.
func (h *Handlers) GetLibraryMatchVoiceComms(w http.ResponseWriter, r *http.Request) {
	m, ok := h.voiceMatch(w, r)
	if !ok {
		return
	}
	round := 0
	if raw := r.URL.Query().Get("round"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > m.RoundCount {
			writeError(w, http.StatusBadRequest, "round must be a round of the match")
			return
		}
		round = n
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
	doc, err := h.readLibraryDocument(m.DemoID)
	if err != nil {
		internalError(w, "read match analysis", err)
		return
	}
	timeline := teamvoice.TimelineOf(doc)
	rounds, totals := teamvoice.TalkTime(teamvoice.TalkInput{Tracks: v.Tracks, OffsetMS: v.Sync.OffsetMS, Timeline: timeline})
	out := voiceComms{MatchID: m.ID, Sync: v.Sync, Transcript: v.Transcript, Tracks: []voiceTrackSpeech{}, Rounds: []teamvoice.RoundTalk{}, Totals: totals, Lines: []voiceCommsLine{}}
	for _, rt := range rounds {
		if round == 0 || rt.Round == round {
			out.Rounds = append(out.Rounds, rt)
		}
	}
	for _, t := range v.Tracks {
		out.Tracks = append(out.Tracks, voiceTrackSpeech{Track: t.Track, Label: t.Label, SteamID64: t.SteamID64, Speech: t.Speech})
	}
	lines, err := h.libraryRepo.ListVoiceLines(r.Context(), library.VoiceLineFilter{MatchIDs: []uuid.UUID{m.ID}, Limit: library.MaxVoiceLineList})
	if err != nil {
		internalError(w, "list voice lines", err)
		return
	}
	for _, l := range lines {
		line := placeVoiceLine(v, timeline, l)
		if round == 0 || line.Round == round {
			out.Lines = append(out.Lines, line)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func placeVoiceLine(v library.MatchVoice, timeline teamvoice.Timeline, l library.VoiceLine) voiceCommsLine {
	at := teamvoice.Locate(timeline, l.StartMS, v.Sync.OffsetMS)
	line := voiceCommsLine{Track: l.Track, StartMS: l.StartMS, EndMS: l.EndMS, Tick: at.Tick, Round: at.Round, RoundMS: at.RoundMS, Phase: at.Phase, Text: l.Text}
	for _, t := range v.Tracks {
		if t.Track == l.Track {
			line.Label, line.SteamID64 = t.Label, t.SteamID64
		}
	}
	return line
}

// voiceSearchHit is a phrase found in a match's comms, with the round and moment it was said.
type voiceSearchHit struct {
	MatchID  uuid.UUID `json:"match_id"`
	Map      string    `json:"map"`
	PlayedAt time.Time `json:"played_at"`
	Teams    []string  `json:"teams"`
	voiceCommsLine
}

const (
	defaultVoiceSearch = 50
	maxVoiceSearch     = 200
	maxVoiceQueryRunes = 120
)

// SearchLibraryVoice handles GET /api/library/voice/search: q in every transcript, narrowed by
// match_id, team_id and the from/to dates of the matches.
func (h *Handlers) SearchLibraryVoice(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	q := r.URL.Query()
	query := strings.Join(strings.Fields(q.Get("q")), " ")
	if query == "" || len([]rune(query)) > maxVoiceQueryRunes {
		writeError(w, http.StatusBadRequest, "q must be 1 to 120 characters")
		return
	}
	limit := defaultVoiceSearch
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > maxVoiceSearch {
			writeError(w, http.StatusBadRequest, "limit must be from 1 to 200")
			return
		}
		limit = n
	}
	matchIDs, err := h.voiceSearchMatches(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	out := struct {
		Hits      []voiceSearchHit `json:"hits"`
		Truncated bool             `json:"truncated"`
	}{Hits: []voiceSearchHit{}}
	if matchIDs != nil && len(matchIDs) == 0 {
		writeJSON(w, http.StatusOK, out)
		return
	}
	lines, err := h.libraryRepo.ListVoiceLines(r.Context(), library.VoiceLineFilter{MatchIDs: matchIDs, Query: query, Limit: limit + 1})
	if err != nil {
		internalError(w, "search voice lines", err)
		return
	}
	out.Truncated = len(lines) > limit
	placed := map[uuid.UUID]*voiceSearchMatch{}
	for _, l := range lines[:min(len(lines), limit)] {
		pm, ok := placed[l.MatchID]
		if !ok {
			pm = h.loadVoiceSearchMatch(r, l.MatchID)
			placed[l.MatchID] = pm
		}
		if pm == nil {
			continue
		}
		out.Hits = append(out.Hits, voiceSearchHit{MatchID: pm.match.ID, Map: pm.match.Map, PlayedAt: pm.match.PlayedAt, Teams: pm.teams,
			voiceCommsLine: placeVoiceLine(pm.voice, pm.timeline, l)})
	}
	writeJSON(w, http.StatusOK, out)
}

type voiceSearchMatch struct {
	match    library.Match
	voice    library.MatchVoice
	timeline teamvoice.Timeline
	teams    []string
}

// loadVoiceSearchMatch reads what places a hit; a match whose analysis is gone gives nil.
func (h *Handlers) loadVoiceSearchMatch(r *http.Request, id uuid.UUID) *voiceSearchMatch {
	m, err := h.libraryRepo.GetMatch(r.Context(), id)
	if err != nil {
		return nil
	}
	v, err := h.libraryRepo.GetMatchVoice(r.Context(), id)
	if err != nil {
		return nil
	}
	doc, err := h.readLibraryDocument(m.DemoID)
	if err != nil {
		return nil
	}
	teams := []string{}
	for _, t := range m.Teams {
		teams = append(teams, cmp.Or(t.TeamName, t.ClanName, t.Key))
	}
	return &voiceSearchMatch{match: m, voice: v, timeline: teamvoice.TimelineOf(doc), teams: teams}
}

// voiceSearchMatches is the match set of a search: nil for every match.
func (h *Handlers) voiceSearchMatches(r *http.Request) ([]uuid.UUID, error) {
	q := r.URL.Query()
	matchID, err := optionalUUIDQuery(q, "match_id")
	if err != nil {
		return nil, err
	}
	if matchID != nil {
		return []uuid.UUID{*matchID}, nil
	}
	filter := library.MatchFilter{Limit: library.MaxListLimit}
	if filter.TeamID, err = optionalUUIDQuery(q, "team_id"); err != nil {
		return nil, err
	}
	if filter.From, err = optionalTimeQuery(q, "from"); err != nil {
		return nil, err
	}
	if filter.To, err = optionalTimeQuery(q, "to"); err != nil {
		return nil, err
	}
	if filter.TeamID == nil && filter.From == nil && filter.To == nil {
		return nil, nil
	}
	ids := []uuid.UUID{}
	for offset := 0; ; offset += library.MaxListLimit {
		filter.Offset = offset
		page, err := h.libraryRepo.ListMatches(r.Context(), filter)
		if err != nil {
			return nil, err
		}
		for _, m := range page {
			ids = append(ids, m.ID)
		}
		if len(page) < library.MaxListLimit {
			return ids, nil
		}
	}
}
