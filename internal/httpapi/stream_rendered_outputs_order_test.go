package httpapi

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/streamclips"
)

func TestStreamRenderedOutputsKeepTheRenderOrder(t *testing.T) {
	repo := newFakeStreamRepo()
	store := newFakeStorage()
	jobID := uuid.New()
	plan := streamclips.DefaultEditPlan()
	plan.Clips = []streamclips.ClipRange{
		{ID: "clip-001", StartSeconds: 0, EndSeconds: 3},
		{ID: "clip-002", StartSeconds: 5, EndSeconds: 8},
		{ID: "clip-003", StartSeconds: 10, EndSeconds: 13},
	}
	plan.Groups = []streamclips.ClipGroup{{ID: "group-001", ClipIDs: []string{"clip-001", "clip-003"}}}
	planJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	repo.jobs[jobID] = streamclips.Job{ID: jobID, Status: streamclips.StatusRendered, EditPlan: planJSON}

	revision := uuid.New()
	var videos []streamclips.VideoEntry
	for _, id := range []string{"group-001", "clip-002"} {
		key, _ := streamclips.RenderRevisionVideoKey(jobID, plan.Variant, revision, id)
		_ = store.Put(key, strings.NewReader("video"))
		videos = append(videos, streamclips.VideoEntry{ClipID: id, Key: key, DurationSeconds: 3})
	}
	result, err := streamclips.NewRenderResult(jobID, plan.Variant, videos, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	resultKey, _ := streamclips.RenderRevisionResultKey(jobID, plan.Variant, revision)
	galleryKey, _ := streamclips.RenderRevisionGalleryKey(jobID, plan.Variant, revision)
	prefix, _ := streamclips.RenderRevisionPrefix(jobID, plan.Variant, revision)
	putStreamOutputJSON(t, store, resultKey, result)
	state, err := streamclips.NewRenderState(jobID, plan.Variant, streamclips.StatusRendered, nil, "", result.Clips)
	if err != nil {
		t.Fatal(err)
	}
	state.ResultKey, state.GalleryKey, state.ArtifactDir = resultKey, galleryKey, prefix
	h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithStreamRepository(repo))
	if err := h.writeStreamRenderState(state); err != nil {
		t.Fatal(err)
	}

	outputs, err := h.streamRenderedOutputs(repo.jobs[jobID])
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(outputs))
	for _, out := range outputs {
		ids = append(ids, out.ClipID)
	}
	if !slices.Equal(ids, []string{"group-001", "clip-002"}) {
		t.Fatalf("outputs = %v, want the joined Short first, where its first moment is", ids)
	}
}
