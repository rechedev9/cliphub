package faceit

import (
	"cmp"
	"slices"
	"strings"
)

// bestMapMinMatches keeps a single lucky win from naming the best map.
const bestMapMinMatches = 2

// BestRecentMap is the map with the highest win rate among the recent matches
// with a result, played at least bestMapMinMatches times; "" when none qualifies.
// Ties go to the map played more, then to the name.
func BestRecentMap(matches []RecentMatch) string {
	type tally struct {
		name          string
		wins, results int
	}
	byMap := map[string]*tally{}
	for _, match := range matches {
		if match.Stats == nil || (match.Stats.Result != "win" && match.Stats.Result != "loss") {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(match.Stats.Map))
		if name == "" {
			continue
		}
		t := byMap[name]
		if t == nil {
			t = &tally{name: name}
			byMap[name] = t
		}
		t.results++
		if match.Stats.Result == "win" {
			t.wins++
		}
	}
	var ranked []tally
	for _, t := range byMap {
		if t.results >= bestMapMinMatches {
			ranked = append(ranked, *t)
		}
	}
	if len(ranked) == 0 {
		return ""
	}
	slices.SortFunc(ranked, func(a, b tally) int {
		// Compare win rates without division: a.wins/a.results vs b.wins/b.results.
		return cmp.Or(cmp.Compare(b.wins*a.results, a.wins*b.results), cmp.Compare(b.results, a.results), cmp.Compare(a.name, b.name))
	})
	return ranked[0].name
}
