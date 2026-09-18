package editor

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The bounded pool must overlap independent boundaries, never exceed its limit
// and still let every probe own exactly one index.
func TestFullDemoTransitionPoolOverlapsProbesAndKeepsOrder(t *testing.T) {
	const count, jobs = 6, 3
	var mu sync.Mutex
	active, peak := 0, 0
	seen := make([]int, count)
	release := make(chan struct{})
	var open sync.Once
	// Safety valve only: a serial pool would never reach the limit and would
	// otherwise hang the whole test binary; the peak assertion below still
	// fails cleanly when the valve opens the gate instead of the rendezvous.
	valve := time.AfterFunc(10*time.Second, func() { open.Do(func() { close(release) }) })
	defer valve.Stop()
	err := runFullDemoTransitionPool(context.Background(), count, jobs, func(_ context.Context, index int) error {
		mu.Lock()
		active++
		seen[index]++
		current := active
		if current > peak {
			peak = current
		}
		mu.Unlock()
		if current == jobs {
			open.Do(func() { close(release) })
		}
		// A serial prefix can never reach the limit, so this rendezvous proves
		// the overlap without timing assumptions.
		<-release
		mu.Lock()
		active--
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if peak != jobs {
		t.Fatalf("peak concurrency = %d, want the %d-probe bound", peak, jobs)
	}
	for index, times := range seen {
		if times != 1 {
			t.Fatalf("boundary %d probed %d times, want exactly one owner", index, times)
		}
	}
}

// Two boundaries failing at once must always report the lowest index, the same
// error the former serial loop returned when it stopped at the first failure.
func TestFullDemoTransitionPoolReportsLowestFailingIndex(t *testing.T) {
	const count = 4
	first, second := errors.New("boundary one failed"), errors.New("boundary three failed")
	for attempt := 0; attempt < 32 && !t.Failed(); attempt++ {
		// Every probe runs concurrently and no probe returns before all of them
		// arrived, so both failures are always recorded before the pool selects.
		// The timer is a safety valve so a serial pool fails instead of hanging.
		var arrived atomic.Int32
		ready := make(chan struct{})
		var open sync.Once
		valve := time.AfterFunc(10*time.Second, func() {
			t.Error("probes never rendezvoused: the pool did not run them concurrently")
			open.Do(func() { close(ready) })
		})
		err := runFullDemoTransitionPool(context.Background(), count, count, func(_ context.Context, index int) error {
			if arrived.Add(1) == count {
				open.Do(func() { close(ready) })
			}
			<-ready
			switch index {
			case 1:
				return first
			case 3:
				return second
			}
			return nil
		})
		valve.Stop()
		if !errors.Is(err, first) {
			t.Fatalf("attempt %d: pool error = %v, want the lowest failing boundary", attempt, err)
		}
	}
}

// A failing boundary must not hide the results of the boundaries that passed:
// each probe owns its own slice element, so the evidence stays indexed.
func TestFullDemoTransitionPoolWritesDisjointResults(t *testing.T) {
	const count = 8
	sentinel := errors.New("boundary five failed")
	directions := make([]string, count)
	err := runFullDemoTransitionPool(context.Background(), count, 3, func(_ context.Context, index int) error {
		if index == 5 {
			return sentinel
		}
		directions[index] = fmt.Sprintf("boundary-%d", index)
		return nil
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("pool error = %v, want the failing boundary", err)
	}
	for index, direction := range directions {
		want := fmt.Sprintf("boundary-%d", index)
		if index == 5 {
			want = ""
		}
		if direction != want {
			t.Fatalf("boundary %d = %q, want %q", index, direction, want)
		}
	}
}

// An already-cancelled render must not launch a single probe.
func TestFullDemoTransitionPoolCancelledBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Probes run on pool goroutines, where t.Fatal is not allowed; count instead.
	var ran atomic.Int32
	err := runFullDemoTransitionPool(ctx, 4, 2, func(context.Context, int) error {
		ran.Add(1)
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("pool error = %v, want context.Canceled", err)
	}
	if n := ran.Load(); n != 0 {
		t.Fatalf("%d transition probes ran after cancellation", n)
	}
}

func TestFullDemoTransitionJobsIsBoundedAndCPUAware(t *testing.T) {
	if got := fullDemoTransitionJobs(0); got != 0 {
		t.Fatalf("jobs for no boundaries = %d, want 0", got)
	}
	if got := fullDemoTransitionJobs(1); got != 1 {
		t.Fatalf("jobs for one boundary = %d, want 1", got)
	}
	for _, count := range []int{2, 3, 5, 40} {
		jobs := fullDemoTransitionJobs(count)
		if jobs < 1 || jobs > 4 || jobs > count {
			t.Fatalf("jobs for %d boundaries = %d, want 1..min(4, count)", count, jobs)
		}
		if jobs > runtime.NumCPU() {
			t.Fatalf("jobs for %d boundaries = %d, above %d CPUs", count, jobs, runtime.NumCPU())
		}
	}
}
