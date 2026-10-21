package httpapi

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/generateintent"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

type storagePathResolver interface {
	ResolvePath(key string) (string, error)
}

type jobStorageUsage struct {
	ID           uuid.UUID `json:"id"`
	CaptureBytes int64     `json:"capture_bytes"`
	RenderBytes  int64     `json:"render_bytes"`
	Freeable     bool      `json:"freeable"`
	newestClip   time.Time
	totalBytes   int64
}

// GetStorage handles GET /api/storage: the disk use of every job tree, with
// the Full Demo round captures that DeleteJobCaptures can free.
func (h *Handlers) GetStorage(w http.ResponseWriter, r *http.Request) {
	resolver, ok := h.storage.(storagePathResolver)
	if !ok {
		writeError(w, http.StatusNotImplemented, "storage backend does not report disk use")
		return
	}
	root, err := resolver.ResolvePath("jobs")
	if err != nil {
		internalError(w, "resolve jobs dir", err)
		return
	}
	entries, err := os.ReadDir(root)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		internalError(w, "list jobs dir", err)
		return
	}
	jobs := []jobStorageUsage{}
	var total int64
	for _, entry := range entries {
		id, err := uuid.Parse(entry.Name())
		if err != nil || !entry.IsDir() {
			continue
		}
		usage, err := measureJobTree(filepath.Join(root, entry.Name()))
		if err != nil {
			internalError(w, "measure job dir", err)
			return
		}
		usage.ID = id
		if usage.CaptureBytes > 0 {
			if usage.Freeable, err = h.fullDemoRenderCovers(id, usage.newestClip); err != nil {
				internalError(w, "read render state", err)
				return
			}
		}
		total += usage.totalBytes
		jobs = append(jobs, usage)
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs, "total_bytes": total})
}

// DeleteJobCaptures handles DELETE /api/jobs/{id}/captures: it frees the Full Demo
// round clips of a ready long video under DeleteJob's locks; a new edit recaptures.
func (h *Handlers) DeleteJobCaptures(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJobMeta(w, r)
	if !ok {
		return
	}
	if jobIsInFlight(j.Status) {
		writeCodedError(w, http.StatusConflict, jobBusyCode, fmt.Sprintf("job is %s; wait for it to settle before deleting captures", j.Status))
		return
	}
	resolver, ok := h.storage.(storagePathResolver)
	if !ok {
		writeError(w, http.StatusNotImplemented, "storage backend does not support delete")
		return
	}
	var freed int64
	err := h.generateIntents.WhileIdle(j.ID, func() error {
		if err := h.requireGenerateRenderIdle(j.ID); err != nil {
			return err
		}
		revisions, err := resolver.ResolvePath(artifacts.FullDemoCaptureRevisionsPrefix(j.ID))
		if err != nil {
			return err
		}
		clips, err := recording.FullDemoCaptureClips(revisions)
		if err != nil {
			return err
		}
		var newest time.Time
		for _, clip := range clips {
			if clip.ModTime.After(newest) {
				newest = clip.ModTime
			}
		}
		if covered, err := h.fullDemoRenderCovers(j.ID, newest); err != nil {
			return err
		} else if !covered {
			return errNoFullDemoRender
		}
		freed, err = recording.DeleteCaptureClips(clips)
		return err
	})
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]any{"freed_bytes": freed})
	case errors.Is(err, errNoFullDemoRender):
		writeCodedError(w, http.StatusConflict, noFullDemoRenderCode, "the long video has no finished render; its captures are still needed")
	case errors.Is(err, generateintent.ErrActiveRun), errors.Is(err, errGenerateRenderActive):
		writeCodedError(w, http.StatusConflict, generateWorkActive, "job has an active render or generate run; wait for it to settle before deleting captures")
	default:
		internalError(w, "delete job captures", err)
	}
}

var errNoFullDemoRender = errors.New("no ready Full Demo render")

const (
	jobBusyCode          = "job_busy"
	noFullDemoRenderCode = "no_full_demo_render"
)

// fullDemoRenderCovers reports whether the Full Demo render is ready and newer than
// every round clip; a later capture whose render never ran still needs its clips.
func (h *Handlers) fullDemoRenderCovers(jobID uuid.UUID, newestClip time.Time) (bool, error) {
	state, exists, err := h.readRenderVariantState(jobID, editor.PresetGameplayPOV60)
	if err != nil || !exists {
		return false, err
	}
	return state.Status == renderplan.RenderVariantStatusReady && !newestClip.After(state.UpdatedAt), nil
}

// measureJobTree sums a job dir; its capture bytes are exactly what DeleteJobCaptures frees.
func measureJobTree(dir string) (jobStorageUsage, error) {
	var usage jobStorageUsage
	err := filepath.WalkDir(dir, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		rel, err := filepath.Rel(dir, file)
		if err != nil {
			return err
		}
		usage.totalBytes += info.Size()
		switch rel = filepath.ToSlash(rel); {
		case recording.IsFullDemoCaptureClip(rel):
			usage.CaptureBytes += info.Size()
			if info.ModTime().After(usage.newestClip) {
				usage.newestClip = info.ModTime()
			}
		case strings.HasPrefix(rel, "renders/"):
			usage.RenderBytes += info.Size()
		}
		return nil
	})
	return usage, err
}
