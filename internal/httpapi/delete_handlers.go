package httpapi

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/timelineplan"
)

// The delete endpoints below follow DeleteJob: refuse while a worker may still
// write into the tree, remove the artifact tree first and the row last so a
// failed cleanup leaves the row as the retry handle, and answer 404 once gone.

// DeleteStreamJob handles DELETE /api/stream-jobs/{id}. The per-job lock is
// the same one the render worker takes for its claim and final commit, so a
// delete cannot interleave with a revision being published.
func (h *Handlers) DeleteStreamJob(w http.ResponseWriter, r *http.Request) {
	release := h.lockStreamJobRequest(r)
	defer release()
	j, ok := h.loadStreamJob(w, r)
	if !ok {
		return
	}
	if j.Status == streamclips.StatusAcquiring || j.Status == streamclips.StatusRendering {
		writeError(w, http.StatusConflict, fmt.Sprintf("stream job is %s; wait for it to settle before deleting", j.Status))
		return
	}
	for _, variant := range streamclips.VariantNames() {
		state, exists, err := h.readStreamRenderState(j.ID, variant)
		if err != nil {
			internalError(w, "read stream render state", err)
			return
		}
		if exists && state.Status == streamclips.StatusRendering {
			writeError(w, http.StatusConflict, fmt.Sprintf("stream render %s is %s; wait for it to settle before deleting", variant, state.Status))
			return
		}
	}
	deleter, ok := h.storage.(jobArtifactDeleter)
	if !ok {
		writeError(w, http.StatusNotImplemented, "storage backend does not support delete")
		return
	}
	if err := deleter.DeleteTree(streamclips.JobPrefix(j.ID)); err != nil {
		internalError(w, "delete stream job artifacts", err)
		return
	}
	if err := h.streamRepo.Delete(r.Context(), j.ID); err != nil {
		internalError(w, "delete stream job", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteEditorProject handles DELETE /api/editor/projects/{id}. Assets stay:
// they are shared across projects and have their own endpoint.
func (h *Handlers) DeleteEditorProject(w http.ResponseWriter, r *http.Request) {
	h.editorPlanMu.Lock()
	defer h.editorPlanMu.Unlock()
	p, ok := h.loadEditorProjectOnly(w, r)
	if !ok {
		return
	}
	if p.Status == timelineplan.StatusRendering {
		writeError(w, http.StatusConflict, "project is rendering; wait for it to settle before deleting")
		return
	}
	deleter, ok := h.storage.(jobArtifactDeleter)
	if !ok {
		writeError(w, http.StatusNotImplemented, "storage backend does not support delete")
		return
	}
	if err := deleter.DeleteTree(timelineplan.ProjectPrefix(p.ID)); err != nil {
		internalError(w, "delete editor project artifacts", err)
		return
	}
	if err := h.editorProjects.Delete(r.Context(), p.ID); err != nil {
		internalError(w, "delete editor project", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteEditorAsset handles DELETE /api/editor/assets/{id}. It refuses while a
// rendering project still reads the media (ffmpeg opens it in place, so the
// tree cannot go away under it); a draft project whose timeline references
// the asset fails its next render validation with a missing-asset error,
// which is the honest outcome of deleting media. The plan lock is the one
// render admission holds, so a render cannot claim the asset between the
// check and the delete.
func (h *Handlers) DeleteEditorAsset(w http.ResponseWriter, r *http.Request) {
	h.editorPlanMu.Lock()
	defer h.editorPlanMu.Unlock()
	asset, ok := h.loadEditorAsset(w, r)
	if !ok {
		return
	}
	owner, err := h.renderingProjectUsingAsset(r.Context(), asset.ID)
	if err != nil {
		internalError(w, "list rendering editor projects", err)
		return
	}
	if owner != uuid.Nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("asset is used by rendering project %s; wait for it to settle before deleting", owner))
		return
	}
	deleter, ok := h.storage.(jobArtifactDeleter)
	if !ok {
		writeError(w, http.StatusNotImplemented, "storage backend does not support delete")
		return
	}
	if err := deleter.DeleteTree(mediaassets.AssetPrefix(asset.ID)); err != nil {
		internalError(w, "delete editor asset media", err)
		return
	}
	if err := h.editorAssets.Delete(r.Context(), asset.ID); err != nil {
		internalError(w, "delete editor asset", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// renderingProjectUsingAsset returns the first project in `rendering` whose
// timeline references the asset, or uuid.Nil when no active render reads it.
func (h *Handlers) renderingProjectUsingAsset(ctx context.Context, assetID uuid.UUID) (uuid.UUID, error) {
	projects, err := h.editorProjects.ListByStatus(ctx, timelineplan.StatusRendering)
	if err != nil {
		return uuid.Nil, err
	}
	want := assetID.String()
	for _, p := range projects {
		if len(p.Plan) == 0 {
			continue
		}
		plan, err := timelineplan.Decode(p.Plan)
		if err != nil {
			return uuid.Nil, fmt.Errorf("decode plan of rendering project %s: %w", p.ID, err)
		}
		for _, track := range plan.Tracks {
			for _, item := range track.Items {
				if item.AssetID == want {
					return p.ID, nil
				}
			}
		}
	}
	return uuid.Nil, nil
}
