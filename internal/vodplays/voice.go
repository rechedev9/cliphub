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
	// minVoicedShare is how much of the track must be non-silent for it to carry a voice.
	minVoicedShare = 0.02
	// The baseline is the median of the voiced windows within this many windows of the
	// current one (about 5 minutes), so a quiet hour and a loud hour each have their own.
	baselineRadiusWindows = 300
	baselineStepWindows   = 20
	// A peak lasts at least minPeakWindows, unless one window is far above the threshold.
	minPeakWindows    = 2
	singleWindowBoost = 6.0
	// Peaks whose clips (lead-in and tail included) come within voiceJoinSlackSeconds
	// of each other are one moment, and one moment spans at most voiceMaxMomentSeconds.
	voiceJoinSlackSeconds = 3.0
	voiceMaxMomentSeconds = 30.0
	// A shout on purpose is far over the local baseline and over almost everything
	// else in the video, held for markedHoldWindows (1 s). Only a few per hour stay marked.
	markedExcessDB      = 27.0
	markedPercentile    = 0.999
	markedHoldWindows   = 2
	maxMarkedPerHour    = 3
	maxVoiceOnlyPerHour = 6
	secondsPerHour      = 3600.0
	// Reactions come after the kill, so the window around a play is lopsided.
	voiceBeforeKillSeconds = 2.0
	voiceAfterKillSeconds  = 6.0
	voiceLeadInSeconds     = 5.0
	voiceTailSeconds       = 3.0
	voiceKillBonus         = 15
	minVoicePlaySeconds    = 2.0
	// A voice-only play scores under a 1-kill play with a shout (20 + the bonus).
	voiceOnlyBaseScore = 10
	voiceOnlyMaxScore  = 28
	voiceMarkedScore   = 30
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
	// ExcessDB is how far the loudest window rose over the local baseline.
	ExcessDB float64 `json:"excess_db"`
	Marked   bool    `json:"marked,omitempty"`
	// LoudestSeconds is where the loudest stretch of the peak sits; zero means the middle.
	LoudestSeconds float64 `json:"loudest_seconds,omitempty"`
}

func (p VoicePeak) loudest() float64 {
	if p.LoudestSeconds > 0 {
		return p.LoudestSeconds
	}
	return (p.StartSeconds + p.EndSeconds) / 2
}

// VoicedEnough tells whether a loudness series holds a voice at all: a mix
// that is silent most of the way, or a muted mic, does not.
func VoicedEnough(levels []float64) bool {
	voiced := 0
	for _, l := range levels {
		if l > SilenceFloorDB {
			voiced++
		}
	}
	return voiced >= minVoicedWindows && float64(voiced) >= minVoicedShare*float64(len(levels))
}

// rollingBaselines returns the median voiced level around every window.
func rollingBaselines(levels []float64) []float64 {
	var all []float64
	for _, l := range levels {
		if l > SilenceFloorDB {
			all = append(all, l)
		}
	}
	sort.Float64s(all)
	global := all[len(all)/2]
	out := make([]float64, len(levels))
	for start := 0; start < len(levels); start += baselineStepWindows {
		end := min(start+baselineStepWindows, len(levels))
		centre := (start + end) / 2
		var near []float64
		for _, l := range levels[max(0, centre-baselineRadiusWindows):min(len(levels), centre+baselineRadiusWindows)] {
			if l > SilenceFloorDB {
				near = append(near, l)
			}
		}
		base := global
		if len(near) >= minVoicedWindows {
			sort.Float64s(near)
			base = near[len(near)/2]
		}
		for i := start; i < end; i++ {
			out[i] = base
		}
	}
	return out
}

// DetectVoicePeaks finds the stretches of a loudness series (dBFS per
// VoiceWindowSeconds window) that clearly exceed the level around them.
func DetectVoicePeaks(levels []float64, sensitivity int) []VoicePeak {
	if !VoicedEnough(levels) {
		return nil
	}
	voiced := make([]float64, 0, len(levels))
	for _, l := range levels {
		if l > SilenceFloorDB {
			voiced = append(voiced, l)
		}
	}
	sort.Float64s(voiced)
	extreme := voiced[min(int(markedPercentile*float64(len(voiced))), len(voiced)-1)]
	bases := rollingBaselines(levels)
	delta := thresholdDB(sensitivity)

	var peaks []VoicePeak
	for i := 0; i < len(levels); {
		if levels[i] < bases[i]+delta {
			i++
			continue
		}
		j, top, topExcess, topAt, held, run := i, levels[i], 0.0, i, 0, 0
		for j < len(levels) && levels[j] >= bases[j]+delta {
			top = math.Max(top, levels[j])
			if levels[j]-bases[j] > topExcess {
				topExcess, topAt = levels[j]-bases[j], j
			}
			if levels[j]-bases[j] >= markedExcessDB && levels[j] >= extreme {
				run++
				held = max(held, run)
			} else {
				run = 0
			}
			j++
		}
		if j-i >= minPeakWindows || topExcess >= delta+singleWindowBoost {
			peaks = append(peaks, VoicePeak{
				StartSeconds: round3(float64(i) * VoiceWindowSeconds),
				EndSeconds:   round3(float64(j) * VoiceWindowSeconds),
				ExcessDB:     math.Round(topExcess*10) / 10,
				Marked:       held >= markedHoldWindows,

				LoudestSeconds: round3((float64(topAt) + 0.5) * VoiceWindowSeconds),
			})
		}
		i = j
	}
	return limitMarked(mergePeaks(peaks))
}

// mergePeaks joins peaks whose clips touch or nearly touch into one moment that
// keeps the highest level, then trims a moment longer than voiceMaxMomentSeconds
// to the stretch around its loudest point.
func mergePeaks(peaks []VoicePeak) []VoicePeak {
	sort.SliceStable(peaks, func(a, b int) bool { return peaks[a].StartSeconds < peaks[b].StartSeconds })
	joinGap := voiceLeadInSeconds + voiceTailSeconds + voiceJoinSlackSeconds
	var out []VoicePeak
	for _, p := range peaks {
		if n := len(out); n > 0 && p.StartSeconds-out[n-1].EndSeconds <= joinGap {
			last := &out[n-1]
			if p.ExcessDB > last.ExcessDB {
				last.LoudestSeconds = p.loudest()
			} else {
				last.LoudestSeconds = last.loudest()
			}
			last.EndSeconds = math.Max(last.EndSeconds, p.EndSeconds)
			last.ExcessDB = math.Max(last.ExcessDB, p.ExcessDB)
			last.Marked = last.Marked || p.Marked
			continue
		}
		out = append(out, p)
	}
	span := voiceMaxMomentSeconds - voiceLeadInSeconds - voiceTailSeconds
	for i := range out {
		if out[i].EndSeconds-out[i].StartSeconds > span {
			centre := out[i].loudest()
			out[i].StartSeconds = round3(math.Max(out[i].StartSeconds, centre-span/2))
			out[i].EndSeconds = round3(math.Min(out[i].EndSeconds, centre+span/2))
		}
	}
	return out
}

// limitMarked keeps the loudest maxMarkedPerHour marked peaks of each hour of
// video; the rest stay peaks but are no longer marked.
func limitMarked(peaks []VoicePeak) []VoicePeak {
	byHour := map[int][]int{}
	for i, p := range peaks {
		if p.Marked {
			hour := int(p.StartSeconds / secondsPerHour)
			byHour[hour] = append(byHour[hour], i)
		}
	}
	for _, idx := range byHour {
		sort.SliceStable(idx, func(a, b int) bool { return peaks[idx[a]].ExcessDB > peaks[idx[b]].ExcessDB })
		for _, i := range idx[min(maxMarkedPerHour, len(idx)):] {
			peaks[i].Marked = false
		}
	}
	return peaks
}

// AddVoicePeaks ties peaks to plays: a peak next to a kill raises that play's
// score and a peak alone becomes a play of its own (at most maxVoiceOnlyPerHour
// per hour of video, the loudest). Kill plays keep their ids; voice plays are
// numbered voice-001 on. The result is in time order.
func AddVoicePeaks(plays []Play, peaks []VoicePeak, durationSeconds float64) []Play {
	out := make([]Play, len(plays))
	copy(out, plays)
	var alone []Play
	for _, peak := range mergePeaks(append([]VoicePeak(nil), peaks...)) {
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
		end := math.Max(p.EndSeconds, peak.EndSeconds+1)
		if durationSeconds > 0 {
			end = math.Min(end, durationSeconds)
		}
		if idx+1 < len(out) {
			end = math.Min(end, math.Max(p.EndSeconds, out[idx+1].StartSeconds))
		}
		p.EndSeconds = end
	}
	return numberVoicePlays(fitAlone(out, capVoiceOnly(alone)))
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
		// A reaction clip that overlaps or touches the kill play is part of it.
		if peak.StartSeconds-voiceLeadInSeconds <= p.EndSeconds && peak.EndSeconds+voiceTailSeconds >= p.StartSeconds {
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
	score := min(voiceOnlyBaseScore+int(peak.ExcessDB), voiceOnlyMaxScore)
	if peak.Marked {
		score = voiceMarkedScore
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

// capVoiceOnly keeps the maxVoiceOnlyPerHour loudest voice-only plays of each
// hour of video, marked ones first, and returns them in time order.
func capVoiceOnly(alone []Play) []Play {
	byHour := map[int][]Play{}
	for _, p := range alone {
		hour := int(p.VoiceSeconds[0] / secondsPerHour)
		byHour[hour] = append(byHour[hour], p)
	}
	var kept []Play
	for _, group := range byHour {
		sort.SliceStable(group, func(a, b int) bool {
			if group[a].Marked != group[b].Marked {
				return group[a].Marked
			}
			return group[a].VoiceDB > group[b].VoiceDB
		})
		kept = append(kept, group[:min(maxVoiceOnlyPerHour, len(group))]...)
	}
	return kept
}

// fitAlone adds the peak-only plays, trimmed so they do not overlap the
// plays beside them; one that no longer holds minVoicePlaySeconds is dropped,
// except a marked one, which keeps its whole span and may touch its neighbours.
func fitAlone(plays, alone []Play) []Play {
	all := append(append([]Play(nil), plays...), alone...)
	sort.SliceStable(all, func(a, b int) bool { return all[a].StartSeconds < all[b].StartSeconds })
	for i := range all {
		if all[i].Kind != VoiceKind {
			continue
		}
		start, end := all[i].StartSeconds, all[i].EndSeconds
		if i > 0 {
			all[i].StartSeconds = math.Max(all[i].StartSeconds, all[i-1].EndSeconds)
		}
		if i+1 < len(all) {
			all[i].EndSeconds = math.Min(all[i].EndSeconds, all[i+1].StartSeconds)
		}
		if all[i].Marked && all[i].EndSeconds-all[i].StartSeconds < minVoicePlaySeconds {
			all[i].StartSeconds, all[i].EndSeconds = start, end
		}
	}
	kept := all[:0]
	for _, p := range all {
		if p.Kind == VoiceKind && !p.Marked && p.EndSeconds-p.StartSeconds < minVoicePlaySeconds {
			continue
		}
		kept = append(kept, p)
	}
	return kept
}

func numberVoicePlays(plays []Play) []Play {
	n := 0
	for i := range plays {
		if plays[i].Kind == VoiceKind {
			n++
			plays[i].ID = fmt.Sprintf("voice-%03d", n)
		}
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

// VoiceTrack picks the audio stream to measure and says whether it is a mic
// track rather than the mix. A track titled "mic" wins; otherwise a recording
// with several tracks usually keeps the mix on the first and the mic on the
// second; a single track is the mix.
func VoiceTrack(audioTitles []string, audioStreams int) (track int, mic bool) {
	for i, title := range audioTitles {
		if strings.Contains(strings.ToLower(title), "mic") {
			return i, true
		}
	}
	if audioStreams >= 2 {
		return 1, true
	}
	return 0, false
}

// MeasureLoudness decodes one audio track to mono and returns its level in
// dBFS for every VoiceWindowSeconds window.
func MeasureLoudness(ctx context.Context, opts LoudnessOptions) ([]float64, error) {
	ffmpeg := opts.FFmpeg
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	// aresample fills timestamp gaps with silence so later peaks do not shift earlier.
	// #nosec G204 -- ffmpeg path and input are local configuration; no shell.
	cmd := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-nostdin", "-v", "error",
		"-i", opts.Input, "-map", fmt.Sprintf("0:a:%d", opts.Track), "-vn", "-sn",
		"-af", "aresample=async=1:first_pts=0",
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
