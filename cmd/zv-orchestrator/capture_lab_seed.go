//go:build capturelab

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/store"
)

// captureLabSeed is an explicit, in-memory-only Studio verification fixture.
// It makes real HTTP handlers serve a synthetic render without weakening any
// production capture validator or teaching workers to accept fake captures.
type captureLabSeed struct {
	SchemaVersion       int    `json:"schema_version"`
	JobID               string `json:"job_id"`
	DemoFileName        string `json:"demo_file_name"`
	Variant             string `json:"variant"`
	KillPlanPath        string `json:"killplan_path"`
	RecordingResultPath string `json:"recording_result_path"`
	ShortsResultPath    string `json:"shorts_result_path"`
	PackManifestPath    string `json:"pack_manifest_path"`
	GalleryPath         string `json:"gallery_path"`
	PublishSummaryPath  string `json:"publish_summary_path"`
}

func seedCaptureLabFromEnvironment(ctx context.Context, cfg config, repo store.JobRepository, store storage.Storage) error {
	seedPath := os.Getenv("ZV_CAPTURE_LAB_SEED")
	evidenceRoot := os.Getenv("ZV_CAPTURE_LAB_EVIDENCE_ROOT")
	if seedPath == "" && evidenceRoot == "" {
		return nil
	}
	if seedPath == "" || evidenceRoot == "" {
		return fmt.Errorf("ZV_CAPTURE_LAB_SEED and ZV_CAPTURE_LAB_EVIDENCE_ROOT must be set together")
	}
	if cfg.DatabaseURL != databaseURLMemory {
		return fmt.Errorf("ZV_CAPTURE_LAB_SEED requires ZV_DATABASE_URL=%q", databaseURLMemory)
	}
	if strings.TrimSpace(cfg.FaceitAPIKey) != "" || strings.TrimSpace(cfg.FirecrawlAPIKey) != "" {
		return fmt.Errorf("capture lab seeding cannot run with network-service credentials")
	}
	for _, variable := range []string{"ZV_STEAM_USERNAME", "ZV_STEAM_PASSWORD", "ZV_STEAM_GUARD"} {
		if strings.TrimSpace(os.Getenv(variable)) != "" {
			return fmt.Errorf("capture lab seeding cannot run with %s", variable)
		}
	}
	if err := seedCaptureLab(ctx, seedPath, evidenceRoot, repo, store); err != nil {
		return err
	}
	log.Printf("Capture Lab: loaded build-tagged in-memory Studio seed")
	return nil
}

func seedCaptureLab(ctx context.Context, seedPath, evidenceRoot string, repo store.JobRepository, store storage.Storage) error {
	seedPath, err := captureLabContainedFile(evidenceRoot, evidenceRoot, seedPath)
	if err != nil {
		return fmt.Errorf("seed path: %w", err)
	}
	// #nosec G304,G703 -- captureLabContainedFile confines this explicit fixture to the evidence root.
	raw, err := os.ReadFile(seedPath)
	if err != nil {
		return fmt.Errorf("read seed: %w", err)
	}
	var seed captureLabSeed
	if err := json.Unmarshal(raw, &seed); err != nil {
		return fmt.Errorf("decode seed: %w", err)
	}
	if seed.SchemaVersion != 1 {
		return fmt.Errorf("schema_version must be 1")
	}
	id, err := uuid.Parse(seed.JobID)
	if err != nil || id == uuid.Nil {
		return fmt.Errorf("job_id must be a non-zero UUID")
	}
	loadout, err := renderplan.LoadoutForVariant(seed.Variant)
	if err != nil {
		return err
	}
	resolveSeedPath := func(path string) (string, error) {
		return captureLabContainedFile(evidenceRoot, filepath.Dir(seedPath), path)
	}
	readJSON := func(label, path string, value any) ([]byte, string, error) {
		path, err := resolveSeedPath(path)
		if err != nil {
			return nil, "", fmt.Errorf("%s path: %w", label, err)
		}
		// #nosec G304 -- captureLabContainedFile confines each fixture to the evidence root.
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, "", fmt.Errorf("read %s: %w", label, err)
		}
		if err := json.Unmarshal(content, value); err != nil {
			return nil, "", fmt.Errorf("decode %s: %w", label, err)
		}
		return content, path, nil
	}
	var plan killplan.Plan
	_, killPlanPath, err := readJSON("kill plan", seed.KillPlanPath, &plan)
	if err != nil {
		return err
	}
	if plan.SchemaVersion != killplan.SchemaVersion || len(plan.Segments) == 0 {
		return fmt.Errorf("kill plan must use schema %q and contain segments", killplan.SchemaVersion)
	}
	var captured recording.RecordingResult
	_, recordingResultPath, err := readJSON("recording result", seed.RecordingResultPath, &captured)
	if err != nil {
		return err
	}
	if captured.CaptureMode != recording.CaptureModeFake || captured.CaptureVerified {
		return fmt.Errorf("seed recording result must retain capture_mode=%q and must not claim POV verification", recording.CaptureModeFake)
	}
	if captured.Error != "" || captured.PublicationPending {
		return fmt.Errorf("seed recording result must be a completed non-publishable fake capture")
	}
	if captured.Plan.CaptureContract != recording.CaptureContractVersion ||
		captured.Plan.KillPlanSchemaVersion != killplan.SchemaVersion ||
		captured.Plan.DemoSHA256 != plan.Demo.SHA256 ||
		captured.Plan.TargetSteamID64 != plan.Target.SteamID64 {
		return fmt.Errorf("seed recording plan does not match the kill plan capture contract")
	}
	planSegmentIDs := make([]string, 0, len(plan.Segments))
	for _, segment := range plan.Segments {
		planSegmentIDs = append(planSegmentIDs, segment.ID)
	}
	if strings.Join(captured.Plan.EditorialSegmentIDs, "\x00") != strings.Join(planSegmentIDs, "\x00") {
		return fmt.Errorf("seed recording editorial segments do not match kill plan order")
	}
	fingerprint, err := recording.CaptureInputFingerprint(captured.Plan)
	if err != nil {
		return fmt.Errorf("fingerprint seed recording plan: %w", err)
	}
	if captured.CaptureInputFingerprint != fingerprint {
		return fmt.Errorf("seed recording capture input fingerprint does not match its plan")
	}
	for _, artifact := range captured.Artifacts {
		if _, err := resolveSeedPath(artifact.Path); err != nil {
			return fmt.Errorf("recording artifact %q: %w", artifact.SegmentID, err)
		}
	}
	var rendered editor.Result
	renderedJSON, _, err := readJSON("shorts result", seed.ShortsResultPath, &rendered)
	if err != nil {
		return err
	}
	if !rendered.Executed || rendered.Error != "" || len(rendered.Shorts) != 1 {
		return fmt.Errorf("shorts result must contain one successful executed synthetic render")
	}
	if len(renderplan.CompleteRenderWarnings(rendered)) > 0 {
		return fmt.Errorf("shorts result has unresolved warnings")
	}
	linkedRecordingResult, err := resolveSeedPath(rendered.RecordingResult)
	if err != nil || linkedRecordingResult != recordingResultPath {
		return fmt.Errorf("shorts result recording_result does not match the seed recording result")
	}
	linkedKillPlan, err := resolveSeedPath(rendered.KillPlan)
	if err != nil || linkedKillPlan != killPlanPath {
		return fmt.Errorf("shorts result killplan does not match the seed kill plan")
	}
	short := rendered.Shorts[0]
	if short.OutputFormat != editor.OutputFormatShort9x16 || len(short.Parts) != len(planSegmentIDs) {
		return fmt.Errorf("shorts result must be one 9:16 compilation containing every planned segment")
	}
	for index, part := range short.Parts {
		if part.SegmentID != planSegmentIDs[index] {
			return fmt.Errorf("shorts result part %d is %q, want %q", index, part.SegmentID, planSegmentIDs[index])
		}
	}

	planCopy := plan
	j := job.Job{
		ID:            id,
		Status:        job.StatusRecorded,
		DemoFileName:  seed.DemoFileName,
		DemoPath:      captured.Plan.DemoPath,
		DemoSHA256:    plan.Demo.SHA256,
		TargetSteamID: plan.Target.SteamID64,
		Rules:         rules.Default(),
		KillPlan:      &planCopy,
	}
	if err := repo.Create(ctx, &j); err != nil {
		return fmt.Errorf("create seed job: %w", err)
	}
	state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID: id, Loadout: loadout, Status: renderplan.RenderVariantStatusReady,
	})
	if err != nil {
		return err
	}
	editDocument, err := renderplan.NewEditDocumentForLoadout(renderplan.NewEditDocumentForLoadoutOptions{
		JobID:      id,
		Loadout:    loadout,
		SegmentIDs: planSegmentIDs,
		Edit: renderplan.EditRequest{
			Format:        renderplan.FormatShort9x16,
			KillEffect:    renderplan.KillEffectClean,
			Transition:    renderplan.TransitionCut,
			CoverStrategy: renderplan.CoverStrategyNone,
		},
	})
	if err != nil {
		return fmt.Errorf("build render edit document: %w", err)
	}
	editDocumentJSON, err := json.Marshal(editDocument)
	if err != nil {
		return err
	}
	if err := store.Put(state.EditDocumentKey, bytes.NewReader(editDocumentJSON)); err != nil {
		return fmt.Errorf("store render edit document: %w", err)
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return err
	}
	stateKey, err := renderplan.RenderVariantStateKey(id, seed.Variant)
	if err != nil {
		return err
	}
	if err := store.Put(stateKey, bytes.NewReader(stateJSON)); err != nil {
		return fmt.Errorf("store render state: %w", err)
	}
	resultRef, err := renderplan.NewRenderVariantArtifactRefForState(state, renderplan.RenderVariantArtifactResult, "")
	if err != nil {
		return err
	}
	if err := store.Put(resultRef.Key, bytes.NewReader(renderedJSON)); err != nil {
		return fmt.Errorf("store shorts result: %w", err)
	}
	videoRef, err := renderplan.NewRenderVariantArtifactRefForState(state, renderplan.RenderVariantArtifactVideo, short.SegmentID)
	if err != nil {
		return err
	}
	videoPath, err := resolveSeedPath(short.PublishPath)
	if err != nil {
		return fmt.Errorf("rendered video path: %w", err)
	}
	if err := putCaptureLabFile(store, videoRef.Key, videoPath); err != nil {
		return fmt.Errorf("store rendered video: %w", err)
	}
	if seed.PackManifestPath != "" {
		packRef, err := renderplan.NewRenderVariantArtifactRefForState(state, renderplan.RenderVariantArtifactPackManifest, "")
		if err != nil {
			return err
		}
		packPath, err := resolveSeedPath(seed.PackManifestPath)
		if err != nil {
			return fmt.Errorf("pack manifest path: %w", err)
		}
		if err := putCaptureLabFile(store, packRef.Key, packPath); err != nil {
			return fmt.Errorf("store pack manifest: %w", err)
		}
	}
	if seed.GalleryPath != "" {
		galleryRef, err := renderplan.NewRenderVariantArtifactRefForState(state, renderplan.RenderVariantArtifactGallery, "")
		if err != nil {
			return err
		}
		galleryPath, err := resolveSeedPath(seed.GalleryPath)
		if err != nil {
			return fmt.Errorf("gallery path: %w", err)
		}
		if err := putCaptureLabFile(store, galleryRef.Key, galleryPath); err != nil {
			return fmt.Errorf("store gallery: %w", err)
		}
	}
	if seed.PublishSummaryPath != "" {
		publishSummaryPath, err := resolveSeedPath(seed.PublishSummaryPath)
		if err != nil {
			return fmt.Errorf("publish summary path: %w", err)
		}
		if err := putCaptureLabFile(store, state.PublishSummaryKey, publishSummaryPath); err != nil {
			return fmt.Errorf("store publish summary: %w", err)
		}
	}
	return nil
}

func captureLabContainedFile(root, base, path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is required")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve evidence root: %w", err)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve file: %w", err)
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("file %q escapes evidence root %q", path, root)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("file %q is not regular", path)
	}
	return path, nil
}

func putCaptureLabFile(store storage.Storage, key, filePath string) error {
	// #nosec G304 -- filePath comes from the explicit local seed or generated result.
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	return store.Put(key, file)
}
