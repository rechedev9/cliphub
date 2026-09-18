package editor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

// The split video and audio item processes must produce exactly the frames and
// samples of the single muxed process they replace, for rounds with voices,
// comms tails, transitions and SFX as well as for the sponsor insert.
func TestFullDemoSplitItemStreamsMatchMuxedItem(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	dir := t.TempDir()
	short, document, _ := fullDemoTransitionCanaryShort(t, ctx, ffmpeg, dir)
	if err := prepareFullDemoTransitions(ctx, &short); err != nil {
		t.Fatal(err)
	}
	if err := prepareFullDemoTracks(ctx, &short, nil); err != nil {
		t.Fatal(err)
	}
	for i, item := range document.Timeline {
		outputs := map[fullDemoItemStreams]string{}
		for streams, name := range map[fullDemoItemStreams]string{fullDemoItemMuxed: "muxed", fullDemoItemVideoOnly: "video", fullDemoItemAudioOnly: "audio"} {
			output := filepath.Join(short.fullDemo.workDir, fmt.Sprintf("%s-%03d.nut", name, i))
			command, err := fullDemoItemStreamCommand(short, item, output, streams)
			if err != nil {
				t.Fatal(err)
			}
			joined := strings.Join(command, " ")
			if streams == fullDemoItemVideoOnly && (strings.Contains(joined, "-c:a") || strings.Contains(joined, ".wav")) {
				t.Fatalf("item %d video command still depends on audio:\n%s", i, joined)
			}
			if streams == fullDemoItemAudioOnly && (strings.Contains(joined, "-c:v") || strings.Contains(joined, "[0:v]")) {
				t.Fatalf("item %d audio command still renders video:\n%s", i, joined)
			}
			if _, err := runFFmpegOutput(ctx, command, "split item "+name); err != nil {
				t.Fatal(err)
			}
			outputs[streams] = output
		}
		wantPCM := fullDemoDecodePCM(t, ctx, ffmpeg, outputs[fullDemoItemMuxed])
		if want := int(item.EndSample-item.StartSample) * 4 * 2; len(wantPCM) != want {
			t.Fatalf("item %d muxed PCM bytes = %d, want %d", i, len(wantPCM), want)
		}
		if string(fullDemoDecodePCM(t, ctx, ffmpeg, outputs[fullDemoItemAudioOnly])) != string(wantPCM) {
			t.Fatalf("item %d audio-only samples differ from the muxed item", i)
		}
		wantFrames := fullDemoFrameHashes(t, ctx, ffmpeg, []string{"-i", outputs[fullDemoItemMuxed], "-map", "0:v:0"})
		gotFrames := fullDemoFrameHashes(t, ctx, ffmpeg, []string{"-i", outputs[fullDemoItemVideoOnly], "-map", "0:v:0"})
		if int64(len(wantFrames)) != item.EndFrame-item.StartFrame || !slices.Equal(gotFrames, wantFrames) {
			t.Fatalf("item %d video-only frames differ from the muxed item: %d vs %d", i, len(gotFrames), len(wantFrames))
		}
	}
}

func TestFullDemoBranchesReturnTheFirstRealFailure(t *testing.T) {
	failure := errors.New("audio_loudness_failed: test")
	cancelled := make(chan struct{})
	err := runFullDemoBranches(context.Background(),
		func(ctx context.Context) error {
			<-ctx.Done()
			close(cancelled)
			return ctx.Err()
		},
		func(context.Context) error { return failure },
	)
	if !errors.Is(err, failure) {
		t.Fatalf("branch error = %v, want the audio failure instead of the cancellation it caused", err)
	}
	select {
	case <-cancelled:
	default:
		t.Fatal("video branch was not cancelled and awaited")
	}
	if err := runFullDemoBranches(context.Background(), func(context.Context) error { return nil }, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestFullDemoBranchProgressIsMonotonicAndShared(t *testing.T) {
	var mu sync.Mutex
	var reported []float64
	branches := newFullDemoBranchProgress(func(_ string, fraction float64) {
		mu.Lock()
		defer mu.Unlock()
		reported = append(reported, fraction)
	})
	video, audio := branches.video(), branches.audio()
	video("v", .5)
	audio("a", 1)
	audio("a", .2)
	video("v", 1)
	if want := []float64{.25, .75, .75, 1}; !slices.Equal(reported, want) {
		t.Fatalf("branch progress = %v, want %v", reported, want)
	}
	if newFullDemoBranchProgress(nil).video() != nil {
		t.Fatal("nil parent progress must stay nil")
	}
}

// The program-audio assembly must write exactly the bytes the unfused assembly
// wrote and, in the same process, produce the measurement a standalone
// measureLoudness pass over that written file produces.
func TestFullDemoProgramAudioAssemblyMeasuresWhatItWrites(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	dir := t.TempDir()
	short, document, options := fullDemoTransitionCanaryShort(t, ctx, ffmpeg, dir)
	target := options.Audio.Loudness
	if err := prepareFullDemoTransitions(ctx, &short); err != nil {
		t.Fatal(err)
	}
	if err := prepareFullDemoTracks(ctx, &short, nil); err != nil {
		t.Fatal(err)
	}
	paths, err := runFullDemoItemPool(ctx, short, fullDemoItemAudioOnly, nil)
	if err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(short.fullDemo.workDir, "concat-audio-reference.txt")
	if err := writeFullDemoItemConcatList(short, list, paths); err != nil {
		t.Fatal(err)
	}
	duration := float64(document.Timeline[len(document.Timeline)-1].EndFrame) / recapplan.OutputFPS
	reference := filepath.Join(dir, "reference-program-audio.nut")
	if _, err := runFFmpegOutput(ctx, buildFullDemoProgramAudioCommand(ffmpeg, list, reference), "reference assembly"); err != nil {
		t.Fatal(err)
	}
	fused := filepath.Join(dir, "fused-program-audio.nut")
	fusedCommand := buildFullDemoProgramAudioMeasuredCommand(ffmpeg, list, fused, target)
	if fusedCommand[len(fusedCommand)-1] != fused {
		t.Fatalf("fused assembly must keep its committed output last: %v", fusedCommand)
	}
	output, err := runFFmpegOutput(ctx, fusedCommand, "fused assembly")
	if err != nil {
		t.Fatal(err)
	}
	referenceBytes, err := os.ReadFile(reference)
	if err != nil {
		t.Fatal(err)
	}
	fusedBytes, err := os.ReadFile(fused)
	if err != nil {
		t.Fatal(err)
	}
	if string(referenceBytes) != string(fusedBytes) {
		t.Fatalf("fused assembly wrote %d bytes, unfused assembly wrote %d; the program audio must stay bit-exact", len(fusedBytes), len(referenceBytes))
	}
	inProcess, err := parseLoudnessMeasurement(output)
	if err != nil {
		t.Fatalf("fused assembly produced no parseable measurement: %v", err)
	}
	standalone, err := measureLoudness(ctx, ffmpeg, fused, target, filepath.Join(dir, "standalone-loudness.txt"), duration, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !fullDemoTimingMeasurementEqual(inProcess, standalone) || inProcess.Status != standalone.Status {
		t.Fatalf("fused measurement %+v differs from the standalone measurement %+v of the file it wrote", inProcess, standalone)
	}
	for _, pair := range [][2]*float64{{inProcess.IntegratedLUFS, standalone.IntegratedLUFS}, {inProcess.TruePeakDBTP, standalone.TruePeakDBTP}, {inProcess.LRA, standalone.LRA}, {inProcess.Threshold, standalone.Threshold}, {inProcess.Offset, standalone.Offset}} {
		if pair[0] == nil || pair[1] == nil || *pair[0] != *pair[1] {
			t.Fatalf("fused and standalone measurement fields are not identical: %+v vs %+v", inProcess, standalone)
		}
	}

	// The production path must return that same measurement for the program
	// audio it commits, so mastering can skip its own first decode.
	assembled, err := prepareFullDemoProgramAudio(ctx, &short, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !assembled.Measured {
		t.Fatal("program audio assembly returned no measurement; mastering would decode the whole program again")
	}
	if assembled.Target != target {
		t.Fatalf("assembly measured target %+v, want the approved %+v", assembled.Target, target)
	}
	committed, err := measureLoudness(ctx, ffmpeg, fullDemoProgramAudioPath(short), target, filepath.Join(dir, "committed-loudness.txt"), duration, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !fullDemoTimingMeasurementEqual(assembled.Measurement, committed) || assembled.Measurement.Status != committed.Status {
		t.Fatalf("assembly measurement %+v differs from the committed program audio %+v", assembled.Measurement, committed)
	}
	// The evidence log keeps its path and its loudnorm block.
	logDir := filepath.Join(dir, "logs")
	reused, err := fullDemoProgramInputLoudness(ctx, ffmpeg, fullDemoProgramAudioPath(short), target, filepath.Join(logDir, "program-input-loudness.txt"), duration, nil, assembled)
	if err != nil {
		t.Fatal(err)
	}
	if !fullDemoTimingMeasurementEqual(reused, committed) {
		t.Fatalf("reused measurement %+v differs from the committed program audio %+v", reused, committed)
	}
	body, err := os.ReadFile(filepath.Join(logDir, "program-input-loudness.txt"))
	if err != nil {
		t.Fatal(err)
	}
	logged, err := parseLoudnessMeasurement(string(body))
	if err != nil || !fullDemoTimingMeasurementEqual(logged, committed) {
		t.Fatalf("program-input-loudness evidence lost its measurement: %v %+v", err, logged)
	}
}

// Reusing the assembly measurement must never run a second measurement pass,
// and anything other than an exact match must fall back to the original one.
func TestFullDemoProgramInputLoudnessReuseIsExactOrFallsBack(t *testing.T) {
	ctx := context.Background()
	target := recapplan.DefaultOptions().Audio.Loudness
	integrated, peak, lra, threshold, offset := -21.5, -5.0, 2.1, -33.0, 0.03
	measurement := LoudnessMeasurement{Status: "measured", IntegratedLUFS: &integrated, TruePeakDBTP: &peak, LRA: &lra, Threshold: &threshold, Offset: &offset}
	assembled := fullDemoProgramAudio{Measured: true, Measurement: measurement, Target: target, Output: `loudnorm evidence
{"input_i":"-21.50","input_tp":"-5.0","input_lra":"2.1","input_thresh":"-33","target_offset":"0.03"}
`}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "logs", "program-input-loudness.txt")
	// A missing FFmpeg proves no measurement subprocess runs on the reuse path.
	got, err := fullDemoProgramInputLoudness(ctx, filepath.Join(dir, "no-such-ffmpeg.exe"), filepath.Join(dir, "program.nut"), target, logPath, 4, nil, assembled)
	if err != nil {
		t.Fatal(err)
	}
	if !fullDemoTimingMeasurementEqual(got, measurement) || got.Status != measurement.Status {
		t.Fatalf("reused measurement = %+v, want %+v", got, measurement)
	}
	body, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != assembled.Output {
		t.Fatalf("evidence log = %q, want the assembly output %q", body, assembled.Output)
	}
	other := target
	other.TargetILUFS -= 1
	for name, mismatch := range map[string]fullDemoProgramAudio{
		"unmeasured":     {Target: target, Measurement: measurement, Output: assembled.Output},
		"other target":   {Measured: true, Target: other, Measurement: measurement, Output: assembled.Output},
		"no measurement": {},
	} {
		if _, err := fullDemoProgramInputLoudness(ctx, filepath.Join(dir, "no-such-ffmpeg.exe"), filepath.Join(dir, "program.nut"), target, logPath, 4, nil, mismatch); err == nil {
			t.Fatalf("%s reused a measurement instead of running the standalone pass", name)
		}
	}
}

// The audio item pool must honour its own worker budget and still return the
// committed paths in timeline order, whatever order the items finish in.
func TestFullDemoAudioItemPoolHonoursItsBudgetAndOrder(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	dir := t.TempDir()
	short, document, _ := fullDemoTransitionCanaryShort(t, ctx, ffmpeg, dir)
	if err := prepareFullDemoTransitions(ctx, &short); err != nil {
		t.Fatal(err)
	}
	if err := prepareFullDemoTracks(ctx, &short, nil); err != nil {
		t.Fatal(err)
	}
	for _, jobs := range []int{1, 2} {
		collected, collector := withFullDemoTimingCollector(ctx)
		paths, err := runFullDemoItemPoolJobs(collected, short, fullDemoItemAudioOnly, nil, jobs)
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) != len(document.Timeline) {
			t.Fatalf("jobs %d: %d item paths, want %d", jobs, len(paths), len(document.Timeline))
		}
		for i := range document.Timeline {
			want := filepath.Join(short.fullDemo.workDir, fmt.Sprintf("item-%03d-audio.nut", i))
			if paths[i] != want {
				t.Fatalf("jobs %d: item %d committed %s, want timeline order %s", jobs, i, paths[i], want)
			}
			if _, err := os.Stat(paths[i]); err != nil {
				t.Fatal(err)
			}
		}
		metrics := collector.snapshot()
		if metrics == nil || len(metrics.Spans) != len(document.Timeline) {
			t.Fatalf("jobs %d: recorded %v audio item spans, want %d", jobs, metrics, len(document.Timeline))
		}
		for _, span := range metrics.Spans {
			if span.Stage != "items_audio" || span.Outcome != "ok" {
				t.Fatalf("jobs %d: unexpected span %+v", jobs, span)
			}
		}
		if concurrent := fullDemoMaxConcurrentSpans(metrics.Spans); concurrent > jobs {
			t.Fatalf("jobs %d: %d audio items ran at once", jobs, concurrent)
		}
	}
}

// fullDemoMaxConcurrentSpans is the largest number of spans whose recorded
// intervals genuinely overlap. Spans that only touch at a millisecond boundary
// are not concurrent.
func fullDemoMaxConcurrentSpans(spans []FullDemoTimingSpan) int {
	type event struct {
		at    int64
		delta int
	}
	events := make([]event, 0, 2*len(spans))
	for _, span := range spans {
		events = append(events, event{span.StartMS, 1}, event{span.EndMS, -1})
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].at != events[j].at {
			return events[i].at < events[j].at
		}
		// Close before opening at the same instant: touching is not overlapping.
		return events[i].delta < events[j].delta
	})
	current, most := 0, 0
	for _, e := range events {
		current += e.delta
		most = max(most, current)
	}
	return most
}

// Opt-in local sweep of the Full Demo audio branch (voices -> audio items ->
// program-audio assembly) over saved, verified replay inputs. It never runs a
// full render, never writes into the saved job or the saved replay outputs, and
// is skipped unless FULL_DEMO_AUDIO_BENCH_DIR points at a phase-a replay label
// directory (the one containing inputs/ and out/edit-manifest.json).
//
// Knobs: FULL_DEMO_AUDIO_BENCH_PHASES (voices,items,assembly), *_VOICE_JOBS,
// *_ITEM_JOBS, *_REPEATS, *_WORKDIR, *_REPORT and *_VIDEO=1, which runs the
// production video item pool as a concurrent load so the audio branch is
// measured under the contention it really has.
func TestFullDemoAudioBranchSweep(t *testing.T) {
	dir := os.Getenv("FULL_DEMO_AUDIO_BENCH_DIR")
	if dir == "" {
		t.Skip("set FULL_DEMO_AUDIO_BENCH_DIR to a saved phase-a replay label directory")
	}
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Minute)
	defer cancel()
	root := os.Getenv("FULL_DEMO_AUDIO_BENCH_WORKDIR")
	if root == "" {
		root = t.TempDir()
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	phases := os.Getenv("FULL_DEMO_AUDIO_BENCH_PHASES")
	if phases == "" {
		phases = "voices,items,assembly"
	}
	repeats := 1
	if value := os.Getenv("FULL_DEMO_AUDIO_BENCH_REPEATS"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			repeats = parsed
		}
	}
	report := map[string]any{"fixture": dir, "cpus": runtime.NumCPU(), "repeats": repeats}
	var rows []map[string]any

	load := os.Getenv("FULL_DEMO_AUDIO_BENCH_VIDEO") == "1"
	base, timeline := fullDemoAudioBenchShort(t, ctx, ffmpeg, dir, root)
	duration := float64(timeline[len(timeline)-1].EndFrame) / recapplan.OutputFPS
	t.Logf("fixture: %d timeline items, %d voice tracks, %.3f s program, %d CPUs, video load %v",
		len(timeline), len(base.fullDemo.execution.VoiceTracks), duration, runtime.NumCPU(), load)

	if strings.Contains(phases, "voices") {
		for _, jobs := range fullDemoAudioBenchJobs("FULL_DEMO_AUDIO_BENCH_VOICE_JOBS", []int{3, 4, 5}) {
			for repeat := 0; repeat < repeats; repeat++ {
				short, cleanup := fullDemoAudioBenchWorkDir(t, base, root, fmt.Sprintf("voices-%d-%d", jobs, repeat))
				stop := fullDemoAudioBenchLoad(t, ctx, short, load)
				collected, elapsed := fullDemoAudioBenchCollect(ctx, func(ctx context.Context) {
					fullDemoAudioBenchVoices(t, ctx, &short, jobs)
				})
				stop()
				row := fullDemoAudioBenchRow(t, "voices", jobs, repeat, elapsed, collected)
				rows = append(rows, row)
				cleanup()
			}
		}
	}

	if strings.Contains(phases, "items") || strings.Contains(phases, "assembly") {
		short, cleanup := fullDemoAudioBenchWorkDir(t, base, root, "audio-items")
		defer cleanup()
		fullDemoAudioBenchVoices(t, ctx, &short, fullDemoVoiceJobs(len(short.fullDemo.execution.VoiceTracks)))
		if strings.Contains(phases, "items") {
			for _, jobs := range fullDemoAudioBenchJobs("FULL_DEMO_AUDIO_BENCH_ITEM_JOBS", []int{1, 2, 3, 4, 6, 8}) {
				for repeat := 0; repeat < repeats; repeat++ {
					stop := fullDemoAudioBenchLoad(t, ctx, short, load)
					collected, elapsed := fullDemoAudioBenchCollect(ctx, func(ctx context.Context) {
						if _, err := runFullDemoItemPoolJobs(ctx, short, fullDemoItemAudioOnly, nil, jobs); err != nil {
							t.Fatal(err)
						}
					})
					stop()
					rows = append(rows, fullDemoAudioBenchRow(t, "items_audio", jobs, repeat, elapsed, collected))
				}
			}
		}
		if strings.Contains(phases, "assembly") {
			paths, err := runFullDemoItemPoolJobs(ctx, short, fullDemoItemAudioOnly, nil, fullDemoAudioItemJobs())
			if err != nil {
				t.Fatal(err)
			}
			list := filepath.Join(short.fullDemo.workDir, "concat-audio-bench.txt")
			if err := writeFullDemoItemConcatList(short, list, paths); err != nil {
				t.Fatal(err)
			}
			target := short.FullDemo.Effective.Options.Audio.Loudness
			for repeat := 0; repeat < repeats; repeat++ {
				stop := fullDemoAudioBenchLoad(t, ctx, short, load)
				reference := filepath.Join(short.fullDemo.workDir, "bench-reference-audio.nut")
				start := time.Now()
				if _, err := runFFmpegOutput(ctx, buildFullDemoProgramAudioCommand(ffmpeg, list, reference), "bench unfused assembly"); err != nil {
					t.Fatal(err)
				}
				assembly := time.Since(start)
				start = time.Now()
				standalone, err := measureLoudness(ctx, ffmpeg, reference, target, "", duration, nil)
				if err != nil {
					t.Fatal(err)
				}
				measurement := time.Since(start)
				fused := filepath.Join(short.fullDemo.workDir, "bench-fused-audio.nut")
				start = time.Now()
				output, err := runFFmpegOutput(ctx, buildFullDemoProgramAudioMeasuredCommand(ffmpeg, list, fused, target), "bench fused assembly")
				if err != nil {
					t.Fatal(err)
				}
				fusedElapsed := time.Since(start)
				stop()
				inProcess, err := parseLoudnessMeasurement(output)
				if err != nil {
					t.Fatal(err)
				}
				equal := fullDemoTimingMeasurementEqual(inProcess, standalone) && inProcess.Status == standalone.Status
				same, err := fullDemoAudioBenchSameBytes(reference, fused)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("assembly repeat %d: unfused %.3fs + measurement %.3fs = %.3fs; fused %.3fs; saved %.3fs; identical bytes %v; identical measurement %v",
					repeat, assembly.Seconds(), measurement.Seconds(), (assembly + measurement).Seconds(), fusedElapsed.Seconds(),
					(assembly + measurement - fusedElapsed).Seconds(), same, equal)
				rows = append(rows, map[string]any{
					"phase": "assembly", "repeat": repeat,
					"unfused_assembly_s": assembly.Seconds(), "standalone_measurement_s": measurement.Seconds(),
					"fused_s": fusedElapsed.Seconds(), "saved_s": (assembly + measurement - fusedElapsed).Seconds(),
					"identical_bytes": same, "identical_measurement": equal,
				})
				if !same || !equal {
					t.Fatalf("fused assembly is not equivalent: bytes %v measurement %v", same, equal)
				}
				for _, path := range []string{reference, fused} {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	}
	report["rows"] = rows
	if path := os.Getenv("FULL_DEMO_AUDIO_BENCH_REPORT"); path != "" {
		body, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(body, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
	}
}

// fullDemoAudioBenchShort loads the saved replay inputs read-only and points
// every generated file at the bench work root.
func fullDemoAudioBenchShort(t *testing.T, ctx context.Context, ffmpeg, dir, root string) (ShortEdit, []recapplan.TimelineItem) {
	t.Helper()
	read := func(name string, into any) {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if err := json.Unmarshal(body, into); err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
	}
	var manifest Manifest
	read(filepath.Join("out", "edit-manifest.json"), &manifest)
	if len(manifest.Shorts) == 0 {
		t.Fatal("edit manifest has no shorts")
	}
	short := manifest.Shorts[0]
	var result recording.RecordingResult
	read(filepath.Join("inputs", "recording-result.json"), &result)
	var execution FullDemoExecution
	read(filepath.Join("inputs", "full-demo-execution.json"), &execution)
	if execution.HUDTelemetry == nil {
		t.Fatal("saved execution has no HUD telemetry")
	}
	hud, err := customhud.Load(execution.HUDTelemetry.Path)
	if err != nil {
		t.Fatalf("load HUD telemetry: %v", err)
	}
	// Never write next to the saved replay output: the program audio and every
	// prepared item follow short.Output.
	short.Output = filepath.Join(root, "bench-program.mp4")
	short.RenderLogPath = filepath.Join(root, "bench-render.log")
	short.FullDemo.TrackLevels = nil
	short.fullDemo = &fullDemoRenderContext{execution: execution, recording: result, hud: &hud, ffmpeg: ffmpeg, workDir: filepath.Join(root, "prepared")}
	if err := prepareFullDemoTransitions(ctx, &short); err != nil {
		t.Fatalf("prepare transitions from saved inputs: %v", err)
	}
	return short, short.FullDemo.Effective.Timeline
}

// fullDemoAudioBenchWorkDir gives one measurement its own prepared directory so
// no run reuses another's voice WAVs or item outputs.
func fullDemoAudioBenchWorkDir(t *testing.T, base ShortEdit, root, name string) (ShortEdit, func()) {
	t.Helper()
	short := base
	evidence := *base.FullDemo
	evidence.TrackLevels = nil
	short.FullDemo = &evidence
	render := *base.fullDemo
	render.voicePaths = nil
	render.preparedInputs = nil
	render.workDir = filepath.Join(root, name)
	short.fullDemo = &render
	short.Output = filepath.Join(root, name, "bench-program.mp4")
	if err := os.MkdirAll(render.workDir, 0o700); err != nil {
		t.Fatal(err)
	}
	return short, func() { os.RemoveAll(render.workDir) }
}

// fullDemoAudioBenchVoices runs the production voice pipeline with an explicit
// worker budget, exactly as prepareFullDemoTracks does with its own.
func fullDemoAudioBenchVoices(t *testing.T, ctx context.Context, short *ShortEdit, jobs int) {
	t.Helper()
	render := short.fullDemo
	if err := os.MkdirAll(render.workDir, 0o700); err != nil {
		t.Fatal(err)
	}
	options := short.FullDemo.Effective.Options.Audio
	voices := make([]fullDemoVoiceInput, len(render.execution.VoiceTracks))
	for i, voice := range render.execution.VoiceTracks {
		voices[i] = fullDemoVoiceInput{Path: voice.Path, StorageKey: voice.StorageKey}
	}
	voiceDuration := 0.0
	if render.recording.Plan.Tickrate > 0 {
		voiceDuration = float64(render.recording.Plan.DemoDurationTicks) / float64(render.recording.Plan.Tickrate)
	}
	prepared, err := prepareFullDemoVoiceTracks(ctx, render.ffmpeg, render.workDir, voices, options, voiceDuration, nil, jobs)
	if err != nil {
		t.Fatal(err)
	}
	render.voicePaths = nil
	for _, voice := range prepared {
		render.voicePaths = append(render.voicePaths, voice.Path)
	}
}

// fullDemoAudioBenchLoad optionally runs the production video item pool as a
// concurrent load and returns a stop function that cancels and awaits it.
func fullDemoAudioBenchLoad(t *testing.T, ctx context.Context, short ShortEdit, enabled bool) func() {
	t.Helper()
	if !enabled {
		return func() {}
	}
	loadCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	load := short
	loadRuntime := *short.fullDemo
	loadRuntime.workDir = filepath.Join(short.fullDemo.workDir, "video-load")
	load.fullDemo = &loadRuntime
	if err := os.MkdirAll(loadRuntime.workDir, 0o700); err != nil {
		t.Fatal(err)
	}
	go func() {
		defer close(done)
		_, _ = runFullDemoItemPool(loadCtx, load, fullDemoItemVideoOnly, nil)
	}()
	// Let the encoders actually start before the measured work begins.
	time.Sleep(3 * time.Second)
	return func() {
		cancel()
		<-done
		os.RemoveAll(loadRuntime.workDir)
	}
}

func fullDemoAudioBenchCollect(ctx context.Context, work func(context.Context)) (*FullDemoTimingMetrics, time.Duration) {
	collected, collector := withFullDemoTimingCollector(ctx)
	start := time.Now()
	work(collected)
	elapsed := time.Since(start)
	return collector.snapshot(), elapsed
}

func fullDemoAudioBenchRow(t *testing.T, phase string, jobs, repeat int, elapsed time.Duration, metrics *FullDemoTimingMetrics) map[string]any {
	t.Helper()
	row := map[string]any{"phase": phase, "jobs": jobs, "repeat": repeat, "elapsed_s": elapsed.Seconds()}
	stages := map[string]any{}
	var report strings.Builder
	fmt.Fprintf(&report, "%s jobs=%d repeat=%d elapsed %.3fs", phase, jobs, repeat, elapsed.Seconds())
	if metrics != nil {
		for _, stage := range metrics.Stages {
			stages[stage.Stage] = map[string]any{"spans": stage.Spans, "wall_s": float64(stage.WallMS) / 1000, "process_sum_s": float64(stage.ProcessElapsedSumMS) / 1000}
			fmt.Fprintf(&report, "; %s union %.3fs sum %.3fs (%d spans)", stage.Stage, float64(stage.WallMS)/1000, float64(stage.ProcessElapsedSumMS)/1000, stage.Spans)
		}
	}
	row["stages"] = stages
	t.Log(report.String())
	return row
}

func fullDemoAudioBenchJobs(name string, fallback []int) []int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	var jobs []int
	for _, field := range strings.Split(value, ",") {
		parsed, err := strconv.Atoi(strings.TrimSpace(field))
		if err == nil && parsed > 0 {
			jobs = append(jobs, parsed)
		}
	}
	if len(jobs) == 0 {
		return fallback
	}
	return jobs
}

func fullDemoAudioBenchSameBytes(a, b string) (bool, error) {
	first, err := os.ReadFile(a)
	if err != nil {
		return false, err
	}
	second, err := os.ReadFile(b)
	if err != nil {
		return false, err
	}
	return string(first) == string(second), nil
}
