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
	Policy        string                      `json:"policy"`
	Input         LoudnessMeasurement         `json:"input"`
	DecodedAAC    []LoudnessMeasurement       `json:"decoded_aac"`
	MasterTargets []recapplan.LoudnessOptions `json:"master_targets"`
	Status        string                      `json:"status"`
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

func measureLoudness(ctx context.Context, ffmpeg, path string, target recapplan.LoudnessOptions, logPath string) (LoudnessMeasurement, error) {
	command := []string{ffmpeg, "-hide_banner", "-nostats", "-v", "info", "-i", path, "-map", "0:a:0", "-vn", "-af", loudnessFilter(target) + ":print_format=json", "-f", "null", "-"}
	output, err := runFFmpegOutput(ctx, command, "Full Demo audio measurement")
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
// already encoded AAC file. The decoded AAC measurement owns acceptance, and
// only a bounded three-attempt correction is allowed.
func masterFullDemoProgram(ctx context.Context, ffmpeg, input, output, logDir string, target recapplan.LoudnessOptions, silentApproved bool) (ProgramLoudnessEvidence, error) {
	e := ProgramLoudnessEvidence{Policy: target.PolicyVersion, DecodedAAC: []LoudnessMeasurement{}, MasterTargets: []recapplan.LoudnessOptions{}, Status: "unverified"}
	measurement, err := measureLoudness(ctx, ffmpeg, input, target, filepath.Join(logDir, "program-input-loudness.txt"))
	if err != nil {
		return e, err
	}
	e.Input = measurement
	if measurement.Status == "silent" && !silentApproved {
		e.Status = "silent"
		return e, fmt.Errorf("audio_silent: the program has no measurable audio; approve a muted program or correct its sources")
	}
	attemptTarget := target
	// Reserve a small initial headroom for lossy AAC reconstruction.
	attemptTarget.TargetTPDBTP -= 0.3
	for attempt := 0; attempt < 3; attempt++ {
		filter := "anull"
		if measurement.Status != "silent" {
			if attempt > 0 {
				measurement, err = measureLoudness(ctx, ffmpeg, input, attemptTarget, filepath.Join(logDir, fmt.Sprintf("program-remaster-%d-input.txt", attempt)))
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
		command := []string{ffmpeg, "-y", "-hide_banner", "-nostats", "-v", "info", "-i", input, "-map", "0:v:0", "-map", "0:a:0", "-c:v", "copy", "-af", filter + ",aresample=48000,aformat=channel_layouts=stereo", "-c:a", "aac", "-b:a", "192k", "-ar", "48000", "-ac", "2", "-movflags", "+faststart", output}
		if err := runFFmpegAtomic(ctx, command, "Full Demo program master", filepath.Join(logDir, fmt.Sprintf("program-master-%d.txt", attempt)), output); err != nil {
			return e, err
		}
		decoded, err := measureLoudness(ctx, ffmpeg, output, target, filepath.Join(logDir, fmt.Sprintf("decoded-aac-%d.txt", attempt)))
		if err != nil {
			return e, err
		}
		e.DecodedAAC = append(e.DecodedAAC, decoded)
		if decoded.Status == "silent" && silentApproved {
			e.Status = "silent-approved"
			return e, nil
		}
		if decoded.Status != "measured" {
			return e, fmt.Errorf("audio_loudness_failed: final AAC is not measurable")
		}
		if math.Abs(*decoded.IntegratedLUFS-target.TargetILUFS) <= 0.5 && *decoded.TruePeakDBTP <= target.TargetTPDBTP {
			e.Status = "verified-decoded-aac"
			return e, nil
		}
		attemptTarget.TargetILUFS += max(-1.0, min(1.0, target.TargetILUFS-*decoded.IntegratedLUFS))
		if *decoded.TruePeakDBTP > target.TargetTPDBTP {
			attemptTarget.TargetTPDBTP -= *decoded.TruePeakDBTP - target.TargetTPDBTP + 0.2
		}
	}
	return e, fmt.Errorf("audio_loudness_failed: decoded AAC remains outside approved loudness/true-peak targets after three masters")
}
