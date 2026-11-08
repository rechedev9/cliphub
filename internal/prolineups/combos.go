package prolineups

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// Combo rules: grenades of one team thrown within comboWindow seconds of each
// other in a side round form a combo; it is published when enough rounds and
// teams run it.
const (
	comboWindow    = 15.0
	minComboRounds = 6
	minComboTeams  = 3
	maxComboSize   = 3
	// absorbShare drops a pair when a triple holding it covers this share of its rounds.
	absorbShare = 0.8
)

// Combo is a set of lineups one team throws together.
type Combo struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Side      tacticalplan.Side `json:"side"`
	Lineups   []string          `json:"lineups"`
	Rounds    Counts            `json:"rounds"`
	TeamCount int               `json:"team_count"`
	RoundWin  float64           `json:"round_win"`
	Clock     float64           `json:"clock"`
	Teams     []Usage           `json:"teams"`
	Examples  []Example         `json:"examples"`
}

var kindLabel = map[tacticalplan.EventKind]string{
	tacticalplan.EventSmoke: "Humo", tacticalplan.EventFlash: "Flash", tacticalplan.EventHE: "HE", tacticalplan.EventMolotov: "Molotov",
}

var kindSlug = map[tacticalplan.EventKind]string{
	tacticalplan.EventSmoke: "smoke", tacticalplan.EventFlash: "flash", tacticalplan.EventHE: "he", tacticalplan.EventMolotov: "molotov",
}

// techniqueLabel names the throw the way players say it.
func techniqueLabel(l Lineup) string {
	switch {
	case l.Jump && l.Movement == "running":
		return "run jumpthrow"
	case l.Jump && l.Movement == "crouching":
		return "jumpthrow agachado"
	case l.Jump:
		return "jumpthrow"
	case l.Movement == "running":
		return "corriendo"
	case l.Movement == "crouching":
		return "agachado"
	case l.Movement == "walking":
		return "andando"
	}
	return ""
}

func names(l Lineup) (slug, title string) {
	land, from := orUnknown(l.LandPlace), orUnknown(l.ThrowPlace)
	title = kindLabel[l.Kind] + " a " + land + " desde " + from
	parts := []string{land, "from", from}
	if tech := techniqueLabel(l); tech != "" {
		title += " (" + tech + ")"
		parts = append(parts, tech)
	}
	return kindSlug[l.Kind] + "/" + slugify(strings.Join(parts, " ")), title
}

func orUnknown(place string) string {
	if place == "" {
		return "zona sin nombre"
	}
	return place
}

func slugify(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		switch {
		case unicode.Is(unicode.Mn, r):
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// uniqueNames tells apart lineups that share a title, such as one smoke thrown
// from each spawn. Variants are numbered by id, not by use, and a variant's slug
// ends in a hash of its id, so names and addresses stay put when use shifts.
func uniqueNames(ls []Lineup) {
	groups := map[string][]int{}
	for i := range ls {
		groups[ls[i].Slug] = append(groups[ls[i].Slug], i)
	}
	for _, g := range groups {
		if len(g) < 2 {
			continue
		}
		slices.SortFunc(g, func(a, b int) int { return cmp.Compare(ls[a].ID, ls[b].ID) })
		for n, i := range g[1:] {
			sum := sha256.Sum256([]byte(ls[i].ID))
			ls[i].Slug += "-" + hex.EncodeToString(sum[:3])
			ls[i].Title += " · variante " + strconv.Itoa(n+2)
		}
	}
}

// sideRound is the lineup throws of one team in one side round, in clock order.
type sideRound struct {
	key    roundKey
	team   string
	won    bool
	played time.Time
	throws []Throw
	ids    []string
}

func recentSideRounds(throws []Throw, memberOf map[int]string, recentFrom time.Time) []*sideRound {
	byKey := map[roundKey]*sideRound{}
	order := []*sideRound{}
	for i, t := range throws {
		id, ok := memberOf[i]
		if !ok || t.PlayedAt.Before(recentFrom) {
			continue
		}
		k := roundKey{t.MapKey, t.Round, t.Side}
		r := byKey[k]
		if r == nil {
			r = &sideRound{key: k, team: t.Team, won: t.RoundWon, played: t.PlayedAt}
			byKey[k] = r
			order = append(order, r)
		}
		r.throws = append(r.throws, t)
		r.ids = append(r.ids, id)
	}
	for _, r := range order {
		idx := make([]int, len(r.throws))
		for i := range idx {
			idx[i] = i
		}
		slices.SortFunc(idx, func(a, b int) int { return cmp.Compare(r.throws[a].Clock, r.throws[b].Clock) })
		ts, ids := make([]Throw, len(idx)), make([]string, len(idx))
		for i, j := range idx {
			ts[i], ids[i] = r.throws[j], r.ids[j]
		}
		r.throws, r.ids = ts, ids
	}
	return order
}

// addPartners fills each lineup's With: the lineups its team throws within the
// combo window in the same round, by share of the lineup's rounds.
func addPartners(ls []Lineup, throws []Throw, memberOf map[int]string, recentFrom time.Time) {
	titles := map[string]string{}
	target := map[string]string{}
	for _, l := range ls {
		titles[l.ID] = l.Title
		target[l.ID] = string(l.Kind) + "|" + l.LandPlace
	}
	rounds := map[string]int{}
	pairs := map[string]map[string]int{}
	for _, r := range recentSideRounds(throws, memberOf, recentFrom) {
		seen := map[string]bool{}
		for i, id := range r.ids {
			if !seen[id] {
				seen[id] = true
				rounds[id]++
			}
			for j, other := range r.ids {
				if other == id || absf(r.throws[i].Clock-r.throws[j].Clock) > comboWindow {
					continue
				}
				if pairs[id] == nil {
					pairs[id] = map[string]int{}
				}
				pairs[id][other]++
			}
		}
	}
	for i := range ls {
		out := []Usage{}
		for other, n := range pairs[ls[i].ID] {
			n = min(n, rounds[ls[i].ID])
			out = append(out, Usage{Key: other, Name: titles[other], Count: n, Share: ratio(n, rounds[ls[i].ID])})
		}
		slices.SortFunc(out, func(a, b Usage) int { return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(a.Key, b.Key)) })
		// Variants of one grenade to one place would crowd the list: keep the most paired.
		taken := map[string]bool{}
		kept := []Usage{}
		for _, u := range out {
			if !taken[target[u.Key]] && len(kept) < topEntries {
				taken[target[u.Key]] = true
				kept = append(kept, u)
			}
		}
		ls[i].With = kept
	}
}

func absf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// rankInLanding ranks lineups by recent throws among those of the same kind
// landing at the same callout.
func rankInLanding(ls []Lineup) {
	groups := map[string][]int{}
	for i, l := range ls {
		k := string(l.Kind) + "|" + l.LandPlace
		groups[k] = append(groups[k], i)
	}
	for _, g := range groups {
		slices.SortFunc(g, func(a, b int) int {
			return cmp.Or(cmp.Compare(ls[b].Throws.Recent, ls[a].Throws.Recent), cmp.Compare(ls[b].Throws.All, ls[a].Throws.All), cmp.Compare(ls[a].ID, ls[b].ID))
		})
		for pos, i := range g {
			ls[i].Rank = RankInLanding{Position: pos + 1, Of: len(g)}
		}
	}
}

type comboTally struct {
	ids      []string
	side     tacticalplan.Side
	rounds   int
	won      int
	teams    map[string]int
	clocks   []float64
	examples []Example
}

// buildCombos counts every set of two or three distinct lineups thrown within
// the combo window in one side round, keeps the common ones and drops a pair
// that a published triple already explains.
func buildCombos(ls []Lineup, throws []Throw, memberOf map[int]string, recentFrom time.Time) []Combo {
	titles := map[string]Lineup{}
	for _, l := range ls {
		titles[l.ID] = l
	}
	tallies := map[string]*comboTally{}
	for _, r := range recentSideRounds(throws, memberOf, recentFrom) {
		for _, set := range roundSets(r) {
			key := strings.Join(set, "+")
			c := tallies[key]
			if c == nil {
				c = &comboTally{ids: set, side: r.key.side, teams: map[string]int{}}
				tallies[key] = c
			}
			c.rounds++
			if r.won {
				c.won++
			}
			if r.team != "" {
				c.teams[r.team]++
			}
			c.clocks = append(c.clocks, r.throws[0].Clock)
			if len(c.examples) < examplesPerItem {
				t := r.throws[0]
				c.examples = append(c.examples, Example{Match: t.Match, MapKey: t.MapKey, Event: t.Event, PlayedAt: t.PlayedAt, Team: t.Team, Player: t.Player, SteamID64: t.SteamID64, Round: t.Round, Clock: round2(t.Clock), Tick: t.ThrowTick, AirTime: round2(t.AirTime)})
			}
		}
	}
	kept := map[string]*comboTally{}
	for k, c := range tallies {
		if c.rounds >= minComboRounds && len(c.teams) >= minComboTeams {
			kept[k] = c
		}
	}
	out := []Combo{}
	for k, c := range kept {
		if len(c.ids) == 2 && absorbed(c, kept) {
			continue
		}
		teams := []Usage{}
		for team, n := range c.teams {
			teams = append(teams, Usage{Key: team, Name: team, Count: n, Share: ratio(n, c.rounds)})
		}
		slices.SortFunc(teams, func(a, b Usage) int { return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(a.Key, b.Key)) })
		slices.SortFunc(c.examples, func(a, b Example) int { return b.PlayedAt.Compare(a.PlayedAt) })
		out = append(out, Combo{
			ID: k, Title: comboTitle(c.ids, titles), Side: c.side, Lineups: c.ids, Rounds: Counts{Recent: c.rounds, All: c.rounds},
			TeamCount: len(c.teams), RoundWin: ratio(c.won, c.rounds), Clock: round2(median(c.clocks)), Teams: teams[:min(len(teams), topEntries)], Examples: c.examples,
		})
	}
	slices.SortFunc(out, func(a, b Combo) int {
		return cmp.Or(cmp.Compare(b.Rounds.Recent, a.Rounds.Recent), cmp.Compare(a.ID, b.ID))
	})
	// Variants of the same lineups read as one combo: keep the most run.
	named := map[string]bool{}
	return slices.DeleteFunc(out, func(c Combo) bool {
		key := string(c.Side) + "|" + c.Title
		if named[key] {
			return true
		}
		named[key] = true
		return false
	})
}

// roundSets lists the distinct lineup sets of size 2 and 3 whose throws all
// fall within the combo window of the first one.
func roundSets(r *sideRound) [][]string {
	seen := map[string]bool{}
	out := [][]string{}
	n := len(r.ids)
	var walk func(start int, set []int)
	walk = func(start int, set []int) {
		if len(set) >= 2 {
			ids := []string{}
			for _, i := range set {
				ids = append(ids, r.ids[i])
			}
			slices.Sort(ids)
			ids = slices.Compact(ids)
			if key := strings.Join(ids, "+"); len(ids) == len(set) && !seen[key] {
				seen[key] = true
				out = append(out, ids)
			}
		}
		if len(set) == maxComboSize {
			return
		}
		for i := start; i < n; i++ {
			if len(set) > 0 && r.throws[i].Clock-r.throws[set[0]].Clock > comboWindow {
				break
			}
			walk(i+1, append(slices.Clone(set), i))
		}
	}
	walk(0, nil)
	return out
}

func absorbed(pair *comboTally, kept map[string]*comboTally) bool {
	for _, c := range kept {
		if len(c.ids) == 3 && slices.Contains(c.ids, pair.ids[0]) && slices.Contains(c.ids, pair.ids[1]) && float64(c.rounds) >= absorbShare*float64(pair.rounds) {
			return true
		}
	}
	return false
}

func comboTitle(ids []string, lineups map[string]Lineup) string {
	parts := []string{}
	for _, id := range ids {
		l := lineups[id]
		parts = append(parts, kindLabel[l.Kind]+" "+orUnknown(l.LandPlace))
	}
	return strings.Join(parts, " + ")
}
