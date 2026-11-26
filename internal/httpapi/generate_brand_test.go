package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/brandkit"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/tasks"
)

func TestStartGenerateCopiesTheBrandKitChoices(t *testing.T) {
	env := newLibraryTestEnv(t)
	repo := store.NewMemoryBrandKitRepository()
	logo := storeBrandImage(t, env)
	if _, err := repo.SaveBrandKit(context.Background(), brandkit.Kit{Logo: &logo, WatermarkCorner: brandkit.CornerTopLeft, WatermarkOpacity: 0.5, ChannelName: "ClipHub RSN"}); err != nil {
		t.Fatal(err)
	}
	plan := killplan.NewPlan()
	plan.Segments = []killplan.Segment{{ID: "seg-001"}}
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	jobs := newFakeRepo()
	jobs.jobs[j.ID] = j
	queue := &fakeQueue{}
	h := NewHandlers(jobs, env.storage, queue, WithCapabilities(Capabilities{RecordEnabled: true}), WithBrandKit(repo))

	rw := postGenerate(t, h, j.ID, `{"preset":"viral-60-clean","segment_ids":["seg-001"],"edit":{"format":"short-9x16","brand_watermark":true,"subscribe_banner":true}}`)
	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	intent, ok, err := tasks.GenerateIntentFromTask(queue.enqueued[0])
	if err != nil || !ok {
		t.Fatalf("GenerateIntentFromTask = %v %v", ok, err)
	}
	if wm := intent.Edit.BrandWatermark; wm == nil || wm.Image != logo || wm.Opacity != 0.5 {
		t.Fatalf("watermark = %+v; want the kit copy", wm)
	}
	if card := intent.Edit.SubscribeBanner; card == nil || card.Logo == nil || *card.Logo != logo || card.ChannelName != "ClipHub RSN" {
		t.Fatalf("subscribe banner = %+v; want the kit's logo and name", card)
	}
	if intent.Edit.BrandBanner != nil {
		t.Fatal("a brand switch left out turned the sponsor banner on")
	}

	retry, err := json.Marshal(map[string]any{"preset": "viral-60-clean", "segment_ids": []string{"seg-001"}, "edit": map[string]any{"format": "short-9x16", "subscribe_banner": intent.Edit.SubscribeBanner}})
	if err != nil {
		t.Fatal(err)
	}
	other := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	jobs.jobs[other.ID] = other
	if rw := postGenerate(t, h, other.ID, string(retry)); rw.Code != http.StatusAccepted {
		t.Fatalf("a retry with the saved copy = %d %s", rw.Code, rw.Body.String())
	}

	noKit := NewHandlers(jobs, env.storage, &fakeQueue{}, WithCapabilities(Capabilities{RecordEnabled: true}))
	third := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	jobs.jobs[third.ID] = third
	if rw := postGenerate(t, noKit, third.ID, `{"preset":"viral-60-clean","segment_ids":["seg-001"],"edit":{"format":"short-9x16","subscribe_banner":true}}`); rw.Code != http.StatusServiceUnavailable {
		t.Fatalf("a Studio without the kit = %d %s; want 503", rw.Code, rw.Body.String())
	}
	if rw := postGenerate(t, noKit, third.ID, `{"preset":"viral-60-clean","segment_ids":["seg-001"],"edit":{"format":"short-9x16","full_demo":null}}`); rw.Code != http.StatusBadRequest {
		t.Fatalf("an explicit null full_demo = %d; want it refused as before", rw.Code)
	}
}
