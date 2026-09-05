package parser

import (
	"fmt"
	"sort"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

// Missing player/controller frames are explicit gaps, never a continuation of
// the last known code. Reserve the final sample for a bounded failure marker.
type crosshairTimeline struct {
	samples  []recapplan.CrosshairSample
	overflow bool
}

func (c *crosshairTimeline) record(tick int, code string) {
	if c.overflow || tick < 0 {
		return
	}
	if len(code) > 64 {
		code = ""
	}
	if n := len(c.samples); n > 0 {
		if tick < c.samples[n-1].Tick {
			c.overflow = true
			return
		}
		if c.samples[n-1].Code == code {
			return
		}
	}
	if len(c.samples) >= 4095 {
		code, c.overflow = "", true
	}
	c.samples = append(c.samples, recapplan.CrosshairSample{Tick: tick, Code: code})
}

func (c *Collector) fullDemoFacts(meta PlanMeta, freezeDeaths []TargetDeath, sawMatchStart bool) recapplan.Facts {
	f := recapplan.Facts{SchemaVersion: recapplan.DocumentVersion, DemoSHA256: meta.SHA256, TargetSteamID64: c.target, ClockKind: recapplan.ClockIngame, TickRate: meta.Tickrate, EndTick: meta.DurationTicks, Complete: true, Rounds: []recapplan.RoundFacts{}, Warnings: []recapplan.Notice{}}
	starts, live, ends := indexRoundStarts(c.roundStarts), indexRoundLiveStarts(c.liveStarts), indexRoundEnds(c.roundEnds)
	deaths := indexTargetDeaths(append(append([]TargetDeath{}, c.targetDeaths...), freezeDeaths...))
	numbers := make([]int, 0, len(starts))
	for number := range starts {
		numbers = append(numbers, number)
	}
	sort.Slice(numbers, func(i, j int) bool { return starts[numbers[i]] < starts[numbers[j]] })
	if !sawMatchStart {
		f.Warnings = append(f.Warnings, recapplan.Notice{Code: "match_start_unobserved", Message: "Warmup/knife reset was not observed; source round numbering is preserved"})
	}
	for i, number := range numbers {
		r := recapplan.RoundFacts{ID: fmt.Sprintf("round-%03d", number), Number: number, StartTick: starts[number], FreezeEndTick: live[number], RoundEndTick: ends[number], Kills: []killplan.Kill{}, Utility: []killplan.UtilityThrow{}, Evidence: "round-events"}
		if i+1 < len(numbers) {
			r.NextStartTick = starts[numbers[i+1]]
		}
		if r.FreezeEndTick <= 0 || r.RoundEndTick <= 0 {
			r.Evidence = "incomplete-round-events"
			f.Complete = false
		}
		for _, death := range deaths[number] {
			if death < r.StartTick || (r.RoundEndTick > 0 && death > r.RoundEndTick) {
				continue
			}
			if r.DeathTick == nil || death < *r.DeathTick {
				tick := death
				r.DeathTick = &tick
			}
		}
		for _, k := range c.allKills {
			if k.Round == number {
				r.Kills = append(r.Kills, buildKillPlanKills([]RawKill{k})...)
			}
		}
		for _, u := range c.utility {
			if u.Round == number {
				r.Utility = append(r.Utility, buildUtilityThrow(u))
			}
		}
		f.Rounds = append(f.Rounds, r)
	}
	return f
}
