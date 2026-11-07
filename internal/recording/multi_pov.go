package recording

import (
	"fmt"
	"slices"

	"github.com/rechedev9/cliphub/internal/killplan"
)

// observedPOV keeps a segment's own POV only when it differs from the plan
// target, so the anchor's segments stay byte-identical to a single-POV plan.
func observedPOV(s killplan.Segment, target killplan.Target) *killplan.Target {
	if s.POV == nil || s.POV.SteamID64 == target.SteamID64 {
		return nil
	}
	pov := *s.POV
	return &pov
}

// ObservedSteamID is the SteamID a plan segment is filmed from.
func (p RecordingPlan) ObservedSteamID(s RecordingSegment) string {
	if s.Observe != nil {
		return s.Observe.SteamID64
	}
	return p.TargetSteamID64
}

// ObservedName is the in-demo name of the player a plan segment films.
func (p RecordingPlan) ObservedName(s RecordingSegment) string {
	if s.Observe != nil {
		return s.Observe.NameInDemo
	}
	return p.TargetNameInDemo
}

// IsMultiPOV reports whether the plan films more than one player.
func (p RecordingPlan) IsMultiPOV() bool {
	for _, s := range p.Segments {
		if s.Observe != nil {
			return true
		}
	}
	return false
}

// povSwitchTicks is the playback a POV change needs between two windows:
// spec_player settles within a few frames, so half a second is ample.
func povSwitchTicks(tickrate int) int {
	return max(1, tickrate/2)
}

// ClashesWith reports whether s, a segment of an earlier capture, cannot sit
// beside p's segments in one plan: windows overlap, or a POV switch has no time.
func (p RecordingPlan) ClashesWith(s RecordingSegment) bool {
	if p.FullDemo != nil {
		return false
	}
	return slices.ContainsFunc(p.Segments, func(own RecordingSegment) bool {
		gap := 0
		if p.ObservedSteamID(own) != p.ObservedSteamID(s) {
			gap = povSwitchTicks(p.Tickrate)
		}
		return s.TickStart < own.TickEnd+gap && own.TickStart < s.TickEnd+gap
	})
}

// validateObserve checks segment i's POV and, when it changes player, that
// the previous window ends early enough for the camera to switch.
func (p RecordingPlan) validateObserve(i int) error {
	s := p.Segments[i]
	if s.Observe != nil {
		if p.FullDemo != nil {
			return fmt.Errorf("segment %s observe is not allowed in a Full Demo plan", s.ID)
		}
		if _, err := AccountIDFromSteamID64(s.Observe.SteamID64); err != nil || s.Observe.SteamID64 == p.TargetSteamID64 {
			return fmt.Errorf("segment %s observe steamid64 %q is not another player", s.ID, s.Observe.SteamID64)
		}
	}
	if i == 0 {
		return nil
	}
	prev := p.Segments[i-1]
	if p.ObservedSteamID(prev) == p.ObservedSteamID(s) {
		return nil
	}
	if EffectiveRecordStartTick(s, p.Tickrate) < prev.TickEnd+povSwitchTicks(p.Tickrate) {
		return fmt.Errorf("segment %s starts too soon after %s to switch POV", s.ID, prev.ID)
	}
	return nil
}
