package vodplays

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"sort"
	"strings"
)

// A voice peak is the streamer raising their voice over their own usual level
// for a moment: a reaction, a laugh or a shout meant to mark a clip.
const (
	// VoiceWindowSeconds is the length of one loudness measurement.
	VoiceWindowSeconds = 0.5
	voiceSampleRate    = 8000
	// DefaultVoiceSensitivity sits in the middle of 1 (only big shouts) to 5.
	DefaultVoiceSensitivity = 3
	MinVoiceSensitivity     = 1
	MaxVoiceSensitivity     = 5
	// SilenceFloorDB keeps digital silence out of the baseline.
	SilenceFloorDB = -60.0
	// minVoicedWindows is how much non-silent audio a baseline needs.
	minVoicedWindows = 20
	// A peak lasts at least minPeakWindows, unless one window is far above the threshold.
	minPeakWindows    = 2
	singleWindowBoost = 6.0
	// Peaks closer than this are one moment.
	voiceMergeGapSeconds = 4.0
	// markedExcessDB is how far over the baseline a deliberate shout goes,
	// whatever the sensitivity; such a moment always stays in the list.
	markedExcessDB = 15.0
	// Reactions come after the kill, so the window around a play is lopsided.
	voiceBeforeKillSeconds = 2.0
	voiceAfterKillSeconds  = 6.0
	voiceLeadInSeconds     = 5.0
	voiceTailSeconds       = 3.0
	voiceKillBonus         = 15
	minVoicePlaySeconds    = 2.0
	markedScoreFloor       = 70
	// VoiceKind and KillKind tell the two kinds of play apart.
	VoiceKind = "voice_peak"
	KillKind  = "kill"
	// VoiceLabel is what the UI and the MCP show for a peak without a kill.
	VoiceLabel = "Pico de voz"
)

// VoiceSettings are the user's choices for the voice peak scan.
type VoiceSettings struct {
	Enabled     bool `json:"enabled"`
	Sensitivity int  `json:"sensitivity"`
}

// DefaultVoiceSettings keep the scan on, since a marked shout must not be missed.
func DefaultVoiceSettings() VoiceSettings {
	return VoiceSettings{Enabled: true, Sensitivity: DefaultVoiceSensitivity}
}

// Normalized puts the sensitivity into its range; zero means the default.
func (s VoiceSettings) Normalized() VoiceSettings {
	if s.Sensitivity == 0 {
		s.Sensitivity = DefaultVoiceSensitivity
	}
	s.Sensitivity = min(max(s.Sensitivity, MinVoiceSensitivity), MaxVoiceSensitivity)
	return s
}

// thresholdDB is how far over the streamer's usual level counts as a peak.
func thresholdDB(sensitivity int) float64 {
	return float64(16 - 2*min(max(sensitivity, MinVoiceSensitivity), MaxVoiceSensitivity))
}

// VoicePeak is a stretch where the voice stayed over the threshold.
type VoicePeak struct {
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds"`
	// ExcessDB is how far the loudest window rose over the streamer's median.
	ExcessDB float64 `json:"excess_db"`
	Marked   bool    `json:"marked,omitempty"`
}

// DetectVoicePeaks finds the stretches of a loudness series (dBFS per
// VoiceWindowSeconds window) that clearly exceed the series' own median.
func DetectVoicePeaks(levels []float64, sensitivity int) []VoicePeak {
	var voiced []float64
	for _, l := range levels {
		if l > SilenceFloorDB {
			voiced = append(voiced, l)
		}
	}
	if len(voiced) < minVoicedWindows {
		return nil
	}
	sort.Float64s(voiced)
	baseline := voiced[len(voiced)/2]
	threshold := baseline + thresholdDB(sensitivity)

	var peaks []VoicePeak
	for i := 0; i < len(levels); {
		if levels[i] < threshold {
			i++
			continue
		}
		j, top := i, levels[i]
		for j < len(levels) && levels[j] >= threshold {
			top = math.Max(top, levels[j])
			j++
		}
		if j-i >= minPeakWindows || top >= threshold+singleWindowBoost {
			peaks = append(peaks, VoicePeak{
				StartSeconds: round3(float64(i) * VoiceWindowSeconds),
				EndSeconds:   round3(float64(j) * VoiceWindowSeconds),
				ExcessDB:     math.Round((top-baseline)*10) / 10,
				Marked:       top-baseline >= markedExcessDB,
			})
		}
		i = j
	}
	return mergePeaks(peaks)
}

func mergePeaks(peaks []VoicePeak) []VoicePeak {
	var out []VoicePeak
	for _, p := range peaks {
		if n := len(out); n > 0 && p.StartSeconds-out[n-1].EndSeconds <= voiceMergeGapSeconds {
			last := &out[n-1]
			last.EndSeconds = p.EndSeconds
			last.ExcessDB = math.Max(last.ExcessDB, p.ExcessDB)
			last.Marked = last.Marked || p.Marked
			continue
		}
		out = append(out, p)
	}
	return out
}

// AddVoicePeaks ties peaks to plays: a peak next to a kill raises that play's
// score and a peak alone becomes a play of its own. Plays are returned in time
// order with fresh ids.
func AddVoicePeaks(plays []Play, peaks []VoicePeak, durationSeconds float64) []Play {
	out := make([]Play, len(plays))
	copy(out, plays)
	var alone []Play
	for _, peak := range peaks {
		idx := nearKillPlay(out, peak)
		if idx < 0 {
			alone = append(alone, voicePlay(peak, durationSeconds))
			continue
		}
		p := &out[idx]
		if len(p.VoiceSeconds) == 0 {
			p.Score = min(p.Score+voiceKillBonus, 100)
		}
		p.VoiceSeconds = append(p.VoiceSeconds, peak.StartSeconds)
		p.VoiceDB = math.Max(p.VoiceDB, peak.ExcessDB)
		p.Marked = p.Marked || peak.Marked
		p.EndSeconds = math.Max(p.EndSeconds, peak.EndSeconds+1)
		if durationSeconds > 0 {
			p.EndSeconds = math.Min(p.EndSeconds, durationSeconds)
		}
		if peak.Marked {
			p.Score = max(p.Score, markedScoreFloor)
		}
	}
	return renumber(fitAlone(out, alone))
}

func nearKillPlay(plays []Play, peak VoicePeak) int {
	for i, p := range plays {
		if len(p.KillSeconds) == 0 {
			continue
		}
		first, last := p.KillSeconds[0], p.KillSeconds[len(p.KillSeconds)-1]
		if peak.EndSeconds >= first-voiceBeforeKillSeconds && peak.StartSeconds <= last+voiceAfterKillSeconds {
			return i
		}
	}
	return -1
}

func voicePlay(peak VoicePeak, durationSeconds float64) Play {
	end := peak.EndSeconds + voiceTailSeconds
	if durationSeconds > 0 {
		end = math.Min(end, durationSeconds)
	}
	score := min(25+int(peak.ExcessDB), 60)
	if peak.Marked {
		score = markedScoreFloor
	}
	return Play{
		Label:        VoiceLabel,
		Kind:         VoiceKind,
		StartSeconds: round3(math.Max(0, peak.StartSeconds-voiceLeadInSeconds)),
		EndSeconds:   round3(end),
		KillSeconds:  []float64{},
		VoiceSeconds: []float64{peak.StartSeconds},
		VoiceDB:      peak.ExcessDB,
		Marked:       peak.Marked,
		Score:        score,
	}
}

// fitAlone adds the peak-only plays, trimmed so they do not overlap the
// plays beside them; one that no longer holds minVoicePlaySeconds is dropped.
func fitAlone(plays, alone []Play) []Play {
	all := append(append([]Play(nil), plays...), alone...)
	sort.SliceStable(all, func(a, b int) bool { return all[a].StartSeconds < all[b].StartSeconds })
	for i := range all {
		if all[i].Kind != VoiceKind {
			continue
		}
		if i > 0 {
			all[i].StartSeconds = math.Max(all[i].StartSeconds, all[i-1].EndSeconds)
		}
		if i+1 < len(all) {
			all[i].EndSeconds = math.Min(all[i].EndSeconds, all[i+1].StartSeconds)
		}
	}
	kept := all[:0]
	for _, p := range all {
		if p.Kind == VoiceKind && p.EndSeconds-p.StartSeconds < minVoicePlaySeconds {
			continue
		}
		kept = append(kept, p)
	}
	return kept
}

func renumber(plays []Play) []Play {
	for i := range plays {
		plays[i].ID = fmt.Sprintf("play-%03d", i+1)
	}
	return plays
}

// LoudnessOptions picks the audio to measure.
type LoudnessOptions struct {
	FFmpeg string
	Input  string
	// Track is the zero-based audio stream: the mic track of a multitrack
	// recording, or 0 for the only one.
	Track int
}

// VoiceTrack picks the audio stream to measure. A recording with several audio
// tracks usually keeps the mix on the first and the mic on the second.
func VoiceTrack(audioStreams int) int {
	if audioStreams >= 2 {
		return 1
	}
	return 0
}

// MeasureLoudness decodes one audio track to mono and returns its level in
// dBFS for every VoiceWindowSeconds window.
func MeasureLoudness(ctx context.Context, opts LoudnessOptions) ([]float64, error) {
	ffmpeg := opts.FFmpeg
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	// #nosec G204 -- ffmpeg path and input are local configuration; no shell.
	cmd := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-nostdin", "-v", "error",
		"-i", opts.Input, "-map", fmt.Sprintf("0:a:%d", opts.Track), "-vn", "-sn",
		"-ac", "1", "-ar", fmt.Sprint(voiceSampleRate), "-f", "s16le", "pipe:1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open ffmpeg pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start ffmpeg: %w", err)
	}
	levels, readErr := windowLevels(bufio.NewReaderSize(stdout, 1<<20))
	_, _ = io.Copy(io.Discard, stdout)
	if err := cmd.Wait(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("ffmpeg audio of %s: %w: %s", opts.Input, err, msg)
		}
		return nil, fmt.Errorf("ffmpeg audio of %s: %w", opts.Input, err)
	}
	return levels, readErr
}

// windowLevels reads 16-bit little-endian mono PCM and returns the RMS level
// of each whole window in dBFS; a trailing partial window is ignored.
func windowLevels(r io.Reader) ([]float64, error) {
	perWindow := int(VoiceWindowSeconds * voiceSampleRate)
	buf := make([]byte, perWindow*2)
	var levels []float64
	for {
		if _, err := io.ReadFull(r, buf); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return levels, nil
			}
			return levels, fmt.Errorf("read audio: %w", err)
		}
		sum := 0.0
		for i := 0; i < perWindow; i++ {
			v := float64(int16(binary.LittleEndian.Uint16(buf[i*2:]))) / 32768
			sum += v * v
		}
		rms := math.Sqrt(sum / float64(perWindow))
		levels = append(levels, math.Max(20*math.Log10(math.Max(rms, 1e-9)), -90))
	}
}
