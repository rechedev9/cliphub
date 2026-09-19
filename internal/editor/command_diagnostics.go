package editor

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/rechedev9/cliphub/internal/obs"
)

// Stream stderr while FFmpeg is alive, including masters that later recover.
// The original output buffer remains available to loudness parsers and callers.
func runDiagnosticFFmpeg(ctx context.Context, cmd *exec.Cmd, label string) error {
	started := time.Now()
	trace := obs.NewTraceWriter(ctx, "ffmpeg "+label)
	if cmd.Stderr == nil {
		cmd.Stderr = trace
	} else {
		cmd.Stderr = io.MultiWriter(cmd.Stderr, trace)
	}
	obs.EmitTrace(ctx, obs.TraceEntry{Event: "tool.started", Message: "ffmpeg " + label})
	// The single place where every FFmpeg subprocess of this package is
	// started, and therefore the only place a scheduling class can be applied
	// before the process exists.
	applyBackgroundProcessPriority(ctx, cmd)
	err := cmd.Run()
	finished := time.Now()
	_ = trace.Close()
	entry := obs.TraceEntry{Event: "tool.finished", Message: "ffmpeg " + label, Outcome: "ok", DurationMS: finished.Sub(started).Milliseconds()}
	if cmd.ProcessState != nil {
		code := int64(cmd.ProcessState.ExitCode())
		entry.ExitCode = &code
	}
	if err != nil {
		entry.Level, entry.Outcome = "error", "error"
		entry.Message += ": " + err.Error()
	}
	// The trace events and the optional per-render collector share this exact
	// interval and outcome, so timing evidence never disagrees with diagnostics.
	// The command args let the collector infer a fixed encoder name.
	fullDemoTimingRecord(ctx, label, cmd.Args, started, finished, err)
	obs.EmitTrace(ctx, entry)
	return err
}

// MultiWriter makes stdout and stderr different writers to os/exec, so protect
// their shared historical CombinedOutput buffer from concurrent pipe copiers.
type diagnosticCommandBuffer struct {
	mu   sync.Mutex
	data bytes.Buffer
}

func (b *diagnosticCommandBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.Write(p)
}

func (b *diagnosticCommandBuffer) String() string { return b.data.String() }
