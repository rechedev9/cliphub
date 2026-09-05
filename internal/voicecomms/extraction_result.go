package voicecomms

import (
	"context"
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
)

const ExtractorVersion = "team-packet-clock-v2"

const (
	Available        = "available"
	NoPackets        = "no_packets"
	NoTeamPackets    = "no_team_packets"
	UnsupportedCodec = "unsupported_codec"
	Silent           = "silent"
	InvalidTimeline  = "invalid_timeline"
	ExtractionFailed = "failed"
)

type ActivityRange struct {
	StartTick int `json:"start_tick"`
	EndTick   int `json:"end_tick"`
}

type ExtractionResult struct {
	Availability       string          `json:"availability"`
	ExtractorVersion   string          `json:"extractor_version"`
	ClockKind          string          `json:"clock_kind"`
	Index              Index           `json:"index"`
	Report             Report          `json:"report"`
	SelectedPackets    int             `json:"selected_packets"`
	ExcludedPackets    int             `json:"excluded_packets"`
	UnsupportedFormats []string        `json:"unsupported_formats"`
	Activity           []ActivityRange `json:"activity"`
}

// ExtractFileWithContext certifies the packet clock and decoded tracks. An
// unavailable voice source is a result; decoder/storage failures are errors.
func ExtractFileWithContext(ctx context.Context, demoPath, target, dir, ffmpegPath string) (ExtractionResult, error) {
	result, err := extractFile(ctx, demoPath, target, dir, true)
	if err != nil || result.Availability != Available {
		return result, err
	}
	if ffmpegPath == "" {
		result.Availability = ExtractionFailed
		return result, fmt.Errorf("ffmpeg is required to validate extracted voice")
	}
	audible := false
	for _, track := range result.Index.Tracks {
		peak, err := decodedTrackPeak(ctx, ffmpegPath, track.Path)
		if err != nil {
			result.Availability = ExtractionFailed
			return result, fmt.Errorf("decode extracted voice: %w", err)
		}
		if peak > -100 {
			audible = true
		}
	}
	if !audible {
		result.Availability = Silent
	}
	return result, nil
}

var peakPattern = regexp.MustCompile(`Peak level dB:\s+(-?inf|[-+0-9.]+)`)

func decodedTrackPeak(ctx context.Context, ffmpegPath, path string) (float64, error) {
	// #nosec G204 -- configured local FFmpeg receives a generated local track as an argv value.
	cmd := exec.CommandContext(ctx, ffmpegPath, "-nostdin", "-nostats", "-hide_banner", "-protocol_whitelist", "file,pipe", "-i", path, "-vn", "-af", "astats=metadata=0:reset=0", "-f", "null", "-")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("voice sample analysis: %w", err)
	}
	matches := peakPattern.FindAllStringSubmatch(string(output), -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf("voice sample analysis returned no measurements")
	}
	peak := math.Inf(-1)
	for _, match := range matches {
		if match[1] == "-inf" {
			continue
		}
		value, err := strconv.ParseFloat(match[1], 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 1) {
			return 0, fmt.Errorf("voice sample analysis returned invalid peak")
		}
		peak = max(peak, value)
	}
	return peak, nil
}

func classifyExtraction(report Report, packets []Packet, sightings []Sighting) ExtractionResult {
	r := ExtractionResult{Availability: NoPackets, ExtractorVersion: ExtractorVersion, ClockKind: "ingame_tick", Report: report, UnsupportedFormats: []string{}, Activity: []ActivityRange{}}
	if report.Tickrate <= 0 || report.Tickrate > 1024 {
		r.Availability = InvalidTimeline
		return r
	}
	for _, p := range packets {
		if p.Bytes == 0 && len(p.Data) == 0 {
			continue
		}
		if !sameSideAt(sightings, report.Target.SteamID64, strconv.FormatUint(p.XUID, 10), p.Tick) {
			r.ExcludedPackets++
			continue
		}
		r.SelectedPackets++
		if p.Tick <= 0 || int64(p.Tick) > int64(report.Tickrate)*43200 || p.ClockKind != "ingame_tick" {
			r.Availability = InvalidTimeline
			return r
		}
		if p.Format != FormatOpus {
			format := p.Format
			if format == "" {
				format = "unknown"
			}
			if !slices.Contains(r.UnsupportedFormats, format) {
				r.UnsupportedFormats = append(r.UnsupportedFormats, format)
			}
		}
		duration := p.DurationSamples
		if duration == 0 && len(p.Data) > 0 {
			for _, frame := range splitVoiceFrames(p.Data, p.Offsets) {
				duration += opusFrameSamples(frame)
			}
		}
		if p.Format == FormatOpus && duration <= 0 {
			r.Availability = InvalidTimeline
			return r
		}
		r.Activity = append(r.Activity, ActivityRange{p.Tick, p.Tick + max(1, (duration*report.Tickrate+47999)/48000)})
	}
	slices.Sort(r.UnsupportedFormats)
	switch {
	case len(r.UnsupportedFormats) > 0:
		r.Availability = UnsupportedCodec
	case r.SelectedPackets > 0:
		r.Availability = Available
	case r.ExcludedPackets > 0:
		r.Availability = NoTeamPackets
	}
	slices.SortFunc(r.Activity, func(a, b ActivityRange) int { return a.StartTick - b.StartTick })
	merged := make([]ActivityRange, 0, len(r.Activity))
	for _, a := range r.Activity {
		if len(merged) > 0 && a.StartTick <= merged[len(merged)-1].EndTick {
			merged[len(merged)-1].EndTick = max(merged[len(merged)-1].EndTick, a.EndTick)
		} else {
			merged = append(merged, a)
		}
	}
	r.Activity = merged
	return r
}
