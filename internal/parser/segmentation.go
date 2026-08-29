package parser

import (
	"sort"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

// RawKill is the normalized representation of a kill produced by the demo
// reader and consumed by the segmentation logic. It is intentionally
// independent of any demoinfocs types so the segmenter can be tested in
// isolation with synthetic data.
type RawKill struct {
	Tick      int
	Round     int
	Weapon    string
	Headshot  bool
	Wallbang  bool
	Killer    killplan.Player
	Victim    killplan.Player
	KillerPos [3]float64
	VictimPos [3]float64
}

// RoundEnd marks the tick at which a given round ended. Segmentation uses
// these to clip a segment's TickEnd if the post-roll would otherwise extend
// past the end of the round.
type RoundEnd struct {
	Round int
	Tick  int
}

// RoundStart marks the tick at which a given round began (CS2 freeze start).
type RoundStart struct {
	Round int
	Tick  int
}

// RoundLiveStart is freeze-end: the first live tick after buy time.
type RoundLiveStart struct {
	Round int
	Tick  int
}

// TargetDeath marks the last live tick for the selected player's POV in a
// round. A full-demo segment cannot legitimately continue on that POV after it.
type TargetDeath struct {
	Round int
	Tick  int
}

// Segment groups a chronologically ordered list of kills into recording
// segments according to the supplied rules and tickrate. The input slice
// must be sorted by Tick ascending.
//
// Segments produced by this function are not yet attached to a kill plan;
// the demo metadata, target identity, and stats are filled in by the parser.
func Segment(kills []RawKill, roundEnds []RoundEnd, r rules.Rules, tickrate int) []killplan.Segment {
	if len(kills) == 0 || tickrate <= 0 {
		return nil
	}

	windowTicks := r.WindowSeconds * tickrate
	preRollTicks := r.PreRollSeconds * tickrate
	postRollTicks := r.PostRollSeconds * tickrate
	roundEndByRound := indexRoundEnds(roundEnds)

	out := make([]killplan.Segment, 0, countKillSegmentGroups(kills, windowTicks, r.MinKillsInWindow))
	groupStart := 0
	for i := 1; i <= len(kills); i++ {
		if i < len(kills) && kills[i].Tick-kills[i-1].Tick <= windowTicks {
			continue
		}

		g := kills[groupStart:i]
		groupStart = i
		if len(g) < r.MinKillsInWindow {
			continue
		}

		first := g[0]
		last := g[len(g)-1]
		tickStart := first.Tick - preRollTicks
		if tickStart < 0 {
			tickStart = 0
		}
		tickEnd := last.Tick + postRollTicks
		// Clip the post-roll at the end of the round where the segment actually
		// ends (last kill's round). For the common single-round group this is the
		// same as the first kill's round; for a group that spans a round boundary
		// it prevents the clip from bleeding into the following round.
		if endTick, ok := roundEndForRound(roundEndByRound, last.Round); ok && endTick < tickEnd && endTick >= last.Tick {
			tickEnd = endTick
		}

		seg := killplan.Segment{
			ID:        killplan.FormatSegmentID(len(out) + 1),
			Round:     first.Round,
			TickStart: tickStart,
			TickEnd:   tickEnd,
			Kills:     buildKillPlanKills(g),
		}
		out = append(out, seg)
	}
	return out
}

// SegmentRecap records each live round from freeze-end to round end so a
// landscape POV recap skips buy time and hard-cuts between rounds.
// Rounds without target kills are kept when live bounds are known.
func SegmentRecap(kills []RawKill, utility []RawUtilityThrow, roundStarts []RoundStart, liveStarts []RoundLiveStart, roundEnds []RoundEnd, targetDeaths []TargetDeath, r rules.Rules, tickrate int) []killplan.Segment {
	if tickrate <= 0 {
		return nil
	}

	preRollTicks := r.PreRollSeconds * tickrate
	postRollTicks := r.PostRollSeconds * tickrate
	startByRound := indexRoundStarts(roundStarts)
	liveByRound := indexRoundLiveStarts(liveStarts)
	endByRound := indexRoundEnds(roundEnds)
	deathByRound := indexTargetDeaths(targetDeaths)
	killsByRound := map[int][]RawKill{}
	for _, kill := range kills {
		killsByRound[kill.Round] = append(killsByRound[kill.Round], kill)
	}
	utilityByRound := map[int][]killplan.UtilityThrow{}
	for _, u := range utility {
		if !r.AllowsRound(u.Round) || !isTrackedUtilityType(u.Type) {
			continue
		}
		utilityByRound[u.Round] = append(utilityByRound[u.Round], buildUtilityThrow(u))
	}

	rounds := recapRounds(killsByRound, utilityByRound, startByRound, liveByRound, endByRound)
	if len(rounds) == 0 {
		return nil
	}

	out := make([]killplan.Segment, 0, len(rounds))
	previousEnd := 0
	for _, round := range rounds {
		if !r.AllowsRound(round) {
			continue
		}
		g := killsByRound[round]
		liveStart := recapLiveStart(round, g, utilityByRound[round], liveByRound, startByRound, previousEnd)
		tickStart, tickEnd := recapRoundWindow(round, g, utilityByRound[round], liveStart, endByRound)
		liveEnd := 0
		hasLiveDeath := false
		if end, ok := endByRound[round]; ok {
			liveEnd = end
		}
		if death, ok := firstTargetDeathInWindow(deathByRound[round], liveStart, liveEnd); ok {
			liveEnd = death
			hasLiveDeath = true
		}
		if liveEnd > 0 && (tickEnd <= liveStart || liveEnd < tickEnd) {
			tickEnd = liveEnd
		}
		tickStart, tickEnd = expandRecapWindowForUtility(tickStart, tickEnd, utilityByRound[round], preRollTicks, postRollTicks, liveEnd)
		if liveStart > 0 && tickStart < liveStart {
			tickStart = liveStart
		}
		// Recording rejects TickEnd <= TickStart; kill-only fallbacks get 1s, not a Shorts post-roll.
		if tickEnd <= tickStart {
			if hasLiveDeath {
				continue
			}
			if len(g) == 0 {
				continue
			}
			tickEnd = g[len(g)-1].Tick + tickrate
			if tickEnd <= tickStart {
				tickEnd = tickStart + 1
			}
		}
		out = append(out, killplan.Segment{
			ID:        killplan.FormatSegmentID(len(out) + 1),
			Round:     round,
			TickStart: tickStart,
			TickEnd:   tickEnd,
			Kills:     killsInRecapWindow(buildKillPlanKills(g), tickStart, tickEnd),
			Utility:   utilityInRecapWindow(utilityByRound[round], tickStart, tickEnd),
		})
		previousEnd = tickEnd
	}
	return out
}

// IntroFreezeSeconds is the native freeze prefix kept on the first live
// recap round so the FACEIT roster overlay can sit on buy time.
const IntroFreezeSeconds = 8

// WithIntroFreeze pulls the first recap window back through at most 8s of
// freeze. Later rounds stay freeze-end to round-end. Utility already listed
// on the segment is left untouched so a buy-time nade cannot appear as a
// new capture abort.
func WithIntroFreeze(segs []killplan.Segment, roundStarts []RoundStart, tickrate int) []killplan.Segment {
	if len(segs) == 0 || tickrate <= 0 {
		return segs
	}
	startByRound := indexRoundStarts(roundStarts)
	roundStart, ok := startByRound[segs[0].Round]
	if !ok || roundStart <= 0 || roundStart >= segs[0].TickStart {
		return segs
	}
	prefix := IntroFreezeSeconds * tickrate
	want := segs[0].TickStart - prefix
	if want < roundStart {
		want = roundStart
	}
	if want < 1 {
		want = 1
	}
	if want >= segs[0].TickStart {
		return segs
	}
	out := append([]killplan.Segment(nil), segs...)
	out[0].TickStart = want
	return out
}

func killsInRecapWindow(kills []killplan.Kill, start, end int) []killplan.Kill {
	if len(kills) == 0 {
		return kills
	}
	var out []killplan.Kill
	for _, kill := range kills {
		if kill.Tick >= start && kill.Tick <= end {
			out = append(out, kill)
		}
	}
	return out
}

func utilityInRecapWindow(utility []killplan.UtilityThrow, start, end int) []killplan.UtilityThrow {
	if len(utility) == 0 {
		return utility
	}
	var out []killplan.UtilityThrow
	for _, throw := range utility {
		if throw.ThrowTick >= start && throw.ThrowTick <= end {
			out = append(out, throw)
		}
	}
	return out
}

func recapRounds(killsByRound map[int][]RawKill, utilityByRound map[int][]killplan.UtilityThrow, startByRound, liveByRound, endByRound map[int]int) []int {
	seen := map[int]struct{}{}
	add := func(round int) {
		if round <= 0 {
			return
		}
		seen[round] = struct{}{}
	}
	for round := range killsByRound {
		add(round)
	}
	for round := range utilityByRound {
		add(round)
	}
	for round := range startByRound {
		add(round)
	}
	for round := range liveByRound {
		add(round)
	}
	for round := range endByRound {
		add(round)
	}
	out := make([]int, 0, len(seen))
	for round := range seen {
		out = append(out, round)
	}
	sort.Ints(out)
	return out
}

func expandRecapWindowForUtility(tickStart, tickEnd int, utility []killplan.UtilityThrow, preRollTicks, postRollTicks, roundEnd int) (int, int) {
	for _, u := range utility {
		start := u.ThrowTick - preRollTicks
		if start < 1 {
			start = 1
		}
		if start < tickStart {
			tickStart = start
		}
		end := u.ThrowTick + postRollTicks
		if u.PopTick > 0 {
			end = u.PopTick + postRollTicks
		}
		if end > tickEnd {
			tickEnd = end
		}
	}
	if roundEnd > 0 && tickEnd > roundEnd {
		tickEnd = roundEnd
	}
	return tickStart, tickEnd
}

func recapLiveStart(round int, kills []RawKill, utility []killplan.UtilityThrow, liveByRound, startByRound map[int]int, previousEnd int) int {
	if tick, ok := liveByRound[round]; ok && tick > 0 {
		return tick
	}
	if len(kills) > 0 {
		return kills[0].Tick
	}
	earliest := 0
	for _, u := range utility {
		if u.ThrowTick > 0 && (earliest == 0 || u.ThrowTick < earliest) {
			earliest = u.ThrowTick
		}
	}
	if earliest > 0 {
		return earliest
	}
	if start, ok := startByRound[round]; ok && start > 0 {
		return start
	}
	if previousEnd > 0 {
		return previousEnd + 1
	}
	return 1
}

func recapRoundWindow(round int, kills []RawKill, utility []killplan.UtilityThrow, liveStart int, endByRound map[int]int) (int, int) {
	tickStart := liveStart
	if tickStart < 1 {
		tickStart = 1
	}

	tickEnd := 0
	if end, ok := endByRound[round]; ok && end > 0 {
		tickEnd = end
	}
	if tickEnd <= 0 {
		if len(kills) > 0 {
			tickEnd = kills[len(kills)-1].Tick
		}
		for _, u := range utility {
			uEnd := u.ThrowTick
			if u.PopTick > uEnd {
				uEnd = u.PopTick
			}
			if uEnd > tickEnd {
				tickEnd = uEnd
			}
		}
	}
	if tickEnd < tickStart {
		return tickStart, tickStart
	}
	return tickStart, tickEnd
}

func indexRoundLiveStarts(liveStarts []RoundLiveStart) map[int]int {
	if len(liveStarts) == 0 {
		return nil
	}
	out := make(map[int]int, len(liveStarts))
	for _, rs := range liveStarts {
		if _, ok := out[rs.Round]; !ok {
			out[rs.Round] = rs.Tick
		}
	}
	return out
}

func indexRoundStarts(roundStarts []RoundStart) map[int]int {
	if len(roundStarts) == 0 {
		return nil
	}
	out := make(map[int]int, len(roundStarts))
	for _, rs := range roundStarts {
		if _, ok := out[rs.Round]; !ok {
			out[rs.Round] = rs.Tick
		}
	}
	return out
}

func countKillSegmentGroups(kills []RawKill, windowTicks, minKills int) int {
	count := 0
	groupStart := 0
	for i := 1; i <= len(kills); i++ {
		if i < len(kills) && kills[i].Tick-kills[i-1].Tick <= windowTicks {
			continue
		}
		if i-groupStart >= minKills {
			count++
		}
		groupStart = i
	}
	return count
}

func indexRoundEnds(roundEnds []RoundEnd) map[int]int {
	if len(roundEnds) == 0 {
		return nil
	}
	out := make(map[int]int, len(roundEnds))
	for _, re := range roundEnds {
		if _, ok := out[re.Round]; !ok {
			out[re.Round] = re.Tick
		}
	}
	return out
}

func indexTargetDeaths(deaths []TargetDeath) map[int][]int {
	if len(deaths) == 0 {
		return nil
	}
	out := make(map[int][]int, len(deaths))
	for _, death := range deaths {
		if death.Round > 0 && death.Tick > 0 {
			out[death.Round] = append(out[death.Round], death.Tick)
		}
	}
	return out
}

func firstTargetDeathInWindow(deaths []int, liveStart, liveEnd int) (int, bool) {
	first := 0
	for _, death := range deaths {
		if death < liveStart || (liveEnd > 0 && death >= liveEnd) {
			continue
		}
		if first == 0 || death < first {
			first = death
		}
	}
	return first, first > 0
}

func roundEndForRound(roundEndByRound map[int]int, round int) (int, bool) {
	if len(roundEndByRound) == 0 {
		return 0, false
	}
	tick, ok := roundEndByRound[round]
	return tick, ok
}

func buildKillPlanKills(in []RawKill) []killplan.Kill {
	out := make([]killplan.Kill, len(in))
	for i, k := range in {
		out[i] = killplan.Kill{
			Tick:      k.Tick,
			Weapon:    k.Weapon,
			Headshot:  k.Headshot,
			Wallbang:  k.Wallbang,
			Killer:    k.Killer,
			Victim:    k.Victim,
			KillerPos: k.KillerPos,
			VictimPos: k.VictimPos,
		}
	}
	return out
}
