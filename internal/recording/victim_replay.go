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
	// The rewind needs this much playback left after the main pass ends.
	victimReplayRewindHeadroomSeconds = 1
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
	end := min(segment.TickEnd, EffectiveRecordEndTick(segment, plan), kill.Tick+plan.Tickrate/victimReplayTailDivisor)
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

// reachesDemoEnd reports whether segment records so close to the end of the
// demo that playback may stop before a rewind can be issued after it.
func reachesDemoEnd(segment RecordingSegment, plan RecordingPlan) bool {
	headroom := max(1, plan.Tickrate) * victimReplayRewindHeadroomSeconds
	return plan.DemoDurationTicks > 1 && EffectiveRecordEndTick(segment, plan)+headroom >= plan.DemoDurationTicks
}

// replayKills are the segment kills the replay window shows: the victim's death.
func replayKills(segment RecordingSegment, replay VictimReplay) []killplan.Kill {
	var kills []killplan.Kill
	for _, kill := range segment.Kills {
		if kill.Tick == replay.KillTick && kill.Victim.SteamID64 == replay.SteamID64 {
			kills = append(kills, kill)
		}
	}
	return kills
}

// CaptureSegments lists every record window in capture order: the plan
// segments, then one replay window per segment that has a VictimReplay. A last
// segment that reaches the demo end is recorded after the replays instead,
// because the demo stops before the rewind could run. HLAE numbers its takes
// in this order.
func (p RecordingPlan) CaptureSegments() []RecordingSegment {
	var replays []RecordingSegment
	for _, segment := range p.Segments {
		replay := segment.VictimReplay
		if replay == nil {
			continue
		}
		replays = append(replays, RecordingSegment{
			ID:           VictimReplayID(segment.ID),
			Round:        segment.Round,
			TickStart:    replay.TickStart,
			TickEnd:      replay.TickEnd,
			Kills:        replayKills(segment, *replay),
			POVSteamID64: replay.SteamID64,
		})
	}
	mains := p.Segments
	var tail []RecordingSegment
	if n := len(mains); len(replays) > 0 && reachesDemoEnd(mains[n-1], p) {
		mains, tail = mains[:n-1], mains[n-1:]
	}
	out := append([]RecordingSegment(nil), mains...)
	out = append(out, replays...)
	return append(out, tail...)
}

func (p RecordingPlan) validateVictimReplays(ids map[string]bool) error {
	for i, segment := range p.Segments {
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
		if i == len(p.Segments)-1 && reachesDemoEnd(segment, p) {
			return fmt.Errorf("segment %s victim_replay cannot follow a play that reaches the demo end", segment.ID)
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
		if len(replayKills(segment, *replay)) == 0 {
			return fmt.Errorf("segment %s victim_replay names no kill of the segment", segment.ID)
		}
	}
	return nil
}
