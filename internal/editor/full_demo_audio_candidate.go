package editor

import (
	"context"
	"fmt"
	"math"
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/filecommit"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

// Full Demo audio retries only need the program's audio. Each candidate is an
// audio-only MP4 so a failed master never copies the multi-gigabyte H.264
// stream, while MP4 (unlike raw ADTS) keeps AAC priming, edit lists, the final
// packet duration correction and the exact playable sample duration that
// delivery validation re-derives from the muxed artifact.
//
// Only after a candidate passes the existing decoded-AAC contract is it muxed
// once with the untouched program video. The actual muxed AAC is measured again
// before the previous output is atomically replaced, so a failed candidate,
// mux or final certification can never publish over a good output.

// fullDemoNativeCandidateCommand builds an audio-only native AAC candidate
// command. It deliberately never maps or copies the program video, but keeps
// the existing loudnorm/arming filter chain, bitrate, channel layout, sample
// count trim and the native -t bound.
func fullDemoNativeCandidateCommand(ffmpeg, input, candidate, filter string, samples int64, duration float64) []string {
	return []string{
		ffmpeg, "-y", "-hide_banner", "-nostats", "-v", "info", "-i", input,
		"-map", "0:a:0",
		"-af", filter + fmt.Sprintf(",aresample=48000,aformat=channel_layouts=stereo,apad=whole_len=%d,atrim=end_sample=%d", samples, samples),
		"-c:a", "aac", "-b:a", "192k", "-ar", "48000", "-ac", "2",
		"-t", decimal(duration),
		"-movflags", "+faststart", candidate,
	}
}

// fullDemoRecoveryCandidateCommand builds an audio-only Media Foundation
// recovery candidate command. The setts packet-duration correction stays: it
// marks only the real samples of Media Foundation's padded final packet as
// playable, so the muxed track keeps the approved timeline.
func fullDemoRecoveryCandidateCommand(ffmpeg, input, candidate, filter, encoder, packetDuration string) []string {
	return []string{
		ffmpeg, "-y", "-hide_banner", "-nostats", "-v", "info", "-i", input,
		"-map", "0:a:0",
		"-af", filter,
		"-c:a", encoder, "-b:a", "192k", "-ar", "48000", "-ac", "2",
		"-bsf:a", packetDuration,
		"-movflags", "+faststart", candidate,
	}
}

// fullDemoFinalMuxCommand combines the untouched program video with a passing
// audio candidate in a single stream copy. It is the only command that writes
// the final output and the only place the complete video is copied.
func fullDemoFinalMuxCommand(ffmpeg, input, candidate, destination string) []string {
	return []string{
		ffmpeg, "-y", "-hide_banner", "-nostats", "-v", "info",
		"-i", input, "-i", candidate,
		"-map", "0:v:0", "-map", "1:a:0",
		"-c", "copy",
		"-movflags", "+faststart", destination,
	}
}

// fullDemoAudioCandidatePath reserves a unique audio-only sibling of the final
// output. Every attempt owns its own path and removes it afterwards, so a
// failed attempt can never be mistaken for a delivered candidate.
func fullDemoAudioCandidatePath(output string) (string, func(), error) {
	path, cleanup, err := filecommit.Attempt(filepath.Join(filepath.Dir(output), "full-demo-audio-candidate.m4a"))
	if err != nil {
		return "", nil, fmt.Errorf("full_demo_output_invalid: audio candidate attempt: %w", err)
	}
	return path, cleanup, nil
}

// fullDemoDecodedAACAccepted applies the existing approved contract to a
// decoded AAC measurement. A measurement that cannot be used at all is a
// terminal error; a usable measurement that misses its targets is retryable.
// Silent approval is only valid for a deliberately muted program.
func fullDemoDecodedAACAccepted(decoded LoudnessMeasurement, target recapplan.LoudnessOptions, silentApproved bool) (bool, error) {
	if decoded.Status == "silent" {
		if silentApproved {
			return true, nil
		}
		return false, fmt.Errorf("audio_loudness_failed: final AAC is not measurable")
	}
	if decoded.Status != "measured" || decoded.IntegratedLUFS == nil || decoded.TruePeakDBTP == nil {
		return false, fmt.Errorf("audio_loudness_failed: final AAC is not measurable")
	}
	return math.Abs(*decoded.IntegratedLUFS-target.TargetILUFS) <= 0.5 && *decoded.TruePeakDBTP <= target.TargetTPDBTP, nil
}

// publishFullDemoAAC rechecks cancellation immediately before the atomic
// replace. A context cancelled after the final measurement passed but before
// publication must never overwrite a previously good output.
func publishFullDemoAAC(ctx context.Context, attempt, output string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := filecommit.Commit(attempt, output); err != nil {
		return fmt.Errorf("full_demo_output_invalid: publish final audio: %w", err)
	}
	return nil
}

// The evidence logs of the final mux and the final certification. They are
// named here because a speculative delivery writes them before its candidate is
// known to be the delivered one, and a discarded speculation must remove them.
const (
	fullDemoFinalMuxLog      = "program-final-mux.txt"
	fullDemoFinalAnalysisLog = "decoded-aac-final.txt"
)

// fullDemoAACDelivery is a muxed and measured, but not yet published, delivery
// attempt. Splitting the delivery at this point lets an accepted candidate be
// muxed and certified before it is known to be the delivered one, while
// publication, the acceptance check and the evidence stay in one place.
type fullDemoAACDelivery struct {
	attempt     string
	cleanup     func()
	measurement LoudnessMeasurement
}

func (d *fullDemoAACDelivery) release() {
	if d != nil && d.cleanup != nil {
		d.cleanup()
		d.cleanup = nil
	}
}

// prepareFullDemoAACDelivery muxes a passing audio candidate once with the
// program video and measures the actual muxed AAC. It never publishes, so its
// attempt can be thrown away without ever having touched output.
func prepareFullDemoAACDelivery(ctx context.Context, ffmpeg, input, candidate, output, logDir string, target recapplan.LoudnessOptions, duration float64, onFraction func(float64)) (fullDemoAACDelivery, error) {
	if err := ctx.Err(); err != nil {
		return fullDemoAACDelivery{}, err
	}
	attempt, cleanup, err := filecommit.Attempt(output)
	if err != nil {
		return fullDemoAACDelivery{}, fmt.Errorf("full_demo_output_invalid: final audio attempt: %w", err)
	}
	delivery := fullDemoAACDelivery{attempt: attempt, cleanup: cleanup}
	var progress fullDemoProgress
	if onFraction != nil {
		progress = func(_ string, fraction float64) {
			onFraction(min(1, max(0, fraction)))
		}
	}
	mux := fullDemoFinalMuxCommand(ffmpeg, input, candidate, attempt)
	if err := runFFmpegWithOptionalLogAndProgress(fullDemoTimingStage(ctx, "final_mux", -1), mux, "Full Demo final audio mux", filepath.Join(logDir, fullDemoFinalMuxLog), duration, progress.pass("Publicando el audio verificado", 0, .5)); err != nil {
		delivery.release()
		return fullDemoAACDelivery{}, err
	}
	delivered, err := measureLoudness(fullDemoTimingStage(ctx, "final_audio_analysis", -1), ffmpeg, attempt, target, filepath.Join(logDir, fullDemoFinalAnalysisLog), duration, progress.pass("Certificando el audio publicado", .5, .99))
	if err != nil {
		delivery.release()
		return fullDemoAACDelivery{}, err
	}
	delivery.measurement = delivered
	return delivery, nil
}

// commitFullDemoAACDelivery applies the approved contract to the actual muxed
// AAC and only then atomically replaces output. A measurement that misses its
// targets or a context cancelled before publication leaves any previous output
// untouched and removes the attempt.
func commitFullDemoAACDelivery(ctx context.Context, delivery fullDemoAACDelivery, output string, target recapplan.LoudnessOptions, silentApproved bool, e ProgramLoudnessEvidence) (ProgramLoudnessEvidence, error) {
	defer delivery.release()
	delivered := delivery.measurement
	accepted, err := fullDemoDecodedAACAccepted(delivered, target, silentApproved)
	if err != nil {
		return e, err
	}
	if !accepted {
		return e, fmt.Errorf("audio_loudness_failed: final muxed AAC misses its approved targets")
	}
	if err := publishFullDemoAAC(ctx, delivery.attempt, output); err != nil {
		return e, err
	}
	e.FinalMuxedAAC = &delivered
	if delivered.Status == "silent" {
		e.Status = "silent-approved"
	} else {
		e.Status = "verified-decoded-aac"
	}
	return e, nil
}

// deliverFullDemoAACCandidate muxes a passing audio candidate once with the
// program video, measures the actual muxed AAC and only then atomically
// replaces output. A failed mux, a failed final measurement or a context
// cancelled at any point before publication leaves any previous output
// untouched and removes its own attempt.
func deliverFullDemoAACCandidate(ctx context.Context, ffmpeg, input, candidate, output, logDir string, target recapplan.LoudnessOptions, silentApproved bool, duration float64, e ProgramLoudnessEvidence, onFraction func(float64)) (ProgramLoudnessEvidence, error) {
	delivery, err := prepareFullDemoAACDelivery(ctx, ffmpeg, input, candidate, output, logDir, target, duration, onFraction)
	if err != nil {
		return e, err
	}
	return commitFullDemoAACDelivery(ctx, delivery, output, target, silentApproved, e)
}
