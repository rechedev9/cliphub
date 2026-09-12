package obs

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const TracePrefix = "[cliphub-diagnostic] "

type TraceContext struct {
	JobID     string `json:"job_id,omitempty"`
	AttemptID string `json:"attempt_id,omitempty"`
	Operation string `json:"operation,omitempty"`
	Attempt   int    `json:"attempt,omitempty"`
}

type traceContextKey struct{}

func WithTrace(ctx context.Context, trace TraceContext) context.Context {
	return context.WithValue(ctx, traceContextKey{}, trace)
}

func TraceFrom(ctx context.Context) TraceContext {
	trace, _ := ctx.Value(traceContextKey{}).(TraceContext)
	return trace
}

// TraceEntry is emitted as one JSON line through the normal Studio log path.
// It deliberately excludes arbitrary attributes, command arguments and payloads.
type TraceEntry struct {
	TraceContext
	Time       time.Time `json:"time"`
	Event      string    `json:"event"`
	Level      string    `json:"level"`
	Message    string    `json:"message"`
	Outcome    string    `json:"outcome,omitempty"`
	DurationMS int64     `json:"duration_ms,omitempty"`
	ExitCode   *int64    `json:"exit_code,omitempty"`
}

func EmitTrace(ctx context.Context, entry TraceEntry) {
	entry.TraceContext = TraceFrom(ctx)
	if entry.Time.IsZero() {
		entry.Time = time.Now().UTC()
	}
	if entry.Level == "" {
		entry.Level = "info"
	}
	encoded, err := json.Marshal(entry)
	if err != nil {
		log.Printf("observability trace encoding failed: %v", err)
		return
	}
	log.Print(TracePrefix + string(encoded))
}

// TaskJobID extracts only the explicit correlation identifiers from a task.
// Never serialize the payload into diagnostics.
func TaskJobID(payload []byte) string {
	var identity struct {
		JobID     string `json:"job_id"`
		ProjectID string `json:"project_id"`
	}
	if json.Unmarshal(payload, &identity) != nil {
		return ""
	}
	for _, value := range []string{identity.JobID, identity.ProjectID} {
		if id, err := uuid.Parse(value); err == nil && id != uuid.Nil {
			return id.String()
		}
	}
	return ""
}

// TraceWriter forwards bounded complete lines, so a process killed halfway
// through a long render still leaves its preceding stderr in Studio's log.
// Overflow is explicit and never holds an unbounded subprocess line in memory.
type TraceWriter struct {
	mu         sync.Mutex
	ctx        context.Context
	pending    string
	discarding bool
	tool       string
}

func NewTraceWriter(ctx context.Context, tool string) *TraceWriter {
	return &TraceWriter{ctx: ctx, tool: tool}
}

func (w *TraceWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, part := range strings.SplitAfter(string(p), "\n") {
		complete := strings.HasSuffix(part, "\n")
		if !w.discarding {
			if len(w.pending)+len(part) > 48<<10 {
				w.pending = ""
				w.discarding = true
				EmitTrace(w.ctx, TraceEntry{Event: "process.output_gap", Level: "warn", Message: w.tool + ": subprocess line exceeded 48 KiB; omitted"})
			} else {
				w.pending += part
			}
		}
		if complete {
			w.emitPending()
			w.discarding = false
		}
	}
	return len(p), nil
}

func (w *TraceWriter) emitPending() {
	if text := strings.TrimSpace(w.pending); text != "" {
		// Child tools emit the same protocol. Restore structured severity and
		// timing while attaching the parent's authoritative job/attempt context.
		if _, body, ok := strings.Cut(text, TracePrefix); ok {
			var entry TraceEntry
			if json.Unmarshal([]byte(body), &entry) == nil {
				switch entry.Event {
				case "tool.started", "tool.finished", "process.stderr", "process.output_gap":
					EmitTrace(w.ctx, entry)
					w.pending = ""
					return
				}
			}
		}
		EmitTrace(w.ctx, TraceEntry{Event: "process.stderr", Message: w.tool + ": " + text})
	}
	w.pending = ""
}

func (w *TraceWriter) Close() error { w.mu.Lock(); defer w.mu.Unlock(); w.emitPending(); return nil }

var _ io.WriteCloser = (*TraceWriter)(nil)
