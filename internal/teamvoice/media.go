package teamvoice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/library"
)

// Media runs the FFmpeg Studio ships over voice recordings.
type Media struct {
	FFmpeg  string
	FFprobe string
}

// ErrNoFFmpeg is a Studio started without FFmpeg, which voice import needs.
var ErrNoFFmpeg = errors.New("teamvoice: FFmpeg is not configured")

// Probe is the first audio stream of a file.
type Probe struct {
	Channels   int
	DurationMS int64
	Codec      string
}

// ErrNotAudio is a file FFmpeg cannot read an audio stream from.
var ErrNotAudio = errors.New("teamvoice: the file has no readable audio")

func (m Media) ffprobe() string {
	if m.FFprobe != "" {
		return m.FFprobe
	}
	if m.FFmpeg == "" {
		return ""
	}
	name := "ffprobe"
	if strings.HasSuffix(strings.ToLower(m.FFmpeg), ".exe") {
		name += ".exe"
	}
	return filepath.Join(filepath.Dir(m.FFmpeg), name)
}

// Probe reads the channels, length and codec of a file's first audio stream.
func (m Media) Probe(ctx context.Context, path string) (Probe, error) {
	probe := m.ffprobe()
	if probe == "" {
		return Probe{}, ErrNoFFmpeg
	}
	// #nosec G204 -- the configured ffprobe receives a stored upload path as an argv value.
	out, err := exec.CommandContext(ctx, probe, "-v", "error", "-protocol_whitelist", localProtocols, "-select_streams", "a:0",
		"-show_entries", "stream=channels,codec_name,duration:format=duration", "-of", "json", path).Output()
	if err != nil {
		return Probe{}, fmt.Errorf("%w: %v", ErrNotAudio, err)
	}
	var doc struct {
		Streams []struct {
			Channels int    `json:"channels"`
			Codec    string `json:"codec_name"`
			Duration string `json:"duration"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &doc); err != nil || len(doc.Streams) == 0 {
		return Probe{}, ErrNotAudio
	}
	s := doc.Streams[0]
	seconds, err := strconv.ParseFloat(s.Duration, 64)
	if err != nil {
		seconds, err = strconv.ParseFloat(doc.Format.Duration, 64)
	}
	if err != nil || seconds <= 0 || math.IsInf(seconds, 0) || s.Channels < 1 {
		return Probe{}, ErrNotAudio
	}
	return Probe{Channels: s.Channels, DurationMS: int64(seconds * 1000), Codec: s.Codec}, nil
}

// localProtocols keeps FFmpeg on local files and pipes: an upload cannot make it open a URL.
const localProtocols = "file,pipe"

// EncodeJob turns a recording into the mono Opus files the Replay plays: one Dst mixes every
// channel, several take one channel each, in order, from a single decode.
type EncodeJob struct {
	Src  string
	Dsts []string
}

var opusArgs = []string{"-c:a", "libopus", "-b:a", "32k", "-application", "voip", "-f", "ogg"}

// Encode writes a job's tracks as Ogg Opus at a voice bitrate.
func (m Media) Encode(ctx context.Context, job EncodeJob) error {
	if m.FFmpeg == "" {
		return ErrNoFFmpeg
	}
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y", "-protocol_whitelist", localProtocols, "-i", job.Src}
	switch len(job.Dsts) {
	case 0:
		return errors.New("teamvoice: an encode needs an output")
	case 1:
		args = append(append(append(args, "-map", "0:a:0", "-vn", "-ac", "1"), opusArgs...), job.Dsts[0])
		return m.run(ctx, args)
	}
	graph := fmt.Sprintf("[0:a:0]asplit=%d", len(job.Dsts))
	for c := range job.Dsts {
		graph += fmt.Sprintf("[s%d]", c)
	}
	for c := range job.Dsts {
		graph += fmt.Sprintf(";[s%d]pan=mono|c0=c%d[o%d]", c, c, c)
	}
	args = append(args, "-filter_complex", graph)
	for c, dst := range job.Dsts {
		args = append(append(append(args, "-map", fmt.Sprintf("[o%d]", c)), opusArgs...), dst)
	}
	return m.run(ctx, args)
}

// WAV writes the 16 kHz mono WAV whisper.cpp reads.
func (m Media) WAV(ctx context.Context, src, dst string) error {
	if m.FFmpeg == "" {
		return ErrNoFFmpeg
	}
	return m.run(ctx, []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y", "-protocol_whitelist", localProtocols, "-i", src, "-map", "0:a:0",
		"-ac", "1", "-ar", strconv.Itoa(SampleRate), "-c:a", "pcm_s16le", "-f", "wav", dst})
}

// Speech decodes a file to PCM and finds where someone talks.
func (m Media) Speech(ctx context.Context, path string) ([]library.VoiceSpan, int64, error) {
	if m.FFmpeg == "" {
		return nil, 0, ErrNoFFmpeg
	}
	// #nosec G204 -- the configured FFmpeg receives a stored track path as an argv value.
	cmd := exec.CommandContext(ctx, m.FFmpeg, "-nostdin", "-hide_banner", "-loglevel", "error", "-protocol_whitelist", localProtocols, "-i", path, "-map", "0:a:0",
		"-ac", "1", "-ar", strconv.Itoa(SampleRate), "-f", "s16le", "-")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, 0, err
	}
	if err := cmd.Start(); err != nil {
		return nil, 0, err
	}
	spans, duration, readErr := DetectSpeech(stdout)
	if err := cmd.Wait(); err != nil {
		return nil, 0, fmt.Errorf("decode voice: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return spans, duration, readErr
}

func (m Media) run(ctx context.Context, args []string) error {
	// #nosec G204 -- the configured FFmpeg receives generated local paths as argv values.
	cmd := exec.CommandContext(ctx, m.FFmpeg, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
