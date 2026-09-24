package editor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/obs"
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

// fullDemoProgramInputLoudness returns the first program measurement. When the
// program-audio assembly already measured the same samples against the same
// target, its parsed measurement is reused and its output is written to the
// same evidence log, so no second decode of the whole program runs. Any other
// case (no fused measurement, an unparseable one, or a different target) runs
// the standalone measurement pass unchanged, including its timing span.
func fullDemoProgramInputLoudness(ctx context.Context, ffmpeg, input string, target recapplan.LoudnessOptions, logPath string, duration float64, onFraction func(float64), assembled fullDemoProgramAudio) (LoudnessMeasurement, error) {
	if !assembled.Measured || assembled.Target != target {
		return measureLoudness(fullDemoTimingStage(ctx, "audio_input_analysis", -1), ffmpeg, input, target, logPath, duration, onFraction)
	}
	if err := writeLogFile(logPath, assembled.Output); err != nil {
		return LoudnessMeasurement{}, err
	}
	if onFraction != nil {
		onFraction(1)
	}
	return assembled.Measurement, nil
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
	return masterFullDemoSplitProgram(ctx, ffmpeg, input, committedFullDemoProgramVideo(input), output, logDir, target, silentApproved, duration, progress)
}

// fullDemoProgramVideo resolves the committed program video for the final mux.
// Mastering only reads audio, so it may run while the video is still being
// encoded; it blocks here only once a candidate has already passed.
type fullDemoProgramVideo func(context.Context) (string, error)

func committedFullDemoProgramVideo(path string) fullDemoProgramVideo {
	return func(context.Context) (string, error) { return path, nil }
}

// masterFullDemoSplitProgram masters input, the lossless program audio, and
// muxes the passing candidate with the separately produced program video.
func masterFullDemoSplitProgram(ctx context.Context, ffmpeg, input string, video fullDemoProgramVideo, output, logDir string, target recapplan.LoudnessOptions, silentApproved bool, duration float64, progress fullDemoProgress) (ProgramLoudnessEvidence, error) {
	return masterFullDemoMeasuredProgram(ctx, ffmpeg, input, video, output, logDir, target, silentApproved, duration, progress, fullDemoProgramAudio{})
}

// masterFullDemoMeasuredProgram is masterFullDemoSplitProgram with the program
// measurement the assembly already produced from the very same samples. The
// candidate sequence, acceptance rules, evidence and log files are unchanged;
// only the first full decode of the program audio is skipped.
func masterFullDemoMeasuredProgram(ctx context.Context, ffmpeg, input string, video fullDemoProgramVideo, output, logDir string, target recapplan.LoudnessOptions, silentApproved bool, duration float64, progress fullDemoProgress, assembled fullDemoProgramAudio) (ProgramLoudnessEvidence, error) {
	fallbackProgress := progress.within(.65, 1)
	progress = progress.within(0, .65)
	e := ProgramLoudnessEvidence{Policy: target.PolicyVersion, DecodedAAC: []LoudnessMeasurement{}, MasterTargets: []recapplan.LoudnessOptions{}, Status: "unverified"}
	// fail classifies an error of the native master chain for remote grouping.
	fail := func(err error) (ProgramLoudnessEvidence, error) {
		return e, fullDemoAudioFailure(ctx, err, obs.SubstageAudioMaster)
	}
	measurement, err := fullDemoProgramInputLoudness(ctx, ffmpeg, input, target, filepath.Join(logDir, "program-input-loudness.txt"), duration, progress.pass("Analizando audio final", 0, .08), assembled)
	if err != nil {
		return fail(err)
	}
	e.Input = measurement
	if measurement.Status == "silent" && !silentApproved {
		e.Status = "silent"
		return e, fmt.Errorf("audio_silent: the program has no measurable audio; approve a muted program or correct its sources")
	}
	// Once the first native master has failed, the recovery chain runs alongside
	// the remaining native masters instead of after them.
	var recovery *fullDemoAACRecoverySpeculation
	defer func() {
		if recovery != nil {
			recovery.discard()
		}
	}()
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
					return fail(err)
				}
			}
			filter, err = measuredLoudnessFilter(attemptTarget, measurement)
			if err != nil {
				return fail(err)
			}
		}
		e.MasterTargets = append(e.MasterTargets, attemptTarget)
		candidate, candidateCleanup, err := fullDemoAudioCandidatePath(output)
		if err != nil {
			return fail(err)
		}
		command := fullDemoNativeCandidateCommand(ffmpeg, input, candidate, filter, masterSamples, duration)
		if err := runFFmpegWithOptionalLogAndProgress(fullDemoTimingStageVariant(ctx, "audio_candidate_encode", attempt, "aac", "native"), command, "Full Demo program master", filepath.Join(logDir, fmt.Sprintf("program-master-%d.txt", attempt)), duration, progress.pass(stage, start+.07, start+.16)); err != nil {
			candidateCleanup()
			return fail(err)
		}
		decoded, err := measureLoudness(fullDemoTimingStageVariant(ctx, "audio_candidate_analysis", attempt, "aac", "native"), ffmpeg, candidate, target, filepath.Join(logDir, fmt.Sprintf("decoded-aac-%d.txt", attempt)), duration, progress.pass(fmt.Sprintf("Comprobando audio final (%d/3)", attempt+1), start+.16, start+.25))
		if err != nil {
			candidateCleanup()
			return fail(err)
		}
		e.DecodedAAC = append(e.DecodedAAC, decoded)
		accepted, err := fullDemoDecodedAACAccepted(decoded, target, silentApproved)
		if err != nil {
			candidateCleanup()
			return fail(err)
		}
		if accepted {
			if recovery != nil {
				recovery.discard()
				recovery = nil
			}
			program, err := video(ctx)
			if err != nil {
				candidateCleanup()
				return e, err
			}
			result, err := deliverFullDemoAACCandidate(ctx, ffmpeg, program, candidate, output, logDir, target, silentApproved, duration, e, progress.pass("Publicando el audio final", .85, .99))
			candidateCleanup()
			return result, fullDemoAudioFailure(ctx, err, obs.SubstageAudioMaster)
		}
		candidateCleanup()
		if recovery == nil {
			recovery = startFullDemoAACRecovery(ctx, ffmpeg, input, video, output, logDir, target, duration, e.Input)
		}
		next, changed := nextMasterTarget(attemptTarget, target, decoded)
		if !changed {
			// loudnorm cannot be pushed any further; another native master
			// would repeat a failed target, so hand over to AAC recovery.
			break
		}
		attemptTarget = next
	}
	// Every path out of the loop has rejected at least one native master, so the
	// recovery chain is already running. Its failures are classified there:
	// audio_master_exhausted when no candidate passed, aac_recovery_failed when
	// the recovery itself broke.
	fallbackProgress.report("Recuperando audio final", 0)
	result := recovery.wait()
	recovery = nil
	return finishFullDemoAACRecovery(ctx, ffmpeg, video, output, logDir, target, duration, e, result, fallbackProgress)
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

// loudnormRangeRejection matches FFmpeg refusing a loudnorm option outside its
// accepted range, as in the Studio 3.0.0 incident:
// "[Parsed_loudnorm_0] Value -10.180000 for parameter 'TP' out of range [-9 - 0]".
var loudnormRangeRejection = regexp.MustCompile(`loudnorm[^\n]*for parameter '[^']+' out of range|option '[^']+' to filter 'loudnorm'`)

// fullDemoAudioFailure attaches a failure code to an error leaving the program
// master (audio_master) or the AAC recovery (aac_recovery). Cancellation wins
// over the error it caused. Only the recovery owns a catch-all code; a native
// master error that is neither a rejected loudnorm option nor a failed FFmpeg
// process keeps its original text and stays unclassified.
func fullDemoAudioFailure(ctx context.Context, err error, substage obs.Substage) error {
	switch {
	case err == nil:
		return nil
	case ctx.Err() != nil:
		return obs.WithFailure(err, obs.FailureRenderInterrupted, substage)
	case loudnormRangeRejection.MatchString(err.Error()):
		return obs.WithFailure(err, obs.FailureLoudnormParamOutOfRange, substage)
	case substage == obs.SubstageAACRecovery:
		return obs.WithFailure(err, obs.FailureAACRecoveryFailed, substage)
	case ffmpegProcessFailed(err):
		return obs.WithFailure(err, obs.FailureFFmpegFailed, substage)
	default:
		return err
	}
}

// ffmpegProcessFailed reports an FFmpeg process that could not start or exited
// unsuccessfully, as opposed to a measurement it printed that was rejected.
func ffmpegProcessFailed(err error) bool {
	var exitErr *exec.ExitError
	var startErr *exec.Error
	return errors.As(err, &exitErr) || errors.As(err, &startErr)
}
