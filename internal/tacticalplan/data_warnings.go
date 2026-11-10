package tacticalplan

import (
	"slices"
	"sort"
	"strings"
	"time"
)

const (
	// DataStaleDays is the age of the team's newest match past which it may have changed its playbook.
	DataStaleDays = 14
	// DataSmallSampleRounds is the round count under which a preparation rests on too little.
	DataSmallSampleRounds = 30
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

// DataWarning is one data warning of Preparar: Days and LastPlayedAt for stale,
// Changed of Matches for lineup, Rounds for small_sample.
type DataWarning struct {
	Kind         DataWarningKind `json:"kind"`
	Days         int             `json:"days,omitempty"`
	LastPlayedAt *time.Time      `json:"last_played_at,omitempty"`
	Changed      int             `json:"changed,omitempty"`
	Matches      int             `json:"matches,omitempty"`
	Rounds       int             `json:"rounds,omitempty"`
}

// DataWarningInput is what the warnings read, each set already loaded with its documents.
type DataWarningInput struct {
	// Newest is when the team's newest analysed match was played, whatever the filters; zero for none.
	Newest time.Time
	// Compared is the matches the scope keeps before its lineup filter.
	Compared []ScoutMatch
	// Kept is the matches the whole scope keeps; Filter's rounds of them are the sample.
	Kept   []ScoutMatch
	Filter ScoutFilter
	Now    time.Time
}

// DataWarnings lists what a coach must know before trusting the sample: the
// team's newest match is old, its five changed between matches, or the rounds are few.
func DataWarnings(in DataWarningInput) []DataWarning {
	out := []DataWarning{}
	if !in.Newest.IsZero() && !in.Now.IsZero() {
		if days := int(dayOf(in.Now) - dayOf(in.Newest)); days > DataStaleDays {
			at := in.Newest
			out = append(out, DataWarning{Kind: DataWarningStale, Days: days, LastPlayedAt: &at})
		}
	}
	if changed := lineupChanges(in.Compared, in.Filter.Lineup); changed > 0 {
		out = append(out, DataWarning{Kind: DataWarningLineup, Changed: changed, Matches: len(in.Compared)})
	}
	rounds := 0
	for _, m := range in.Kept {
		rounds += len(in.Filter.rounds(m))
	}
	// No rounds at all is the empty sample the views already say, not a small one.
	if rounds > 0 && rounds < DataSmallSampleRounds {
		out = append(out, DataWarning{Kind: DataWarningSmallSample, Rounds: rounds})
	}
	return out
}

// matchFive is the team's five who played the most rounds of the match, sorted by SteamID64.
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

// lineupChanges counts the matches whose five hold a player outside the chosen
// lineup or, without one, differ from the five the team fielded most (the newest on a tie).
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
