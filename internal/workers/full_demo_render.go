package workers

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/overlayassets"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/voicecomms"
)

func fullDemoExecutionArgs(path string) []string {
	// The editor CLI defaults to trimming kill clips 1.5 seconds after the
	// last kill. Full Demo timing belongs to the approved plan instead.
	// Full Demo is one chronological program even when the approved demo
	// contains only one round; the ordinary single-Short default is different.
	return []string{"--full-demo-execution", path, "--tail-trim=0", "--compile-segments"}
}

func (w *RenderWorker) materializeFullDemoExecution(ctx context.Context, j job.Job, snapshot recapplan.Snapshot, dir, ffmpeg string) (string, error) {
	execution := editor.FullDemoExecution{SchemaVersion: "1.0", Approved: snapshot, Assets: []editor.FullDemoLocalMedia{}, VoiceTracks: []editor.FullDemoLocalVoice{}}
	if snapshot.Document.Options.Overlays.HUDTheme != "" {
		telemetry, err := w.materializeFullDemoHUD(ctx, j, snapshot.Document, dir)
		if err != nil {
			return "", err
		}
		execution.HUDTelemetry = telemetry
	}
	for _, ref := range snapshot.Document.Options.AssetReferences() {
		id, err := uuid.Parse(ref.ID)
		if err != nil {
			return "", err
		}
		path := filepath.Join(dir, "asset-"+id.String()+".media")
		key := mediaassets.MediaKey(id)
		if snapshot.Document.Options.IsOverlayImage(ref) {
			key = overlayassets.MediaKey(id)
		}
		if err := materializeStorageFile(w.storage, key, path); err != nil {
			return "", err
		}
		execution.Assets = append(execution.Assets, editor.FullDemoLocalMedia{Ref: ref, Path: path})
	}
	if snapshot.Document.Options.Audio.Voice.Enabled && snapshot.Document.Voice.Availability == voicecomms.Available {
		extraction, err := voicecomms.EnsureStored(ctx, w.storage, j.ID, j.DemoPath, snapshot.Document.Input.DemoSHA256, snapshot.Document.Input.TargetSteamID64, ffmpeg)
		if err != nil {
			return "", err
		}
		if extraction.IndexHash != snapshot.Document.Voice.IndexHash {
			return "", fmt.Errorf("full_demo_plan_stale: voice extraction identity changed")
		}
		for i, track := range extraction.Result.Index.Tracks {
			path := filepath.Join(dir, fmt.Sprintf("voice-%02d.ogg", i))
			if err := materializeStorageFile(w.storage, track.Path, path); err != nil {
				return "", err
			}
			execution.VoiceTracks = append(execution.VoiceTracks, editor.FullDemoLocalVoice{SteamID64: track.SteamID64, StorageKey: track.Path, SHA256: extraction.TrackHashes[track.Path], Path: path})
		}
	}
	path := filepath.Join(dir, "full-demo-execution.json")
	return path, writeJSONFile(path, execution)
}

func (w *RenderWorker) verifyFullDemoCaptureContent(ctx context.Context, id uuid.UUID, result recording.RecordingResult) error {
	for _, artifact := range result.Artifacts {
		if !isSegmentClip(artifact) {
			continue
		}
		if len(artifact.ContentSHA256) != 64 {
			return recording.MarkNotReusable(fmt.Errorf("captured segment lacks a content digest"))
		}
		key, err := result.SegmentClipKey(id, artifact.SegmentID)
		if err != nil {
			return err
		}
		if err := mediaassets.VerifyContent(ctx, w.storage, key, artifact.ContentSHA256, 8<<30); err != nil {
			return recording.MarkNotReusable(err)
		}
	}
	return nil
}

func fullDemoRenderFingerprint(result recording.RecordingResult, variant string, snapshot recapplan.Snapshot) (string, error) {
	if err := recording.ValidateUploadResult(result); err != nil {
		return "", err
	}
	if result.FullDemoEvidence == nil {
		return "", fmt.Errorf("pov_contract_failed: missing Full Demo capture evidence")
	}
	effective, err := recapplan.ApplyCertifiedEnds(snapshot, result.FullDemoEvidence.CertifiedEnds)
	if err != nil {
		return "", err
	}
	if err := result.ValidateFullDemoFrames(effective); err != nil {
		return "", recording.MarkNotReusable(err)
	}
	type input struct {
		SegmentID, ContentSHA256 string
		StartTick, EndTick       int
	}
	inputs := []input{}
	for _, artifact := range result.Artifacts {
		if !isSegmentClip(artifact) {
			continue
		}
		if len(artifact.ContentSHA256) != 64 {
			return "", fmt.Errorf("captured segment lacks content SHA-256")
		}
		for _, segment := range result.Plan.Segments {
			if segment.ID == artifact.SegmentID {
				inputs = append(inputs, input{segment.ID, artifact.ContentSHA256, segment.TickStart, result.FullDemoEvidence.CertifiedEnds[segment.ID]})
				break
			}
		}
	}
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].SegmentID < inputs[j].SegmentID })
	// Timing changes must invalidate rendered overlays without invalidating
	// the source captures or unaffected renders with the roster disabled.
	var introWindow *[2]float64
	if effective.Options.Overlays.Roster {
		introWindow = &[2]float64{demooverlay.IntroOverlayStart(), demooverlay.IntroOverlayEnd()}
	}
	policy := "full-demo-render-v1"
	if snapshot.Document.Options.Overlays.HUDTheme != "" {
		policy += "/" + customhud.Version
	}
	return recapplan.HashValue(struct {
		Policy, Variant, EffectivePlanHash string
		Captures                           []input
		IntroWindow                        *[2]float64 `json:"intro_window,omitempty"`
	}{policy, variant, effective.PlanHash, inputs, introWindow})
}
