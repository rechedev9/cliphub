package editor

import (
	"fmt"

	"github.com/rechedev9/cliphub/internal/recording"
)

// victimReplaySpeed slows the victim POV replay to half speed.
const victimReplaySpeed = 0.5

// victimReplayFlashSeconds is the white flash that marks the cut to the replay.
const victimReplayFlashSeconds = 0.15

// replaySource is a segment whose victim replay may follow it in a Short.
type replaySource struct {
	result  recording.RecordingResult
	segment recording.RecordingSegment
	baseDir string
}

// part is the slowed victim POV part that follows the segment's own part.
// Without a recorded clip it returns a warning instead.
func (s replaySource) part() (ShortPart, string, bool) {
	if s.segment.VictimReplay == nil {
		return ShortPart{}, "", false
	}
	clip, ok := recording.ResolveVictimReplayClip(s.result, s.segment.ID)
	if !ok || clip.DurationSeconds <= 0 {
		return ShortPart{}, fmt.Sprintf("segment %s has no victim replay clip; rendering it without one", s.segment.ID), false
	}
	return ShortPart{
		SegmentID:       clip.SegmentID,
		Input:           resolvePath(s.baseDir, clip.Path),
		SourceArtifact:  clip.Artifact,
		DurationSeconds: clip.DurationSeconds / victimReplaySpeed,
		Speed:           victimReplaySpeed,
	}, "", true
}

// appendTo turns a single-clip Short into the clip followed by its victim
// replay, rendered through the compilation path.
func (s replaySource) appendTo(edit ShortEdit, replay ShortPart) ShortEdit {
	segment, plan := s.segment, s.result.Plan
	main := ShortPart{
		SegmentID:       segment.ID,
		Input:           edit.Input,
		SourceArtifact:  edit.SourceArtifact,
		DurationSeconds: edit.DurationSeconds,
		TickStart:       segment.TickStart,
		TickEnd:         segment.TickEnd,
		// Capture ticks keep the comms aligned with the main part.
		CaptureTickStart: recording.EffectiveRecordStartTick(segment, plan.Tickrate),
		CaptureTickEnd:   recording.EffectiveRecordEndTick(segment, plan),
		Kills:            edit.Kills,
	}
	replay.TimelineStartSeconds = edit.DurationSeconds
	edit.Parts = []ShortPart{main, replay}
	edit.DurationSeconds += replay.DurationSeconds
	edit.Tickrate = plan.Tickrate
	return edit
}

func isSlowMotion(part ShortPart) bool {
	return part.Speed > 0 && part.Speed < 1
}

// slowMotionFilters return the video prefix, video suffix and audio suffix
// that slow a part down; a normal-speed part gets empty strings.
func slowMotionFilters(part ShortPart) (videoIn, videoOut, audio string) {
	if !isSlowMotion(part) {
		return "", "", ""
	}
	return fmt.Sprintf("setpts=PTS/%.3f,", part.Speed),
		fmt.Sprintf(",fade=t=in:st=0:d=%.3f:color=white", victimReplayFlashSeconds),
		fmt.Sprintf(",atempo=%.3f", part.Speed)
}
