package recording

import (
	"fmt"
	"maps"
	"slices"

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
// segments, then one replay window per segment that has a VictimReplay, then a
// segment marked AfterReplays. HLAE numbers its takes in this order.
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
	var out, tail []RecordingSegment
	for _, segment := range p.Segments {
		if segment.ReplayOnly {
			continue
		} else if segment.AfterReplays {
			tail = append(tail, segment)
		} else {
			out = append(out, segment)
		}
	}
	out = append(out, replays...)
	return append(out, tail...)
}

// MarkReplayOnly records only the victim replay of the given segments, whose
// main clips a stored capture already holds.
func (p *RecordingPlan) MarkReplayOnly(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	marked := *p
	marked.Segments = slices.Clone(p.Segments)
	for i := range marked.Segments {
		if want[marked.Segments[i].ID] {
			marked.Segments[i].ReplayOnly = true
			delete(want, marked.Segments[i].ID)
		}
	}
	if len(want) > 0 {
		return fmt.Errorf("replay-only segments are not in the plan: %v", slices.Sorted(maps.Keys(want)))
	}
	if err := marked.Validate(); err != nil {
		return err
	}
	*p = marked
	return nil
}

func (p RecordingPlan) validateVictimReplays(ids map[string]bool) error {
	for i, segment := range p.Segments {
		if segment.POVSteamID64 != "" {
			return fmt.Errorf("segment %s pov_steamid64 is reserved for replay windows", segment.ID)
		}
		if segment.AfterReplays && (i != len(p.Segments)-1 || segment.VictimReplay != nil) {
			return fmt.Errorf("segment %s after_replays needs to be the last play, without a replay", segment.ID)
		}
		replay := segment.VictimReplay
		if replay == nil {
			if segment.ReplayOnly {
				return fmt.Errorf("segment %s replay_only needs a victim_replay", segment.ID)
			}
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
		if len(replayKills(segment, *replay)) == 0 {
			return fmt.Errorf("segment %s victim_replay names no kill of the segment", segment.ID)
		}
	}
	return nil
}
