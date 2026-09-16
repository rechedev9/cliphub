package editor

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

// LoudnessMeasurement describes the decoded input of a measurement pass.
// Silent material has no finite integrated loudness; never serialize a fake
// LUFS value or an infinity as if normalization had succeeded.
type LoudnessMeasurement struct {
	Status         string   `json:"status"`
	IntegratedLUFS *float64 `json:"integrated_lufs"`
	TruePeakDBTP   *float64 `json:"true_peak_dbtp"`
	LRA            *float64 `json:"lra"`
	Threshold      *float64 `json:"threshold"`
	Offset         *float64 `json:"offset"`
}

type ProgramLoudnessEvidence struct {
	Policy          string                      `json:"policy"`
	Input           LoudnessMeasurement         `json:"input"`
	DecodedAAC      []LoudnessMeasurement       `json:"decoded_aac"`
	MasterTargets   []recapplan.LoudnessOptions `json:"master_targets"`
	FallbackMasters []ProgramAACFallbackMaster  `json:"fallback_masters,omitempty"`
	Status          string                      `json:"status"`
	// FinalMuxedAAC is measured from the actual muxed output after the passing
	// audio candidate is combined with the program video. It certifies the
	// delivered media, not only the intermediate audio-only candidate, and is
	// absent whenever a candidate, mux or final measurement failed.
	FinalMuxedAAC *LoudnessMeasurement `json:"final_muxed_aac,omitempty"`
}

func decimal(v float64) string { return strconv.FormatFloat(v, 'f', 6, 64) }

func loudnessFilter(target recapplan.LoudnessOptions) string {
	return "loudnorm=I=" + decimal(target.TargetILUFS) + ":TP=" + decimal(target.TargetTPDBTP) + ":LRA=" + decimal(target.TargetLRA)
}

func parseLoudnessMeasurement(output string) (LoudnessMeasurement, error) {
	var m LoudnessMeasurement
	// FFmpeg writes diagnostics before its single trailing loudnorm JSON block.
	start, end := strings.LastIndex(output, "{"), strings.LastIndex(output, "}")
	if start < 0 || end < start || end-start > 16384 {
		return m, fmt.Errorf("audio_loudness_failed: missing bounded loudnorm measurement")
	}
	var raw map[string]string
	if err := json.Unmarshal([]byte(output[start:end+1]), &raw); err != nil {
		return m, fmt.Errorf("decode loudness measurement: %w", err)
	}
	if raw["input_i"] == "-inf" && raw["input_tp"] == "-inf" {
		m.Status = "silent"
		return m, nil
	}
	for _, field := range []struct {
		key   string
		value **float64
	}{
		{"input_i", &m.IntegratedLUFS}, {"input_tp", &m.TruePeakDBTP}, {"input_lra", &m.LRA}, {"input_thresh", &m.Threshold}, {"target_offset", &m.Offset},
	} {
		number, err := strconv.ParseFloat(raw[field.key], 64)
		if err != nil || math.IsInf(number, 0) || math.IsNaN(number) {
			return m, fmt.Errorf("audio_loudness_failed: non-finite %s", field.key)
		}
		*field.value = &number
	}
	m.Status = "measured"
	return m, nil
}

func measureLoudness(ctx context.Context, ffmpeg, path string, target recapplan.LoudnessOptions, logPath string, duration float64, onFraction func(float64)) (LoudnessMeasurement, error) {
	command := []string{ffmpeg, "-hide_banner", "-nostats", "-v", "info", "-i", path, "-map", "0:a:0", "-vn", "-af", loudnessFilter(target) + ":print_format=json", "-f", "null", "-"}
	output, err := runFFmpegOutputProgress(ctx, command, "Full Demo audio measurement", duration, onFraction)
	if logPath != "" {
		if writeErr := writeLogFile(logPath, output); writeErr != nil {
			return LoudnessMeasurement{}, writeErr
		}
	}
	if err != nil {
		return LoudnessMeasurement{}, err
	}
	return parseLoudnessMeasurement(output)
}

func measuredLoudnessFilter(target recapplan.LoudnessOptions, measured LoudnessMeasurement) (string, error) {
	if measured.Status != "measured" || measured.IntegratedLUFS == nil || measured.TruePeakDBTP == nil || measured.LRA == nil || measured.Threshold == nil || measured.Offset == nil {
		return "", fmt.Errorf("audio_loudness_failed: finite first-pass measurement is required")
	}
	return loudnessFilter(target) + ":measured_I=" + decimal(*measured.IntegratedLUFS) + ":measured_TP=" + decimal(*measured.TruePeakDBTP) + ":measured_LRA=" + decimal(*measured.LRA) + ":measured_thresh=" + decimal(*measured.Threshold) + ":offset=" + decimal(*measured.Offset) + ":linear=true:print_format=json", nil
}

// masterFullDemoProgram always remasters the lossless mixed program, never an
// already encoded AAC file. Each native candidate is an audio-only AAC MP4 so a
// failed master never copies the complete video; only a passing candidate is
// muxed once with the program video. The decoded AAC measurement owns
// acceptance, and the original three attempts precede a bounded Windows AAC
// recovery.
func masterFullDemoProgram(ctx context.Context, ffmpeg, input, output, logDir string, target recapplan.LoudnessOptions, silentApproved bool, duration float64, progress fullDemoProgress) (ProgramLoudnessEvidence, error) {
	fallbackProgress := progress.within(.65, 1)
	progress = progress.within(0, .65)
	e := ProgramLoudnessEvidence{Policy: target.PolicyVersion, DecodedAAC: []LoudnessMeasurement{}, MasterTargets: []recapplan.LoudnessOptions{}, Status: "unverified"}
	measurement, err := measureLoudness(fullDemoTimingStage(ctx, "audio_input_analysis", -1), ffmpeg, input, target, filepath.Join(logDir, "program-input-loudness.txt"), duration, progress.pass("Analizando audio final", 0, .08))
	if err != nil {
		return e, err
	}
	e.Input = measurement
	if measurement.Status == "silent" && !silentApproved {
		e.Status = "silent"
		return e, fmt.Errorf("audio_silent: the program has no measurable audio; approve a muted program or correct its sources")
	}
	attemptTarget := target
	masterSamples := int64(math.Round(duration * recapplan.SampleRate))
	// Reserve a small initial headroom for lossy AAC reconstruction.
	attemptTarget.TargetTPDBTP -= 0.3
	for attempt := 0; attempt < 3; attempt++ {
		start := .08 + float64(attempt)*.25
		stage := fmt.Sprintf("Ajustando audio final (%d/3)", attempt+1)
		filter := "anull"
		if measurement.Status != "silent" {
			if attempt > 0 {
				measurement, err = measureLoudness(fullDemoTimingStage(ctx, "audio_input_analysis", attempt), ffmpeg, input, attemptTarget, filepath.Join(logDir, fmt.Sprintf("program-remaster-%d-input.txt", attempt)), duration, progress.pass(stage, start, start+.07))
				if err != nil {
					return e, err
				}
			}
			filter, err = measuredLoudnessFilter(attemptTarget, measurement)
			if err != nil {
				return e, err
			}
		}
		e.MasterTargets = append(e.MasterTargets, attemptTarget)
		candidate, candidateCleanup, err := fullDemoAudioCandidatePath(output)
		if err != nil {
			return e, err
		}
		command := fullDemoNativeCandidateCommand(ffmpeg, input, candidate, filter, masterSamples, duration)
		if err := runFFmpegWithOptionalLogAndProgress(fullDemoTimingStageVariant(ctx, "audio_candidate_encode", attempt, "aac", "native"), command, "Full Demo program master", filepath.Join(logDir, fmt.Sprintf("program-master-%d.txt", attempt)), duration, progress.pass(stage, start+.07, start+.16)); err != nil {
			candidateCleanup()
			return e, err
		}
		decoded, err := measureLoudness(fullDemoTimingStageVariant(ctx, "audio_candidate_analysis", attempt, "aac", "native"), ffmpeg, candidate, target, filepath.Join(logDir, fmt.Sprintf("decoded-aac-%d.txt", attempt)), duration, progress.pass(fmt.Sprintf("Comprobando audio final (%d/3)", attempt+1), start+.16, start+.25))
		if err != nil {
			candidateCleanup()
			return e, err
		}
		e.DecodedAAC = append(e.DecodedAAC, decoded)
		accepted, err := fullDemoDecodedAACAccepted(decoded, target, silentApproved)
		if err != nil {
			candidateCleanup()
			return e, err
		}
		if accepted {
			result, err := deliverFullDemoAACCandidate(ctx, ffmpeg, input, candidate, output, logDir, target, silentApproved, duration, e, progress.pass("Publicando el audio final", .85, .99))
			candidateCleanup()
			return result, err
		}
		candidateCleanup()
		next, changed := nextMasterTarget(attemptTarget, target, decoded)
		if !changed {
			// loudnorm cannot be pushed any further; another native master
			// would repeat a failed target, so hand over to AAC recovery.
			break
		}
		attemptTarget = next
	}
	return recoverFullDemoAAC(ctx, ffmpeg, input, output, logDir, target, duration, e, fallbackProgress)
}

// loudnorm rejects targets outside these ranges, so retargeting must stay
// within them instead of failing the whole render with "Result too large".
const (
	loudnormMinILUFS  = -70.0
	loudnormMaxILUFS  = -5.0
	loudnormMinTPDBTP = -9.0
	loudnormMaxTPDBTP = 0.0
)

// nextMasterTarget derives the next native master target from the decoded AAC
// measurement, clamped to loudnorm's accepted ranges. It reports false when the
// clamped target is identical to the current one, meaning no headroom remains.
func nextMasterTarget(current, target recapplan.LoudnessOptions, decoded LoudnessMeasurement) (recapplan.LoudnessOptions, bool) {
	next := current
	next.TargetILUFS += max(-1.0, min(1.0, target.TargetILUFS-*decoded.IntegratedLUFS))
	if *decoded.TruePeakDBTP > target.TargetTPDBTP {
		next.TargetTPDBTP -= *decoded.TruePeakDBTP - target.TargetTPDBTP + 0.2
	}
	next.TargetILUFS = max(loudnormMinILUFS, min(loudnormMaxILUFS, next.TargetILUFS))
	next.TargetTPDBTP = max(loudnormMinTPDBTP, min(loudnormMaxTPDBTP, next.TargetTPDBTP))
	return next, next != current
}
