package workers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rechedev9/cliphub/internal/obs"
)

type commandRunner interface {
	Run(ctx context.Context, exe string, args ...string) ([]byte, error)
}

type execCommandRunner struct{}

func (r execCommandRunner) Run(ctx context.Context, exe string, args ...string) ([]byte, error) {
	return r.run(ctx, exe, nil, args...)
}

// RunProgress sends stdout to progress alone, so FFmpeg's -progress records
// never fill the output an error quotes.
func (r execCommandRunner) RunProgress(ctx context.Context, exe string, progress io.Writer, args ...string) ([]byte, error) {
	return r.run(ctx, exe, progress, args...)
}

func (execCommandRunner) run(ctx context.Context, exe string, progress io.Writer, args ...string) ([]byte, error) {
	// #nosec G204 -- media workers execute configured local binaries with argument slices, not shell strings.
	cmd := exec.CommandContext(ctx, exe, args...)
	tool := filepath.Base(exe)
	// Ending the tool alone would leave what it started (ffmpeg, the overlay
	// renderer, CS2) working against whatever this machine does next.
	cmd.Cancel = func() error { return killProcessTree(cmd.Process, tool) }
	started := time.Now()
	obs.EmitTrace(ctx, obs.TraceEntry{Event: "process.started", Message: tool + " started"})
	var output commandOutputBuffer
	stderr := obs.NewTraceWriter(ctx, tool)
	cmd.Stdout = &output
	if progress != nil {
		cmd.Stdout = progress
	}
	cmd.Stderr = io.MultiWriter(&output, stderr)
	err := cmd.Run()
	_ = stderr.Close()
	out := output.Bytes()
	code := int64(-1)
	if cmd.ProcessState != nil {
		code = int64(cmd.ProcessState.ExitCode())
	}
	entry := obs.TraceEntry{Event: "process.finished", Message: tool + " completed", Outcome: "ok", DurationMS: time.Since(started).Milliseconds(), ExitCode: &code}
	if err != nil {
		entry.Level, entry.Outcome, entry.Message = "error", "error", tool+": "+err.Error()
	}
	if ctx.Err() != nil {
		entry.Message += ": " + ctx.Err().Error()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			entry.Outcome = "timeout"
		} else {
			entry.Outcome = "cancelled"
		}
	}
	obs.EmitTrace(ctx, entry)
	if err == nil {
		return out, nil
	}
	if text := strings.TrimSpace(string(out)); text != "" {
		return out, fmt.Errorf("%s failed: %w: %s", exe, err, text)
	}
	return out, fmt.Errorf("%s failed: %w", exe, err)
}

// stdout and stderr are copied concurrently by os/exec when stderr is also
// streamed into diagnostics. Keep the returned combined output race-free.
type commandOutputBuffer struct {
	mu   sync.Mutex
	data bytes.Buffer
}

func (b *commandOutputBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.Write(p)
}
func (b *commandOutputBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.data.Bytes()...)
}
