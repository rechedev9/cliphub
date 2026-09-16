package editor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

// Prepared voice material must not depend on how many tracks ran at once: the
// same filters, gain policy and materialization have to produce byte-identical
// WAVs and identical measurements in approved order.
func TestFullDemoVoicePrepSerialAndParallelProduceIdenticalPCM(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	dir := t.TempDir()
	specs := []string{
		"sine=frequency=220:duration=2:sample_rate=48000",
		"aevalsrc=0.08*sin(2*PI*880*t):s=48000:d=2",
		"anullsrc=r=48000:cl=stereo:d=2",
	}
	inputs := make([]fullDemoVoiceInput, len(specs))
	for i, spec := range specs {
		path := filepath.Join(dir, fmt.Sprintf("source-%d.wav", i))
		command := []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", spec, "-ac", "2", "-ar", "48000", "-c:a", "pcm_f32le", path}
		if _, err := runFFmpegOutput(ctx, command, "voice fixture"); err != nil {
			t.Fatal(err)
		}
		inputs[i] = fullDemoVoiceInput{Path: path, StorageKey: fmt.Sprintf("team/voice-%d", i)}
	}
	options := recapplan.DefaultOptions().Audio
	run := func(jobs int) ([]preparedFullDemoVoice, []float64) {
		work := filepath.Join(dir, fmt.Sprintf("work-%d", jobs))
		if err := os.MkdirAll(work, 0700); err != nil {
			t.Fatal(err)
		}
		var fractions []float64
		progress := func(_ string, fraction float64) {
			fractions = append(fractions, fraction)
		}
		prepared, err := prepareFullDemoVoiceTracks(ctx, ffmpeg, work, inputs, options, 2, progress, jobs)
		if err != nil {
			t.Fatal(err)
		}
		return prepared, fractions
	}
	serial, serialProgress := run(1)
	parallel, parallelProgress := run(3)
	if len(serial) != len(inputs) || len(parallel) != len(inputs) {
		t.Fatalf("prepared tracks = %d/%d, want %d", len(serial), len(parallel), len(inputs))
	}
	for i := range inputs {
		if serial[i].Ref != parallel[i].Ref || serial[i].Ref != inputs[i].StorageKey {
			t.Fatalf("track %d ref = %q/%q, want %q", i, serial[i].Ref, parallel[i].Ref, inputs[i].StorageKey)
		}
		if !reflect.DeepEqual(serial[i].Measurement, parallel[i].Measurement) {
			t.Fatalf("track %d measurement differs: %+v vs %+v", i, serial[i].Measurement, parallel[i].Measurement)
		}
		if serial[i].GainDB != parallel[i].GainDB {
			t.Fatalf("track %d gain = %v, want %v", i, parallel[i].GainDB, serial[i].GainDB)
		}
		serialPCM, err := os.ReadFile(serial[i].Path)
		if err != nil {
			t.Fatal(err)
		}
		parallelPCM, err := os.ReadFile(parallel[i].Path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(serialPCM, parallelPCM) {
			t.Fatalf("track %d PCM differs between serial and parallel preparation (%d vs %d bytes)", i, len(serialPCM), len(parallelPCM))
		}
	}
	for name, fractions := range map[string][]float64{"serial": serialProgress, "parallel": parallelProgress} {
		for i := 1; i < len(fractions); i++ {
			if fractions[i] < fractions[i-1] {
				t.Fatalf("%s progress regressed: %v", name, fractions)
			}
		}
		if len(fractions) == 0 || fractions[len(fractions)-1] < 0.9 {
			t.Fatalf("%s progress never advanced: %v", name, fractions)
		}
	}
}

// A bounded pool must actually overlap independent tracks but never exceed its
// limit, and must still return results keyed by the original index.
func TestFullDemoVoicePoolBoundsWorkersAndKeepsOrder(t *testing.T) {
	var mu sync.Mutex
	active, peak := 0, 0
	release := make(chan struct{})
	var atLimit sync.Once
	results, err := runFullDemoVoicePool(context.Background(), 6, 3, nil, func(_ context.Context, index int, _, _ func(float64)) (preparedFullDemoVoice, error) {
		mu.Lock()
		active++
		current := active
		if current > peak {
			peak = current
		}
		mu.Unlock()
		if current == 3 {
			atLimit.Do(func() { close(release) })
		}
		<-release
		mu.Lock()
		active--
		mu.Unlock()
		time.Sleep(time.Duration(6-index) * time.Millisecond)
		return preparedFullDemoVoice{Ref: fmt.Sprintf("team/voice-%d", index)}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if peak != 3 {
		t.Fatalf("peak concurrency = %d, want the 3-worker bound", peak)
	}
	if len(results) != 6 {
		t.Fatalf("results = %d, want 6", len(results))
	}
	for i, result := range results {
		if result.Ref != fmt.Sprintf("team/voice-%d", i) {
			t.Fatalf("result %d = %q, want original index order", i, result.Ref)
		}
	}
}

// A track failure is reported as the single first error and cancels the pool.
func TestFullDemoVoicePoolFirstErrorCancelsPool(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sentinel := errors.New("voice analysis failed")
	_, err := runFullDemoVoicePool(ctx, 6, 2, nil, func(ctx context.Context, index int, _, _ func(float64)) (preparedFullDemoVoice, error) {
		if index == 1 {
			return preparedFullDemoVoice{}, sentinel
		}
		<-ctx.Done()
		return preparedFullDemoVoice{}, ctx.Err()
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("pool error = %v, want the first track failure", err)
	}
}

// An already-cancelled context must not run any track pipeline at all.
func TestFullDemoVoicePoolCancelledBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := runFullDemoVoicePool(ctx, 4, 2, nil, func(context.Context, int, func(float64), func(float64)) (preparedFullDemoVoice, error) {
		t.Fatal("voice pipeline ran after cancellation")
		return preparedFullDemoVoice{}, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("pool error = %v, want context.Canceled", err)
	}
}

func TestFullDemoVoiceJobsIsBoundedAndCPUAware(t *testing.T) {
	if got := fullDemoVoiceJobs(0); got != 0 {
		t.Fatalf("jobs for no tracks = %d, want 0", got)
	}
	if got := fullDemoVoiceJobs(1); got != 1 {
		t.Fatalf("jobs for one track = %d, want 1", got)
	}
	for _, count := range []int{2, 3, 5, 20} {
		jobs := fullDemoVoiceJobs(count)
		if jobs < 1 || jobs > 3 || jobs > count {
			t.Fatalf("jobs for %d tracks = %d, want 1..min(3, count)", count, jobs)
		}
		if jobs > runtime.NumCPU() {
			t.Fatalf("jobs for %d tracks = %d, above %d CPUs", count, jobs, runtime.NumCPU())
		}
	}
}

// A single track must advance through its materialization phase instead of
// stalling, while still reserving 1 for successful completion.
func TestFullDemoVoiceProgressSingleTrackAdvancesThroughMaterialization(t *testing.T) {
	var mu sync.Mutex
	var seen []float64
	aggregate := newFullDemoVoiceProgress(1, func(_ string, fraction float64) {
		mu.Lock()
		seen = append(seen, fraction)
		mu.Unlock()
	})
	aggregate.phase(0, false, 0.5, "analysis")
	aggregate.phase(0, false, 1, "analysis")
	aggregate.phase(0, true, 0.5, "material")
	aggregate.phase(0, true, 1, "material")
	aggregate.complete(0, "material")

	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 5 {
		t.Fatalf("aggregate reported %v, want five reports", seen)
	}
	for i := 1; i < len(seen); i++ {
		if seen[i] <= seen[i-1] {
			t.Fatalf("single-track progress stalled or regressed: %v", seen)
		}
	}
	if seen[1] != 0.5 {
		t.Fatalf("analysis completion = %v, want 0.5", seen[1])
	}
	if seen[2] <= seen[1] {
		t.Fatalf("materialization did not advance past analysis: %v", seen)
	}
	if seen[len(seen)-2] >= 1 {
		t.Fatalf("aggregate reached 1 before all tracks completed: %v", seen)
	}
	if seen[len(seen)-1] != 1 {
		t.Fatalf("final aggregate = %v, want 1", seen[len(seen)-1])
	}
}

// A short track finishing first must never report completion while earlier
// tracks are still running: the reported value is the average across every
// track's two phases, and 1 is reserved for all tracks finishing.
func TestFullDemoVoiceProgressAggregatesWithoutPrematureCompletion(t *testing.T) {
	var mu sync.Mutex
	var seen []float64
	progress := func(_ string, fraction float64) {
		mu.Lock()
		seen = append(seen, fraction)
		mu.Unlock()
	}
	aggregate := newFullDemoVoiceProgress(2, progress)

	// Track 1 is the short track: analysis and materialization both complete.
	aggregate.phase(1, false, 1, "analysis")
	aggregate.phase(1, true, 1, "material")
	aggregate.complete(1, "material")

	mu.Lock()
	premature := append([]float64{}, seen...)
	mu.Unlock()
	if len(premature) == 0 {
		t.Fatal("aggregate reported no progress")
	}
	for _, fraction := range premature {
		if fraction >= 1 {
			t.Fatalf("short track reported completion while another track was unfinished: %v", premature)
		}
	}
	// Two tracks x two phases: one fully done track is exactly half the work.
	if last := premature[len(premature)-1]; last < 0.49 || last > 0.51 {
		t.Fatalf("aggregate fraction after one of two tracks = %v, want ~0.5", last)
	}

	// The slow track finishes: only now may the aggregate reach 1.
	aggregate.phase(0, false, 0.5, "analysis")
	aggregate.phase(0, true, 0.5, "material")
	aggregate.complete(0, "material")

	mu.Lock()
	final := seen[len(seen)-1]
	mu.Unlock()
	if final != 1 {
		t.Fatalf("aggregate did not report completion after every track finished: %v", seen)
	}
	if premature[len(premature)-1] >= final {
		t.Fatalf("premature and final completion are indistinguishable: %v", seen)
	}
}

// A queued track must not start once the pool is cancelled while the semaphore
// is full.
func TestFullDemoVoicePoolQueuedTrackDoesNotStartAfterCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan int, 2)
	var mu sync.Mutex
	var launched []int
	prepare := func(ctx context.Context, index int, _, _ func(float64)) (preparedFullDemoVoice, error) {
		mu.Lock()
		launched = append(launched, index)
		mu.Unlock()
		started <- index
		<-ctx.Done()
		return preparedFullDemoVoice{}, ctx.Err()
	}
	done := make(chan error, 1)
	go func() {
		_, err := runFullDemoVoicePool(ctx, 2, 1, nil, prepare)
		done <- err
	}()
	if got := <-started; got != 0 {
		t.Fatalf("first started track = %d, want 0", got)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("pool error = %v, want context.Canceled", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(launched) != 1 || launched[0] != 0 {
		t.Fatalf("queued track started after cancellation: %v", launched)
	}
}
