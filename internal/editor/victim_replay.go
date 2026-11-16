package editor

import (
	"fmt"
	"strings"

	"github.com/rechedev9/cliphub/internal/recording"
)

// victimReplaySpeed is the default speed of the victim POV replay: half speed.
const victimReplaySpeed = 0.5

// victimReplaySpeeds are the speeds a replay may be asked for (renderplan.VictimReplaySpeeds).
var victimReplaySpeeds = []float64{0.25, victimReplaySpeed, 1}

// victimReplayFlashSeconds is the white flash that marks the cut to the replay.
const victimReplayFlashSeconds = 0.15

// replaySource is a segment whose victim replay may follow it in a Short.
type replaySource struct {
	result  recording.RecordingResult
	segment recording.RecordingSegment
	baseDir string
	// speed 0 keeps victimReplaySpeed.
	speed float64
}

// part is the victim POV part, at the chosen speed, that follows the segment's own part.
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
		DurationSeconds: clip.DurationSeconds / s.playbackSpeed(),
		Speed:           s.playbackSpeed(),
	}, "", true
}

func (s replaySource) playbackSpeed() float64 {
	if s.speed > 0 {
		return s.speed
	}
	return victimReplaySpeed
}

// attach appends the replay to a single-clip Short, or adds the warning of a missing clip.
func (s replaySource) attach(edit ShortEdit, warnings []string) (ShortEdit, []string) {
	replay, warning, ok := s.part()
	if ok {
		return s.appendTo(edit, replay), warnings
	}
	if warning != "" {
		warnings = append(warnings, warning)
	}
	return edit, warnings
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

func isVictimReplayPart(part ShortPart) bool {
	return strings.HasSuffix(part.SegmentID, recording.VictimReplaySuffix)
}

// slowMotionFilters return the video prefix, video suffix and audio suffix of
// a victim replay: the white flash at any speed, the slow down below 1x.
func slowMotionFilters(part ShortPart) (videoIn, videoOut, audio string) {
	if isVictimReplayPart(part) {
		videoOut = fmt.Sprintf(",fade=t=in:st=0:d=%.3f:color=white", victimReplayFlashSeconds)
	}
	if !isSlowMotion(part) {
		return "", videoOut, ""
	}
	return fmt.Sprintf("setpts=PTS/%.3f,", part.Speed), videoOut, atempoChain(part.Speed)
}

// atempoChain slows audio by speed; one atempo filter goes no lower than 0.5.
func atempoChain(speed float64) string {
	var b strings.Builder
	for speed < 0.5 {
		b.WriteString(",atempo=0.500")
		speed /= 0.5
	}
	fmt.Fprintf(&b, ",atempo=%.3f", speed)
	return b.String()
}
