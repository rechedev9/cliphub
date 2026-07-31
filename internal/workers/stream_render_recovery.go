package workers

import (
	"github.com/google/uuid"

	"github.com/rechedev9/tickcut/internal/streamclips"
	"github.com/rechedev9/tickcut/internal/tasks"
)

// writeRecoverableStreamRenderState persists the stable machine-readable code
// Studio uses to keep the editor open instead of surfacing a dead render.
// Every recoverable render failure is now a superseded plan or variant state,
// so the code is constant; the render state keeps carrying it so clients can
// keep distinguishing a recoverable failure from a real one.
func (w *StreamRenderWorker) writeRecoverableStreamRenderState(
	id uuid.UUID,
	variant string,
	intent tasks.StreamRenderIntent,
	hasIntent bool,
	message string,
) (bool, error) {
	return w.writeOwnedStreamRenderAttempt(
		id, variant, intent, hasIntent,
		streamclips.StatusFailed, nil, message, streamclips.RenderErrorCodeSuperseded,
	)
}
