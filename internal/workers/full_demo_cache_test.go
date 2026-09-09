package workers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

// Model only the storage contract. These bytes and receipts never enter the
// production render worker or claim a media/capture verification.
func seedFullDemoCache(t *testing.T, hudTheme ...string) (*fakeStorage, renderplan.RenderVariantState, string) {
	t.Helper()
	store := newFakeStorage()
	recording, _, _ := fullDemoPublicationFixture(t, "unused")
	doc := *recording.Plan.FullDemo
	if len(hudTheme) > 0 {
		doc.Options.Overlays.HUDTheme = hudTheme[0]
		doc.Options.Capture.HUDProfile = customhud.CaptureProfile
		var err error
		doc.PlanHash, err = doc.Hash()
		if err != nil {
			t.Fatal(err)
		}
	}
	snapshot := recapplan.Snapshot{Document: doc, Approval: recapplan.Approval{PlanHash: doc.PlanHash, AllowSafeTailTrim: true, Timestamp: time.Now().UTC()}}
	loadout, err := renderplan.LoadoutForVariant("gameplay-pov-60")
	if err != nil {
		t.Fatal(err)
	}
	state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{JobID: uuid.New(), Loadout: loadout, Status: renderplan.RenderVariantStatusReady, FullDemo: &snapshot, RevisionID: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	video, err := renderplan.NewRenderVariantArtifactRefForState(state, renderplan.RenderVariantArtifactVideo, compilationSegmentID)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("modeled committed video")
	digest := sha256.Sum256(body)
	frames := doc.Timeline[len(doc.Timeline)-1].EndFrame
	integrated, peak := -14.0, -1.8
	evidence := editor.FullDemoRenderEvidence{SchemaVersion: "1.0", Approved: snapshot, Effective: doc,
		Delivery:        &editor.FullDemoDeliveryEvidence{FullDecode: true, FrameCount: frames, SampleRate: 48000, Channels: 2, DurationSeconds: float64(frames) / 60, ContentSHA256: hex.EncodeToString(digest[:])},
		ProgramLoudness: &editor.ProgramLoudnessEvidence{Status: "verified-decoded-aac", DecodedAAC: []editor.LoudnessMeasurement{{Status: "measured", IntegratedLUFS: &integrated, TruePeakDBTP: &peak}}}}
	if len(hudTheme) > 0 {
		evidence.HUD = &editor.FullDemoHUDEvidence{RendererVersion: customhud.Version, Theme: hudTheme[0], DemoSHA256: doc.Input.DemoSHA256, TelemetrySHA256: hex.EncodeToString(digest[:]), SnapshotCount: 2}
	}
	if err := evidence.ValidateCompleted(); err != nil {
		t.Fatal(err)
	}
	result := editor.Result{InputFingerprint: "same-render-inputs", Shorts: []editor.ShortResult{{SegmentID: compilationSegmentID, FullDemo: &evidence}}}
	store.files[state.RenderResultKey], err = json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{state.EditDocumentKey, state.EditManifestKey, state.PackManifestKey, state.GalleryKey, state.PublishSummaryKey} {
		store.files[key] = []byte("existing document")
	}
	caption, err := renderplan.NewRenderVariantArtifactRefForState(state, renderplan.RenderVariantArtifactCaption, compilationSegmentID)
	if err != nil {
		t.Fatal(err)
	}
	store.files[caption.Key], store.files[video.Key] = []byte("caption"), body
	for name, value := range editor.FullDemoDocumentFiles(evidence) {
		store.files[path.Join(state.ArtifactPrefix, name)], err = json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
	}
	return store, state, video.Key
}

func TestFullDemoHUDPublicationAndCache(t *testing.T) {
	store, state, _ := seedFullDemoCache(t, "arena")
	var result editor.Result
	if err := json.Unmarshal(store.files[state.RenderResultKey], &result); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	result.Shorts[0].Output = filepath.Join(dir, "video.mp4")
	result.Shorts[0].CaptionPath = filepath.Join(dir, "caption.txt")
	targets, err := renderplan.NewRenderVariantUploadTargets(renderplan.NewRenderVariantUploadTargetsOptions{JobID: state.JobID, Variant: state.Variant, RevisionID: uuid.MustParse(path.Base(state.ArtifactPrefix)), OutDir: dir, PublishDir: dir, ResultPath: filepath.Join(dir, "result.json"), Result: result})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, target := range targets[:len(targets)-1] {
		if target.Key == path.Join(state.ArtifactPrefix, "full-demo-hud.json") {
			found = target.Required && target.Path == filepath.Join(dir, "full-demo-hud.json")
		}
	}
	if !found || targets[len(targets)-1].Key != state.RenderResultKey {
		t.Fatal("HUD evidence must be published before the render commit marker")
	}
	ready, _, _, err := renderVariantOutputsReadyContext(context.Background(), store, state.JobID, state.Variant, "same-render-inputs", &state)
	if err != nil || !ready {
		t.Fatalf("complete custom HUD cache: %v / %v", ready, err)
	}
	delete(store.files, path.Join(state.ArtifactPrefix, "full-demo-hud.json"))
	ready, _, _, err = renderVariantOutputsReadyContext(context.Background(), store, state.JobID, state.Variant, "same-render-inputs", &state)
	if err != nil || ready {
		t.Fatalf("missing HUD evidence accepted: %v / %v", ready, err)
	}
	for _, mutate := range []func(*editor.FullDemoRenderEvidence){
		func(e *editor.FullDemoRenderEvidence) { e.HUD = nil },
		func(e *editor.FullDemoRenderEvidence) { e.HUD.Theme = "apex" },
		func(e *editor.FullDemoRenderEvidence) { e.HUD.DemoSHA256 = "other-demo" },
		func(e *editor.FullDemoRenderEvidence) { e.HUD.TelemetrySHA256 = "invalid" },
		func(e *editor.FullDemoRenderEvidence) { e.HUD.SnapshotCount = 0 },
		func(e *editor.FullDemoRenderEvidence) { e.HUD.RendererVersion = "other-renderer" },
	} {
		original := result.Shorts[0].FullDemo
		changed := *original
		hud := *original.HUD
		changed.HUD = &hud
		mutate(&changed)
		if err := changed.ValidateCompleted(); err == nil {
			t.Fatal("custom HUD delivery accepted missing or mismatched evidence")
		}
	}
}

func TestFullDemoCacheRequiresCommittedBytesAndDocuments(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*fakeStorage, *renderplan.RenderVariantState, string)
		ready  bool
	}{
		{"ready", func(*fakeStorage, *renderplan.RenderVariantState, string) {}, true},
		{"queued retry", func(_ *fakeStorage, s *renderplan.RenderVariantState, _ string) {
			s.Status = renderplan.RenderVariantStatusQueued
		}, true},
		{"interrupted retry", func(_ *fakeStorage, s *renderplan.RenderVariantState, _ string) {
			s.Status = renderplan.RenderVariantStatusFailed
		}, true},
		{"altered video", func(s *fakeStorage, _ *renderplan.RenderVariantState, key string) {
			s.files[key] = []byte("different bytes")
		}, false},
		{"missing video", func(s *fakeStorage, _ *renderplan.RenderVariantState, key string) { delete(s.files, key) }, false},
		{"changed approval", func(_ *fakeStorage, s *renderplan.RenderVariantState, _ string) {
			s.FullDemo.Approval.PlanHash = "different"
		}, false},
		{"legacy request", func(_ *fakeStorage, s *renderplan.RenderVariantState, _ string) { s.FullDemo = nil }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, state, key := seedFullDemoCache(t)
			tc.mutate(store, &state, key)
			ready, _, _, err := renderVariantOutputsReadyContext(context.Background(), store, state.JobID, state.Variant, "same-render-inputs", &state)
			if err != nil || ready != tc.ready {
				t.Fatalf("ready = %v, err = %v", ready, err)
			}
		})
	}
	for _, name := range []string{"approved", "effective", "audio", "loudness", "delivery"} {
		for _, mode := range []string{"missing", "altered", "invalid JSON"} {
			t.Run(name+"/"+mode, func(t *testing.T) {
				store, state, _ := seedFullDemoCache(t)
				key := path.Join(state.ArtifactPrefix, "full-demo-"+name+".json")
				switch mode {
				case "missing":
					delete(store.files, key)
				case "altered":
					store.files[key] = []byte(`{"status":"different"}`)
				case "invalid JSON":
					store.files[key] = []byte(`{`)
				}
				ready, _, _, err := renderVariantOutputsReadyContext(context.Background(), store, state.JobID, state.Variant, "same-render-inputs", &state)
				if err != nil || ready {
					t.Fatalf("damaged evidence accepted: %v / %v", ready, err)
				}
			})
		}
	}
}

func TestFullDemoCommittedCaptureChecksContent(t *testing.T) {
	for _, damage := range []string{"none", "altered", "missing", "cancelled"} {
		t.Run(damage, func(t *testing.T) {
			store, id := newFakeStorage(), uuid.New()
			result, dir, resultPath := fullDemoPublicationFixture(t, "original clip")
			if _, err := uploadFullDemoRecordingOutputs(store, id, dir, resultPath, result, result, false); err != nil {
				t.Fatal(err)
			}
			stored, err := decodeStoredRecordingResult(store, id)
			if err != nil {
				t.Fatal(err)
			}
			key, err := stored.SegmentClipKey(id, "round-001")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch damage {
			case "altered":
				store.files[key] = []byte("other clip")
			case "missing":
				delete(store.files, key)
			case "cancelled":
				cancel()
			}
			ready, err := recordingCommitReady(store, id, stored, ctx)
			if ready != (damage == "none") || (err != nil) != (damage == "cancelled") {
				t.Fatalf("ready = %v, err = %v", ready, err)
			}
		})
	}
}
