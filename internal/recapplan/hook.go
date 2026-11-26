package recapplan

import (
	"fmt"

	"github.com/rechedev9/cliphub/internal/killplan"
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

func (t TimelineItem) ShowsTick(tick, rate int) bool {
	if tick < t.SourceStartTick {
		return false
	}
	frame, err := TickFrames(tick-t.SourceStartTick, rate)
	return err == nil && frame >= t.SourceOffsetFrames && frame < t.SourceOffsetFrames+t.EndFrame-t.StartFrame
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
	var segments []killplan.Segment
	var hooks []Hook
	for _, r := range rounds {
		if min(r.LiveEndTick+1, r.RequestedEndTick)-r.RequestedStartTick < HookMinSeconds*rate {
			continue
		}
		for _, play := range roundPlays(r, rate) {
			segments = append(segments, play.segment(fmt.Sprintf("%06d", len(hooks)), r, rate))
			hooks = append(hooks, Hook{RoundID: r.ID, FirstKillTick: play.first(), LastKillTick: play.last(), Kills: len(play.Kills)})
		}
	}
	ranked := rankSegments(segments, rate)
	if len(ranked) == 0 {
		return Hook{}, false
	}
	for i, segment := range segments {
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
