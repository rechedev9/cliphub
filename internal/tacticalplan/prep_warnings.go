package tacticalplan

import (
	"slices"
	"sort"
	"strings"
	"time"
)

const (
	// PrepStaleDays is the age of the newest match past which a team may have changed its playbook.
	PrepStaleDays = 14
	// PrepSmallSampleRounds is the round count under which a preparation rests on too little.
	PrepSmallSampleRounds = 30
	// lineupSize is the five a team fields.
	lineupSize = 5
)

// DataWarningKind names a reason to distrust a preparation.
type DataWarningKind string

// DataWarningKind values.
const (
	DataWarningStale       DataWarningKind = "stale"
	DataWarningLineup      DataWarningKind = "lineup"
	DataWarningSmallSample DataWarningKind = "small_sample"
)

// DataWarning is one data warning: Days and LastPlayedAt for stale, Changed of
// Matches for lineup, Rounds for small_sample.
type DataWarning struct {
	Kind         DataWarningKind `json:"kind"`
	Days         int             `json:"days,omitempty"`
	LastPlayedAt *time.Time      `json:"last_played_at,omitempty"`
	Changed      int             `json:"changed,omitempty"`
	Matches      int             `json:"matches,omitempty"`
	Rounds       int             `json:"rounds,omitempty"`
}

// DataWarningInput is the sample a preparation kept: its matches, the scope's
// lineup, the rounds it counted and the time it is read at.
type DataWarningInput struct {
	Matches []ScoutMatch
	Lineup  Lineup
	Rounds  int
	Now     time.Time
}

// DataWarnings lists what a coach must know before trusting the sample: the
// newest match is old, the five changed between matches, or the rounds are few.
func DataWarnings(in DataWarningInput) []DataWarning {
	out := []DataWarning{}
	if len(in.Matches) == 0 {
		return out
	}
	newest := in.Matches[0].PlayedAt
	for _, m := range in.Matches[1:] {
		if m.PlayedAt.After(newest) {
			newest = m.PlayedAt
		}
	}
	if !newest.IsZero() && !in.Now.IsZero() {
		if days := int(in.Now.Sub(newest).Hours() / 24); days > PrepStaleDays {
			at := newest
			out = append(out, DataWarning{Kind: DataWarningStale, Days: days, LastPlayedAt: &at})
		}
	}
	if changed := lineupChanges(in.Matches, in.Lineup); changed > 0 {
		out = append(out, DataWarning{Kind: DataWarningLineup, Changed: changed, Matches: len(in.Matches)})
	}
	// No rounds at all is the empty sample the views already say, not a small one.
	if in.Rounds > 0 && in.Rounds < PrepSmallSampleRounds {
		out = append(out, DataWarning{Kind: DataWarningSmallSample, Rounds: in.Rounds})
	}
	return out
}

// matchFive is the team's five who played the most rounds of the match, as a sorted key.
func matchFive(m ScoutMatch) []string {
	rounds := map[string]int{}
	for _, r := range m.Document.Rounds {
		side := m.Document.TeamSide(m.TeamKey, r)
		for _, pr := range r.Players {
			if pr.Side != side || side == "" {
				continue
			}
			if p, ok := m.Document.PlayerBySlot(pr.Slot); ok && p.SteamID64 != "" {
				rounds[p.SteamID64]++
			}
		}
	}
	ids := make([]string, 0, len(rounds))
	for id := range rounds {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if rounds[ids[i]] != rounds[ids[j]] {
			return rounds[ids[i]] > rounds[ids[j]]
		}
		return ids[i] < ids[j]
	})
	if len(ids) > lineupSize {
		ids = ids[:lineupSize]
	}
	slices.Sort(ids)
	return ids
}

// lineupChanges counts the matches whose five differ from the scope's lineup
// or, without one, from the five the team fielded most (the newest on a tie).
func lineupChanges(matches []ScoutMatch, lineup Lineup) int {
	fives := make([][]string, 0, len(matches))
	for _, m := range matches {
		fives = append(fives, matchFive(m))
	}
	if len(lineup) > 0 {
		changed := 0
		for _, five := range fives {
			if slices.ContainsFunc(five, func(id string) bool { return !slices.Contains(lineup, id) }) {
				changed++
			}
		}
		return changed
	}
	if len(matches) < 2 {
		return 0
	}
	counts := map[string]int{}
	usual := ""
	for _, five := range fives {
		key := strings.Join(five, ",")
		counts[key]++
		if usual == "" || counts[key] > counts[usual] {
			usual = key
		}
	}
	return len(matches) - counts[usual]
}
