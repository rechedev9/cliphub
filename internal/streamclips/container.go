package streamclips

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// ErrSourceNotPlayable marks a source that is not MP4 and could not be remuxed to MP4.
var ErrSourceNotPlayable = errors.New("stream source is not an MP4 Studio can play")

// NeedsRemux reports a container the editor's <video> cannot open, such as the
// MPEG-TS that yt-dlp leaves without FFmpeg or an FLV. MP4 and WebM/Matroska play
// as they are, and an unknown container is left alone.
func (p SourceProbe) NeedsRemux() bool {
	names := strings.Split(p.Container, ",")
	return p.Container != "" && !slices.Contains(names, "mp4") && !slices.Contains(names, "webm")
}

// RemuxOptions configures RemuxToMP4.
type RemuxOptions struct {
	FFmpeg string
	Input  string
	// Output receives the MP4; empty rewrites Input in place.
	Output string
	// Run executes FFmpeg; nil runs it directly.
	Run func(ctx context.Context, exe string, args ...string) ([]byte, error)
}

// RemuxToMP4 copies the video and audio streams into an MP4 without re-encoding.
func RemuxToMP4(ctx context.Context, opts RemuxOptions) error {
	if opts.FFmpeg == "" {
		return fmt.Errorf("%w: ffmpeg is not configured", ErrSourceNotPlayable)
	}
	output := opts.Output
	if output == "" {
		output = opts.Input + ".remux.mp4"
	}
	run := opts.Run
	if run == nil {
		run = runCombined
	}
	// Twitch's MPEG-TS carries an ID3 data stream that MP4 cannot hold, so only video and audio are mapped.
	_, err := run(ctx, opts.FFmpeg,
		"-hide_banner", "-nostdin", "-loglevel", "error", "-y",
		"-i", opts.Input,
		"-map", "0:v", "-map", "0:a?",
		"-c", "copy",
		"-movflags", "+faststart",
		output,
	)
	if err != nil {
		_ = os.Remove(output)
		if ctx.Err() != nil {
			return fmt.Errorf("remux: %w", ctx.Err())
		}
		return fmt.Errorf("%w: remux: %w", ErrSourceNotPlayable, err)
	}
	if opts.Output != "" {
		return nil
	}
	if err := os.Rename(output, opts.Input); err != nil {
		_ = os.Remove(output)
		return fmt.Errorf("replace stream source with remuxed MP4: %w", err)
	}
	return nil
}

func runCombined(ctx context.Context, exe string, args ...string) ([]byte, error) {
	// #nosec G204 -- ffmpeg path is local config and args are not shell-expanded.
	out, err := exec.CommandContext(ctx, exe, args...).CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("%s: %w: %s", exe, err, strings.TrimSpace(string(out)))
	}
	return out, nil
}
