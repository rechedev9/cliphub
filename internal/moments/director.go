package moments

import (
	"fmt"
	"math"
	"sort"

	"github.com/rechedev9/cliphub/internal/killplan"
)

// Auto-POV budget: a three-minute summary by default, and the range a request may ask for.
const (
	DefaultDirectorSeconds  = 180.0
	MinDirectorSeconds      = 15.0
	MaxDirectorSeconds      = 600.0
	MaxDirectorExtraSeconds = 15.0
)

// Observer cue kinds: what a broadcast observer would cut to.
const (
	CueClutch   = "clutch"
	CueDuel     = "duel"
	CueBomb     = "bomb"
	CueMomentum = "momentum"
)

// Cue details.
const (
	CueClutchWon     = "won"
	CueClutchLost    = "lost"
	CueDuelOpening   = "opening"
	CueDuelChained   = "chained"
	CueBombRetake    = "retake"
	CueBombPostPlant = "post_plant"
	CueMomentumSwing = "swing"
	CueComeback      = "comeback"
	CueRun           = "run"
)

// varietyDecay lowers a player's next play each time the director already picked one of theirs.
const varietyDecay = 0.88

// swingCue is the round-win chance a play must add to count as a momentum swing.
const swingCue = 0.3

// streakRounds is how many rounds in a row make a comeback (lost before) or a run (won, this one included).
const streakRounds = 3

// DirectorOptions sets the Auto-POV budget. ExtraSeconds is what each play adds in the edit, such as a victim replay.
type DirectorOptions struct {
	TargetSeconds float64
	ExtraSeconds  float64
}

// Normalize fills the default budget and rejects one outside the allowed range.
func (o DirectorOptions) Normalize() (DirectorOptions, error) {
	if o.TargetSeconds == 0 {
		o.TargetSeconds = DefaultDirectorSeconds
	}
	if math.IsNaN(o.TargetSeconds) || o.TargetSeconds < MinDirectorSeconds || o.TargetSeconds > MaxDirectorSeconds {
		return o, fmt.Errorf("target_seconds must be between %g and %g", MinDirectorSeconds, MaxDirectorSeconds)
	}
	if math.IsNaN(o.ExtraSeconds) || o.ExtraSeconds < 0 || o.ExtraSeconds > MaxDirectorExtraSeconds {
		return o, fmt.Errorf("extra_seconds must be between 0 and %g", MaxDirectorExtraSeconds)
	}
	return o, nil
}

// Cue is one reason the director cut to a play. Count is the clutch size or the rounds of a
// streak; Value the win-chance swing or the bomb-clock tension (0 to 1).
type Cue struct {
	Kind   string  `json:"kind"`
	Detail string  `json:"detail,omitempty"`
	Count  int     `json:"count,omitempty"`
	Value  float64 `json:"value,omitempty"`
}

// DirectedPlay is a picked play: its moment, the observer interest (0 to 1) and the cues behind it.
type DirectedPlay struct {
	Moment
	Interest float64 `json:"interest"`
	Cues     []Cue   `json:"cues,omitempty"`
}

// DirectedPlayer is one filmed player and how much of the summary the director gave them; Plays 0 leaves them out.
type DirectedPlayer struct {
	SteamID64 string  `json:"steamid64"`
	Name      string  `json:"name"`
	Plays     int     `json:"plays"`
	Seconds   float64 `json:"seconds"`
}

// Direction is the Auto-POV pick of a summary, in match order.
type Direction struct {
	TargetSeconds float64          `json:"target_seconds"`
	ExtraSeconds  float64          `json:"extra_seconds,omitempty"`
	TotalSeconds  float64          `json:"total_seconds"`
	SegmentIDs    []string         `json:"segment_ids"`
	Plays         []DirectedPlay   `json:"plays"`
	Players       []DirectedPlayer `json:"players"`
	// RoundResults says whether the plan has round winners, so comebacks and runs could count.
	RoundResults bool `json:"round_results"`
}

type directorCandidate struct {
	play DirectedPlay
	seg  killplan.Segment
	pov  killplan.Target
}

// Direct picks the plays of a summary like a broadcast observer: each kill play is ranked by its score
// plus its cues, then the best are taken across players within the budget, never two POVs at once.
func Direct(plan killplan.Plan, opts DirectorOptions) Direction {
	segs := make(map[string]killplan.Segment, len(plan.Segments))
	for _, s := range plan.Segments {
		segs[s.ID] = s
	}
	rounds := make(map[int]killplan.RoundResult, len(plan.Rounds))
	for _, r := range plan.Rounds {
		rounds[r.Number] = r
	}
	var cands []directorCandidate
	for _, m := range Rank(plan) {
		s, ok := segs[m.SegmentID]
		if !ok || m.Death != nil || m.Events.Kills == 0 || m.DurationSeconds <= 0 {
			continue
		}
		cues, points := observerCues(m, s, rounds)
		cands = append(cands, directorCandidate{play: DirectedPlay{Moment: m, Interest: lift(m.Score, points), Cues: cues}, seg: s, pov: plan.SegmentPOV(s)})
	}
	picked := pickDirected(plan, cands, opts)
	out := Direction{TargetSeconds: opts.TargetSeconds, ExtraSeconds: opts.ExtraSeconds, SegmentIDs: []string{}, Plays: []DirectedPlay{}, RoundResults: len(plan.Rounds) > 0}
	players := directedPlayers(plan)
	total := 0.0
	for _, c := range picked {
		out.SegmentIDs = append(out.SegmentIDs, c.play.SegmentID)
		out.Plays = append(out.Plays, c.play)
		secs := c.play.DurationSeconds + opts.ExtraSeconds
		total += secs
		for i := range players {
			if players[i].SteamID64 == c.pov.SteamID64 {
				players[i].Plays++
				players[i].Seconds = math.Round((players[i].Seconds+secs)*10) / 10
			}
		}
	}
	out.TotalSeconds = math.Round(total*10) / 10
	out.Players = players
	return out
}

// pickDirected takes the most interesting open play until nothing fits; the first always goes in.
func pickDirected(plan killplan.Plan, cands []directorCandidate, opts DirectorOptions) []directorCandidate {
	open := make([]bool, len(cands))
	for i := range open {
		open[i] = true
	}
	var picked []directorCandidate
	perPlayer := map[string]int{}
	used := 0.0
	for {
		best, bestValue := -1, 0.0
		for i, c := range cands {
			if !open[i] {
				continue
			}
			// Both checks only tighten as plays go in, so a closed candidate never reopens.
			if (len(picked) > 0 && used+c.play.DurationSeconds+opts.ExtraSeconds > opts.TargetSeconds+1e-9) || clashesWith(plan, c.seg, picked) {
				open[i] = false
				continue
			}
			value := c.play.Interest * math.Pow(varietyDecay, float64(perPlayer[c.pov.SteamID64]))
			if best < 0 || value > bestValue {
				best, bestValue = i, value
			}
		}
		if best < 0 {
			break
		}
		open[best] = false
		picked = append(picked, cands[best])
		perPlayer[cands[best].pov.SteamID64]++
		used += cands[best].play.DurationSeconds + opts.ExtraSeconds
	}
	sort.Slice(picked, func(i, j int) bool {
		if picked[i].seg.TickStart != picked[j].seg.TickStart {
			return picked[i].seg.TickStart < picked[j].seg.TickStart
		}
		return picked[i].seg.ID < picked[j].seg.ID
	})
	return picked
}

func clashesWith(plan killplan.Plan, s killplan.Segment, picked []directorCandidate) bool {
	for _, p := range picked {
		if plan.POVClash(s, p.seg) {
			return true
		}
	}
	return false
}

// directedPlayers lists the plan's filmed players, the anchor first, then in order of their first play.
func directedPlayers(plan killplan.Plan) []DirectedPlayer {
	out := []DirectedPlayer{{SteamID64: plan.Target.SteamID64, Name: plan.Target.NameInDemo}}
	for _, s := range plan.Segments {
		pov := plan.SegmentPOV(s)
		known := false
		for _, p := range out {
			known = known || p.SteamID64 == pov.SteamID64
		}
		if !known {
			out = append(out, DirectedPlayer{SteamID64: pov.SteamID64, Name: pov.NameInDemo})
		}
	}
	return out
}

// lift adds observer points to a 1-e^-x score in the same point scale, so it stays below 1.
func lift(score, points float64) float64 {
	return math.Round((1-(1-score)*math.Exp(-points))*1000) / 1000
}

// observerCues names what an observer would cut to in a play and the points it adds to its score.
func observerCues(m Moment, s killplan.Segment, rounds map[int]killplan.RoundResult) ([]Cue, float64) {
	var cues []Cue
	points := 0.0
	won := m.ClutchWon != nil && *m.ClutchWon
	if m.ClutchVs >= 2 || (m.ClutchVs == 1 && won) {
		cue := Cue{Kind: CueClutch, Count: m.ClutchVs}
		points += 0.05 * float64(m.ClutchVs)
		if m.ClutchWon != nil {
			cue.Detail = CueClutchLost
			if won {
				cue.Detail = CueClutchWon
				points += 0.08
			}
		}
		cues = append(cues, cue)
	}
	if hasKill(s, func(k killplan.Kill) bool { return k.Entry }) {
		cues = append(cues, Cue{Kind: CueDuel, Detail: CueDuelOpening})
		points += 0.08
	}
	if hasFastReason(m.ReasonCodes) {
		cues = append(cues, Cue{Kind: CueDuel, Detail: CueDuelChained})
		points += 0.04
	}
	if cue, ok := bombCue(s); ok {
		cues = append(cues, cue)
		points += 0.06 + 0.06*cue.Value
		if cue.Detail == CueBombRetake {
			points += 0.04
		}
	}
	if m.WinProbSwing >= swingCue {
		cues = append(cues, Cue{Kind: CueMomentum, Detail: CueMomentumSwing, Value: m.WinProbSwing})
		points += 0.05
	}
	if cue, ok := streakCue(m, rounds); ok {
		cues = append(cues, cue)
		if cue.Detail == CueComeback {
			points += 0.06 + 0.02*float64(min(cue.Count-streakRounds, 3))
		} else {
			points += 0.04
		}
	}
	return cues, points
}

func hasKill(s killplan.Segment, ok func(killplan.Kill) bool) bool {
	for _, k := range s.Kills {
		if ok(k) {
			return true
		}
	}
	return false
}

func hasFastReason(reasons []string) bool {
	for _, r := range reasons {
		if _, ok := fastBonus[r]; ok {
			return true
		}
	}
	return false
}

// bombCue is set when a kill came with the bomb planted, from the kill flag or, on older plans,
// a kill inside the post-plant win-chance spans. Value is how close that chance was to even.
func bombCue(s killplan.Segment) (Cue, bool) {
	planted := hasKill(s, func(k killplan.Kill) bool { return k.PostPlant })
	tension := 0.0
	for _, k := range s.Kills {
		for _, span := range s.WinProb {
			if k.Tick >= span.TickStart && k.Tick <= span.TickEnd {
				planted = true
				tension = max(tension, 1-math.Abs(float64(span.CTPct-50))/50)
			}
		}
	}
	if !planted {
		return Cue{}, false
	}
	cue := Cue{Kind: CueBomb, Detail: CueBombPostPlant, Value: math.Round(tension*100) / 100}
	if hasKill(s, func(k killplan.Kill) bool { return k.Retake }) {
		cue.Detail = CueBombRetake
	}
	return cue, true
}

// streakCue reads the team's rounds before a won one: a comeback after streakRounds or more losses,
// else a run of streakRounds or more wins with this one. An unknown round ends the count.
func streakCue(m Moment, rounds map[int]killplan.RoundResult) (Cue, bool) {
	this, ok := rounds[m.Round]
	side := m.Side
	if side == "" {
		side = this.TargetSide
	}
	if !ok || side == "" || this.Winner != side {
		return Cue{}, false
	}
	n, wonBefore := 0, false
	for r := m.Round - 1; r > 0; r-- {
		prev, ok := rounds[r]
		if !ok || prev.TargetSide == "" {
			break
		}
		teamWon := prev.Winner == prev.TargetSide
		if n == 0 {
			wonBefore = teamWon
		} else if teamWon != wonBefore {
			break
		}
		n++
	}
	if !wonBefore && n >= streakRounds {
		return Cue{Kind: CueMomentum, Detail: CueComeback, Count: n}, true
	}
	if wonBefore && n+1 >= streakRounds {
		return Cue{Kind: CueMomentum, Detail: CueRun, Count: n + 1}, true
	}
	return Cue{}, false
}
