package workers

import (
	"errors"
	"github.com/google/uuid"
	"strings"

	"github.com/rechedev9/cliphub/internal/obs"
)

// recordStageFailure appends a terminal worker failure to the local obs journal
// so orchestrator failures show up in the same error log as CLI and batch runs.
// Task is the existing Asynq task type (parse:demo, record:demo, …); Class is
// the queryable code. When class is empty the task type is reused so we do not
// invent a parallel taxonomy. It is best-effort: observability never blocks
// job processing.
func recordStageFailure(id uuid.UUID, stage, task, class string, err error) {
	rec := obs.Default()
	if rec == nil {
		return
	}
	if class == "" {
		class = task
	}
	if class == "" {
		class = "unknown"
	}
	_ = rec.RecordError(obs.Event{
		JobID:   id.String(),
		Stage:   stage,
		Task:    task,
		Class:   class,
		Message: workerDiagnosticMessage(err),
	})
}

// recordFailure intentionally condenses stderr for the UI. Keep its underlying
// subprocess output in the journal so remote diagnostics can explain the cause.
func workerDiagnosticMessage(err error) string {
	text := err.Error()
	var failure *recordFailure
	if errors.As(err, &failure) && failure.err != nil && !strings.Contains(text, failure.err.Error()) {
		text += "\nRecorder output:\n" + failure.err.Error()
	}
	// Stay below the journal reader's per-poll bound even for verbose subprocesses.
	if len(text) > 64*1024 {
		text = text[:16*1024] + "\n[truncated]\n" + text[len(text)-48*1024:]
	}
	return text
}

// errorClass is the queryable obs class for a worker failure. Known codes
// (missing plate, capture flake, …) win; otherwise the task type stays the
// class so metrics keep grouping by the existing worker vocabulary.
func errorClass(taskType string, err error) string {
	if err == nil {
		return taskType
	}
	if class := obs.ClassOf(err.Error()); class != "" {
		return class
	}
	return taskType
}
