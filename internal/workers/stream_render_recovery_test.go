package workers

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/fragforge/internal/streamclips"
	"github.com/rechedev9/fragforge/internal/tasks"
)

// TestWriteRecoverableStreamRenderStateCodesSuperseded pins the machine-readable
// contract Studio depends on: a recoverable render failure lands as a failed
// render state carrying the superseded code, not as a bare message.
func TestWriteRecoverableStreamRenderStateCodesSuperseded(t *testing.T) {
	store := newFakeStorage()
	worker := NewStreamRenderWorker(newFakeStreamRepo(), store, StreamRenderWorkerConfig{})
	jobID := uuid.New()
	intent := tasks.StreamRenderIntent{AttemptID: uuid.New()}
	initial, err := streamclips.NewRenderState(jobID, streamclips.VariantStreamer4060, streamclips.StatusRendering, nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	initial.AttemptID = intent.AttemptID
	if err := worker.writeStreamRenderState(initial); err != nil {
		t.Fatal(err)
	}
	owned, err := worker.writeRecoverableStreamRenderState(
		jobID, streamclips.VariantStreamer4060, intent, true, "recoverable",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !owned {
		t.Fatal("recoverable state was not owned")
	}
	key, err := streamclips.RenderStateKey(jobID, streamclips.VariantStreamer4060)
	if err != nil {
		t.Fatal(err)
	}
	var state streamclips.RenderState
	if err := json.Unmarshal(store.files[key], &state); err != nil {
		t.Fatal(err)
	}
	if state.Status != streamclips.StatusFailed ||
		state.Error != "recoverable" ||
		state.ErrorCode != streamclips.RenderErrorCodeSuperseded {
		t.Fatalf("state = %#v, want failed with error code %q", state, streamclips.RenderErrorCodeSuperseded)
	}
}
