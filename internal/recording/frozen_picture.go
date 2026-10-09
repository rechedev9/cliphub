package recording

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
)

const (
	// FrozenPictureMinSeconds is the shortest repeated-picture run reported as a
	// freeze. Calibration against real captures: docs/incidents.md.
	FrozenPictureMinSeconds = 0.25
	// Largest per-pixel luma change the encoder leaves between two frames of one
	// unchanged picture; real low-motion gameplay always exceeds it somewhere.
	frozenPictureLumaTolerance = 8
	// pts_time is printed with six significant digits, so allow one millisecond.
	frozenPictureRoundingSeconds = 0.001
)

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
	filter := fmt.Sprintf(
		"extractplanes=y,tblend=all_mode=difference,blackframe=amount=100:threshold=%d,metadata=mode=print:key=lavfi.blackframe.pblack:file=-",
		frozenPictureLumaTolerance+1,
	)
	// #nosec G204 -- ffmpegPath is configured locally and path is passed as an argument.
	cmd := exec.CommandContext(ctx, ffmpegPath, "-v", "error", "-nostdin", "-i", path, "-map", "0:v:0", "-vf", filter, "-f", "null", "-")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg frame difference: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return frozenRuns(out)
}

// frozenRuns groups the consecutive frame indexes that the metadata filter printed.
func frozenRuns(output []byte) ([]FrozenRun, error) {
	var runs []FrozenRun
	var first, last float64
	frames, previous := 0, int64(-1)
	flush := func() {
		if frames > 1 {
			seconds := (last - first) / float64(frames-1) * float64(frames)
			if seconds+frozenPictureRoundingSeconds >= FrozenPictureMinSeconds {
				runs = append(runs, FrozenRun{StartSeconds: first, Seconds: seconds, Frames: frames})
			}
		}
		frames = 0
	}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 3 || !strings.HasPrefix(fields[0], "frame:") || !strings.HasPrefix(fields[2], "pts_time:") {
			continue
		}
		index, err := strconv.ParseInt(strings.TrimPrefix(fields[0], "frame:"), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse frame difference output %q: %w", scanner.Text(), err)
		}
		seconds, err := strconv.ParseFloat(strings.TrimPrefix(fields[2], "pts_time:"), 64)
		if err != nil {
			return nil, fmt.Errorf("parse frame difference output %q: %w", scanner.Text(), err)
		}
		if frames > 0 && index != previous+1 {
			flush()
		}
		if frames == 0 {
			first = seconds
		}
		last, previous = seconds, index
		frames++
	}
	if err := scanner.Err(); err != nil {
		return nil, err
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
