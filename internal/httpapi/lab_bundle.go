package httpapi

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/workers"
)

// LabBundlePreparer writes the render inputs of a job's last committed render
// of a variant into a directory, for zv-editor lab to replay one stage.
type LabBundlePreparer interface {
	PrepareLabBundle(ctx context.Context, id uuid.UUID, variant, dir string) (workers.LabBundle, error)
}

// WithLabBundles enables POST /api/jobs/{id}/renders/{variant}/lab-bundle,
// writing each bundle under root.
func WithLabBundles(preparer LabBundlePreparer, root string) Option {
	return func(h *Handlers) {
		h.labBundles = preparer
		h.labBundleRoot = root
	}
}

// PrepareRenderLabBundle handles POST /api/jobs/{id}/renders/{variant}/lab-bundle.
// It copies the recorded clips and writes the editor inputs of the variant's
// last committed render into <root>/<job>-<variant>, overwriting the files an
// earlier bundle wrote there. It never enqueues a render or writes render state.
func (h *Handlers) PrepareRenderLabBundle(w http.ResponseWriter, r *http.Request) {
	if h.labBundles == nil || h.labBundleRoot == "" {
		writeError(w, http.StatusNotImplemented, "render lab bundles need the render worker")
		return
	}
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	variant := chi.URLParam(r, "variant")
	if _, err := renderplan.LoadoutForVariant(variant); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !h.labBundleMu.TryLock() {
		writeError(w, http.StatusConflict, "another render lab bundle is being written; retry when it finishes")
		return
	}
	defer h.labBundleMu.Unlock()
	// Copying a Full Demo capture takes longer than the control response
	// deadline; the request stays bounded by the caller's context.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(30 * time.Minute))
	dir := filepath.Join(h.labBundleRoot, j.ID.String()+"-"+variant)
	bundle, err := h.labBundles.PrepareLabBundle(r.Context(), j.ID, variant, dir)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, bundle)
}
