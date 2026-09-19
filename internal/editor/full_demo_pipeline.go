package editor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rechedev9/cliphub/internal/composition"
	"github.com/rechedev9/cliphub/internal/filecommit"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

// prepareFullDemoCompilation prepares both halves of the program: the video
// items with their concat list, and the mixed lossless program audio. The two
// branches share no data, so they run concurrently; the first error cancels the
// other. Production additionally masters the audio inside the audio branch (see
// renderFullDemoProgram); this staged form stops before assembly and mastering.
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

// runFullDemoBranches runs the independent video and audio branches under one
// cancellation scope and returns the first real failure, not the cancellation it
// caused in the other branch.
func runFullDemoBranches(ctx context.Context, video, audio func(context.Context) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		once     sync.Once
		firstErr error
	)
	fail := func(err error) {
		if err != nil {
			once.Do(func() {
				firstErr = err
				cancel()
			})
		}
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		fail(video(ctx))
	}()
	fail(audio(ctx))
	<-done
	return firstErr
}

// fullDemoBranchProgress folds the concurrent video and audio branches into one
// monotonic fraction. Each branch owns half of the range; the stage text follows
// whichever branch reported last.
type fullDemoBranchProgress struct {
	mu        sync.Mutex
	parent    fullDemoProgress
	fractions [2]float64
	reported  float64
}

func newFullDemoBranchProgress(parent fullDemoProgress) *fullDemoBranchProgress {
	return &fullDemoBranchProgress{parent: parent}
}

func (p *fullDemoBranchProgress) video() fullDemoProgress { return p.branch(0) }
func (p *fullDemoBranchProgress) audio() fullDemoProgress { return p.branch(1) }

func (p *fullDemoBranchProgress) branch(index int) fullDemoProgress {
	if p.parent == nil {
		return nil
	}
	return func(stage string, fraction float64) {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.fractions[index] = max(p.fractions[index], min(1, max(0, fraction)))
		p.reported = max(p.reported, (p.fractions[0]+p.fractions[1])/2)
		p.parent(stage, p.reported)
	}
}

// prepareFullDemoVideoItems encodes every timeline item's video and writes the
// program concat list. It needs neither voice tracks nor mixed audio.
func prepareFullDemoVideoItems(ctx context.Context, short *ShortEdit, progress fullDemoProgress) error {
	paths, err := runFullDemoItemPool(ctx, *short, fullDemoItemVideoOnly, progress)
	if err != nil {
		return err
	}
	short.fullDemo.preparedInputs = paths
	if err := writeFullDemoItemConcatList(*short, fullDemoConcatListPath(*short), paths); err != nil {
		return err
	}
	// The program command was built before preparation against the raw-part
	// list. Rebuild it now that the prepared item list is in place, so the
	// item/program overlay decision comes from the same plan and the copy path
	// never runs over unprepared parts.
	short.FFmpegCommand = BuildFFmpegCommand(short.fullDemo.ffmpeg, *short)
	return nil
}

// fullDemoProgramAudio is the outcome of the program-audio assembly: the
// committed lossless program plus the first loudness measurement, produced by
// the assembly process itself from the same decoded samples instead of by a
// second full decode of the written file.
type fullDemoProgramAudio struct {
	// Measured is false when the assembly produced no parseable measurement;
	// mastering then runs its own measurement pass exactly as before.
	Measured    bool
	Measurement LoudnessMeasurement
	// Target is the loudness target the fused filter used. Mastering reuses the
	// measurement only for the identical target.
	Target recapplan.LoudnessOptions
	// Output is the assembly process's FFmpeg output, which carries the same
	// loudnorm JSON block a standalone measurement would have written, so the
	// program-input-loudness evidence log keeps its path and its measurement.
	Output string
}

// prepareFullDemoProgramAudio prepares the voice buses, mixes every timeline
// item's audio and joins the items into the lossless program audio that owns
// mastering. Generated buses and item audio are released as soon as the program
// audio is committed. The assembly process also measures the program loudness
// it is writing, so mastering starts from a measurement instead of decoding the
// whole program again.
func prepareFullDemoProgramAudio(ctx context.Context, short *ShortEdit, progress fullDemoProgress) (fullDemoProgramAudio, error) {
	target := short.FullDemo.Effective.Options.Audio.Loudness
	measured := fullDemoProgramAudio{Target: target}
	if err := prepareFullDemoTracks(ctx, short, progress.within(0, .6)); err != nil {
		return measured, err
	}
	paths, err := runFullDemoItemPool(ctx, *short, fullDemoItemAudioOnly, progress.within(.6, .9))
	if err != nil {
		return measured, err
	}
	if err := releaseFullDemoAudio(*short); err != nil {
		return measured, err
	}
	list := filepath.Join(short.fullDemo.workDir, "concat-audio-list.txt")
	if err := writeFullDemoItemConcatList(*short, list, paths); err != nil {
		return measured, err
	}
	timeline := short.FullDemo.Effective.Timeline
	duration := float64(timeline[len(timeline)-1].EndFrame) / recapplan.OutputFPS
	destination := fullDemoProgramAudioPath(*short)
	command := buildFullDemoProgramAudioMeasuredCommand(short.fullDemo.ffmpeg, list, destination, target)
	output, err := runFullDemoAtomicMeasuredWithProgress(fullDemoTimingScope(ctx, "audio_assembly", -1, -1, duration), command, "Full Demo program audio", filepath.Join(short.fullDemo.workDir, "program-audio.log"), destination, duration, progress.pass("Ensamblando audio completo", .9, 1))
	if err != nil {
		return measured, err
	}
	// A missing or unparseable measurement is never fatal here: mastering owns
	// the loudness gate and falls back to its own measurement pass.
	if measurement, parseErr := parseLoudnessMeasurement(output); parseErr == nil {
		measured.Measured, measured.Measurement, measured.Output = true, measurement, output
	}
	return measured, removeFullDemoTemporaryFiles(short.fullDemo.workDir, append(paths, list))
}

// buildFullDemoProgramAudioCommand joins the prepared item audio into the
// program audio. PCM remains lossless until the full-program master. It is the
// assembly without the fused measurement and stays the equivalence reference
// for the written program audio.
func buildFullDemoProgramAudioCommand(ffmpeg, list, destination string) []string {
	return []string{ffmpeg, "-y", "-v", "error", "-f", "concat", "-safe", "0", "-i", list, "-map", "0:a:0", "-c:a", "pcm_f32le", "-ar", "48000", "-ac", "2", destination}
}

// buildFullDemoProgramAudioMeasuredCommand is the assembly command with the
// program loudness measurement folded in as a second, discarded output of the
// same process. The measurement output is declared first so the committed
// program audio stays the command's last argument, and it uses the filter
// measureLoudness uses, over the same decoded samples, so the parsed
// measurement is the one a standalone pass over the written file produces.
// -v info is required for loudnorm to print its JSON block at all; it changes
// no encoder option, so the written PCM is unchanged.
func buildFullDemoProgramAudioMeasuredCommand(ffmpeg, list, destination string, target recapplan.LoudnessOptions) []string {
	return []string{ffmpeg, "-y", "-hide_banner", "-nostats", "-v", "info", "-f", "concat", "-safe", "0", "-i", list,
		"-map", "0:a:0", "-vn", "-af", loudnessFilter(target) + ":print_format=json", "-f", "null", "-",
		"-map", "0:a:0", "-c:a", "pcm_f32le", "-ar", "48000", "-ac", "2", destination}
}

// runFullDemoAtomicMeasuredWithProgress is runFFmpegAtomicWithProgress with
// FFmpeg's output returned: the fused assembly carries its loudness
// measurement there. Atomic commit, the failure log and progress reporting are
// unchanged.
func runFullDemoAtomicMeasuredWithProgress(ctx context.Context, command []string, label, logPath, destination string, expectedDurationSec float64, onFraction func(float64)) (string, error) {
	if len(command) == 0 || destination == "" {
		return "", fmt.Errorf("%s output path is required", label)
	}
	attempt, cleanup, err := filecommit.Attempt(destination)
	if err != nil {
		return "", fmt.Errorf("%s attempt: %w", label, err)
	}
	defer cleanup()
	attemptCommand := append([]string(nil), command...)
	if attemptCommand[len(attemptCommand)-1] != destination {
		return "", fmt.Errorf("%s command output does not match destination", label)
	}
	attemptCommand[len(attemptCommand)-1] = attempt
	output, err := runFFmpegOutputProgress(ctx, attemptCommand, label, expectedDurationSec, onFraction)
	if err != nil {
		// The failure log keeps the shape runFFmpegAtomicWithProgress gives it
		// under progress reporting: the label, the exit status and FFmpeg's
		// trimmed output, all of which the error already carries.
		if logPath != "" {
			_ = writeLogFile(logPath, err.Error()+"\n")
		}
		return output, err
	}
	if err := filecommit.Commit(attempt, destination); err != nil {
		return output, fmt.Errorf("%s publish: %w", label, err)
	}
	return output, nil
}

func writeFullDemoItemConcatList(short ShortEdit, listPath string, paths []string) error {
	var list strings.Builder
	list.WriteString("ffconcat version 1.0\n")
	for i, path := range paths {
		item := short.FullDemo.Effective.Timeline[i]
		list.WriteString(composition.ConcatFileLine(path))
		// NUT's last packet timestamp is not the duration of a complete CFR
		// interval. Declare the canonical duration so concat never loses one
		// frame at each join or advances the next audio bus too early.
		fmt.Fprintf(&list, "duration %.9f\n", float64(item.EndFrame-item.StartFrame)/recapplan.OutputFPS)
	}
	return os.WriteFile(listPath, []byte(list.String()), 0600)
}

// runFullDemoItemPool renders one stream kind of every timeline item in a
// bounded worker pool and returns the committed paths in timeline order. Each
// stream kind owns its budget: video items are encoder-bound, audio items are
// pure filter graphs (see fullDemoItemPoolJobs).
func runFullDemoItemPool(ctx context.Context, short ShortEdit, streams fullDemoItemStreams, progress fullDemoProgress) ([]string, error) {
	return runFullDemoItemPoolJobs(ctx, short, streams, progress, fullDemoItemPoolJobs(streams))
}

// fullDemoItemPoolJobs is the worker budget of one item stream kind.
func fullDemoItemPoolJobs(streams fullDemoItemStreams) int {
	if streams == fullDemoItemAudioOnly {
		return fullDemoAudioItemJobs()
	}
	return fullDemoItemJobs()
}

// fullDemoItemPoolBackground reports whether one item stream kind runs off the
// audio critical path. Only the video-only stream does: it is the render's
// widest CPU consumer and its branch finishes long before mastering does, so it
// is the work that can afford to yield. Audio-only items are a link of the
// audio critical path, and the muxed form carries that same audio, so both keep
// normal priority.
func fullDemoItemPoolBackground(streams fullDemoItemStreams) bool {
	return streams == fullDemoItemVideoOnly
}

func runFullDemoItemPoolJobs(ctx context.Context, short ShortEdit, streams fullDemoItemStreams, progress fullDemoProgress, jobs int) ([]string, error) {
	if jobs < 1 {
		jobs = 1
	}
	pattern, timingStage, label, stageText := "item-%03d", "items", "Full Demo timeline item", "Montando corte %d de %d"
	if streams == fullDemoItemAudioOnly {
		pattern, timingStage, label, stageText = "item-%03d-audio", "items_audio", "Full Demo timeline item audio", "Mezclando corte %d de %d"
	}
	if fullDemoItemPoolBackground(streams) {
		ctx = withBackgroundProcessPriority(ctx)
	}
	// Item video no longer waits for voice preparation to create the directory.
	if err := os.MkdirAll(short.fullDemo.workDir, 0700); err != nil {
		return nil, err
	}
	timeline := short.FullDemo.Effective.Timeline
	paths := make([]string, len(timeline))
	tracker := &fullDemoItemProgress{
		fractions: make([]float64, len(timeline)),
		done:      make([]bool, len(timeline)),
		weights:   make([]float64, len(timeline)),
		total:     float64(timeline[len(timeline)-1].EndFrame),
		progress:  progress,
	}
	for i, item := range timeline {
		tracker.weights[i] = float64(item.EndFrame - item.StartFrame)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		wg       sync.WaitGroup
		once     sync.Once
		firstErr error
	)
	sem := make(chan struct{}, jobs)
	for i, item := range timeline {
		path := filepath.Join(short.fullDemo.workDir, fmt.Sprintf(pattern+".nut", i))
		command, err := fullDemoItemStreamCommand(short, item, path, streams)
		if err != nil {
			cancel()
			wg.Wait()
			return nil, err
		}
		paths[i] = path
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, item recapplan.TimelineItem, command []string, path string) {
			defer wg.Done()
			defer func() { <-sem }()
			stage := fmt.Sprintf(stageText, i+1, len(timeline))
			onFraction := func(fraction float64) { tracker.set(i, stage, fraction) }
			duration := float64(item.EndFrame-item.StartFrame) / recapplan.OutputFPS
			itemCtx := fullDemoTimingScope(ctx, timingStage, i, -1, duration)
			if err := runFFmpegAtomicWithProgress(itemCtx, command, label, filepath.Join(short.fullDemo.workDir, fmt.Sprintf(pattern+".log", i)), path, duration, onFraction); err != nil {
				once.Do(func() {
					firstErr = err
					cancel()
				})
				return
			}
			tracker.markDone(i, stage)
		}(i, item, command, path)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return paths, nil
}

// renderFullDemoProgram renders the program video and masters the program
// audio concurrently, then muxes the passing AAC candidate with the committed
// video. Video items start immediately instead of waiting for voice
// preparation, and every loudness pass and AAC candidate overlaps the item
// encodes instead of following them. The candidate sequence, acceptance rules
// and evidence are those of masterFullDemoSplitProgram, unchanged. The returned
// evidence is nil when the render failed before mastering started.
func (p *shortPackRenderer) renderFullDemoProgram(ctx context.Context, i int, short *ShortEdit, duration float64, progress fullDemoProgress) (*ProgramLoudnessEvidence, error) {
	if err := prepareFullDemoTransitions(ctx, short); err != nil {
		return nil, err
	}
	branches := newFullDemoBranchProgress(progress)
	program := fullDemoProgramPath(*short)
	ready := make(chan struct{})
	var videoErr error
	video := func(ctx context.Context) error {
		defer close(ready)
		videoErr = func() error {
			progress := branches.video()
			if err := prepareFullDemoVideoItems(ctx, short, progress.within(0, .9)); err != nil {
				return err
			}
			// Preparation can rebuild short.FFmpegCommand (Full Demo overlay
			// consolidation switches the program to a video copy path). The result
			// clone was taken before preparation, so refresh it before assembly:
			// otherwise shorts-result.json and reuse validation would report a
			// legacy re-encode that never ran, including when assembly later fails.
			p.copyPreparedCommand(i, short)
			if err := runFFmpegAtomicWithProgress(fullDemoTimingScope(ctx, "assembly", i, -1, duration), short.FFmpegCommand, "short edit", short.RenderLogPath, program, duration, progress.pass("Ensamblando vídeo completo", .9, 1)); err != nil {
				return err
			}
			return releaseFullDemoItems(*short)
		}()
		return videoErr
	}
	committedVideo := func(ctx context.Context) (string, error) {
		select {
		case <-ready:
			return program, videoErr
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	// The audio branch works on its own copy of the short: the video branch
	// rebuilds short.FFmpegCommand, and both share only the render context and
	// evidence pointers, where they touch disjoint fields.
	audioShort := *short
	var evidence *ProgramLoudnessEvidence
	audio := func(ctx context.Context) error {
		progress := branches.audio()
		assembled, err := prepareFullDemoProgramAudio(ctx, &audioShort, progress.within(0, .25))
		if err != nil {
			return err
		}
		options := audioShort.FullDemo.Effective.Options
		silentApproved := options.Audio.Game.Gain == 0 && (!options.Audio.Voice.Enabled || options.Audio.Voice.Gain == 0) && !options.Audio.Music.Enabled && !options.Sponsor.Enabled && !audioShort.FullDemo.Effective.HasTransitionSFX()
		mastered, err := masterFullDemoMeasuredProgram(fullDemoTimingScope(ctx, "full_demo", i, -1, duration), audioShort.fullDemo.ffmpeg, fullDemoProgramAudioPath(audioShort), committedVideo, audioShort.Output, filepath.Join(p.opts.OutputDir, "logs"), options.Audio.Loudness, silentApproved, duration, progress.within(.25, 1), assembled)
		evidence = &mastered
		return err
	}
	err := runFullDemoBranches(ctx, video, audio)
	return evidence, err
}
