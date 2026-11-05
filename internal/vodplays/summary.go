package vodplays

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// The summary is a 1080p landscape video, so a 4:3 or vertical source is
// letterboxed rather than cropped and no kill feed is lost.
const (
	summaryWidth        = 1920
	summaryHeight       = 1080
	summaryFadeSeconds  = 0.3
	minSegmentSeconds   = 1
	summaryVideoQuality = "20"
)

type SummaryOptions struct {
	FFmpeg string
	Input  string
	Output string
	// WorkDir holds the per-play segments; it must exist and is not removed.
	WorkDir  string
	Plays    []Play
	HasAudio bool
	// OnProgress receives the share of the summary encoded, from 0 to 1.
	OnProgress func(float64)
}

// RenderSummary writes one landscape video with every play in order. Each
// play is encoded on its own with the same settings and the parts are joined
// without re-encoding, so memory stays flat however many plays there are.
func RenderSummary(ctx context.Context, opts SummaryOptions) error {
	segments := summarySegments(opts.Plays)
	if len(segments) == 0 {
		return errors.New("no plays to summarize")
	}
	total := 0.0
	for _, s := range segments {
		total += s.EndSeconds - s.StartSeconds
	}
	var list strings.Builder
	done := 0.0
	for i, s := range segments {
		part := filepath.Join(opts.WorkDir, fmt.Sprintf("part-%03d.mp4", i+1))
		if err := runFFmpeg(ctx, opts.FFmpeg, segmentArgs(opts, s, part)); err != nil {
			return fmt.Errorf("encode play %s: %w", s.ID, err)
		}
		fmt.Fprintf(&list, "file '%s'\n", filepath.ToSlash(part))
		done += s.EndSeconds - s.StartSeconds
		if opts.OnProgress != nil {
			opts.OnProgress(min(done/total, 0.99))
		}
	}
	listPath := filepath.Join(opts.WorkDir, "parts.txt")
	if err := os.WriteFile(listPath, []byte(list.String()), 0o600); err != nil {
		return err
	}
	return runFFmpeg(ctx, opts.FFmpeg, []string{
		"-hide_banner", "-nostdin", "-loglevel", "error", "-y",
		"-f", "concat", "-safe", "0", "-i", listPath,
		"-c", "copy", "-movflags", "+faststart", opts.Output,
	})
}

// summarySegments keeps the plays long enough to show, in source order.
func summarySegments(plays []Play) []Play {
	var out []Play
	for _, p := range plays {
		if p.EndSeconds-p.StartSeconds >= minSegmentSeconds {
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].StartSeconds < out[b].StartSeconds })
	return out
}

func segmentArgs(opts SummaryOptions, s Play, output string) []string {
	duration := s.EndSeconds - s.StartSeconds
	fadeOut := strconv.FormatFloat(duration-summaryFadeSeconds, 'f', 3, 64)
	fade := strconv.FormatFloat(summaryFadeSeconds, 'f', 3, 64)
	video := fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=decrease:flags=lanczos,pad=%d:%d:-1:-1:color=black,setsar=1,format=yuv420p,fade=t=in:st=0:d=%s,fade=t=out:st=%s:d=%s",
		summaryWidth, summaryHeight, summaryWidth, summaryHeight, fade, fadeOut, fade,
	)
	args := []string{
		"-hide_banner", "-nostdin", "-loglevel", "error", "-y",
		"-ss", strconv.FormatFloat(s.StartSeconds, 'f', 3, 64),
		"-t", strconv.FormatFloat(duration, 'f', 3, 64),
		"-i", opts.Input,
		"-map", "0:v:0", "-vf", video,
		"-c:v", "libx264", "-preset", "veryfast", "-crf", summaryVideoQuality,
		"-video_track_timescale", "90000",
	}
	if opts.HasAudio {
		args = append(args,
			"-map", "0:a:0",
			"-af", fmt.Sprintf("aresample=48000,afade=t=in:st=0:d=%s,afade=t=out:st=%s:d=%s", fade, fadeOut, fade),
			"-c:a", "aac", "-b:a", "160k", "-ac", "2",
		)
	} else {
		args = append(args, "-an")
	}
	return append(args, output)
}

func runFFmpeg(ctx context.Context, ffmpeg string, args []string) error {
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	// #nosec G204 -- ffmpeg path and arguments are built locally; no shell.
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}
