package library

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

// DefaultMapPool is the pool the simulator starts from. The user can edit it,
// because Valve rotates the active duty pool.
var DefaultMapPool = []string{"de_ancient", "de_anubis", "de_dust2", "de_inferno", "de_mirage", "de_nuke", "de_train"}

// Veto model. Both constants travel in the response so the UI shows the rule.
const (
	// VetoPriorRounds is the weight, in rounds, of the 50 % prior that every
	// map's round win rate is pulled toward, so one lucky match does not decide a veto.
	VetoPriorRounds = 24
	// VetoHistoryMin is how many known vetoes of the opponent are needed before
	// its own habits, not the map edge, predict its bans and picks.
	VetoHistoryMin = 2
	// Pool size limits of the simulator.
	MinVetoPool = 2
)

// Bases of a veto step: why the simulator chose that map.
const (
	VetoBasisChosen   = "chosen"
	VetoBasisEdge     = "edge"
	VetoBasisHistory  = "history"
	VetoBasisLeftover = "leftover"
)

// VetoRequest is a veto to simulate. Our team is slot a and the opponent slot
// b; First is the slot that acts first. Done lists the maps already taken, in
// step order; the simulator recommends the rest.
type VetoRequest struct {
	Format string
	Pool   []string
	First  string
	Done   []string
	Ours   []MapRecord
	Theirs []MapRecord
}

// VetoModel is the rule the simulator applied.
type VetoModel struct {
	PriorRounds int `json:"prior_rounds"`
	HistoryMin  int `json:"history_min"`
}

// VetoMap is one map of the pool with both teams' numbers. Edge is our
// smoothed round win rate minus theirs; a positive edge favours us.
type VetoMap struct {
	Map            string  `json:"map"`
	OurMatches     int     `json:"our_matches"`
	OurRoundsWon   int     `json:"our_rounds_won"`
	OurRounds      int     `json:"our_rounds"`
	OurRate        float64 `json:"our_rate"`
	TheirMatches   int     `json:"their_matches"`
	TheirRoundsWon int     `json:"their_rounds_won"`
	TheirRounds    int     `json:"their_rounds"`
	TheirRate      float64 `json:"their_rate"`
	Edge           float64 `json:"edge"`
	TheirBans      int     `json:"their_bans"`
	TheirPicks     int     `json:"their_picks"`
}

// VetoPlanStep is one step of the simulated veto.
type VetoPlanStep struct {
	Index  int    `json:"index"`
	Team   string `json:"team"`
	Action string `json:"action"`
	Map    string `json:"map"`
	Basis  string `json:"basis"`
}

// VetoPlan is the full simulated veto: the steps given and the ones recommended.
type VetoPlan struct {
	Format string         `json:"format"`
	First  string         `json:"first"`
	Pool   []string       `json:"pool"`
	Model  VetoModel      `json:"model"`
	Maps   []VetoMap      `json:"maps"`
	Steps  []VetoPlanStep `json:"steps"`
	// Played lists the maps the series would be played on, in order.
	Played []string `json:"played"`
	// OpponentHistory is how many known bans and picks of the opponent the plan read.
	OpponentHistory int `json:"opponent_history"`
}

// VetoSequence is the step order of a format over a pool: two leading bans,
// the picks, the remaining bans and the decider, teams alternating from first.
func VetoSequence(format string, pool int, first string) ([]VetoPlanStep, error) {
	picks := map[string]int{FormatBO1: 0, FormatBO3: 2, FormatBO5: 4}
	n, ok := picks[format]
	if !ok {
		return nil, errors.New("format must be bo1, bo3 or bo5")
	}
	if first != SeriesTeamA && first != SeriesTeamB {
		return nil, errors.New("first must be a or b")
	}
	if pool < max(MinVetoPool, n+1) || pool > MaxVetoSteps {
		return nil, fmt.Errorf("a %s needs a pool of %d to %d maps", format, max(MinVetoPool, n+1), MaxVetoSteps)
	}
	bans := pool - n - 1
	lead := min(2, bans)
	actions := make([]string, 0, pool)
	for range lead {
		actions = append(actions, VetoBan)
	}
	for range n {
		actions = append(actions, VetoPick)
	}
	for range bans - lead {
		actions = append(actions, VetoBan)
	}
	other := SeriesTeamB
	if first == SeriesTeamB {
		other = SeriesTeamA
	}
	steps := make([]VetoPlanStep, 0, pool)
	for i, action := range actions {
		team := first
		if i%2 == 1 {
			team = other
		}
		steps = append(steps, VetoPlanStep{Index: i + 1, Team: team, Action: action})
	}
	return append(steps, VetoPlanStep{Index: pool, Action: VetoDecider}), nil
}

// smoothedRate pulls a round win rate toward 50 % by VetoPriorRounds.
func smoothedRate(won, rounds int) float64 {
	return (float64(won) + VetoPriorRounds*0.5) / (float64(rounds) + VetoPriorRounds)
}

func round4(v float64) float64 { return math.Round(v*1e4) / 1e4 }

func vetoMaps(pool []string, ours, theirs []MapRecord) []VetoMap {
	index := func(records []MapRecord) map[string]MapRecord {
		out := map[string]MapRecord{}
		for _, r := range records {
			out[r.Map] = r
		}
		return out
	}
	o, t := index(ours), index(theirs)
	out := make([]VetoMap, 0, len(pool))
	for _, name := range pool {
		a, b := o[name], t[name]
		m := VetoMap{
			Map:        name,
			OurMatches: a.Matches, OurRoundsWon: a.RoundsWon, OurRounds: a.RoundsWon + a.RoundsLost,
			TheirMatches: b.Matches, TheirRoundsWon: b.RoundsWon, TheirRounds: b.RoundsWon + b.RoundsLost,
			TheirBans: b.Veto.Banned.Count, TheirPicks: b.Veto.Picked.Count,
		}
		m.OurRate = round4(smoothedRate(m.OurRoundsWon, m.OurRounds))
		m.TheirRate = round4(smoothedRate(m.TheirRoundsWon, m.TheirRounds))
		m.Edge = round4(m.OurRate - m.TheirRate)
		out = append(out, m)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Edge != out[j].Edge {
			return out[i].Edge > out[j].Edge
		}
		return out[i].Map < out[j].Map
	})
	return out
}

// canonicalMapName lowercases a map name and gives a bare name the defusal
// prefix, so a veto step "mirage" and a demo map "de_mirage" are one map.
func canonicalMapName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name != "" && !strings.Contains(name, "_") {
		return "de_" + name
	}
	return name
}

// cleanPool lowercases, dedupes and sorts the pool, so the plan never depends
// on the order the maps were given in.
func cleanPool(pool []string) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, m := range pool {
		m = canonicalMapName(m)
		if !playMapPattern.MatchString(m) {
			return nil, errors.New("pool maps must match ^[a-z0-9_]+$")
		}
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	sort.Strings(out)
	return out, nil
}

// SimulateVeto plays the veto out. Given steps keep their map; every other
// step takes the map the rule recommends. Our bans drop the map with the
// lowest edge and our picks take the highest. The opponent mirrors that,
// unless it has at least VetoHistoryMin known vetoes: then it bans or picks
// what it banned or picked most, the edge breaking ties. Ties left over go to
// the map name, so the plan is deterministic.
func SimulateVeto(req VetoRequest) (VetoPlan, error) {
	pool, err := cleanPool(req.Pool)
	if err != nil {
		return VetoPlan{}, err
	}
	steps, err := VetoSequence(req.Format, len(pool), req.First)
	if err != nil {
		return VetoPlan{}, err
	}
	if len(req.Done) > len(steps)-1 {
		return VetoPlan{}, fmt.Errorf("at most %d steps can be given; the decider is what is left", len(steps)-1)
	}
	maps := vetoMaps(pool, req.Ours, req.Theirs)
	history := 0
	for _, m := range maps {
		history += m.TheirBans + m.TheirPicks
	}
	left := map[string]bool{}
	for _, m := range pool {
		left[m] = true
	}
	plan := VetoPlan{
		Format: req.Format, First: req.First, Pool: pool, Maps: maps, Played: []string{},
		Model:           VetoModel{PriorRounds: VetoPriorRounds, HistoryMin: VetoHistoryMin},
		OpponentHistory: history,
	}
	for i, step := range steps {
		switch {
		case i < len(req.Done):
			name := canonicalMapName(req.Done[i])
			if !left[name] {
				return VetoPlan{}, fmt.Errorf("step %d: %q is not left in the pool", i+1, req.Done[i])
			}
			step.Map, step.Basis = name, VetoBasisChosen
		case step.Action == VetoDecider:
			for _, m := range pool {
				if left[m] {
					step.Map = m
				}
			}
			step.Basis = VetoBasisLeftover
		default:
			step.Map, step.Basis = recommend(step, maps, left, history >= VetoHistoryMin)
		}
		left[step.Map] = false
		if step.Action != VetoBan {
			plan.Played = append(plan.Played, step.Map)
		}
		plan.Steps = append(plan.Steps, step)
	}
	return plan, nil
}

// recommend picks the map of one open step. maps is sorted by edge, highest first.
func recommend(step VetoPlanStep, maps []VetoMap, left map[string]bool, useHistory bool) (string, string) {
	open := make([]VetoMap, 0, len(maps))
	for _, m := range maps {
		if left[m.Map] {
			open = append(open, m)
		}
	}
	// wantHigh: the step takes the open map with the highest edge for us.
	wantHigh := (step.Team == SeriesTeamA) == (step.Action == VetoPick)
	if step.Team == SeriesTeamB && useHistory {
		habit := func(m VetoMap) int {
			if step.Action == VetoPick {
				return m.TheirPicks
			}
			return m.TheirBans
		}
		best := -1
		for _, m := range open {
			best = max(best, habit(m))
		}
		if best > 0 {
			for _, m := range orderByEdge(open, wantHigh) {
				if habit(m) == best {
					return m.Map, VetoBasisHistory
				}
			}
		}
	}
	return orderByEdge(open, wantHigh)[0].Map, VetoBasisEdge
}

func orderByEdge(maps []VetoMap, high bool) []VetoMap {
	out := append([]VetoMap{}, maps...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Edge != out[j].Edge {
			if high {
				return out[i].Edge > out[j].Edge
			}
			return out[i].Edge < out[j].Edge
		}
		return out[i].Map < out[j].Map
	})
	return out
}
