package vodplays

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// DefaultSamplesPerSecond keeps every notice (on screen for several seconds)
// in at least a few samples while decoding stays the only real cost.
const DefaultSamplesPerSecond = 4.0

type ScanOptions struct {
	FFmpeg           string
	Input            string
	SamplesPerSecond float64
	// StartSeconds and DurationSeconds limit the scan to part of the video;
	// zero scans from the start or to the end.
	StartSeconds    float64
	DurationSeconds float64
}

// Sample is the rings found in the feed corner at one moment of the video.
// Frame is that feed frame, RGB24; it is reused after the callback returns.
type Sample struct {
	Seconds float64
	Rings   []Ring
	Frame   []byte
}

// Scan decodes the video once and calls onSample for every sampled frame, in
// order. FFmpeg crops and scales the feed corner so Go only sees small frames.
func Scan(ctx context.Context, opts ScanOptions, onSample func(Sample)) error {
	rate := opts.SamplesPerSecond
	if rate <= 0 {
		rate = DefaultSamplesPerSecond
	}
	args := scanArgs(opts, rate)
	ffmpeg := opts.FFmpeg
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	// #nosec G204 -- ffmpeg path and input are local configuration; no shell.
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("open ffmpeg pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ffmpeg: %w", err)
	}
	readErr := readFrames(bufio.NewReaderSize(stdout, 1<<20), rate, opts.StartSeconds, onSample)
	_, _ = io.Copy(io.Discard, stdout)
	if err := cmd.Wait(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("ffmpeg scan of %s: %w: %s", opts.Input, err, msg)
		}
		return fmt.Errorf("ffmpeg scan of %s: %w", opts.Input, err)
	}
	return readErr
}

func scanArgs(opts ScanOptions, rate float64) []string {
	filter := fmt.Sprintf("fps=%g,crop=iw*%g:ih*%g:iw*%g:0,scale=%d:%d:flags=neighbor,format=rgb24",
		rate, feedSpan, feedDepth, feedLeft, FeedWidth, FeedHeight)
	// No -hwaccel: copying every GPU frame back costs more than decoding on the
	// CPU, which ran a 1080p60 VOD about four times faster.
	args := []string{"-hide_banner", "-nostdin", "-v", "error"}
	if opts.StartSeconds > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", opts.StartSeconds))
	}
	if opts.DurationSeconds > 0 {
		args = append(args, "-t", fmt.Sprintf("%.3f", opts.DurationSeconds))
	}
	return append(args,
		"-i", opts.Input,
		"-map", "0:v:0", "-an", "-sn",
		"-vf", filter,
		"-f", "rawvideo", "pipe:1",
	)
}

func readFrames(r io.Reader, rate, offset float64, onSample func(Sample)) error {
	frame := make([]byte, FeedWidth*FeedHeight*3)
	for index := 0; ; index++ {
		if _, err := io.ReadFull(r, frame); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil
			}
			return fmt.Errorf("read feed frame: %w", err)
		}
		onSample(Sample{Seconds: offset + float64(index)/rate, Rings: DetectRings(frame), Frame: frame})
	}
}
