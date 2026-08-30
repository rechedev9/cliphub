package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/rechedev9/cliphub/internal/jobprogress"
	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/mediafont"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/tasks"
	"github.com/rechedev9/cliphub/internal/timelineplan"
	"github.com/rechedev9/cliphub/internal/timelinerender"
)

type TimelineRenderRepository interface {
	Get(ctx context.Context, id uuid.UUID) (timelineplan.Project, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, s timelineplan.Status, failureReason string) error
}

type TimelineAssetRepository interface {
	Get(ctx context.Context, id uuid.UUID) (mediaassets.Asset, error)
}

type TimelineRenderWorkerConfig struct {
	WorkDir     string
	FFmpegPath  string
	Timeout     string
	MusicDir    string
	AssetLookup TimelineAssetRepository
}

type TimelineRenderWorker struct {
	repo    TimelineRenderRepository
	assets  TimelineAssetRepository
	storage storage.Storage
	cfg     TimelineRenderWorkerConfig
}

func NewTimelineRenderWorker(repo TimelineRenderRepository, store storage.Storage, cfg TimelineRenderWorkerConfig) *TimelineRenderWorker {
	return &TimelineRenderWorker{repo: repo, assets: cfg.AssetLookup, storage: store, cfg: cfg}
}

func (w *TimelineRenderWorker) HandleRenderTimeline(ctx context.Context, t *asynq.Task) error {
	var payload tasks.RenderTimelinePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("decode timeline render payload: %w", err)
	}
	if err := w.render(ctx, payload); err != nil {
		recordStageFailure(payload.ProjectID, obs.StageEditor, tasks.TypeRenderTimeline, err)
		if updErr := w.repo.UpdateStatus(ctx, payload.ProjectID, timelineplan.StatusFailed, err.Error()); updErr != nil {
			return fmt.Errorf("render timeline: %w (also failed to mark failed: %v)", err, updErr)
		}
		failed := timelineplan.RenderState{
			ProjectID:   payload.ProjectID,
			Status:      timelineplan.StatusFailed,
			Fingerprint: payload.Fingerprint,
			Error:       err.Error(),
			UpdatedAt:   time.Now().UTC(),
		}
		if previous, ok, readErr := w.readState(payload.ProjectID); readErr == nil && ok {
			failed.VideoKey = previous.VideoKey
			failed.CoverKey = previous.CoverKey
			failed.ResultKey = previous.ResultKey
		}
		_ = w.writeState(failed)
		return err
	}
	return nil
}

func (w *TimelineRenderWorker) render(ctx context.Context, payload tasks.RenderTimelinePayload) error {
	if w.cfg.FFmpegPath == "" {
		return fmt.Errorf("ffmpeg path is required")
	}
	if w.assets == nil {
		return fmt.Errorf("editor asset repository is required")
	}
	p, err := w.repo.Get(ctx, payload.ProjectID)
	if err != nil {
		return fmt.Errorf("load editor project: %w", err)
	}
	if len(p.Plan) == 0 {
		return fmt.Errorf("editor project has no timeline")
	}
	doc, err := timelineplan.Decode(p.Plan)
	if err != nil {
		return err
	}
	if err := doc.ValidateForRender(); err != nil {
		return err
	}
	fp, err := timelineplan.Fingerprint(doc)
	if err != nil {
		return err
	}
	if fp != payload.Fingerprint {
		return fmt.Errorf("timeline changed after render was admitted")
	}

	progress := jobprogress.NewKeyedReporter(w.storage, timelineplan.ProgressKey(p.ID), jobprogress.StageRender, jobprogress.UnitClips, "clips")
	if err := progress.Update(0, 1); err != nil {
		return fmt.Errorf("write editor render progress: %w", err)
	}

	timeout := 20 * time.Minute
	if w.cfg.Timeout != "" {
		if parsed, parseErr := time.ParseDuration(w.cfg.Timeout); parseErr == nil && parsed > 0 {
			timeout = parsed
		}
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	workDir := filepath.Join(w.cfg.WorkDir, "editor-"+payload.ProjectID.String())
	if err := os.MkdirAll(workDir, 0o750); err != nil {
		return fmt.Errorf("create editor work dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	inputs := timelinerender.Inputs{
		Assets:     map[string]timelinerender.AssetInput{},
		OutputPath: filepath.Join(workDir, "final.mp4"),
	}
	for _, track := range doc.Tracks {
		for _, item := range track.Items {
			if _, ok := inputs.Assets[item.AssetID]; ok {
				continue
			}
			assetID, err := uuid.Parse(item.AssetID)
			if err != nil {
				return fmt.Errorf("item %s asset id: %w", item.ID, err)
			}
			asset, err := w.assets.Get(ctx, assetID)
			if err != nil {
				return fmt.Errorf("load asset %s: %w", item.AssetID, err)
			}
			path, err := w.materialize(asset.MediaKey, filepath.Join(workDir, asset.ID.String()+".mp4"))
			if err != nil {
				return err
			}
			inputs.Assets[item.AssetID] = timelinerender.AssetInput{Path: path, HasAudio: asset.Probe.HasAudio || asset.Probe.AudioCodec != ""}
		}
	}
	if doc.Music.Key != "" {
		inputs.MusicPath = resolveMusicFile(w.cfg.MusicDir, doc.Music.Key)
	}
	if len(doc.Overlays) > 0 {
		fontPath, err := mediafont.Materialize()
		if err != nil {
			return fmt.Errorf("materialize editor font: %w", err)
		}
		inputs.FontPath = fontPath
		paths, err := timelinerender.WriteOverlayTexts(filepath.Join(workDir, "texts"), doc.Overlays)
		if err != nil {
			return err
		}
		inputs.TextOverlayPaths = paths
	}

	result, err := timelinerender.Render(ctx, w.cfg.FFmpegPath, inputs, doc)
	if err != nil {
		return err
	}

	revision := uuid.New()
	videoKey, err := timelineplan.RenderRevisionVideoKey(p.ID, revision)
	if err != nil {
		return err
	}
	coverKey, err := timelineplan.RenderRevisionCoverKey(p.ID, revision)
	if err != nil {
		return err
	}
	resultKey, err := timelineplan.RenderRevisionResultKey(p.ID, revision)
	if err != nil {
		return err
	}
	if err := w.putFile(videoKey, result.OutputPath); err != nil {
		return err
	}
	if err := w.putFile(coverKey, result.CoverPath); err != nil {
		return err
	}
	record := timelineplan.RenderResult{
		ProjectID:   p.ID,
		AttemptID:   revision,
		Fingerprint: fp,
		VideoKey:    videoKey,
		CoverKey:    coverKey,
		Duration:    result.Duration,
		Width:       result.Width,
		Height:      result.Height,
		Warnings:    result.Warnings,
		Performance: result.Performance,
		CreatedAt:   time.Now().UTC(),
	}
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	if err := w.storage.Put(resultKey, bytes.NewReader(append(raw, '\n'))); err != nil {
		return err
	}
	delivery, err := timelineplan.RenderRevisionDeliveryDir(p.ID, revision)
	if err != nil {
		return err
	}
	if err := w.putFile(delivery+"/final.mp4", result.OutputPath); err != nil {
		return err
	}
	if err := w.putFile(delivery+"/cover.jpg", result.CoverPath); err != nil {
		return err
	}
	if err := w.writeState(timelineplan.RenderState{
		ProjectID:   p.ID,
		AttemptID:   revision,
		Status:      timelineplan.StatusRendered,
		Fingerprint: fp,
		VideoKey:    videoKey,
		CoverKey:    coverKey,
		ResultKey:   resultKey,
		Warnings:    result.Warnings,
		UpdatedAt:   time.Now().UTC(),
	}); err != nil {
		return err
	}
	if err := progress.Complete(1); err != nil {
		return fmt.Errorf("complete editor render progress: %w", err)
	}
	return w.repo.UpdateStatus(ctx, p.ID, timelineplan.StatusRendered, "")
}

func (w *TimelineRenderWorker) materialize(key, dest string) (string, error) {
	if resolver, ok := w.storage.(interface{ ResolvePath(string) (string, error) }); ok {
		path, err := resolver.ResolvePath(key)
		if err == nil {
			if info, statErr := os.Stat(path); statErr == nil && info.Size() > 0 {
				return path, nil
			}
		}
	}
	rc, err := w.storage.Open(key)
	if err != nil {
		return "", fmt.Errorf("open asset %s: %w", key, err)
	}
	defer rc.Close()
	f, err := os.Create(dest) // #nosec G304 -- dest is the caller's resolved local run output path
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, rc); err != nil {
		_ = f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return dest, nil
}

func (w *TimelineRenderWorker) putFile(key, path string) error {
	f, err := os.Open(path) // #nosec G304 -- path is a local run artifact path owned by this worker
	if err != nil {
		return err
	}
	defer f.Close()
	return w.storage.Put(key, f)
}

func (w *TimelineRenderWorker) writeState(state timelineplan.RenderState) error {
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return w.storage.Put(timelineplan.RenderStateKey(state.ProjectID), bytes.NewReader(append(raw, '\n')))
}

func (w *TimelineRenderWorker) readState(id uuid.UUID) (timelineplan.RenderState, bool, error) {
	rc, err := w.storage.Open(timelineplan.RenderStateKey(id))
	if err != nil {
		if storage.IsNotExist(err) {
			return timelineplan.RenderState{}, false, nil
		}
		return timelineplan.RenderState{}, false, err
	}
	defer rc.Close()
	var state timelineplan.RenderState
	if err := json.NewDecoder(rc).Decode(&state); err != nil {
		return timelineplan.RenderState{}, false, err
	}
	return state, true, nil
}
