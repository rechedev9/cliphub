package composition

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/filecommit"
	"github.com/rechedev9/cliphub/internal/recording"
)

func ComposeConcat(ctx context.Context, ffmpegPath, ffprobePath string, clips []recording.SegmentClip, outputPath, workDir string) error {
	if ffmpegPath == "" {
		return fmt.Errorf("ffmpeg path is required")
	}
	if len(clips) == 0 {
		return fmt.Errorf("at least one clip is required")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o750); err != nil {
		return err
	}
	attemptPath, cleanupAttempt, err := filecommit.Attempt(outputPath)
	if err != nil {
		return fmt.Errorf("create composition attempt: %w", err)
	}
	defer cleanupAttempt()
	if workDir == "" {
		workDir = filepath.Dir(outputPath)
	}
	if err := os.MkdirAll(workDir, 0o750); err != nil {
		return err
	}
	listPath := filepath.Join(workDir, "concat-list.txt")
	if err := os.WriteFile(listPath, []byte(ConcatList(clips)), 0o600); err != nil {
		return err
	}

	// When every clip is a lossless-copy-eligible constant frame rate H.264
	// stream, concat with -c copy re-muxes without an extra re-encode pass. A
	// copy pass can still fail at mux time (for example mismatched audio
	// timebases in a set that passes the metadata checks), so a failed copy
	// retries once with the lossy re-encode path instead of failing the run.
	copyOpt := CopyConcatEligible(clips)
	args := reencodeConcatArgs(listPath, attemptPath)
	if copyOpt {
		args = copyConcatArgs(listPath, attemptPath)
	}
	if err := runConcat(ctx, ffmpegPath, args); err != nil {
		if copyOpt {
			if reErr := runConcat(ctx, ffmpegPath, reencodeConcatArgs(listPath, attemptPath)); reErr != nil {
				return reErr
			}
		} else {
			return err
		}
	}
	if err := filecommit.Commit(attemptPath, outputPath); err != nil {
		return fmt.Errorf("publish composition: %w", err)
	}
	return nil
}

// copyConcatArgs builds the lossless -c copy concat command line used when
// every clip is CopyConcatEligible. The muxer re-times each input; no video or
// audio payload is re-encoded.
func copyConcatArgs(listPath, outputPath string) []string {
	return []string{
		"-f", "concat",
		"-safe", "0",
		"-i", listPath,
		"-c", "copy",
		"-movflags", "+faststart",
		outputPath,
	}
}

// reencodeConcatArgs builds the lossy re-encode concat command line, preserving
// the historical zv-composer behavior for non-eligible sets and as the fallback
// when a copy pass fails at mux time.
func reencodeConcatArgs(listPath, outputPath string) []string {
	return []string{
		"-f", "concat",
		"-safe", "0",
		"-i", listPath,
		"-vf", "fps=60,format=yuv420p",
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "18",
		"-c:a", "aac",
		"-b:a", "192k",
		"-movflags", "+faststart",
		outputPath,
	}
}

// runConcat executes one ffmpeg concat command line and formats any failure
// with the ffmpeg stderr detail.
func runConcat(ctx context.Context, ffmpegPath string, args []string) error {
	// #nosec G204 -- ffmpegPath is configured locally and args are not shell-interpolated.
	cmd := exec.CommandContext(ctx, ffmpegPath, append([]string{"-y", "-v", "error"}, args...)...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("ffmpeg concat: %w: %s", err, msg)
		}
		return fmt.Errorf("ffmpeg concat: %w", err)
	}
	return nil
}

// CopyConcatEligible reports whether every clip is safe to concatenate with a
// lossless -c copy pass: 60fps H.264 at 1920x1080, with probed frame counts
// that agree with their durations (constant frame rate). Any clip with missing
// or mismatched metadata disqualifies the whole group. An empty set and any
// clip with empty metadata both report false so ffmpeg falls back to the
// re-encode path rather than producing a desynced or non-CFR output.
func CopyConcatEligible(clips []recording.SegmentClip) bool {
	if len(clips) == 0 {
		return false
	}
	for _, clip := range clips {
		a := clip.Artifact
		if a.Codec != "h264" || a.Width != 1920 || a.Height != 1080 {
			return false
		}
		fps, ok := parseFrameRate(a.FrameRate)
		if !ok || fps != 60 {
			return false
		}
		if a.FrameCount <= 0 {
			return false
		}
		if math.Abs(float64(a.FrameCount)-a.DurationSeconds*60) > 2 {
			return false
		}
	}
	return true
}

// parseFrameRate parses an ffprobe frame-rate string ("60/1", "30000/1001", or
// a plain "60") into its numeric frames-per-second value. It reports false when
// the value cannot be read.
func parseFrameRate(raw string) (float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	if idx := strings.IndexByte(raw, '/'); idx >= 0 {
		num, err1 := strconv.ParseFloat(strings.TrimSpace(raw[:idx]), 64)
		den, err2 := strconv.ParseFloat(strings.TrimSpace(raw[idx+1:]), 64)
		if err1 != nil || err2 != nil || den == 0 {
			return 0, false
		}
		return num / den, true
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func ConcatList(clips []recording.SegmentClip) string {
	var sb strings.Builder
	for _, clip := range clips {
		sb.WriteString("file '")
		// FFmpeg concat lists always want forward slashes, so normalize
		// backslashes unconditionally. filepath.ToSlash only rewrites on
		// Windows, which left Windows-style paths unconverted (and this test
		// failing) when the pipeline or its tests run on Linux/WSL.
		sb.WriteString(escapeConcatPath(strings.ReplaceAll(clip.Path, "\\", "/")))
		sb.WriteString("'\n")
	}
	return sb.String()
}

func escapeConcatPath(path string) string {
	return strings.ReplaceAll(path, "'", "'\\''")
}

func ValidateFinalArtifact(artifact recording.RecordingArtifact, width, height, fps int, expectedDuration float64) []string {
	var warnings []string
	if artifact.ProbeError != "" {
		warnings = append(warnings, fmt.Sprintf("final output probe failed: %s", artifact.ProbeError))
		return warnings
	}
	if artifact.Path == "" || artifact.SizeBytes == 0 {
		warnings = append(warnings, "final output is missing or empty")
	}
	if artifact.Codec != "h264" {
		warnings = append(warnings, fmt.Sprintf("final output codec = %q, want h264", artifact.Codec))
	}
	if artifact.Width != width || artifact.Height != height {
		warnings = append(warnings, fmt.Sprintf("final output resolution = %dx%d, want %dx%d", artifact.Width, artifact.Height, width, height))
	}
	wantFPS := fmt.Sprintf("%d/1", fps)
	if artifact.FrameRate != "" && artifact.FrameRate != wantFPS {
		warnings = append(warnings, fmt.Sprintf("final output frame_rate = %q, want %s", artifact.FrameRate, wantFPS))
	}
	if expectedDuration > 0 && artifact.DurationSeconds > 0 && math.Abs(artifact.DurationSeconds-expectedDuration) > 0.5 {
		warnings = append(warnings, fmt.Sprintf("final output duration %.3fs differs from segment sum %.3fs", artifact.DurationSeconds, expectedDuration))
	}
	return warnings
}

func ClipDurationSum(clips []recording.SegmentClip) float64 {
	var total float64
	for _, clip := range clips {
		total += clip.DurationSeconds
	}
	return total
}
