package httpapi

import (
	"context"
	"log"
	"slices"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/library"
)

// groupProSeries puts the imported maps of one HLTV match in one series, made on the
// first import with the catalog's format, event and veto. After that only the new map
// joins, so a map the user took out stays out.
func (h *Handlers) groupProSeries(ctx context.Context, header community.ProAnalysisHeader, added library.Match) {
	matches := h.importedProMatches(ctx, header.Match.MatchID)
	if !slices.ContainsFunc(matches, func(m library.Match) bool { return m.ID == added.ID }) {
		matches = append(matches, added)
	}
	var seriesID *uuid.UUID
	for _, m := range matches {
		if m.SeriesID != nil && seriesID == nil {
			seriesID = m.SeriesID
		}
	}
	if seriesID == nil {
		s := h.newProSeries(ctx, proSeriesSource{sidecar: header.Match, imported: len(matches), added: added})
		if err := h.libraryRepo.CreateSeries(ctx, &s); err != nil {
			log.Printf("httpapi: create pro series: %v", err)
			return
		}
		seriesID = &s.ID
	} else {
		h.fillProSeries(ctx, proSeriesFill{id: *seriesID, sidecar: header.Match})
		matches = []library.Match{added}
	}
	for _, m := range matches {
		if m.SeriesID != nil {
			continue
		}
		if _, err := h.libraryRepo.UpdateMatch(ctx, m.ID, library.MatchPatch{SeriesID: seriesID}); err != nil {
			log.Printf("httpapi: attach pro map %s to its series: %v", m.ID, err)
		}
	}
}

// importedProMatches is the library match of every imported map of an HLTV match.
func (h *Handlers) importedProMatches(ctx context.Context, matchID int) []library.Match {
	records, err := h.libraryRepo.ListProMaps(ctx, matchID)
	if err != nil {
		log.Printf("httpapi: group pro series: %v", err)
		return nil
	}
	matches := []library.Match{}
	for _, rec := range records {
		if rec.DemoID == nil {
			continue
		}
		if d, err := h.libraryRepo.GetDemo(ctx, *rec.DemoID); err == nil && d.MatchID != nil {
			if m, err := h.libraryRepo.GetMatch(ctx, *d.MatchID); err == nil && !slices.ContainsFunc(matches, func(x library.Match) bool { return x.ID == m.ID }) {
				matches = append(matches, m)
			}
		}
	}
	return matches
}

// proSeriesSource is the HLTV match of a new series, its maps imported so far and the map just added.
type proSeriesSource struct {
	sidecar  community.ProSidecar
	imported int
	added    library.Match
}

// newProSeries is the series of an HLTV match, marked as written by the import.
func (h *Handlers) newProSeries(ctx context.Context, src proSeriesSource) library.Series {
	base := library.Series{Format: proSeriesFormat(len(src.sidecar.Maps), src.imported), Event: src.sidecar.Event, TeamIDs: proSeriesTeams(src.added)}
	return h.proSeriesOver(ctx, proSeriesFill{sidecar: src.sidecar, base: base})
}

// proSeriesFill is a series of an HLTV match and the sidecar of a map just stored in it.
type proSeriesFill struct {
	id      uuid.UUID
	sidecar community.ProSidecar
	base    library.Series
}

// proSeriesOver lays the catalog's format, teams in HLTV order and veto over a series.
func (h *Handlers) proSeriesOver(ctx context.Context, f proSeriesFill) library.Series {
	s, info := f.base, community.ProSeriesOf(f.sidecar)
	if library.ValidSeriesFormat(info.Format) {
		s.Format = info.Format
	}
	teams := h.proLibraryTeams(ctx, f.sidecar.Teams)
	if pair := proTeamPair(f.sidecar.Teams, teams); pair != nil {
		s.TeamIDs = pair
	}
	s.Vetoes, s.VetoAuto = proLibraryVetoes(info, proSlots(s.TeamIDs, teams)), true
	if cleaned, err := library.CleanSeries(s); err == nil {
		return cleaned
	}
	s.Vetoes = proDecider(info)
	if cleaned, err := library.CleanSeries(s); err == nil {
		return cleaned
	}
	s.Vetoes = nil
	return s
}

// proSeriesRefreshable is a series an import wrote and no person edited since; a series of an
// older build without the marker counts while it has no veto and was never edited.
func proSeriesRefreshable(s library.Series) bool {
	return s.VetoAuto || (len(s.Vetoes) == 0 && s.UpdatedAt.Equal(s.CreatedAt))
}

// fillProSeries refreshes a series an import wrote with what the catalog knows now, so a
// veto read later replaces a decider guessed from the map count. A person's edit is kept.
func (h *Handlers) fillProSeries(ctx context.Context, f proSeriesFill) {
	s, err := h.libraryRepo.GetSeries(ctx, f.id)
	if err != nil || !proSeriesRefreshable(s) {
		return
	}
	f.base = s
	next := h.proSeriesOver(ctx, f)
	// A map prepared before the veto was read must not wipe a veto another map brought.
	if len(next.Vetoes) < len(s.Vetoes) || (next.Format == s.Format && slices.Equal(next.TeamIDs, s.TeamIDs) && slices.Equal(next.Vetoes, s.Vetoes) && s.VetoAuto) {
		return
	}
	if _, err := h.libraryRepo.UpdateSeries(ctx, f.id, next); err != nil {
		log.Printf("httpapi: fill pro series %s: %v", f.id, err)
	}
}

// proLibraryTeams maps each HLTV team id to its library team, when the library has one.
func (h *Handlers) proLibraryTeams(ctx context.Context, teams []community.ProSidecarTeam) map[int]uuid.UUID {
	out := map[int]uuid.UUID{}
	for _, t := range teams {
		if team, found, err := h.findTeamByName(ctx, cleanLibraryText(t.Name)); err == nil && found {
			out[t.ID] = team.ID
		}
	}
	return out
}

// proTeamPair is the library teams of both HLTV teams in HLTV order, or nil unless both are two different teams.
func proTeamPair(teams []community.ProSidecarTeam, ids map[int]uuid.UUID) []uuid.UUID {
	if len(teams) != 2 {
		return nil
	}
	a, b := ids[teams[0].ID], ids[teams[1].ID]
	if a == uuid.Nil || b == uuid.Nil || a == b {
		return nil
	}
	return []uuid.UUID{a, b}
}

// proSlots maps each HLTV team id to its slot in a series of teamIDs.
func proSlots(teamIDs []uuid.UUID, ids map[int]uuid.UUID) map[int]string {
	out := map[int]string{}
	for hltv, id := range ids {
		if i := slices.Index(teamIDs, id); i == 0 || i == 1 {
			out[hltv] = [2]string{library.SeriesTeamA, library.SeriesTeamB}[i]
		}
	}
	return out
}

// proLibraryVetoes is the catalog veto in library slots. A step of a team without a slot
// goes to the other slot when only one team has one; otherwise it is left out.
func proLibraryVetoes(info community.ProSeries, slots map[int]string) []library.VetoStep {
	if len(info.Veto) == 0 {
		return proDecider(info)
	}
	only := ""
	if len(slots) == 1 {
		for _, slot := range slots {
			only = map[string]string{library.SeriesTeamA: library.SeriesTeamB, library.SeriesTeamB: library.SeriesTeamA}[slot]
		}
	}
	steps := []library.VetoStep{}
	for _, v := range info.Veto {
		slot := slots[v.TeamID]
		switch {
		case v.Action == community.ProVetoDecider:
			steps = append(steps, library.VetoStep{Action: library.VetoDecider, Map: v.Map})
		case slot != "":
			steps = append(steps, library.VetoStep{Team: slot, Action: v.Action, Map: v.Map})
		case only != "" && v.TeamID != 0:
			steps = append(steps, library.VetoStep{Team: only, Action: v.Action, Map: v.Map})
		}
	}
	return steps
}

func proDecider(info community.ProSeries) []library.VetoStep {
	for _, m := range info.Maps {
		if m.Decider {
			return []library.VetoStep{{Action: library.VetoDecider, Map: m.Map}}
		}
	}
	return []library.VetoStep{}
}

func proSeriesFormat(sidecarMaps, imported int) string {
	if max(sidecarMaps, imported) > 3 {
		return library.FormatBO5
	}
	return library.FormatBO3
}

// proSeriesTeams is both resolved teams of the map, or none when either side has none.
func proSeriesTeams(m library.Match) []uuid.UUID {
	ids := []uuid.UUID{}
	for _, t := range m.Teams {
		if t.TeamID == nil || slices.Contains(ids, *t.TeamID) {
			return []uuid.UUID{}
		}
		ids = append(ids, *t.TeamID)
	}
	if len(ids) != 2 {
		return []uuid.UUID{}
	}
	return ids
}
