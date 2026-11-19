package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/generateintent"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/storage"
)

// renderVersion is one committed render revision: what Mis vídeos lists under
// Versiones and what Editar reopens. PlanID and Rounds name a Full Demo's approved plan.
type renderVersion struct {
	Revision   uuid.UUID                 `json:"revision"`
	CreatedAt  time.Time                 `json:"created_at"`
	Current    bool                      `json:"current"`
	Videos     []string                  `json:"videos"`
	SegmentIDs []string                  `json:"segment_ids,omitempty"`
	Edit       renderplan.EditRequest    `json:"edit"`
	Music      *renderplan.MusicSnapshot `json:"music,omitempty"`
	PlanID     string                    `json:"plan_id,omitempty"`
	Rounds     []int                     `json:"rounds,omitempty"`
}

const versionCurrentCode = "version_current"

// ListRenderVersions handles GET /api/jobs/{id}/renders/{variant}/versions,
// newest first. A revision without its MP4 or edit document is not a version.
func (h *Handlers) ListRenderVersions(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJobMeta(w, r)
	if !ok {
		return
	}
	variant := chi.URLParam(r, "variant")
	if _, err := renderplan.LoadoutForVariant(variant); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	versions, err := h.renderVersions(j.ID, variant)
	if err != nil {
		internalError(w, "list render versions", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": versions})
}

func (h *Handlers) renderVersions(id uuid.UUID, variant string) ([]renderVersion, error) {
	resolver, ok := h.storage.(storagePathResolver)
	if !ok {
		return []renderVersion{}, nil
	}
	base, err := artifacts.RenderVariantPrefix(id, variant)
	if err != nil {
		return nil, err
	}
	dir, err := resolver.ResolvePath(base + "/revisions")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	state, _, err := h.readRenderVariantState(id, variant)
	if err != nil {
		return nil, err
	}
	versions := []renderVersion{}
	for _, entry := range entries {
		revision, err := uuid.Parse(entry.Name())
		if err != nil || !entry.IsDir() {
			continue
		}
		version, ok, err := h.readRenderVersion(id, variant, revision, filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		if ok {
			version.Current = state != nil && state.ArtifactPrefix == path.Join(base, "revisions", revision.String())
			versions = append(versions, version)
		}
	}
	slices.SortFunc(versions, func(a, b renderVersion) int { return b.CreatedAt.Compare(a.CreatedAt) })
	return versions, nil
}

func (h *Handlers) readRenderVersion(id uuid.UUID, variant string, revision uuid.UUID, dir string) (renderVersion, bool, error) {
	videos, err := os.ReadDir(filepath.Join(dir, "videos"))
	if errors.Is(err, fs.ErrNotExist) {
		return renderVersion{}, false, nil
	}
	if err != nil {
		return renderVersion{}, false, err
	}
	version := renderVersion{Revision: revision, Videos: []string{}}
	for _, video := range videos {
		if name, ok := strings.CutSuffix(video.Name(), ".mp4"); ok && !video.IsDir() {
			version.Videos = append(version.Videos, name)
		}
	}
	if len(version.Videos) == 0 {
		return renderVersion{}, false, nil
	}
	ref, err := renderplan.NewRenderVariantRevisionArtifactRef(id, variant, revision, renderplan.RenderVariantArtifactEditDocument, "")
	if err != nil {
		return renderVersion{}, false, err
	}
	rc, err := h.storage.Open(ref.Key)
	if storage.IsNotExist(err) {
		return renderVersion{}, false, nil
	}
	if err != nil {
		return renderVersion{}, false, err
	}
	defer rc.Close()
	var doc renderplan.EditDocument
	if err := json.NewDecoder(rc).Decode(&doc); err != nil {
		return renderVersion{}, false, fmt.Errorf("decode edit document of revision %s: %w", revision, err)
	}
	version.CreatedAt, version.SegmentIDs, version.Edit, version.Music = doc.CreatedAt, doc.Selection.SegmentIDs, doc.Edit, doc.Music
	if doc.Edit.FullDemo != nil {
		// The plan is served by its own route; the version names it.
		for _, round := range doc.Edit.FullDemo.Document.Rounds {
			version.Rounds = append(version.Rounds, round.Number)
		}
		version.PlanID, version.Edit.FullDemo = doc.Edit.FullDemo.Document.PlanID, nil
	}
	return version, true, nil
}

// DeleteRenderVersion handles DELETE /api/jobs/{id}/renders/{variant}/versions/{revision}.
// The current version stays: the card and the render state point at it.
func (h *Handlers) DeleteRenderVersion(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJobMeta(w, r)
	if !ok {
		return
	}
	variant := chi.URLParam(r, "variant")
	revision, err := uuid.Parse(chi.URLParam(r, "revision"))
	if err != nil || revision == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid render revision id")
		return
	}
	prefix, err := renderplan.RenderVariantRevisionPrefix(j.ID, variant, revision)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	deleter, ok := h.storage.(interface{ DeleteTree(key string) error })
	if !ok {
		writeError(w, http.StatusNotImplemented, "storage backend does not support delete")
		return
	}
	err = h.generateIntents.WhileIdle(j.ID, func() error {
		if err := h.requireGenerateRenderIdle(j.ID); err != nil {
			return err
		}
		state, _, err := h.readRenderVariantState(j.ID, variant)
		if err != nil {
			return err
		}
		if state != nil && state.ArtifactPrefix == prefix {
			return errVersionCurrent
		}
		return deleter.DeleteTree(prefix)
	})
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]any{"deleted": revision})
	case errors.Is(err, errVersionCurrent):
		writeCodedError(w, http.StatusConflict, versionCurrentCode, "the current version stays; delete the whole video instead")
	case errors.Is(err, generateintent.ErrActiveRun), errors.Is(err, errGenerateRenderActive):
		writeCodedError(w, http.StatusConflict, generateWorkActive, "job has an active render or generate run; wait for it to settle before deleting a version")
	default:
		internalError(w, "delete render version", err)
	}
}

var errVersionCurrent = errors.New("render version is current")
