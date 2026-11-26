package workers

import "github.com/google/uuid"

func (w *RecordWorker) forgetGenerateFailure(id uuid.UUID) {
	if err := w.generateIntents.ClearFailure(id); err != nil {
		logWorkerError(id, "forget generate failure after capture", err)
	}
}
