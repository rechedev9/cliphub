// Package teamvoice is the team voice of a match: speech found in a recording,
// its sync with the demo, talk time by round phase and local transcription.
package teamvoice

import (
	"bufio"
	"cmp"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"slices"

	"github.com/rechedev9/cliphub/internal/library"
)

// SampleRate is the PCM rate speech detection and transcription read: mono s16le.
const SampleRate = 16000

const (
	frameMS      = 20
	frameSamples = SampleRate * frameMS / 1000
	// A speech run survives a pause this short and must last this long.
	bridgeMS  = 300
	minRunMS  = 160
	padMS     = 80
	floorDB   = -55.0
	ceilingDB = -20.0
	marginDB  = 12.0
)

// DetectSpeech reads 16 kHz mono s16le PCM and returns where someone talks, and the length read.
// The threshold sits a margin over the recording's own noise floor, so a hissy TS export still splits.
func DetectSpeech(r io.Reader) ([]library.VoiceSpan, int64, error) {
	levels, err := frameLevels(r)
	if err != nil {
		return nil, 0, err
	}
	durationMS := int64(len(levels)) * frameMS
	if len(levels) == 0 {
		return []library.VoiceSpan{}, 0, nil
	}
	threshold := speechThreshold(levels)
	var spans []library.VoiceSpan
	start := -1
	for i, db := range levels {
		switch {
		case db > threshold && start < 0:
			start = i
		case db <= threshold && start >= 0:
			spans = append(spans, library.VoiceSpan{StartMS: int64(start) * frameMS, EndMS: int64(i) * frameMS})
			start = -1
		}
	}
	if start >= 0 {
		spans = append(spans, library.VoiceSpan{StartMS: int64(start) * frameMS, EndMS: durationMS})
	}
	return cleanSpans(spans, durationMS), durationMS, nil
}

// frameLevels is the RMS level of each 20 ms frame in dBFS; a short last frame is dropped.
func frameLevels(r io.Reader) ([]float64, error) {
	br := bufio.NewReaderSize(r, 64<<10)
	buf := make([]byte, frameSamples*2)
	var levels []float64
	for {
		_, err := io.ReadFull(br, buf)
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return levels, nil
		}
		if err != nil {
			return nil, err
		}
		var sum float64
		for i := 0; i < len(buf); i += 2 {
			s := float64(int16(binary.LittleEndian.Uint16(buf[i:]))) / 32768
			sum += s * s
		}
		rms := math.Sqrt(sum / frameSamples)
		levels = append(levels, 20*math.Log10(max(rms, 1e-6)))
	}
}

// speechThreshold is the noise floor (the quietest 15% of frames) plus a margin, kept in a sane band.
func speechThreshold(levels []float64) float64 {
	sorted := slices.Clone(levels)
	slices.Sort(sorted)
	floor := sorted[len(sorted)*15/100]
	return min(max(floor+marginDB, floorDB), ceilingDB)
}

// cleanSpans bridges short pauses, drops blips and pads each run, clamped to the recording.
func cleanSpans(spans []library.VoiceSpan, durationMS int64) []library.VoiceSpan {
	merged := []library.VoiceSpan{}
	for _, s := range spans {
		if n := len(merged); n > 0 && s.StartMS-merged[n-1].EndMS < bridgeMS {
			merged[n-1].EndMS = s.EndMS
			continue
		}
		merged = append(merged, s)
	}
	out := []library.VoiceSpan{}
	for _, s := range merged {
		if s.EndMS-s.StartMS < minRunMS {
			continue
		}
		s.StartMS, s.EndMS = max(0, s.StartMS-padMS), min(durationMS, s.EndMS+padMS)
		if n := len(out); n > 0 && s.StartMS <= out[n-1].EndMS {
			out[n-1].EndMS = s.EndMS
			continue
		}
		out = append(out, s)
	}
	return out
}

// UnionSpans merges the speech of several tracks into one sorted list.
func UnionSpans(tracks ...[]library.VoiceSpan) []library.VoiceSpan {
	var all []library.VoiceSpan
	for _, t := range tracks {
		all = append(all, t...)
	}
	slices.SortFunc(all, func(a, b library.VoiceSpan) int { return cmp.Compare(a.StartMS, b.StartMS) })
	out := []library.VoiceSpan{}
	for _, s := range all {
		if n := len(out); n > 0 && s.StartMS <= out[n-1].EndMS {
			out[n-1].EndMS = max(out[n-1].EndMS, s.EndMS)
			continue
		}
		out = append(out, s)
	}
	return out
}

// SpeechMS is the summed length of spans.
func SpeechMS(spans []library.VoiceSpan) int64 {
	var total int64
	for _, s := range spans {
		total += s.EndMS - s.StartMS
	}
	return total
}
