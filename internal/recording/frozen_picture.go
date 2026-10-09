package recording

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"sync"
)

const (
	// FrozenPictureMinSeconds is the shortest repeated-picture run reported as a
	// freeze. Calibration against real captures: docs/incidents.md.
	FrozenPictureMinSeconds = 0.25
	// Largest per-pixel luma change the encoder leaves between two frames of one
	// unchanged picture; real low-motion gameplay always exceeds it somewhere.
	frozenPictureLumaTolerance = 8
	// Frame times are whole microseconds, so an exact run can read a hair short.
	frozenPictureRoundingSeconds = 0.001
)

// blackframe logs one line per marked frame; last_keyframe is the index of the
// latest keyframe it saw, marked or not.
var blackframeLine = regexp.MustCompile(`frame:(\d+) pblack:\d+ pts:\S+ t:(-?[\d.]+) type:\S+ last_keyframe:(-?\d+)`)

// FrozenRun is a stretch of video whose frames repeat the picture before them.
type FrozenRun struct {
	StartSeconds float64
	Seconds      float64
	Frames       int
}

// DetectFrozenRuns decodes the video and returns every run of repeated pictures
// that lasts at least FrozenPictureMinSeconds.
func DetectFrozenRuns(ctx context.Context, ffmpegPath, path string) ([]FrozenRun, error) {
	// blackframe marks a difference frame only when every luma pixel is within
	// the tolerance, so one small moving element keeps a frame out of a run.
	filter := fmt.Sprintf("extractplanes=y,tblend=all_mode=difference,blackframe=amount=100:threshold=%d", frozenPictureLumaTolerance+1)
	// #nosec G204 -- ffmpegPath is configured locally and path is passed as an argument.
	cmd := exec.CommandContext(ctx, ffmpegPath, "-hide_banner", "-nostats", "-nostdin", "-v", "info",
		"-i", path, "-map", "0:v:0", "-vf", filter, "-f", "null", "-")
	var log bytes.Buffer
	cmd.Stderr = &log
	if err := cmd.Run(); err != nil {
		lines := bytes.Split(bytes.TrimSpace(log.Bytes()), []byte("\n"))
		return nil, fmt.Errorf("ffmpeg frame difference: %w: %s", err, bytes.TrimSpace(lines[len(lines)-1]))
	}
	return frozenRuns(log.Bytes())
}

// frozenRuns groups the marked frames into runs. A keyframe re-encodes a still
// picture past the tolerance, so a single unmarked keyframe does not end a run.
func frozenRuns(log []byte) ([]FrozenRun, error) {
	var runs []FrozenRun
	var firstIndex, lastIndex int64
	var first, last float64
	open := false
	flush := func() {
		frames := lastIndex - firstIndex + 1
		if open && frames > 1 {
			seconds := (last - first) / float64(frames-1) * float64(frames)
			if seconds+frozenPictureRoundingSeconds >= FrozenPictureMinSeconds {
				runs = append(runs, FrozenRun{StartSeconds: first, Seconds: seconds, Frames: int(frames)})
			}
		}
		open = false
	}
	for _, match := range blackframeLine.FindAllSubmatch(log, -1) {
		index, indexErr := strconv.ParseInt(string(match[1]), 10, 64)
		seconds, secondsErr := strconv.ParseFloat(string(match[2]), 64)
		keyframe, keyframeErr := strconv.ParseInt(string(match[3]), 10, 64)
		if indexErr != nil || secondsErr != nil || keyframeErr != nil {
			return nil, fmt.Errorf("parse frame difference output %q", match[0])
		}
		acrossKeyframe := index == lastIndex+2 && keyframe == lastIndex+1
		if open && index != lastIndex+1 && !acrossKeyframe {
			flush()
		}
		if !open {
			firstIndex, first, open = index, seconds, true
		}
		lastIndex, last = index, seconds
	}
	flush()
	return runs, nil
}

// FrozenPictureWarnings checks every usable segment clip and returns one
// message per frozen run or failed check. The caller decides whether to fail.
func FrozenPictureWarnings(ctx context.Context, ffmpegPath string, artifacts []RecordingArtifact) []string {
	var clips []RecordingArtifact
	for _, artifact := range artifacts {
		if isUsableSegmentClip(artifact) {
			clips = append(clips, artifact)
		}
	}
	if len(clips) == 0 {
		return nil
	}
	if ffmpegPath == "" {
		return []string{"frozen picture check skipped: ffmpeg not found"}
	}
	perClip := make([][]string, len(clips))
	limit := make(chan struct{}, maxConcurrentArtifactProbes)
	var workers sync.WaitGroup
	for i, clip := range clips {
		workers.Go(func() {
			limit <- struct{}{}
			defer func() { <-limit }()
			perClip[i] = frozenClipWarnings(ctx, ffmpegPath, clip)
		})
	}
	workers.Wait()
	return slices.Concat(perClip...)
}

func frozenClipWarnings(ctx context.Context, ffmpegPath string, clip RecordingArtifact) []string {
	runs, err := DetectFrozenRuns(ctx, ffmpegPath, clip.Path)
	if err != nil {
		return []string{fmt.Sprintf("segment %s frozen picture check failed: %v", clip.SegmentID, err)}
	}
	warnings := make([]string, 0, len(runs))
	for _, run := range runs {
		warnings = append(warnings, fmt.Sprintf(
			"segment %s picture frozen for %.3fs from %.3fs (%d repeated frames); the capture stopped delivering new frames",
			clip.SegmentID, run.Seconds, run.StartSeconds, run.Frames))
	}
	return warnings
}
