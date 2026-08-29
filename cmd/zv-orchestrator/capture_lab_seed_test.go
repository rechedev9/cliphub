//go:build capturelab

package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/storage"
)

func writeCaptureLabJSON(t *testing.T, path string, value any) {
	t.Helper()
	content, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}

func captureLabSeedFixture(t *testing.T, mode recording.CaptureMode, verified bool) (string, uuid.UUID, string) {
	t.Helper()
	dir := t.TempDir()
	id := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	video := filepath.Join(dir, "rendered.mp4")
	if err := os.WriteFile(video, []byte("synthetic-video"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan := killplan.NewPlan()
	plan.Demo.SHA256 = strings.Repeat("a", 64)
	plan.Demo.Tickrate = 64
	plan.Demo.DurationTicks = 1000
	plan.Target.SteamID64 = "76561198377256168"
	plan.Target.NameInDemo = "target"
	plan.Segments = []killplan.Segment{{ID: "seg-001", TickStart: 100, TickEnd: 200}}
	killPlanPath := filepath.Join(dir, "killplan.json")
	writeCaptureLabJSON(t, killPlanPath, plan)
	capturePlan, err := recording.NewPlanFromKillPlan(plan, filepath.Join(dir, "fixture.dem"), filepath.Join(dir, "capture"), recording.DefaultStreamConfig())
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := recording.CaptureInputFingerprint(capturePlan)
	if err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(dir, "captured.mp4")
	if err := os.WriteFile(artifactPath, []byte("synthetic-capture"), 0o600); err != nil {
		t.Fatal(err)
	}
	recordingResultPath := filepath.Join(dir, "recording-result.json")
	writeCaptureLabJSON(t, recordingResultPath, recording.RecordingResult{
		CaptureMode: mode, CaptureVerified: verified,
		Plan: capturePlan, CaptureInputFingerprint: fingerprint,
		Artifacts: []recording.RecordingArtifact{{SegmentID: "seg-001", Role: "segment", Type: "video", Path: artifactPath}},
	})
	shortsResultPath := filepath.Join(dir, "shorts-result.json")
	writeCaptureLabJSON(t, shortsResultPath, editor.Result{
		RecordingResult: recordingResultPath,
		KillPlan:        killPlanPath,
		OutputFormat:    editor.OutputFormatShort9x16,
		Executed:        true,
		Shorts: []editor.ShortResult{{
			SegmentID: "demo-compilation", PublishPath: video,
			OutputFormat: editor.OutputFormatShort9x16,
			Parts:        []editor.ShortPart{{SegmentID: "seg-001"}},
		}},
	})
	seedPath := filepath.Join(dir, "seed.json")
	writeCaptureLabJSON(t, seedPath, captureLabSeed{
		SchemaVersion: 1, JobID: id.String(), DemoFileName: "capturelab.dem",
		Variant: "viral-60-clean", KillPlanPath: killPlanPath,
		RecordingResultPath: recordingResultPath, ShortsResultPath: shortsResultPath,
	})
	return seedPath, id, video
}

func TestSeedCaptureLabPublishesFakeEvidenceOnlyInsideInMemoryServer(t *testing.T) {
	seedPath, id, _ := captureLabSeedFixture(t, recording.CaptureModeFake, false)
	repo := newMemoryJobRepository()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := seedCaptureLab(context.Background(), seedPath, filepath.Dir(seedPath), repo, store); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != job.StatusRecorded || got.KillPlan == nil || got.DemoFileName != "capturelab.dem" {
		t.Fatalf("seeded job = %+v", got)
	}
	stateKey, err := renderplan.RenderVariantStateKey(id, "viral-60-clean")
	if err != nil {
		t.Fatal(err)
	}
	rc, err := store.Open(stateKey)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	var state renderplan.RenderVariantState
	if err := json.NewDecoder(rc).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if state.Status != renderplan.RenderVariantStatusReady {
		t.Fatalf("state status = %q", state.Status)
	}
	editArtifact, err := store.Open(state.EditDocumentKey)
	if err != nil {
		t.Fatal(err)
	}
	defer editArtifact.Close()
	var editDocument renderplan.EditDocument
	if err := json.NewDecoder(editArtifact).Decode(&editDocument); err != nil {
		t.Fatal(err)
	}
	if editDocument.Edit.Format != renderplan.FormatShort9x16 ||
		editDocument.Edit.KillEffect != renderplan.KillEffectClean ||
		editDocument.Edit.Transition != renderplan.TransitionCut ||
		editDocument.Edit.CoverStrategy != renderplan.CoverStrategyNone {
		t.Fatalf("seed edit contract = %+v", editDocument.Edit)
	}
	videoRef, err := renderplan.NewRenderVariantArtifactRefForState(state, renderplan.RenderVariantArtifactVideo, "demo-compilation")
	if err != nil {
		t.Fatal(err)
	}
	video, err := store.Open(videoRef.Key)
	if err != nil {
		t.Fatal(err)
	}
	defer video.Close()
	content, err := io.ReadAll(video)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "synthetic-video" {
		t.Fatalf("video content = %q", content)
	}
}

func TestSeedCaptureLabRejectsEvidenceOutsideRoot(t *testing.T) {
	seedPath, _, _ := captureLabSeedFixture(t, recording.CaptureModeFake, false)
	var seed captureLabSeed
	content, err := os.ReadFile(seedPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(content, &seed); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside-killplan.json")
	if err := os.WriteFile(outside, []byte(`{"schema_version":"1.0"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	seed.KillPlanPath = outside
	writeCaptureLabJSON(t, seedPath, seed)
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = seedCaptureLab(context.Background(), seedPath, filepath.Dir(seedPath), newMemoryJobRepository(), store)
	if err == nil || !strings.Contains(err.Error(), "escapes evidence root") {
		t.Fatalf("seedCaptureLab error = %v", err)
	}
}

func TestSeedCaptureLabRejectsMismatchedCaptureFingerprint(t *testing.T) {
	seedPath, _, _ := captureLabSeedFixture(t, recording.CaptureModeFake, false)
	var seed captureLabSeed
	content, err := os.ReadFile(seedPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(content, &seed); err != nil {
		t.Fatal(err)
	}
	var result recording.RecordingResult
	content, err = os.ReadFile(seed.RecordingResultPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(content, &result); err != nil {
		t.Fatal(err)
	}
	result.CaptureInputFingerprint = strings.Repeat("b", 64)
	writeCaptureLabJSON(t, seed.RecordingResultPath, result)
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = seedCaptureLab(context.Background(), seedPath, filepath.Dir(seedPath), newMemoryJobRepository(), store)
	if err == nil || !strings.Contains(err.Error(), "fingerprint") {
		t.Fatalf("seedCaptureLab error = %v", err)
	}
}

func TestCaptureLabBuildRequiresIsolatedConfiguration(t *testing.T) {
	seedPath, _, _ := captureLabSeedFixture(t, recording.CaptureModeFake, false)
	tests := []struct {
		name string
		cfg  config
		env  map[string]string
		want string
	}{
		{name: "sqlite", cfg: config{DatabaseURL: databaseURLSQLite}, want: "requires ZV_DATABASE_URL"},
		{name: "faceit", cfg: config{DatabaseURL: databaseURLMemory, FaceitAPIKey: "secret"}, want: "network-service credentials"},
		{name: "firecrawl", cfg: config{DatabaseURL: databaseURLMemory, FirecrawlAPIKey: "secret"}, want: "network-service credentials"},
		{name: "steam", cfg: config{DatabaseURL: databaseURLMemory}, env: map[string]string{"ZV_STEAM_PASSWORD": "secret"}, want: "ZV_STEAM_PASSWORD"},
		{name: "missing evidence root", cfg: config{DatabaseURL: databaseURLMemory}, env: map[string]string{"ZV_CAPTURE_LAB_EVIDENCE_ROOT": ""}, want: "must be set together"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ZV_CAPTURE_LAB_SEED", seedPath)
			t.Setenv("ZV_CAPTURE_LAB_EVIDENCE_ROOT", filepath.Dir(seedPath))
			for name, value := range tt.env {
				t.Setenv(name, value)
			}
			store, err := storage.NewLocal(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			err = seedCaptureLabFromEnvironment(context.Background(), tt.cfg, newMemoryJobRepository(), store)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("seedCaptureLabFromEnvironment error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestSeedCaptureLabRejectsAnythingClaimingRealVerification(t *testing.T) {
	tests := []struct {
		name     string
		mode     recording.CaptureMode
		verified bool
	}{
		{name: "real", mode: recording.CaptureModeReal, verified: true},
		{name: "fake verified", mode: recording.CaptureModeFake, verified: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seedPath, _, _ := captureLabSeedFixture(t, tt.mode, tt.verified)
			store, err := storage.NewLocal(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			err = seedCaptureLab(context.Background(), seedPath, filepath.Dir(seedPath), newMemoryJobRepository(), store)
			if err == nil || !strings.Contains(err.Error(), `capture_mode="fake"`) {
				t.Fatalf("seedCaptureLab error = %v", err)
			}
		})
	}
}
