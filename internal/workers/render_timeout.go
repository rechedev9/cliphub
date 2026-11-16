package workers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

// A slow PC renders a Full Demo at about 2.5 times its length, so four times
// the program plus setup bounds a hung render without cutting a long one.
const (
	renderTimeoutFactor = 4
	renderTimeoutSlack  = 15 * time.Minute
)

// renderTimeoutPrefix is the stable reason the web UI matches when zv-editor ran out of time.
const renderTimeoutPrefix = "render_timeout:"

// renderTimeout is the editor's deadline: the configured one, raised for a Full
// Demo whose approved program needs more time than it allows.
func (c RenderWorkerConfig) renderTimeout(fullDemo *recapplan.Snapshot) time.Duration {
	base, err := time.ParseDuration(c.Timeout)
	if err != nil || fullDemo == nil || len(fullDemo.Document.Timeline) == 0 {
		return c.timeoutDuration()
	}
	timeline := fullDemo.Document.Timeline
	program := time.Duration(timeline[len(timeline)-1].EndFrame) * time.Second / recapplan.OutputFPS
	need := renderTimeoutFactor*program + renderTimeoutSlack
	if need <= base {
		return base
	}
	return need.Round(time.Second)
}

// renderTimeoutError is a zv-editor run its deadline cut, keeping the runner's error.
type renderTimeoutError struct {
	limit time.Duration
	err   error
}

func (e *renderTimeoutError) Error() string {
	return fmt.Sprintf("%s zv-editor did not finish within %s: %v", renderTimeoutPrefix, e.limit, e.err)
}

func (e *renderTimeoutError) Unwrap() error { return e.err }

// runEditor runs zv-editor and names a run that ctx's deadline cut.
func (w *RenderWorker) runEditor(ctx context.Context, args []string) error {
	started := time.Now()
	_, err := w.runner.Run(ctx, w.cfg.withDefaults().EditorPath, args...)
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		deadline, _ := ctx.Deadline()
		return &renderTimeoutError{limit: deadline.Sub(started).Round(time.Second), err: err}
	}
	return err
}
