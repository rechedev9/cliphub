package library

import (
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// mapExampleLimit caps the example rounds kept for each side count.
const mapExampleLimit = 8

// MapHistoryMatch is one match of the team whose map history is read. A nil
// Document leaves the match out of the side counts but not out of the result.
type MapHistoryMatch struct {
	Match    Match
	TeamKey  string
	Document *tacticalplan.Document
}

// MapCount is a count with the rounds behind it, most recent match first.
type MapCount struct {
	Count    int                     `json:"count"`
	Examples []tacticalplan.Evidence `json:"examples"`
}

// SideRecord is a team's rounds on one side of one map, read from the
// tactical documents.
type SideRecord struct {
	Side       string   `json:"side"`
	Rounds     int      `json:"rounds"`
	Won        MapCount `json:"won"`
	Lost       MapCount `json:"lost"`
	PistolWon  MapCount `json:"pistol_won"`
	PistolLost MapCount `json:"pistol_lost"`
}

// VetoCount is how often a veto action happened, with the series it is read from.
type VetoCount struct {
	Count     int         `json:"count"`
	SeriesIDs []uuid.UUID `json:"series_ids"`
}

// VetoRecord is a team's known vetoes of one map, from its own point of view.
type VetoRecord struct {
	Picked         VetoCount `json:"picked"`
	Banned         VetoCount `json:"banned"`
	OpponentPicked VetoCount `json:"opponent_picked"`
	OpponentBanned VetoCount `json:"opponent_banned"`
	Decider        VetoCount `json:"decider"`
}

// MapRecord is a team's history on one map: results from the match scores,
// side rounds from the tactical documents and vetoes from its series.
type MapRecord struct {
	Map             string       `json:"map"`
	Matches         int          `json:"matches"`
	Wins            int          `json:"wins"`
	Losses          int          `json:"losses"`
	Ties            int          `json:"ties"`
	RoundsWon       int          `json:"rounds_won"`
	RoundsLost      int          `json:"rounds_lost"`
	MatchIDs        []uuid.UUID  `json:"match_ids"`
	MeasuredMatches int          `json:"measured_matches"`
	Sides           []SideRecord `json:"sides"`
	LastPlayedAt    *time.Time   `json:"last_played_at,omitempty"`
	Veto            VetoRecord   `json:"veto"`
}

func newMapRecord(name string) *MapRecord {
	r := &MapRecord{Map: name, MatchIDs: []uuid.UUID{}, Sides: []SideRecord{
		newSideRecord(tacticalplan.SideCT), newSideRecord(tacticalplan.SideT),
	}}
	r.Veto = VetoRecord{Picked: newVetoCount(), Banned: newVetoCount(), OpponentPicked: newVetoCount(), OpponentBanned: newVetoCount(), Decider: newVetoCount()}
	return r
}

func newSideRecord(side tacticalplan.Side) SideRecord {
	empty := func() MapCount { return MapCount{Examples: []tacticalplan.Evidence{}} }
	return SideRecord{Side: string(side), Won: empty(), Lost: empty(), PistolWon: empty(), PistolLost: empty()}
}

func newVetoCount() VetoCount { return VetoCount{SeriesIDs: []uuid.UUID{}} }

func (c *MapCount) add(e tacticalplan.Evidence) {
	c.Count++
	if len(c.Examples) < mapExampleLimit {
		c.Examples = append(c.Examples, e)
	}
}

func (c *VetoCount) add(seriesID uuid.UUID) {
	c.Count++
	c.SeriesIDs = append(c.SeriesIDs, seriesID)
}

// TeamMapHistory reads a team's record on every map it played or vetoed. It
// is pure: the same matches and series give the same records whatever their
// order. Maps are sorted by matches played, then by name.
func TeamMapHistory(teamID uuid.UUID, matches []MapHistoryMatch, series []Series) []MapRecord {
	ordered := append([]MapHistoryMatch{}, matches...)
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i].Match, ordered[j].Match
		if !a.PlayedAt.Equal(b.PlayedAt) {
			return a.PlayedAt.After(b.PlayedAt)
		}
		return a.ID.String() < b.ID.String()
	})
	records := map[string]*MapRecord{}
	record := func(name string) *MapRecord {
		name = canonicalMapName(name)
		if r, ok := records[name]; ok {
			return r
		}
		r := newMapRecord(name)
		records[name] = r
		return r
	}
	for _, hm := range ordered {
		addMatch(record(hm.Match.Map), hm)
	}
	sortedSeries := append([]Series{}, series...)
	sort.SliceStable(sortedSeries, func(i, j int) bool { return sortedSeries[i].ID.String() < sortedSeries[j].ID.String() })
	for _, s := range sortedSeries {
		slot := s.Slot(teamID)
		if slot == "" {
			continue
		}
		for _, step := range s.Vetoes {
			v := &record(step.Map).Veto
			switch {
			case step.Action == VetoDecider:
				v.Decider.add(s.ID)
			case step.Team == slot && step.Action == VetoPick:
				v.Picked.add(s.ID)
			case step.Team == slot && step.Action == VetoBan:
				v.Banned.add(s.ID)
			case step.Action == VetoPick:
				v.OpponentPicked.add(s.ID)
			case step.Action == VetoBan:
				v.OpponentBanned.add(s.ID)
			}
		}
	}
	out := make([]MapRecord, 0, len(records))
	for _, r := range records {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Matches != out[j].Matches {
			return out[i].Matches > out[j].Matches
		}
		return out[i].Map < out[j].Map
	})
	return out
}

func addMatch(r *MapRecord, hm MapHistoryMatch) {
	var own, rival *MatchTeam
	for i := range hm.Match.Teams {
		t := &hm.Match.Teams[i]
		if t.Key == hm.TeamKey {
			own = t
		} else {
			rival = t
		}
	}
	if own == nil || rival == nil {
		return
	}
	r.Matches++
	r.MatchIDs = append(r.MatchIDs, hm.Match.ID)
	r.RoundsWon += own.Score
	r.RoundsLost += rival.Score
	switch {
	case own.Score > rival.Score:
		r.Wins++
	case own.Score < rival.Score:
		r.Losses++
	default:
		r.Ties++
	}
	if r.LastPlayedAt == nil {
		played := hm.Match.PlayedAt
		r.LastPlayedAt = &played
	}
	if hm.Document == nil {
		return
	}
	r.MeasuredMatches++
	doc := *hm.Document
	for _, round := range doc.Rounds {
		side := doc.TeamSide(hm.TeamKey, round)
		var rec *SideRecord
		for i := range r.Sides {
			if r.Sides[i].Side == string(side) {
				rec = &r.Sides[i]
			}
		}
		if rec == nil || round.Winner == "" {
			continue
		}
		e := tacticalplan.Evidence{MatchID: hm.Match.ID, Round: round.Number, Tick: round.TickFreezeEnd}
		rec.Rounds++
		won := round.Winner == side
		if won {
			rec.Won.add(e)
		} else {
			rec.Lost.add(e)
		}
		if round.Class.HasTag(tacticalplan.TagPistol) {
			if won {
				rec.PistolWon.add(e)
			} else {
				rec.PistolLost.add(e)
			}
		}
	}
}
