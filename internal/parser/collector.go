package parser

import (
	"errors"
	"fmt"
	"sort"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

// Collector accumulates the events the parser observes for the target player
// and assembles a kill plan when Build is called.
//
// A Collector is single-use: feed it events via RecordKill /
// RecordRoundEnd / RecordTargetIdentity in chronological order, then call
// Build exactly once.
type Collector struct {
	target string
	rules  rules.Rules

	kills       []RawKill
	utility     []RawUtilityThrow
	roundStarts []RoundStart
	roundEnds   []RoundEnd

	totalKillsTarget    int
	killsAfterFilters   int
	totalUtilityTarget  int
	utilityAfterFilters int
	totalSmokesTarget   int
	smokesAfterFilters  int

	targetName        string
	targetTeamAtStart string
	targetSeen        bool
	identityCaptured  bool
}

// PlanMeta carries the metadata about the source demo that the segmenter
// itself cannot derive (path, hash, map name, tickrate, demo duration).
type PlanMeta struct {
	DemoPath      string
	SHA256        string
	Map           string
	Tickrate      int
	DurationTicks int
}

// NewCollector returns a Collector configured for the given target SteamID64
// and rules.
func NewCollector(target string, r rules.Rules) *Collector {
	return &Collector{target: target, rules: r}
}

// resetForMatchStart discards provisional warmup/knife-round observations.
// Callers deliberately invoke it only when the demo contains MatchStart; demos
// without that event retain the documented collect-from-first-event fallback.
func (c *Collector) resetForMatchStart() {
	c.kills = nil
	c.utility = nil
	c.roundStarts = nil
	c.roundEnds = nil
	c.totalKillsTarget = 0
	c.killsAfterFilters = 0
	c.totalUtilityTarget = 0
	c.utilityAfterFilters = 0
	c.totalSmokesTarget = 0
	c.smokesAfterFilters = 0
	// Keep warmup identity as fallback for matches where the target has no live
	// events, but let the first live observation replace its provisional values.
	c.identityCaptured = false
}

// RecordTargetIdentity captures the in-demo display name and starting team
// of the target player. Called once at the start of the demo (or whenever
// the target is first observed).
func (c *Collector) RecordTargetIdentity(name, teamAtStart string) {
	if c.identityCaptured {
		return
	}
	c.targetName = name
	c.targetTeamAtStart = teamAtStart
	c.targetSeen = true
	c.identityCaptured = true
}

// RecordKill processes one kill attributed to the target. The kill is
// always counted in TotalKillsTarget; it is appended to the segmenter input
// only if it passes the configured filters.
func (c *Collector) RecordKill(k RawKill) {
	c.totalKillsTarget++

	if !c.rules.AllowsWeapon(k.Weapon) {
		return
	}
	if c.rules.IncludeHeadshotOnly && !k.Headshot {
		return
	}
	if !c.rules.AllowsRound(k.Round) {
		return
	}

	c.kills = append(c.kills, k)
	c.killsAfterFilters++
}

func (c *Collector) RecordUtility(u RawUtilityThrow) {
	if !isTrackedUtilityType(u.Type) {
		return
	}
	c.totalUtilityTarget++
	if u.Type == SmokeGrenadeType {
		c.totalSmokesTarget++
	}
	if !c.rules.AllowsRound(u.Round) {
		return
	}
	c.utility = append(c.utility, u)
	c.utilityAfterFilters++
	if u.Type == SmokeGrenadeType {
		c.smokesAfterFilters++
	}
}

func (c *Collector) RecordRoundStart(rs RoundStart) {
	c.roundStarts = append(c.roundStarts, rs)
}

// RecordRoundEnd remembers the tick at which a given round ended; used by
// segmentation to clip a segment's TickEnd if the post-roll would otherwise
// extend past the end of the round.
func (c *Collector) RecordRoundEnd(re RoundEnd) {
	c.roundEnds = append(c.roundEnds, re)
}

// TotalKillsTarget reports the number of kills attributed to the target
// observed in the demo, before filters.
func (c *Collector) TotalKillsTarget() int { return c.totalKillsTarget }

// KillsAfterFilters reports the number of kills that survived the weapon /
// headshot / round filters.
func (c *Collector) KillsAfterFilters() int { return c.killsAfterFilters }

// Build assembles the final kill plan. It returns an error if the target
// player was never observed in the demo.
func (c *Collector) Build(m PlanMeta) (killplan.Plan, error) {
	return c.build(m, SegmentModeKills)
}

func (c *Collector) build(m PlanMeta, mode SegmentMode) (killplan.Plan, error) {
	if !c.targetSeen {
		return killplan.Plan{}, fmt.Errorf("target steamid %q: %w", c.target, ErrTargetNotFound)
	}
	if m.Tickrate <= 0 {
		return killplan.Plan{}, errors.New("tickrate must be > 0")
	}

	sortRawKillsByTick(c.kills)
	sortUtilityThrowsByThrowTick(c.utility)
	for i := range c.utility {
		if c.utility[i].ID == "" {
			c.utility[i].ID = utilityOrdinalID(utilityIDPrefix(c.utility[i].Type), i+1)
		}
	}

	var segs []killplan.Segment
	switch mode {
	case SegmentModeRecap:
		segs = SegmentRecap(c.kills, c.utility, c.roundStarts, c.roundEnds, c.rules, m.Tickrate)
	default:
		segs = Segment(c.kills, c.roundEnds, c.rules, m.Tickrate)
	}
	segs = clampSegmentsToDuration(segs, m.DurationTicks, m.Tickrate)
	if segs == nil {
		segs = []killplan.Segment{}
	}

	plan := killplan.NewPlan()
	plan.Demo = killplan.Demo{
		Path:          m.DemoPath,
		SHA256:        m.SHA256,
		Map:           m.Map,
		Tickrate:      m.Tickrate,
		DurationTicks: m.DurationTicks,
	}
	plan.Target = killplan.Target{
		SteamID64:   c.target,
		NameInDemo:  c.targetName,
		TeamAtStart: c.targetTeamAtStart,
	}
	plan.Rules = c.rules
	plan.Segments = segs
	plan.Stats = killplan.Stats{
		TotalKillsTarget:     c.totalKillsTarget,
		KillsAfterFilters:    c.killsAfterFilters,
		TotalUtilityTarget:   c.totalUtilityTarget,
		UtilityAfterFilters:  c.utilityAfterFilters,
		TotalSmokesTarget:    c.totalSmokesTarget,
		SmokesAfterFilters:   c.smokesAfterFilters,
		SegmentsCreated:      len(segs),
		DurationSecondsTotal: totalSegmentSeconds(segs, m.Tickrate),
	}
	return plan, nil
}

// demoEndSafetyMarginSeconds keeps segment ends away from CS2 demo EOF.
// Recording into the final ticks freezes or corrupts the last frames ("se
// raya"), and a record-end scheduled at DurationTicks often never fires
// because playback stops before that tick, so the runtime cannot shut down
// cleanly.
const demoEndSafetyMarginSeconds = 2

// demoEndShortTailSeconds is the maximum post-event pad kept when a kill or
// utility throw already sits inside the EOF safety zone. Full post-roll into
// the glitch zone is worse than a short clean tail.
const demoEndShortTailSeconds = 1

func demoEndSafetyMarginTicks(tickrate, durationTicks int) int {
	if durationTicks <= 1 {
		return 0
	}
	if tickrate <= 0 {
		tickrate = 64
	}
	margin := demoEndSafetyMarginSeconds * tickrate
	if margin < 1 {
		margin = 1
	}
	// Never reserve more than a quarter of a short demo fixture.
	if maxMargin := durationTicks / 4; maxMargin > 0 && margin > maxMargin {
		margin = maxMargin
	}
	if margin >= durationTicks {
		margin = durationTicks - 1
	}
	return margin
}

func lastSegmentEventTick(segment killplan.Segment) int {
	last := 0
	for _, kill := range segment.Kills {
		if kill.Tick > last {
			last = kill.Tick
		}
	}
	for _, utility := range segment.Utility {
		if utility.ThrowTick > last {
			last = utility.ThrowTick
		}
		if utility.PopTick > last {
			last = utility.PopTick
		}
	}
	return last
}

// clampSegmentsToDuration trims segment ends that would run past the demo, and
// prefers stopping before the EOF glitch zone so capture ends cleanly.
func clampSegmentsToDuration(segments []killplan.Segment, durationTicks, tickrate int) []killplan.Segment {
	if durationTicks <= 0 || len(segments) == 0 {
		return segments
	}
	if tickrate <= 0 {
		tickrate = 64
	}
	margin := demoEndSafetyMarginTicks(tickrate, durationTicks)
	softCap := durationTicks - margin
	if softCap < 1 {
		softCap = durationTicks
	}
	shortTail := demoEndShortTailSeconds * tickrate
	if shortTail < 1 {
		shortTail = 1
	}

	out := segments[:0]
	for _, segment := range segments {
		if segment.TickStart >= durationTicks {
			continue
		}
		end := segment.TickEnd
		if end > softCap {
			end = softCap
		}
		lastEvent := lastSegmentEventTick(segment)
		// Soft margin may sit before the last selected event. Keep a short clean
		// tail after that event instead of full post-roll into EOF junk.
		if lastEvent > 0 && end < lastEvent {
			end = lastEvent + shortTail
		}
		// Prefer one tick of headroom so record-end still fires while playback
		// advances, unless the last event is already on the final tick.
		if durationTicks > 1 && end >= durationTicks {
			if lastEvent > 0 && lastEvent < durationTicks-1 {
				end = durationTicks - 1
			} else if lastEvent <= 0 {
				end = durationTicks - 1
			} else {
				end = durationTicks
			}
		}
		if end > durationTicks {
			end = durationTicks
		}
		// Validator requires every kill/throw inside [TickStart, TickEnd].
		if lastEvent > end {
			continue
		}
		segment.TickEnd = end
		if segment.TickEnd <= segment.TickStart {
			continue
		}
		segment.ID = killplan.FormatSegmentID(len(out) + 1)
		out = append(out, segment)
	}
	return out
}

func totalSegmentSeconds(segs []killplan.Segment, tickrate int) float64 {
	total := 0.0
	for _, s := range segs {
		total += float64(s.TickEnd-s.TickStart) / float64(tickrate)
	}
	return total
}

func sortRawKillsByTick(kills []RawKill) {
	if rawKillsSortedByTick(kills) {
		return
	}
	sort.SliceStable(kills, func(i, j int) bool { return kills[i].Tick < kills[j].Tick })
}

func rawKillsSortedByTick(kills []RawKill) bool {
	for i := 1; i < len(kills); i++ {
		if kills[i].Tick < kills[i-1].Tick {
			return false
		}
	}
	return true
}
