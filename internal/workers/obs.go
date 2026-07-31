package workers

import (
	"github.com/google/uuid"

	"github.com/rechedev9/tickcut/internal/obs"
)

// recordStageFailure appends a terminal worker failure to the local obs journal
// so orchestrator failures show up in the same error log as CLI and batch runs.
// Most workers pass obs.StageWorker and their task type as the class; a stage
// that owns a label vocabulary of its own (see obs.StageTactical) passes that
// instead. It is best-effort: observability never blocks job processing.
func recordStageFailure(id uuid.UUID, stage, class string, err error) {
	rec := obs.Default()
	if rec == nil {
		return
	}
	_ = rec.RecordError(obs.Event{
		Stage:   stage,
		Class:   class,
		Message: id.String() + ": " + err.Error(),
	})
}
