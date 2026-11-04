package recording

import (
	"fmt"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/killplan"
)

// VictimReplaySuffix turns a segment ID into the capture ID of its replay clip.
const VictimReplaySuffix = "-victim"

const (
	victimReplayLeadSeconds = 3
	// The tail shows the hit landing before the death camera takes over.
	victimReplayTailDivisor = 2
)

// VictimReplayID is the segment ID the replay clip of segmentID records under.
func VictimReplayID(segmentID string) string {
	return segmentID + VictimReplaySuffix
}

// victimReplayFor picks the segment's last kill of a real player and frames
// the seconds before that death, inside the segment's own capture window.
func victimReplayFor(segment RecordingSegment, plan RecordingPlan) *VictimReplay {
	var kill *killplan.Kill
	for i := range segment.Kills {
		candidate := &segment.Kills[i]
		if candidate.Victim.SteamID64 == plan.TargetSteamID64 {
			continue
		}
		if _, err := AccountIDFromSteamID64(candidate.Victim.SteamID64); err != nil {
			continue // Bots have no SteamID to spectate by.
		}
		if kill == nil || candidate.Tick >= kill.Tick {
			kill = candidate
		}
	}
	if kill == nil {
		return nil
	}
	start := max(segment.TickStart, kill.Tick-victimReplayLeadSeconds*plan.Tickrate)
	end := min(EffectiveRecordEndTick(segment, plan), kill.Tick+plan.Tickrate/victimReplayTailDivisor)
	if start <= 0 || end <= kill.Tick || start >= kill.Tick {
		return nil
	}
	return &VictimReplay{
		SteamID64: kill.Victim.SteamID64,
		KillTick:  kill.Tick,
		TickStart: start,
		TickEnd:   end,
	}
}

// CaptureSegments lists every record window in capture order: the plan
// segments, then one replay window per segment that has a VictimReplay. HLAE
// numbers its takes in this order.
func (p RecordingPlan) CaptureSegments() []RecordingSegment {
	out := append([]RecordingSegment(nil), p.Segments...)
	for _, segment := range p.Segments {
		replay := segment.VictimReplay
		if replay == nil {
			continue
		}
		var kills []killplan.Kill
		for _, kill := range segment.Kills {
			if kill.Tick == replay.KillTick && kill.Victim.SteamID64 == replay.SteamID64 {
				kills = append(kills, kill)
			}
		}
		out = append(out, RecordingSegment{
			ID:           VictimReplayID(segment.ID),
			Round:        segment.Round,
			TickStart:    replay.TickStart,
			TickEnd:      replay.TickEnd,
			Kills:        kills,
			POVSteamID64: replay.SteamID64,
		})
	}
	return out
}

func (p RecordingPlan) validateVictimReplays(ids map[string]bool) error {
	for _, segment := range p.Segments {
		if segment.POVSteamID64 != "" {
			return fmt.Errorf("segment %s pov_steamid64 is reserved for replay windows", segment.ID)
		}
		replay := segment.VictimReplay
		if replay == nil {
			continue
		}
		if !p.Stream.VictimReplay || p.FullDemo != nil {
			return fmt.Errorf("segment %s victim_replay requires a Short plan with stream victim_replay", segment.ID)
		}
		id := VictimReplayID(segment.ID)
		if err := artifacts.ValidateArtifactToken("victim replay id", id); err != nil {
			return err
		}
		if ids[id] {
			return fmt.Errorf("victim replay id %q collides with a segment id", id)
		}
		if _, err := AccountIDFromSteamID64(replay.SteamID64); err != nil || replay.SteamID64 == p.TargetSteamID64 {
			return fmt.Errorf("segment %s victim_replay steamid64 %q is not another player", segment.ID, replay.SteamID64)
		}
		if replay.TickStart < segment.TickStart || replay.TickEnd > segment.TickEnd ||
			replay.KillTick <= replay.TickStart || replay.KillTick >= replay.TickEnd {
			return fmt.Errorf("segment %s victim_replay window %d-%d around kill %d is outside the segment", segment.ID, replay.TickStart, replay.TickEnd, replay.KillTick)
		}
		found := false
		for _, kill := range segment.Kills {
			found = found || (kill.Tick == replay.KillTick && kill.Victim.SteamID64 == replay.SteamID64)
		}
		if !found {
			return fmt.Errorf("segment %s victim_replay names no kill of the segment", segment.ID)
		}
	}
	return nil
}
