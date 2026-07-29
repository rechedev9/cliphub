package streamcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/fragforge/internal/storage"
	"github.com/rechedev9/fragforge/internal/streamclips"
)

type fakeStreamService struct {
	probe             streamclips.SourceProbe
	probeErr          error
	renderResult      streamRenderResult
	renderErr         error
	ffmpegErr         error
	probeCalls        int
	renderCalls       int
	ffmpegChecks      int
	probeHasDeadline  bool
	ffmpegHasDeadline bool
	renderHasDeadline bool
	renderRequest     streamRenderRequest
}

func (f *fakeStreamService) Probe(ctx context.Context, _ string, _ string) (streamclips.SourceProbe, error) {
	f.probeCalls++
	_, f.probeHasDeadline = ctx.Deadline()
	return f.probe, f.probeErr
}

func (f *fakeStreamService) ValidateFFmpeg(ctx context.Context, _ string) error {
	f.ffmpegChecks++
	_, f.ffmpegHasDeadline = ctx.Deadline()
	return f.ffmpegErr
}

func TestReplaceLocalPublishDirectoryDoesNotFailAfterPublicationOnCleanupError(t *testing.T) {
	dir := t.TempDir()
	staging := filepath.Join(dir, "staging")
	publish := filepath.Join(dir, "shortslistosparasubir")
	for _, path := range []string{staging, publish} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(staging, "new.txt"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(publish, "old.txt"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	cleanupErr := errors.New("file is temporarily in use")
	if err := replaceLocalPublishDirectoryWithCleanup(staging, publish, func(string) error { return cleanupErr }); err != nil {
		t.Fatalf("replaceLocalPublishDirectoryWithCleanup error = %v, want publication success", err)
	}
	if body, err := os.ReadFile(filepath.Join(publish, "new.txt")); err != nil || string(body) != "new" {
		t.Fatalf("published file = %q, error = %v", body, err)
	}
	backups, err := filepath.Glob(publish + ".previous-*")
	if err != nil || len(backups) != 1 {
		t.Fatalf("backup dirs = %#v, error = %v, want one deferred cleanup", backups, err)
	}
}

func (f *fakeStreamService) Render(ctx context.Context, request streamRenderRequest) (streamRenderResult, error) {
	f.renderCalls++
	_, f.renderHasDeadline = ctx.Deadline()
	f.renderRequest = request
	return f.renderResult, f.renderErr
}

func TestRunStreamVariantsJSONIsMachineReadable(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runStreamWithService([]string{"variants", "--format", "json"}, &stdout, &stderr, &fakeStreamService{})
	if code != exitSuccess || stderr.Len() != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	var result struct {
		OK       bool               `json:"ok"`
		Variants []streamVariantRow `json:"variants"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
	}
	if !result.OK || len(result.Variants) != len(streamclips.VariantNames()) {
		t.Fatalf("result = %#v", result)
	}
	if got, want := result.Variants[0].Name, streamclips.DefaultVariant().Name; got != want {
		t.Fatalf("first variant = %q, want %q", got, want)
	}
}

func TestRunStreamPlanDryRunBuildsValidatedPlanWithoutWriting(t *testing.T) {
	out := filepath.Join(t.TempDir(), "edit-plan.json")
	service := &fakeStreamService{probe: streamclips.SourceProbe{
		Width: 1920, Height: 1080, DurationSeconds: 12.5, VideoCodec: "h264", AudioCodec: "aac",
	}}
	args := []string{
		"plan",
		"--input", "stream.mp4",
		"--out", out,
		"--gameplay-crop", "0.70,0.02,0.29,0.28",
		"--dry-run",
		"--format", "json",
	}
	var stdout, stderr bytes.Buffer
	code := runStreamWithService(args, &stdout, &stderr, service)
	if code != exitSuccess || stderr.Len() != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("dry-run output stat error = %v, want not exist", err)
	}
	var result streamPlanResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.DryRun || result.Executed {
		t.Fatalf("result = %#v", result)
	}
	wantCrop := streamclips.CropRect{X: 0.70, Y: 0.02, Width: 0.29, Height: 0.28}
	if result.Plan.GameplayCrop != wantCrop {
		t.Fatalf("gameplay crop = %#v, want %#v", result.Plan.GameplayCrop, wantCrop)
	}
	if got, want := result.Plan.Clips[0].EndSeconds, 12.5; got != want {
		t.Fatalf("clip end = %v, want %v", got, want)
	}
}

func TestRunStreamPlanWritesEditablePlan(t *testing.T) {
	out := filepath.Join(t.TempDir(), "nested", "edit-plan.json")
	service := &fakeStreamService{probe: streamclips.SourceProbe{DurationSeconds: 9}}
	var stdout, stderr bytes.Buffer
	code := runStreamWithService([]string{
		"plan", "--input", "stream.mp4", "--out", out,
		"--variant", streamclips.VariantStreamerFullframeNoCam,
		"--clip-start", "1", "--clip-end", "8",
	}, &stdout, &stderr, service)
	if code != exitSuccess || stderr.Len() != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var plan streamclips.EditPlan
	if err := json.Unmarshal(body, &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Variant != streamclips.VariantStreamerFullframeNoCam || len(plan.Clips) != 1 {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestRunStreamPlanRefusesToOverwriteSourceVideo(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "stream.mp4")
	if err := os.WriteFile(source, []byte("source-video"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := &fakeStreamService{probe: streamclips.SourceProbe{DurationSeconds: 9}}
	var stdout, stderr bytes.Buffer
	code := runStreamWithService([]string{
		"plan", "--input", source, "--out", source,
	}, &stdout, &stderr, service)
	if code != exitInvalidArgs || !strings.Contains(stderr.String(), "must not overwrite --input") {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	if service.probeCalls != 0 {
		t.Fatalf("probe calls = %d, want 0", service.probeCalls)
	}
	body, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(body), "source-video"; got != want {
		t.Fatalf("source = %q, want %q", got, want)
	}
}

func TestRunStreamPlanReportsProbeFailureAsRuntimeError(t *testing.T) {
	service := &fakeStreamService{probeErr: errors.New("probe unavailable")}
	var stdout, stderr bytes.Buffer
	code := runStreamWithService([]string{
		"plan", "--input", "stream.mp4", "--out", "edit-plan.json", "--format", "json",
	}, &stdout, &stderr, service)
	if code != exitUnexpected || stderr.Len() != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	var result streamErrorResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Error, "probe unavailable") {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunStreamRenderDryRunDoesNotInvokeRenderer(t *testing.T) {
	dir := t.TempDir()
	planPath := writeValidStreamPlan(t, dir, 10)
	service := &fakeStreamService{probe: streamclips.SourceProbe{DurationSeconds: 10, VideoCodec: "h264", AudioCodec: "aac"}}
	var stdout, stderr bytes.Buffer
	code := runStreamWithService([]string{
		"render", "--input", "stream.mp4", "--plan", planPath, "--out", filepath.Join(dir, "run"),
		"--dry-run", "--format", "json",
	}, &stdout, &stderr, service)
	if code != exitSuccess || stderr.Len() != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	if service.renderCalls != 0 {
		t.Fatalf("render calls = %d, want 0", service.renderCalls)
	}
	if service.ffmpegChecks != 1 {
		t.Fatalf("ffmpeg checks = %d, want one ordinary render check", service.ffmpegChecks)
	}
	if !service.probeHasDeadline || !service.ffmpegHasDeadline {
		t.Fatalf("render preflight deadlines: probe=%v ffmpeg=%v", service.probeHasDeadline, service.ffmpegHasDeadline)
	}
	var result streamRenderResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.DryRun || result.Executed || !strings.HasSuffix(result.PublishDir, "shortslistosparasubir") {
		t.Fatalf("result = %#v", result)
	}
}

// TestStreamRenderAcceptsAudibleClipWithoutCaptions pins the removal of the
// Spanish-caption readiness gate. An audible source with a plan that carries no
// caption data is exactly what that gate rejected; render must now accept it and
// reach the renderer instead of failing preflight.
func TestStreamRenderAcceptsAudibleClipWithoutCaptions(t *testing.T) {
	dir := t.TempDir()
	planPath := writeValidStreamPlan(t, dir, 10)
	service := &fakeStreamService{
		probe: streamclips.SourceProbe{
			Width: 1920, Height: 1080, DurationSeconds: 10, VideoCodec: "h264", AudioCodec: "aac",
		},
		renderResult: streamRenderResult{
			OK: true, Executed: true, Variant: streamclips.DefaultVariant().Name,
			PublishDir: filepath.Join(dir, "run", "shortslistosparasubir"),
			Videos:     []streamLocalVideo{},
			Warnings:   []string{},
		},
	}

	var stdout, stderr bytes.Buffer
	code := runStreamWithService([]string{
		"render", "--input", "stream.mp4", "--plan", planPath, "--out", filepath.Join(dir, "run"),
		"--format", "json",
	}, &stdout, &stderr, service)
	if code != exitSuccess || stderr.Len() != 0 {
		t.Fatalf("code = %d, stderr = %q, want an accepted audible render", code, stderr.String())
	}
	if service.renderCalls != 1 {
		t.Fatalf("render calls = %d, want the audible clip to reach the renderer", service.renderCalls)
	}
	var result streamRenderResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("result = %#v, want ok", result)
	}
}

func TestRunStreamRenderDryRunRejectsUnavailableFFmpeg(t *testing.T) {
	dir := t.TempDir()
	planPath := writeValidStreamPlan(t, dir, 10)
	service := &fakeStreamService{
		probe:     streamclips.SourceProbe{DurationSeconds: 10},
		ffmpegErr: errors.New("ffmpeg missing"),
	}
	var stdout, stderr bytes.Buffer
	code := runStreamWithService([]string{
		"render", "--input", "stream.mp4", "--plan", planPath, "--out", filepath.Join(dir, "run"),
		"--ffmpeg", "missing-ffmpeg", "--dry-run", "--format", "json",
	}, &stdout, &stderr, service)
	if code != exitUnexpected || stderr.Len() != 0 || service.renderCalls != 0 {
		t.Fatalf("code = %d, stderr = %q, render calls = %d", code, stderr.String(), service.renderCalls)
	}
	var result streamErrorResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Error, "ffmpeg missing") {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunStreamRenderDryRunRejectsEmptyPlan(t *testing.T) {
	dir := t.TempDir()
	plan := streamclips.DefaultEditPlan()
	planPath := filepath.Join(dir, "empty-plan.json")
	body, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	service := &fakeStreamService{probe: streamclips.SourceProbe{DurationSeconds: 10}}
	var stdout, stderr bytes.Buffer
	code := runStreamWithService([]string{
		"render", "--input", "stream.mp4", "--plan", planPath, "--out", filepath.Join(dir, "run"),
		"--dry-run", "--format", "json",
	}, &stdout, &stderr, service)
	if code != exitInvalidArgs || service.renderCalls != 0 || service.ffmpegChecks != 0 {
		t.Fatalf("code = %d render calls = %d ffmpeg checks = %d", code, service.renderCalls, service.ffmpegChecks)
	}
	if !strings.Contains(stdout.String(), "edit plan has no clips") {
		t.Fatalf("stdout = %s, want empty-plan error", stdout.String())
	}
}

func TestStreamRenderJobIDUsesSemanticPlanFingerprint(t *testing.T) {
	left := streamclips.DefaultEditPlan()
	left.Clips = []streamclips.ClipRange{{ID: "clip-001", StartSeconds: 0, EndSeconds: 10}}
	right := left
	right.UpdatedAt = left.UpdatedAt.Add(time.Hour)

	leftID, err := streamRenderJobID("source-sha", left)
	if err != nil {
		t.Fatal(err)
	}
	rightID, err := streamRenderJobID("source-sha", right)
	if err != nil {
		t.Fatal(err)
	}
	if leftID != rightID {
		t.Fatalf("semantic-equivalent plans produced %s and %s", leftID, rightID)
	}
	right.Clips = []streamclips.ClipRange{{ID: "clip-001", StartSeconds: 1, EndSeconds: 10}}
	changedID, err := streamRenderJobID("source-sha", right)
	if err != nil {
		t.Fatal(err)
	}
	if changedID == leftID {
		t.Fatal("timing change did not change job identity")
	}
}

func TestRunStreamRenderRejectsSourceInsideReplacedPublishDirectory(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "run")
	publishDir := filepath.Join(outDir, "shortslistosparasubir")
	planPath := writeValidStreamPlan(t, dir, 10)
	service := &fakeStreamService{probe: streamclips.SourceProbe{DurationSeconds: 10}}
	var stdout, stderr bytes.Buffer
	code := runStreamWithService([]string{
		"render", "--input", filepath.Join(publishDir, "old.mp4"), "--plan", planPath,
		"--out", outDir, "--dry-run", "--format", "json",
	}, &stdout, &stderr, service)
	if code != exitInvalidArgs || stderr.Len() != 0 || service.renderCalls != 0 || service.ffmpegChecks != 0 {
		t.Fatalf("code = %d, stderr = %q, render calls = %d, ffmpeg checks = %d", code, stderr.String(), service.renderCalls, service.ffmpegChecks)
	}
	var result streamErrorResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Error, "must not be inside publish directory") {
		t.Fatalf("result = %#v", result)
	}
}

// TestRunStreamRenderAcceptsPlanWithRetiredKillfeedAndCaptionKeys pins the
// compatibility promise of the killfeed and burned-caption removal on the
// surface that actually decodes persisted plans strictly: a plan written
// before the removal keeps rendering, without killfeed and without subtitles.
func TestRunStreamRenderAcceptsPlanWithRetiredKillfeedAndCaptionKeys(t *testing.T) {
	dir := t.TempDir()
	planPath := writeStreamPlanJSON(t, dir, legacyStreamPlanJSON)
	service := &fakeStreamService{probe: streamclips.SourceProbe{DurationSeconds: 20}}
	var stdout, stderr bytes.Buffer
	code := runStreamWithService([]string{
		"render", "--input", "stream.mp4", "--plan", planPath, "--out", filepath.Join(dir, "run"),
		"--dry-run", "--format", "json",
	}, &stdout, &stderr, service)
	if code != exitSuccess || stderr.Len() != 0 {
		t.Fatalf(
			"code = %d, stdout = %s, stderr = %q, want a legacy plan to keep loading",
			code, stdout.String(), stderr.String(),
		)
	}
	var result streamRenderResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.DryRun {
		t.Fatalf("result = %#v, want an accepted dry run", result)
	}
}

func TestRunStreamRenderRejectsUnknownPlanKeys(t *testing.T) {
	for _, testCase := range []struct {
		name string
		body string
	}{
		{
			name: "misspelled plan key",
			body: strings.Replace(legacyStreamPlanJSON, `"killfeed_crop"`, `"killfeed_crops"`, 1),
		},
		{
			name: "misspelled captions key",
			body: strings.Replace(legacyStreamPlanJSON, `"captions"`, `"captionz"`, 1),
		},
		{
			name: "misspelled clip key",
			body: strings.Replace(legacyStreamPlanJSON, `"killfeed_seconds"`, `"killfeed_second"`, 1),
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			dir := t.TempDir()
			planPath := writeStreamPlanJSON(t, dir, testCase.body)
			service := &fakeStreamService{probe: streamclips.SourceProbe{DurationSeconds: 20}}
			var stdout, stderr bytes.Buffer
			code := runStreamWithService([]string{
				"render", "--input", "stream.mp4", "--plan", planPath, "--out", filepath.Join(dir, "run"),
				"--dry-run",
			}, &stdout, &stderr, service)
			if code != exitInvalidArgs || !strings.Contains(stderr.String(), "unknown field") {
				t.Fatalf("code = %d, stderr = %q, want a rejected unknown field", code, stderr.String())
			}
			if service.probeCalls != 0 || service.renderCalls != 0 {
				t.Fatalf("probe calls = %d, render calls = %d, want 0, 0", service.probeCalls, service.renderCalls)
			}
		})
	}
}

func TestRunStreamRenderRejectsAdditionalJSONValues(t *testing.T) {
	dir := t.TempDir()
	planPath := writeValidStreamPlan(t, dir, 10)
	f, err := os.OpenFile(planPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\n{}\n"); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	service := &fakeStreamService{probe: streamclips.SourceProbe{DurationSeconds: 10}}
	var stdout, stderr bytes.Buffer
	code := runStreamWithService([]string{
		"render", "--input", "stream.mp4", "--plan", planPath, "--out", filepath.Join(dir, "run"),
	}, &stdout, &stderr, service)
	if code != exitInvalidArgs || !strings.Contains(stderr.String(), "multiple JSON values") {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	if service.probeCalls != 0 || service.renderCalls != 0 {
		t.Fatalf("probe calls = %d, render calls = %d, want 0, 0", service.probeCalls, service.renderCalls)
	}
}

func TestRunStreamRenderInvokesLocalServiceAndReturnsOneJSONDocument(t *testing.T) {
	dir := t.TempDir()
	planPath := writeValidStreamPlan(t, dir, 10)
	want := streamRenderResult{
		OK: true, Executed: true, Variant: streamclips.DefaultVariant().Name,
		PublishDir: filepath.Join(dir, "run", "shortslistosparasubir"),
		Videos:     []streamLocalVideo{{ClipID: "clip-001", Path: filepath.Join(dir, "clip-001.mp4")}},
		Warnings:   []string{},
	}
	service := &fakeStreamService{
		probe:        streamclips.SourceProbe{DurationSeconds: 10, VideoCodec: "h264", AudioCodec: "aac"},
		renderResult: want,
	}
	var stdout, stderr bytes.Buffer
	code := runStreamWithService([]string{
		"render", "--input", "stream.mp4", "--plan", planPath, "--out", filepath.Join(dir, "run"), "--format", "json",
	}, &stdout, &stderr, service)
	if code != exitSuccess || stderr.Len() != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	if service.renderCalls != 1 || service.renderRequest.Plan.Variant != streamclips.DefaultVariant().Name {
		t.Fatalf("render calls = %d request = %#v", service.renderCalls, service.renderRequest)
	}
	if !service.probeHasDeadline || !service.ffmpegHasDeadline || !service.renderHasDeadline {
		t.Fatalf("render deadlines: probe=%v ffmpeg=%v render=%v", service.probeHasDeadline, service.ffmpegHasDeadline, service.renderHasDeadline)
	}
	var got streamRenderResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, stdout.String())
	}
	if got.PublishDir != want.PublishDir || len(got.Videos) != 1 {
		t.Fatalf("got = %#v, want %#v", got, want)
	}
}

func TestRunStreamRenderReportsRenderFailureAsRuntimeError(t *testing.T) {
	dir := t.TempDir()
	planPath := writeValidStreamPlan(t, dir, 10)
	service := &fakeStreamService{
		probe:     streamclips.SourceProbe{DurationSeconds: 10},
		renderErr: errors.New("encoder stopped"),
	}
	var stdout, stderr bytes.Buffer
	code := runStreamWithService([]string{
		"render", "--input", "stream.mp4", "--plan", planPath,
		"--out", filepath.Join(dir, "run"), "--format", "json",
	}, &stdout, &stderr, service)
	if code != exitUnexpected || stderr.Len() != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	var result streamErrorResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Error, "encoder stopped") || service.renderCalls != 1 {
		t.Fatalf("result = %#v render calls = %d", result, service.renderCalls)
	}
}

func TestRunStreamRenderRejectsInvalidTimeoutBeforeIO(t *testing.T) {
	service := &fakeStreamService{}
	var stdout, stderr bytes.Buffer
	code := runStreamWithService([]string{
		"render", "--input", "stream.mp4", "--plan", "missing.json",
		"--out", "run", "--timeout", "eventually",
	}, &stdout, &stderr, service)
	if code != exitInvalidArgs || !strings.Contains(stderr.String(), "invalid --timeout") {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	if service.probeCalls != 0 || service.renderCalls != 0 {
		t.Fatalf("probe calls = %d render calls = %d, want 0, 0", service.probeCalls, service.renderCalls)
	}
}

func TestPublishLocalStreamResultReplacesStalePack(t *testing.T) {
	outDir := t.TempDir()
	store, err := storage.NewLocal(outDir)
	if err != nil {
		t.Fatal(err)
	}
	for key, body := range map[string]string{
		"worker/clip-001.mp4":                             "new-video",
		"shortslistosparasubir/old.mp4":                   "old-video",
		"shortslistosparasubir/nested/old.jpg":            "old-cover",
		"shortslistosparasubir/stream-render-result.json": "old-manifest",
	} {
		if err := store.Put(key, strings.NewReader(body)); err != nil {
			t.Fatalf("put %s: %v", key, err)
		}
	}
	job := streamclips.Job{ID: uuid.New(), Title: "test pack"}
	plan := streamclips.DefaultEditPlan()
	result, err := publishLocalStreamResult(context.Background(), store, job, streamRenderRequest{
		Input: "stream.mp4", PlanPath: "edit-plan.json", OutDir: outDir, Plan: plan,
	}, streamclips.RenderResult{Clips: []streamclips.VideoEntry{{
		ClipID: "clip-001", Key: "worker/clip-001.mp4", DurationSeconds: 1,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"shortslistosparasubir/old.mp4",
		"shortslistosparasubir/nested/old.jpg",
	} {
		exists, err := store.Exists(key)
		if err != nil {
			t.Fatal(err)
		}
		if exists {
			t.Fatalf("stale artifact remains: %s", key)
		}
	}
	for _, key := range []string{
		"shortslistosparasubir/clip-001.mp4",
		"shortslistosparasubir/index.html",
		"shortslistosparasubir/stream-render-result.json",
	} {
		exists, err := store.Exists(key)
		if err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("published artifact missing: %s", key)
		}
	}
	if result.Warnings == nil {
		t.Fatal("warnings = nil, want an empty JSON array")
	}
}

type fakeStreamCoverGenerator struct {
	calls int
	at    float64
}

func (f *fakeStreamCoverGenerator) Generate(_ context.Context, _, _, coverPath string, atSeconds float64) error {
	f.calls++
	f.at = atSeconds
	return os.WriteFile(coverPath, []byte("cover"), 0o600)
}

func TestPublishLocalStreamResultGeneratesCoverInFirstThirdOfClip(t *testing.T) {
	outDir := t.TempDir()
	store, err := storage.NewLocal(outDir)
	if err != nil {
		t.Fatal(err)
	}
	videoKey := "worker/clip-001.mp4"
	if err := store.Put(videoKey, strings.NewReader("video")); err != nil {
		t.Fatal(err)
	}
	plan := streamclips.DefaultEditPlan()
	plan.Clips = []streamclips.ClipRange{{
		ID:           "clip-001",
		StartSeconds: 10,
		EndSeconds:   20,
		Edit:         &streamclips.ClipEdit{Speed: 2},
	}}
	generator := &fakeStreamCoverGenerator{}
	result, err := publishLocalStreamResult(context.Background(), store, streamclips.Job{ID: uuid.New()}, streamRenderRequest{
		Input: "stream.mp4", PlanPath: "edit-plan.json", OutDir: outDir,
		Plan: plan, FFmpeg: "ffmpeg", CoverGenerator: generator,
	}, streamclips.RenderResult{Clips: []streamclips.VideoEntry{{
		ClipID: "clip-001", Key: videoKey, DurationSeconds: 5,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if generator.calls != 1 {
		t.Fatalf("cover calls = %d, want 1", generator.calls)
	}
	if got, want := generator.at, 1.75; math.Abs(got-want) > 0.0001 {
		t.Fatalf("cover timestamp = %.3f, want %.3f", got, want)
	}
	if len(result.Videos) != 1 || !strings.HasSuffix(result.Videos[0].CoverPath, "clip-001.cover.jpg") {
		t.Fatalf("videos = %#v, want generated cover path", result.Videos)
	}
	cover, err := os.ReadFile(result.Videos[0].CoverPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(cover), "cover"; got != want {
		t.Fatalf("cover = %q, want %q", got, want)
	}
	gallery, err := os.ReadFile(result.Gallery)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gallery), "clip-001.cover.jpg") {
		t.Fatalf("gallery does not reference cover: %s", gallery)
	}
}

func TestRunStreamJSONErrorStaysOnStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runStreamWithService([]string{"render", "--format", "json"}, &stdout, &stderr, &fakeStreamService{})
	if code != exitInvalidArgs || stderr.Len() != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	var result streamErrorResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.OK || result.Executed || !strings.Contains(result.Error, "required") {
		t.Fatalf("result = %#v", result)
	}
}

// legacyStreamPlanJSON mirrors a plan persisted before the killfeed and
// burned-caption removal, carrying every retired key at both the plan and the
// clip level.
const legacyStreamPlanJSON = `{
  "schema_version": "1.1",
  "variant": "streamer-vertical-stack-40-60",
  "face_crop": {"x": 0.006, "y": 0.21, "width": 0.25, "height": 0.3},
  "gameplay_crop": {"x": 0, "y": 0, "width": 1, "height": 1},
  "killfeed_crop": {"x": 0.8, "y": 0.05, "width": 0.2, "height": 0.16},
  "killfeed_analysis": {
    "generation_id": "0f1c9a24-2f21-4a7b-9a52-2ab3f1e0c111",
    "status": "applied"
  },
  "captions": {"enabled": true, "language": "es"},
  "clips": [
    {
      "id": "clip-001",
      "start_seconds": 0,
      "end_seconds": 15.15,
      "title": "ZaCkk AWP doble en Inferno",
      "killfeed_seconds": [2.75, 8.625],
      "killfeed_kills": [[{"attacker_side": "CT", "attacker_name": "ZaCkk", "victim_side": "T", "victim_name": "bot", "weapon": "awp"}], []],
      "killfeed_cue_provenance": [{"cue_seconds": 2.75, "origin": "automatic"}],
      "caption_words": [{"word": "hola", "start_seconds": 1, "end_seconds": 1.4}],
      "caption_reviewed": true
    }
  ],
  "updated_at": "2026-07-17T15:47:12.7055431Z"
}`

func writeStreamPlanJSON(t *testing.T, dir string, body string) string {
	t.Helper()
	path := filepath.Join(dir, "edit-plan.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeValidStreamPlan(t *testing.T, dir string, duration float64) string {
	t.Helper()
	plan := streamclips.DefaultEditPlan()
	plan.Clips = []streamclips.ClipRange{{ID: "clip-001", StartSeconds: 0, EndSeconds: duration}}
	body, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "edit-plan.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
