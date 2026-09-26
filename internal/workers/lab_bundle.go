package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

// resolvedRenderRequest is a render request resolved against the job: the
// selected recording and the music that will actually be mixed.
type resolvedRenderRequest struct {
	recordingResult recording.RecordingResult
	musicKey        string
	musicPath       string
	music           *renderplan.MusicSnapshot
	musicVolume     float64
	gameVolume      *float64
}

// resolveRenderRequest selects the recording segments, resolves a Full Demo
// approval (adapting the job's kill plan to it) and the effective music. The
// render and the render lab bundle share it so both start from the same inputs.
func (w *RenderWorker) resolveRenderRequest(ctx context.Context, cfg RenderWorkerConfig, j *job.Job, loadout renderplan.Loadout, edit *renderplan.EditRequest, musicKey string, musicVolume float64, gameVolume *float64, segmentIDs []string) (resolvedRenderRequest, error) {
	recordingResult, err := readStoredRecordingResult(w.storage, j.ID)
	if err != nil {
		return resolvedRenderRequest{}, err
	}
	recordingResult, err = selectRenderSegments(recordingResult, segmentIDs)
	if err != nil {
		return resolvedRenderRequest{}, err
	}
	if isFullDemoNativeMix(loadout.Preset, *edit) {
		musicKey = ""
		musicVolume = 0
		gameVolume = nil
	}
	if edit.FullDemo != nil {
		ffmpeg := cfg.FFmpegPath
		if ffmpeg == "" {
			ffmpeg = recording.FindFFmpeg()
		}
		snapshot, approvalErr := recapplan.ResolveApproval(ctx, w.storage, j.ID, j.DemoPath, j.TargetSteamID, ffmpeg, *edit.FullDemo)
		if approvalErr != nil {
			return resolvedRenderRequest{}, approvalErr
		}
		edit.FullDemo = &snapshot
		adapted := snapshot.Document.KillPlan(*j.KillPlan)
		j.KillPlan = &adapted
	}
	resolved := resolvedRenderRequest{recordingResult: recordingResult, musicKey: musicKey, gameVolume: gameVolume, music: &renderplan.MusicSnapshot{}}
	resolved.musicPath = resolveMusicFile(cfg.MusicDir, musicKey)
	if resolved.musicPath != "" {
		resolved.musicVolume = musicVolume
		if resolved.musicVolume <= 0 {
			resolved.musicVolume = 1
		}
		resolved.music = &renderplan.MusicSnapshot{Key: musicKey, Volume: resolved.musicVolume, GameVolume: gameVolume}
	}
	return resolved, nil
}

// LabBundle describes a render input bundle written by PrepareLabBundle.
type LabBundle struct {
	SchemaVersion   string    `json:"schema_version"`
	JobID           uuid.UUID `json:"job_id"`
	Variant         string    `json:"variant"`
	EditDocumentKey string    `json:"edit_document_key"`
	Dir             string    `json:"dir"`
	Args            []string  `json:"args"`
	// Env carries the process environment the editor reads besides its
	// arguments, as the orchestrator had it; zv-editor lab applies it when unset.
	Env map[string]string `json:"env,omitempty"`
}

// labBundleEnv lists the environment variables the editor reads during a
// render that Studio sets on the orchestrator.
var labBundleEnv = []string{"ZV_OVERLAY_RENDERER_PATH"}

// PrepareLabBundle writes into dir the editor inputs a render of the job's
// last committed revision of variant would receive, and the editor arguments
// as editor.LabBundleFile. It replays that revision's edit document, so it
// needs one earlier render of the variant. It never enqueues, runs or records a render:
// the render state is only read.
func (w *RenderWorker) PrepareLabBundle(ctx context.Context, id uuid.UUID, variant, dir string) (LabBundle, error) {
	loadout, err := renderplan.LoadoutForVariant(variant)
	if err != nil {
		return LabBundle{}, err
	}
	j, err := w.repo.Get(ctx, id)
	if err != nil {
		return LabBundle{}, fmt.Errorf("load job %s: %w", id, err)
	}
	if j.KillPlan == nil {
		return LabBundle{}, fmt.Errorf("job %s has no kill plan", id)
	}
	state, ok, err := w.readRenderVariantState(id, variant)
	if err != nil {
		return LabBundle{}, fmt.Errorf("read render state: %w", err)
	}
	if !ok || state == nil || state.EditDocumentKey == "" {
		return LabBundle{}, fmt.Errorf("job %s has no committed %s render to replay; render it once first", id, variant)
	}
	rc, err := w.storage.Open(state.EditDocumentKey)
	if err != nil {
		return LabBundle{}, fmt.Errorf("open edit document: %w", err)
	}
	var doc renderplan.EditDocument
	decodeErr := json.NewDecoder(rc).Decode(&doc)
	_ = rc.Close()
	if decodeErr != nil {
		return LabBundle{}, fmt.Errorf("decode edit document: %w", decodeErr)
	}
	edit := renderplan.NormalizeEditRequest(doc.Edit)
	musicKey, musicVolume := "", 0.0
	var gameVolume *float64
	if doc.Music != nil {
		musicKey, musicVolume, gameVolume = doc.Music.Key, doc.Music.Volume, doc.Music.GameVolume
	}
	cfg := w.cfg.withDefaults()
	resolved, err := w.resolveRenderRequest(ctx, cfg, &j, loadout, &edit, musicKey, musicVolume, gameVolume, doc.Selection.SegmentIDs)
	if err != nil {
		return LabBundle{}, err
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return LabBundle{}, err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return LabBundle{}, err
	}
	invocation, err := w.writeEditorInputs(ctx, cfg, editorInputs{
		job:             j,
		loadout:         loadout,
		edit:            edit,
		recordingResult: resolved.recordingResult,
		music:           resolved.music,
		musicKey:        resolved.musicKey,
		musicPath:       resolved.musicPath,
		musicVolume:     resolved.musicVolume,
		gameVolume:      resolved.gameVolume,
	}, dir)
	if err != nil {
		return LabBundle{}, err
	}
	bundle := LabBundle{SchemaVersion: "1.0", JobID: id, Variant: variant, EditDocumentKey: state.EditDocumentKey, Dir: dir, Args: invocation.args}
	for _, name := range labBundleEnv {
		if value := os.Getenv(name); value != "" {
			if bundle.Env == nil {
				bundle.Env = map[string]string{}
			}
			bundle.Env[name] = value
		}
	}
	if err := writeJSONFile(filepath.Join(dir, editor.LabBundleFile), bundle); err != nil {
		return LabBundle{}, err
	}
	return bundle, nil
}
