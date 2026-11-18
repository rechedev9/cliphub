package httpapi

import (
	"cmp"
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/teamvoice"
)

// demoCommsVoice reads a match's demo transcript as team voice, so the comms analysis works without
// TeamSpeak: one track per player, already on the demo's clock. No phrase is ErrNotFound.
func (h *Handlers) demoCommsVoice(ctx context.Context, matchID uuid.UUID) (library.MatchVoice, []library.VoiceLine, error) {
	t, err := h.libraryRepo.GetDemoVoice(ctx, matchID)
	if err != nil {
		return library.MatchVoice{}, nil, err
	}
	phrases, err := h.libraryRepo.ListDemoVoiceLines(ctx, library.DemoVoiceLineFilter{MatchIDs: []uuid.UUID{matchID}})
	if err != nil {
		return library.MatchVoice{}, nil, err
	}
	if len(phrases) == 0 {
		return library.MatchVoice{}, nil, library.ErrNotFound
	}
	v := library.MatchVoice{MatchID: matchID, Tracks: []library.VoiceTrack{},
		Sync:       library.VoiceSync{State: library.VoiceAuto, Method: library.VoiceMethodDemoVoice, Confidence: 1},
		Transcript: library.VoiceTranscript{State: t.State, Model: t.Model, Error: t.Error, UpdatedAt: t.UpdatedAt}}
	track := map[string]int{}
	lines := make([]library.VoiceLine, 0, len(phrases))
	for _, p := range phrases {
		n, ok := track[p.SteamID64]
		if !ok {
			n = len(v.Tracks) + 1
			track[p.SteamID64] = n
			v.Tracks = append(v.Tracks, library.VoiceTrack{Track: n, Label: cmp.Or(p.Name, p.SteamID64), SteamID64: p.SteamID64, Source: "demo"})
		}
		v.Tracks[n-1].Speech = append(v.Tracks[n-1].Speech, library.VoiceSpan{StartMS: p.StartMS, EndMS: p.EndMS})
		lines = append(lines, library.VoiceLine{MatchID: matchID, Track: n, StartMS: p.StartMS, EndMS: p.EndMS, Text: p.Text})
	}
	return v, lines, nil
}

// demoVoiceSearch is a comms search over the demo transcripts, after the TeamSpeak hits.
type demoVoiceSearch struct {
	query    string
	matchIDs []uuid.UUID
	phase    string
	limit    int
}

// demoVoiceHits finds the query in the demo transcripts; truncated says more matched than limit.
func (h *Handlers) demoVoiceHits(r *http.Request, s demoVoiceSearch) ([]voiceSearchHit, bool, error) {
	read := s.limit + 1
	if s.phase != "" {
		read = library.MaxVoiceLineList
	}
	phrases, err := h.libraryRepo.ListDemoVoiceLines(r.Context(), library.DemoVoiceLineFilter{MatchIDs: s.matchIDs, Query: s.query, Limit: read})
	if err != nil {
		return nil, false, err
	}
	hits := []voiceSearchHit{}
	placed := map[uuid.UUID]*voiceSearchMatch{}
	for _, p := range phrases {
		if len(hits) == s.limit {
			return hits, true, nil
		}
		pm, ok := placed[p.MatchID]
		if !ok {
			pm = h.loadDemoSearchMatch(r, p.MatchID)
			placed[p.MatchID] = pm
		}
		if pm == nil {
			continue
		}
		at := teamvoice.Locate(teamvoice.LocateInput{Timeline: pm.timeline, AudioMS: p.StartMS, SteamID64: p.SteamID64})
		hit := voiceSearchHit{MatchID: pm.match.ID, Map: pm.match.Map, PlayedAt: pm.match.PlayedAt, Teams: pm.teams,
			voiceCommsLine: voiceCommsLine{Label: cmp.Or(p.Name, p.SteamID64), SteamID64: p.SteamID64, StartMS: p.StartMS, EndMS: p.EndMS,
				Tick: at.Tick, Round: at.Round, RoundMS: at.RoundMS, Phase: at.Phase, Text: p.Text}}
		if s.phase == "" || hit.Phase == s.phase {
			hits = append(hits, hit)
		}
	}
	return hits, false, nil
}

func (h *Handlers) loadDemoSearchMatch(r *http.Request, id uuid.UUID) *voiceSearchMatch {
	m, err := h.libraryRepo.GetMatch(r.Context(), id)
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
	return &voiceSearchMatch{match: m, timeline: teamvoice.TimelineOf(doc), teams: teams}
}
