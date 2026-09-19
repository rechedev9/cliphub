package editor

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

// ProgramAACFallbackMaster records the extra processing applied only when all
// ordinary masters failed. Acceptance still belongs to the decoded AAC.
type ProgramAACFallbackMaster struct {
	Encoder     string  `json:"encoder"`
	GainDB      float64 `json:"gain_db"`
	CeilingDBFS float64 `json:"ceiling_dbfs"`
}

func hasMediaFoundationAAC(ctx context.Context, ffmpeg string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	output, err := runFFmpegOutput(ctx, []string{ffmpeg, "-hide_banner", "-encoders"}, "AAC recovery encoder probe")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "aac_mf" {
			return true
		}
	}
	return false
}

func (m ProgramAACFallbackMaster) filter(base string) string {
	// Explicit oversampling and disabled automatic level compensation preserve
	// the peak ceiling; latency compensation preserves the approved timeline.
	return base + ",aresample=48000,aformat=channel_layouts=stereo,volume=" + decimal(m.GainDB) + "dB,aresample=192000,alimiter=limit=" + decimal(math.Pow(10, m.CeilingDBFS/20)) + ":attack=1:release=5:level=false:latency=true,aresample=48000"
}

func (m ProgramAACFallbackMaster) corrected(decoded LoudnessMeasurement, target recapplan.LoudnessOptions) ProgramAACFallbackMaster {
	m.GainDB = max(-3, min(6, m.GainDB+max(-1.5, min(1.5, target.TargetILUFS-*decoded.IntegratedLUFS))))
	if *decoded.TruePeakDBTP > target.TargetTPDBTP {
		m.CeilingDBFS = max(target.TargetTPDBTP-8, m.CeilingDBFS-min(2, *decoded.TruePeakDBTP-target.TargetTPDBTP+.2))
	}
	return m
}

// fullDemoAACRecoveryAttempt is one Media Foundation master and, once its
// candidate was measured, the decoded AAC that owns acceptance.
type fullDemoAACRecoveryAttempt struct {
	master  ProgramAACFallbackMaster
	decoded *LoudnessMeasurement
}

// fullDemoAACRecoveryResult is the complete outcome of the bounded recovery
// chain. The chain depends only on the first program measurement, never on the
// native masters, so it can be produced before the native masters finish and is
// folded into the evidence afterwards in the approved order.
type fullDemoAACRecoveryResult struct {
	unavailable bool
	attempts    []fullDemoAACRecoveryAttempt
	candidate   string
	cleanup     func()
	logs        []string
	err         error
	// delivery is the muxed and certified, but unpublished, delivery of the
	// accepted candidate when it was produced speculatively. deliveryErr is the
	// failure that stopped it; both are nil when no speculative delivery ran.
	delivery    *fullDemoAACDelivery
	deliveryErr error
}

func (r *fullDemoAACRecoveryResult) release() {
	r.delivery.release()
	r.delivery = nil
	if r.cleanup != nil {
		r.cleanup()
		r.cleanup = nil
	}
}

func runFullDemoAACRecovery(ctx context.Context, ffmpeg, input, output, logDir string, target recapplan.LoudnessOptions, duration float64, first LoudnessMeasurement, progress fullDemoProgress) (result fullDemoAACRecoveryResult) {
	if !hasMediaFoundationAAC(ctx, ffmpeg) {
		result.unavailable, result.err = true, ctx.Err()
		return result
	}
	// Keep the initial normalization fixed. Feeding lossy peak overshoot back
	// into loudnorm's target TP can lower the whole mix and defeat its LUFS
	// target. Here gain and the post-normalization limiter are independent.
	base, err := measuredLoudnessFilter(fullDemoAACRecoveryTarget(target), first)
	if err != nil {
		result.err = err
		return result
	}
	master := ProgramAACFallbackMaster{Encoder: "aac_mf", GainDB: 3, CeilingDBFS: target.TargetTPDBTP - 3.5}
	// Rebuild timestamps from the canonical sample clock as well as bounding
	// sample count: filter timestamps can otherwise extend the stream duration.
	// AAC packet padding is checked separately by delivery validation.
	samples := strconv.FormatInt(int64(math.Round(duration*recapplan.SampleRate)), 10)
	// Media Foundation reports a full 1024-sample duration for its padded final
	// packet. Mark only its real samples as playable, matching native AAC's
	// short final packet duration instead of extending the MP4 track.
	packetDuration := "setts=duration=min(DURATION\\,max(0\\," + samples + "/48000/TB-PTS))"
	for attempt := 0; attempt < 3; attempt++ {
		start := float64(attempt) * .26
		stage := fmt.Sprintf("Recuperando audio final (%d/3)", attempt+1)
		result.attempts = append(result.attempts, fullDemoAACRecoveryAttempt{master: master})
		candidate, candidateCleanup, err := fullDemoAudioCandidatePath(output)
		if err != nil {
			result.err = err
			return result
		}
		filter := master.filter(base) + ",apad=whole_len=" + samples + ",atrim=end_sample=" + samples + ",asetpts=N/SR/TB"
		command := fullDemoRecoveryCandidateCommand(ffmpeg, input, candidate, filter, master.Encoder, packetDuration)
		encodeLog := filepath.Join(logDir, fmt.Sprintf("program-aac-recovery-%d.txt", attempt))
		result.logs = append(result.logs, encodeLog)
		if err := runFFmpegWithOptionalLogAndProgress(fullDemoTimingStageVariant(ctx, "audio_candidate_encode", attempt, master.Encoder, "recovery"), command, "Full Demo AAC recovery", encodeLog, duration, progress.pass(stage, start, start+.13)); err != nil {
			candidateCleanup()
			result.err = fmt.Errorf("audio_loudness_failed: AAC recovery: %w", err)
			return result
		}
		decodedLog := filepath.Join(logDir, fmt.Sprintf("decoded-aac-recovery-%d.txt", attempt))
		result.logs = append(result.logs, decodedLog)
		decoded, err := measureLoudness(fullDemoTimingStageVariant(ctx, "audio_candidate_analysis", attempt, "aac_mf", "recovery"), ffmpeg, candidate, target, decodedLog, duration, progress.pass(fmt.Sprintf("Comprobando audio recuperado (%d/3)", attempt+1), start+.13, start+.26))
		if err != nil {
			candidateCleanup()
			result.err = err
			return result
		}
		result.attempts[attempt].decoded = &decoded
		accepted, err := fullDemoDecodedAACAccepted(decoded, target, false)
		if err != nil {
			candidateCleanup()
			result.err = err
			return result
		}
		if accepted {
			result.candidate, result.cleanup = candidate, candidateCleanup
			return result
		}
		candidateCleanup()
		master = master.corrected(decoded, target)
	}
	return result
}

func fullDemoAACRecoveryTarget(target recapplan.LoudnessOptions) recapplan.LoudnessOptions {
	target.TargetTPDBTP -= .3
	return target
}

// finishFullDemoAACRecovery folds a recovery result into the evidence exactly
// as the attempts would have been recorded one by one, then delivers the
// accepted candidate.
func finishFullDemoAACRecovery(ctx context.Context, ffmpeg string, video fullDemoProgramVideo, output, logDir string, target recapplan.LoudnessOptions, duration float64, e ProgramLoudnessEvidence, result fullDemoAACRecoveryResult, progress fullDemoProgress) (ProgramLoudnessEvidence, error) {
	defer result.release()
	if result.unavailable {
		if result.err != nil {
			return e, result.err
		}
		return e, fullDemoAACFailure("Media Foundation AAC recovery is unavailable after three masters", e)
	}
	for _, attempt := range result.attempts {
		e.MasterTargets = append(e.MasterTargets, fullDemoAACRecoveryTarget(target))
		e.FallbackMasters = append(e.FallbackMasters, attempt.master)
		if attempt.decoded != nil {
			e.DecodedAAC = append(e.DecodedAAC, *attempt.decoded)
		}
	}
	if result.err != nil {
		return e, result.err
	}
	if result.candidate == "" {
		return e, fullDemoAACFailure("approved targets remain unmet after three masters and three recovery attempts", e)
	}
	if result.deliveryErr != nil {
		return e, result.deliveryErr
	}
	// A speculative delivery already ran the very same mux and final
	// measurement while the remaining native masters were being evaluated, so
	// only the acceptance check and the atomic publication are left.
	if result.delivery != nil {
		delivery := *result.delivery
		result.delivery = nil
		progress.report("Publicando el audio recuperado", .82)
		evidence, err := commitFullDemoAACDelivery(ctx, delivery, output, target, false, e)
		if err == nil {
			progress.report("Publicando el audio recuperado", .99)
		}
		return evidence, err
	}
	program, err := video(ctx)
	if err != nil {
		return e, err
	}
	return deliverFullDemoAACCandidate(ctx, ffmpeg, program, result.candidate, output, logDir, target, false, duration, e, progress.pass("Publicando el audio recuperado", .82, .99))
}

func recoverFullDemoAAC(ctx context.Context, ffmpeg, input string, video fullDemoProgramVideo, output, logDir string, target recapplan.LoudnessOptions, duration float64, e ProgramLoudnessEvidence, progress fullDemoProgress) (ProgramLoudnessEvidence, error) {
	result := runFullDemoAACRecovery(ctx, ffmpeg, input, output, logDir, target, duration, e.Input, progress)
	return finishFullDemoAACRecovery(ctx, ffmpeg, video, output, logDir, target, duration, e, result, progress)
}

// fullDemoAACRecoverySpeculation runs the recovery chain, and the delivery of
// its accepted candidate, while the remaining native masters are still being
// tried. Native masters keep precedence: the result is only consumed after
// every native master failed, and is discarded, together with its candidate,
// its unpublished delivery attempt and its logs, as soon as one passes.
type fullDemoAACRecoverySpeculation struct {
	cancel context.CancelFunc
	done   chan struct{}
	result fullDemoAACRecoveryResult
}

func startFullDemoAACRecovery(ctx context.Context, ffmpeg, input string, video fullDemoProgramVideo, output, logDir string, target recapplan.LoudnessOptions, duration float64, first LoudnessMeasurement) *fullDemoAACRecoverySpeculation {
	// The speculation keeps the NORMAL priority class on purpose. It looks like
	// background work beside the native chain, but whenever every native master
	// is rejected (the saved replay, and every render that needs recovery) this
	// chain is the delivered master and therefore the critical path. Marking it
	// BELOW_NORMAL was measured: its chain grew from 95.6 s to 174.2 s and gave
	// back ~38 s of the upstream gain (see the audit doc, mastering chain pair).
	ctx, cancel := context.WithCancel(ctx)
	s := &fullDemoAACRecoverySpeculation{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(s.done)
		result := runFullDemoAACRecovery(ctx, ffmpeg, input, output, logDir, target, duration, first, nil)
		speculateFullDemoAACDelivery(ctx, &result, ffmpeg, video, output, logDir, target, duration)
		s.result = result
	}()
	return s
}

// speculateFullDemoAACDelivery muxes and certifies an accepted recovery
// candidate before it is known to be the delivered one. It publishes nothing
// and records no evidence: finishFullDemoAACRecovery still folds the attempts
// in the approved order and owns the acceptance check and the publication, and
// a native master that passes discards this attempt exactly as it discards the
// candidate. On the saved replay the recovery candidate is accepted 35 s before
// the native chain exhausts, and the mux plus the final certification are the
// 17 s that then ran serially in an otherwise idle machine.
func speculateFullDemoAACDelivery(ctx context.Context, result *fullDemoAACRecoveryResult, ffmpeg string, video fullDemoProgramVideo, output, logDir string, target recapplan.LoudnessOptions, duration float64) {
	if video == nil || result.err != nil || result.candidate == "" {
		return
	}
	result.logs = append(result.logs, filepath.Join(logDir, fullDemoFinalMuxLog), filepath.Join(logDir, fullDemoFinalAnalysisLog))
	program, err := video(ctx)
	if err != nil {
		result.deliveryErr = err
		return
	}
	delivery, err := prepareFullDemoAACDelivery(ctx, ffmpeg, program, result.candidate, output, logDir, target, duration, nil)
	if err != nil {
		result.deliveryErr = err
		return
	}
	result.delivery = &delivery
}

func (s *fullDemoAACRecoverySpeculation) wait() fullDemoAACRecoveryResult {
	<-s.done
	s.cancel()
	return s.result
}

func (s *fullDemoAACRecoverySpeculation) discard() {
	s.cancel()
	<-s.done
	s.result.release()
	for _, path := range s.result.logs {
		os.Remove(path)
	}
}

func fullDemoAACFailure(reason string, e ProgramLoudnessEvidence) error {
	var attempts []string
	for i, decoded := range e.DecodedAAC {
		if decoded.IntegratedLUFS != nil && decoded.TruePeakDBTP != nil {
			attempts = append(attempts, fmt.Sprintf("#%d %.2f LUFS / %.2f dBTP", i+1, *decoded.IntegratedLUFS, *decoded.TruePeakDBTP))
		}
	}
	return fmt.Errorf("audio_loudness_failed: %s; decoded AAC: %s", reason, strings.Join(attempts, "; "))
}
