package tacticalplan

import (
	"sort"
	"time"
)

// ScoutFilter selects matches by map, date and rival, and rounds by the side
// the scouted team played and its lineup. Zero fields do not filter; From and
// To are inclusive.
type ScoutFilter struct {
	Maps      []string   `json:"maps,omitempty"`
	Side      Side       `json:"side,omitempty"`
	From      *time.Time `json:"from,omitempty"`
	To        *time.Time `json:"to,omitempty"`
	Opponents []string   `json:"opponents,omitempty"`
	Lineup    Lineup     `json:"lineup,omitempty"`
}

// Select returns the matches the filter keeps, most recent first. The order is
// the order every example list follows, so the newest rounds come first.
func (f ScoutFilter) Select(matches []ScoutMatch) []ScoutMatch {
	out := make([]ScoutMatch, 0, len(matches))
	for _, m := range matches {
		if len(f.Maps) > 0 && !containsString(f.Maps, m.Document.Demo.Map) {
			continue
		}
		if f.From != nil && m.PlayedAt.Before(*f.From) {
			continue
		}
		if f.To != nil && m.PlayedAt.After(*f.To) {
			continue
		}
		if len(f.Opponents) > 0 && !containsString(f.Opponents, m.OpponentID) {
			continue
		}
		out = append(out, m)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].PlayedAt.Equal(out[j].PlayedAt) {
			return out[i].PlayedAt.After(out[j].PlayedAt)
		}
		return out[i].MatchID.String() < out[j].MatchID.String()
	})
	return out
}

func (f ScoutFilter) rounds(m ScoutMatch) []Round {
	return f.roundFilter(m).Apply(m.Document)
}

func (f ScoutFilter) roundFilter(m ScoutMatch) Filter {
	return Filter{TeamKey: m.TeamKey, Side: f.Side, Lineup: f.Lineup}
}
