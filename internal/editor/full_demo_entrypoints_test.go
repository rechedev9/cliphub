package editor

// Test entry points into the Full Demo pipeline. Each one fixes an argument
// that production passes explicitly, so tests drive the same code paths.

import (
	"context"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

// prepareFullDemoCompilation runs the video and audio branches of
// renderFullDemoProgram but stops before assembly and mastering.
func prepareFullDemoCompilation(ctx context.Context, short *ShortEdit, progress fullDemoProgress) error {
	if short.fullDemo == nil {
		return nil
	}
	if err := prepareFullDemoTransitions(ctx, short); err != nil {
		return err
	}
	branches := newFullDemoBranchProgress(progress)
	// The audio branch works on its own copy of the short: the video branch
	// rebuilds short.FFmpegCommand, and both share only the render context and
	// evidence pointers, where they touch disjoint fields.
	audioShort := *short
	return runFullDemoBranches(ctx,
		func(ctx context.Context) error { return prepareFullDemoVideoItems(ctx, short, branches.video()) },
		func(ctx context.Context) error {
			// The staged form stops before mastering, so the program measurement
			// the assembly produced has no consumer here.
			_, err := prepareFullDemoProgramAudio(ctx, &audioShort, branches.audio())
			return err
		},
	)
}

// masterFullDemoProgram masters a program whose video is already committed.
func masterFullDemoProgram(ctx context.Context, ffmpeg, input, output, logDir string, target recapplan.LoudnessOptions, silentApproved bool, duration float64, progress fullDemoProgress) (ProgramLoudnessEvidence, error) {
	return masterFullDemoSplitProgram(ctx, ffmpeg, input, committedFullDemoProgramVideo(input), output, logDir, target, silentApproved, duration, progress)
}

// masterFullDemoSplitProgram masters input, the lossless program audio, and
// muxes the passing candidate with the separately produced program video.
func masterFullDemoSplitProgram(ctx context.Context, ffmpeg, input string, video fullDemoProgramVideo, output, logDir string, target recapplan.LoudnessOptions, silentApproved bool, duration float64, progress fullDemoProgress) (ProgramLoudnessEvidence, error) {
	return masterFullDemoMeasuredProgram(ctx, ffmpeg, input, video, output, logDir, target, silentApproved, duration, progress, fullDemoProgramAudio{})
}

// recoverFullDemoAAC runs the recovery chain serially, without the speculation.
func recoverFullDemoAAC(ctx context.Context, ffmpeg, input string, video fullDemoProgramVideo, output, logDir string, target recapplan.LoudnessOptions, duration float64, e ProgramLoudnessEvidence, progress fullDemoProgress) (ProgramLoudnessEvidence, error) {
	result := runFullDemoAACRecovery(ctx, ffmpeg, input, output, logDir, target, duration, e.Input, progress)
	return finishFullDemoAACRecovery(ctx, ffmpeg, video, output, logDir, target, duration, e, result, progress)
}

// verifyFullDemoDelivery is the strict delivery decode without diagnostics.
func verifyFullDemoDelivery(ctx context.Context, ffmpeg, ffprobe, path string, frames int64, progress fullDemoProgress) (*FullDemoDeliveryEvidence, error) {
	outcome, err := verifyFullDemoDeliveryWithDiagnostics(ctx, ffmpeg, ffprobe, path, frames, progress, nil)
	if err != nil {
		return nil, err
	}
	return outcome.Evidence, nil
}

// fullDemoRoundAudio uses one canonical frame/sample window for every bus.
// All mixing is unnormalized float audio; the full program owns mastering.
func fullDemoRoundAudio(options recapplan.AudioOptions, gameStartSample, samples int64, voiceCount int) string {
	return fullDemoRoundAudioWithTransitions(options, gameStartSample, samples, voiceCount, fullDemoTransitionEdges{})
}

func fullDemoItemCommand(short ShortEdit, item recapplan.TimelineItem, output string) ([]string, error) {
	return fullDemoItemStreamCommand(short, item, output, fullDemoItemMuxed)
}

func fullDemoItemVideoCommand(short ShortEdit, item recapplan.TimelineItem, output string) ([]string, error) {
	return fullDemoItemStreamCommand(short, item, output, fullDemoItemVideoOnly)
}

func fullDemoItemAudioCommand(short ShortEdit, item recapplan.TimelineItem, output string) ([]string, error) {
	return fullDemoItemStreamCommand(short, item, output, fullDemoItemAudioOnly)
}

// fullDemoTimingStageEncoder is fullDemoTimingStage with an explicit encoder.
func fullDemoTimingStageEncoder(ctx context.Context, stage string, attempt int, encoder string) context.Context {
	return fullDemoTimingStageVariant(ctx, stage, attempt, encoder, "")
}

// buildFullDemoProgramAudioCommand is the assembly without the fused
// measurement: the equivalence reference for the written program audio.
func buildFullDemoProgramAudioCommand(ffmpeg, list, destination string) []string {
	return []string{ffmpeg, "-y", "-v", "error", "-f", "concat", "-safe", "0", "-i", list, "-map", "0:a:0", "-c:a", "pcm_f32le", "-ar", "48000", "-ac", "2", destination}
}
