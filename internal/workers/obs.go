package workers

import (
	"github.com/google/uuid"

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
		Message: err.Error(),
	})
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
