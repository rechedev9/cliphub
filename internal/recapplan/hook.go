package recapplan

import (
	"fmt"
	"slices"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/moments"
)

const (
	HookReason         = "hook"
	WarnHookSkipped    = "hook_skipped"
	HookLeadSeconds    = 3.0
	HookTailSeconds    = 1.5
	HookMinSeconds     = 8
	HookMaxSeconds     = 20
	HookPlayGapSeconds = 8
)

type HookOptions struct {
	Enabled bool `json:"enabled"`
}

type Hook struct {
	RoundID       string `json:"round_id"`
	FirstKillTick int    `json:"first_kill_tick"`
	LastKillTick  int    `json:"last_kill_tick"`
	Kills         int    `json:"kills"`
}

func canonicalHook(h *HookOptions) *HookOptions {
	if h == nil || !h.Enabled {
		return nil
	}
	return &HookOptions{Enabled: true}
}

func (t TimelineItem) IsHook() bool {
	return t.Role == "round" && t.Reason == HookReason
}

func (d *Document) planHook() {
	d.Hook = nil
	if d.Options.Hook == nil || !d.Options.Hook.Enabled {
		return
	}
	if len(d.Rounds) < 2 {
		d.Warnings = append(d.Warnings, Notice{Code: WarnHookSkipped, Message: "El vídeo tiene una sola ronda, así que sale sin gancho inicial."})
		return
	}
	hook, ok := bestHook(d.Rounds, d.Clock.TickRate)
	if !ok {
		d.Warnings = append(d.Warnings, Notice{Code: WarnHookSkipped, Message: "El jugador no tiene ninguna baja que enseñar en el gancho inicial, así que el vídeo empieza en la primera ronda."})
		return
	}
	d.Hook = &hook
}

func bestHook(rounds []Round, rate int) (Hook, bool) {
	plan := killplan.Plan{Demo: killplan.Demo{Tickrate: rate}}
	var hooks []Hook
	lead, tail, gap := secondsTicks(HookLeadSeconds, rate), secondsTicks(HookTailSeconds, rate), HookPlayGapSeconds*rate
	for _, r := range rounds {
		if min(r.LiveEndTick+1, r.RequestedEndTick)-r.RequestedStartTick < HookMinSeconds*rate {
			continue
		}
		kills := slices.Clone(r.Kills)
		slices.SortStableFunc(kills, func(a, b killplan.Kill) int { return a.Tick - b.Tick })
		var play []killplan.Kill
		flush := func() {
			if len(play) == 0 {
				return
			}
			first, last := play[0].Tick, play[len(play)-1].Tick
			plan.Segments = append(plan.Segments, killplan.Segment{ID: fmt.Sprintf("%06d", len(hooks)), Round: r.Number, TickStart: first - lead, TickEnd: last + tail, Kills: play})
			hooks = append(hooks, Hook{RoundID: r.ID, FirstKillTick: first, LastKillTick: last, Kills: len(play)})
			play = nil
		}
		for _, kill := range kills {
			if kill.Tick < r.RequestedStartTick || kill.Tick > r.LiveEndTick {
				continue
			}
			if len(play) > 0 && kill.Tick-play[len(play)-1].Tick > gap {
				flush()
			}
			play = append(play, kill)
		}
		flush()
	}
	ranked := moments.Rank(plan)
	if len(ranked) == 0 {
		return Hook{}, false
	}
	for i, segment := range plan.Segments {
		if segment.ID == ranked[0].SegmentID {
			return hooks[i], true
		}
	}
	return Hook{}, false
}

func (h Hook) window(r Round, rate int) (int, int) {
	shortest, longest := HookMinSeconds*rate, HookMaxSeconds*rate
	end := min(h.LastKillTick+secondsTicks(HookTailSeconds, rate), r.EffectiveEndTick)
	start := max(h.FirstKillTick-secondsTicks(HookLeadSeconds, rate), r.RequestedStartTick)
	if end-start < shortest {
		start = max(r.RequestedStartTick, end-shortest)
	}
	if end-start < shortest {
		end = min(r.EffectiveEndTick, start+shortest)
	}
	if end-start > longest {
		start = end - longest
	}
	return start, end
}

func (d *Document) hookItem(start int64) (TimelineItem, bool, error) {
	if d.Hook == nil {
		return TimelineItem{}, false, nil
	}
	for _, r := range d.Rounds {
		if r.ID != d.Hook.RoundID {
			continue
		}
		from, to := d.Hook.window(r, d.Clock.TickRate)
		total, err := TickFrames(r.EffectiveEndTick-r.RequestedStartTick, d.Clock.TickRate)
		if err != nil {
			return TimelineItem{}, false, err
		}
		offset, err := TickFrames(from-r.RequestedStartTick, d.Clock.TickRate)
		if err != nil {
			return TimelineItem{}, false, err
		}
		last, err := TickFrames(to-r.RequestedStartTick, d.Clock.TickRate)
		if err != nil {
			return TimelineItem{}, false, err
		}
		last = min(last, total)
		if last <= offset {
			return TimelineItem{}, false, fmt.Errorf("opening hook has no frames in %s", r.ID)
		}
		return TimelineItem{Role: "round", SourceRef: r.ID, SourceStartTick: r.RequestedStartTick, SourceEndTick: r.EffectiveEndTick, SourceOffsetFrames: offset, StartFrame: start, EndFrame: start + last - offset, Reason: HookReason}, true, nil
	}
	return TimelineItem{}, false, fmt.Errorf("opening hook references a round outside the video: %s", d.Hook.RoundID)
}

func (d Document) validateHook() error {
	h := d.Hook
	if h == nil {
		return nil
	}
	if d.Options.Hook == nil || !d.Options.Hook.Enabled {
		return fmt.Errorf("opening hook is not enabled in the options")
	}
	for _, r := range d.Rounds {
		if r.ID != h.RoundID {
			continue
		}
		if h.Kills < 1 || h.FirstKillTick < r.RequestedStartTick || h.LastKillTick < h.FirstKillTick || h.LastKillTick > r.LiveEndTick {
			return fmt.Errorf("opening hook lies outside the certified play of %s", r.ID)
		}
		return nil
	}
	return fmt.Errorf("opening hook references a round outside the video: %s", h.RoundID)
}
