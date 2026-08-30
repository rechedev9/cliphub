package httpapi

import (
	"bytes"
	"encoding/json"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/jobprogress"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/storage"
)

// captureProgressView is the HTTP shape for any long wait: percent plus
// current/total in the worker's real units. Capture still fills it from
// capture-progress.json; other stages load jobprogress snapshots.
type captureProgressView struct {
	Done    int    `json:"done"`
	Total   int    `json:"total"`
	Percent int    `json:"percent"`
	Unit    string `json:"unit,omitempty"`
	Label   string `json:"label,omitempty"`
	Stage   string `json:"stage,omitempty"`
}

func viewFromSnapshot(s jobprogress.Snapshot) captureProgressView {
	return captureProgressView{
		Done:    int(s.Done),
		Total:   int(s.Total),
		Percent: s.Percent,
		Unit:    s.Unit,
		Label:   s.Label,
		Stage:   s.Stage,
	}
}

func loadProgressView(store storage.Storage, key string) (captureProgressView, bool) {
	snap, ok, err := jobprogress.Load(store, key)
	if err != nil || !ok {
		return captureProgressView{}, false
	}
	return viewFromSnapshot(snap), true
}

func progressRelevant(status job.Status, stage string) bool {
	switch status {
	case job.StatusScanning:
		return stage == jobprogress.StageScan
	case job.StatusParsing:
		return stage == jobprogress.StageParse
	case job.StatusComposing:
		return stage == jobprogress.StageCompose
	case job.StatusRecording:
		return stage == jobprogress.StageRecord
	default:
		return false
	}
}

func renderWaitStage(stage string) bool {
	return stage == jobprogress.StageCompose || stage == jobprogress.StageRender
}

func renderWaitProgress(store storage.Storage, id uuid.UUID, startedAt time.Time) (captureProgressView, bool) {
	return loadProgressViewIf(store, artifacts.ProgressKey(id), func(snap jobprogress.Snapshot) bool {
		return renderWaitStage(snap.Stage) && snapshotFromThisRun(snap.UpdatedAt, startedAt) && !zeroRenderWait(snap)
	})
}

// zeroRenderWait is the 0/N placeholder written at encode start. Biblioteca
// must not snap EDITANDO from 8/20 down to 0/20 while ffmpeg concats.
func zeroRenderWait(snap jobprogress.Snapshot) bool {
	return renderWaitStage(snap.Stage) && snap.Done == 0 && snap.Total > 0
}

func inFlightRenderState(store storage.Storage, id uuid.UUID) (renderplan.RenderVariantState, bool) {
	for _, loadout := range renderplan.LoadoutCatalog() {
		key, err := renderplan.RenderVariantStateKey(id, loadout.Variant)
		if err != nil {
			continue
		}
		var state renderplan.RenderVariantState
		found, err := readRenderStateJSON(store, key, &state)
		if err != nil || !found {
			continue
		}
		if state.Status == renderplan.RenderVariantStatusQueued || state.Status == renderplan.RenderVariantStatusRendering {
			return state, true
		}
	}
	return renderplan.RenderVariantState{}, false
}

func readRenderStateJSON(store storage.Storage, key string, dst *renderplan.RenderVariantState) (bool, error) {
	rc, err := store.Open(key)
	if err != nil {
		if storage.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer rc.Close()
	if err := json.NewDecoder(rc).Decode(dst); err != nil {
		return true, err
	}
	return true, nil
}

type captureKindDocument struct {
	Recap bool `json:"recap"`
}

func writeCaptureKind(store storage.Storage, id uuid.UUID, recap bool) error {
	body, err := json.Marshal(captureKindDocument{Recap: recap})
	if err != nil {
		return err
	}
	return store.Put(artifacts.CaptureKindKey(id), bytes.NewReader(body))
}

func captureIsRecap(store storage.Storage, id uuid.UUID) bool {
	rc, err := store.Open(artifacts.CaptureKindKey(id))
	if err != nil {
		return false
	}
	defer rc.Close()
	var doc captureKindDocument
	if err := json.NewDecoder(rc).Decode(&doc); err != nil {
		return false
	}
	return doc.Recap
}

func captureLabels(store storage.Storage, id uuid.UUID) (unit, label string) {
	if captureIsRecap(store, id) {
		return jobprogress.UnitRounds, "rondas"
	}
	return jobprogress.UnitSegments, "segmentos"
}

func snapshotFromThisRun(updatedAt, startedAt time.Time) bool {
	if startedAt.IsZero() || updatedAt.IsZero() {
		return true
	}
	return !updatedAt.Before(startedAt)
}

func loadProgressViewIf(store storage.Storage, key string, keep func(jobprogress.Snapshot) bool) (captureProgressView, bool) {
	snap, ok, err := jobprogress.Load(store, key)
	if err != nil || !ok {
		return captureProgressView{}, false
	}
	if keep != nil && !keep(snap) {
		return captureProgressView{}, false
	}
	return viewFromSnapshot(snap), true
}

func decorateCaptureProgress(store storage.Storage, id uuid.UUID, progress captureProgressView) captureProgressView {
	progress.Unit, progress.Label = captureLabels(store, id)
	progress.Stage = jobprogress.StageRecord
	return progress
}

func jobProgressView(store storage.Storage, id uuid.UUID, status job.Status, segmentCount int) (captureProgressView, bool) {
	if progress, ok := captureProgressWithTotal(store, id, status, segmentCount); ok {
		return decorateCaptureProgress(store, id, progress), true
	}
	if snap, ok, err := jobprogress.LoadJob(store, id); err == nil && ok && progressRelevant(status, snap.Stage) && !zeroRenderWait(snap) {
		return viewFromSnapshot(snap), true
	}
	if state, ok := inFlightRenderState(store, id); ok {
		if progress, ok := renderWaitProgress(store, id, state.UpdatedAt); ok {
			return progress, true
		}
	}
	return captureProgressView{}, false
}

// captureProgress derives capture progress for a recording job from durable
// state alone, so the web poll can render a real percent without a side channel.
// Progress is scoped to the in-flight reel: the record worker persists the
// ordered segment ids it will capture (the capture-selection artifact), so total
// is that reel's segment count and done counts only the reel's completed clips,
// ignoring stale clips a previous reel of the same job left behind. When no
// selection artifact exists (an older job recorded before this was added), it
// falls back to the whole kill plan and counts every clip under the segments dir.
//
// It reports ok=false - so the caller omits progress and the card keeps its
// existing rendering - whenever progress is not meaningful: the job is not
// capturing, neither a current capture selection nor a kill plan is available,
// the storage backend cannot list a directory, or no completed segment exists
// yet (the segments dir is still absent/empty).
// A segment mid-write is briefly counted or missed; the poll tolerates that.
func captureProgress(store storage.Storage, j job.Job) (captureProgressView, bool) {
	total := 0
	if j.KillPlan != nil {
		total = len(j.KillPlan.Segments)
	}
	return captureProgressWithTotal(store, j.ID, j.Status, total)
}

func captureProgressWithTotal(store storage.Storage, id uuid.UUID, status job.Status, fallbackTotal int) (captureProgressView, bool) {
	if status != job.StatusRecording {
		return captureProgressView{}, false
	}
	if progress, ok := captureProgressDocument(store, id); ok {
		return progress, true
	}
	// Resolve the segments directory from the same key builder the recorder
	// writes through, so the two never drift on the on-disk layout.
	ref, err := artifacts.SegmentClipKey(id, artifactNamePlaceholder)
	if err != nil {
		return captureProgressView{}, false
	}
	files, ok := listArtifactDir(store, ref)
	if !ok {
		return captureProgressView{}, false
	}

	selection, hasSelection, err := readCaptureSelection(store, id)
	if err != nil {
		return captureProgressView{}, false
	}

	total := 0
	done := 0
	if hasSelection {
		total = len(selection)
		inSelection := make(map[string]bool, len(selection))
		for _, id := range selection {
			inSelection[id] = true
		}
		for _, f := range files {
			if strings.EqualFold(path.Ext(f), ".mp4") && inSelection[strings.TrimSuffix(f, path.Ext(f))] {
				done++
			}
		}
	} else {
		if fallbackTotal == 0 {
			return captureProgressView{}, false
		}
		total = fallbackTotal
		for _, f := range files {
			if strings.EqualFold(path.Ext(f), ".mp4") {
				done++
			}
		}
	}
	if total == 0 || done == 0 {
		return captureProgressView{}, false
	}
	if done > total {
		done = total
	}
	return captureProgressView{Done: done, Total: total, Percent: recording.CaptureWorkPercent(total, done, nil, 0)}, true
}

func captureProgressDocument(store storage.Storage, id uuid.UUID) (captureProgressView, bool) {
	rc, err := store.Open(artifacts.CaptureProgressKey(id))
	if err != nil {
		return captureProgressView{}, false
	}
	defer rc.Close()
	var progress recording.CaptureProgress
	if err := json.NewDecoder(rc).Decode(&progress); err != nil || progress.Validate() != nil {
		return captureProgressView{}, false
	}
	return captureProgressView{
		Done:    len(progress.CompletedSegmentIDs),
		Total:   len(progress.SegmentIDs),
		Percent: documentPercent(progress),
	}, true
}

func documentPercent(p recording.CaptureProgress) int {
	if p.Percent > 0 || len(p.CompletedSegmentIDs) == 0 {
		if p.Percent < 0 {
			return 0
		}
		if p.Percent > 100 {
			return 100
		}
		return p.Percent
	}
	return recording.CaptureWorkPercent(len(p.SegmentIDs), len(p.CompletedSegmentIDs), nil, 0)
}

// readCaptureSelection reads the ordered segment ids the in-flight record run
// will capture. A missing artifact (an older job) is not an error: hasSelection
// is false and the caller falls back to the whole kill plan.
func readCaptureSelection(store storage.Storage, id uuid.UUID) (ids []string, hasSelection bool, err error) {
	rc, err := store.Open(artifacts.CaptureSelectionKey(id))
	if err != nil {
		if storage.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer rc.Close()
	if err := json.NewDecoder(rc).Decode(&ids); err != nil {
		return nil, false, err
	}
	return ids, true, nil
}
