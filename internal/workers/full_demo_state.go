package workers

import (
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

// Serializes the ownership check and pointer swap with HTTP admission and the
// capture handoff. Failed, cached and successful attempts use the same fence.
func (w *RenderWorker) writeOwnedRenderState(state renderplan.RenderVariantState, approved *recapplan.Snapshot) error {
	state.FullDemo = approved
	return w.generateIntents.WhileIdle(state.JobID, func() error {
		current, found, err := w.readRenderVariantState(state.JobID, state.Variant)
		if err != nil {
			return err
		}
		if found && !renderplan.SameFullDemoRequest(current.FullDemo, approved) {
			return &recapplan.Error{Code: recapplan.ErrPlanStale, Detail: "Render task no longer owns the approved plan"}
		}
		return w.writeRenderVariantState(state)
	})
}
