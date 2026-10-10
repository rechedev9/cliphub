package httpapi

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/rechedev9/cliphub/internal/cloudclient"
)

// cloudNotConfigured is the code every /api/cloud route answers with when
// this Studio runs with ZV_CLOUD_URL=off.
const cloudNotConfigured = "not_configured"

// CloudService is the ClipHub cloud client behind the /api/cloud routes.
// *cloudclient.Service satisfies it. The device token stays inside it: the
// Studio UI only ever sees these same-origin routes.
type CloudService interface {
	Account(ctx context.Context) cloudclient.Account
	StartLink(ctx context.Context) (cloudclient.Account, error)
	Unlink(ctx context.Context) error
	Submit(ctx context.Context, in cloudclient.SubmitInput) (cloudclient.LocalJob, error)
	Jobs() []cloudclient.LocalJob
	Cancel(ctx context.Context, cloudJobID string) (cloudclient.LocalJob, error)
	Remove(ctx context.Context, cloudJobID string) error
	VideoPath(cloudJobID, name string) (string, error)
}

// WithCloud enables the /api/cloud routes. Without it (or with a nil
// service) they answer 503 not_configured.
func WithCloud(service CloudService) Option {
	return func(h *Handlers) {
		h.cloud = service
	}
}

// cloudService returns the configured client, or writes the 503 itself.
func (h *Handlers) cloudService(w http.ResponseWriter) (CloudService, bool) {
	if h.cloud == nil {
		writeCodedError(w, http.StatusServiceUnavailable, cloudNotConfigured, "La nube de ClipHub está desactivada en este equipo.")
		return nil, false
	}
	return h.cloud, true
}

// writeCloudError maps a cloud client error onto its own status and code;
// anything else is an internal error whose detail stays in the log.
func writeCloudError(w http.ResponseWriter, op string, err error) {
	var cloudErr *cloudclient.Error
	if errors.As(err, &cloudErr) {
		writeCodedError(w, cloudErr.Status, cloudErr.Code, cloudErr.Message)
		return
	}
	internalError(w, op, err)
}

// GetCloudAccount handles GET /api/cloud/account.
func (h *Handlers) GetCloudAccount(w http.ResponseWriter, r *http.Request) {
	cloud, ok := h.cloudService(w)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, cloud.Account(r.Context()))
}

// StartCloudLink handles POST /api/cloud/link. It answers at once with the
// code to show; the client polls the portal in the background.
func (h *Handlers) StartCloudLink(w http.ResponseWriter, r *http.Request) {
	cloud, ok := h.cloudService(w)
	if !ok {
		return
	}
	account, err := cloud.StartLink(r.Context())
	if err != nil {
		writeCloudError(w, "start cloud link", err)
		return
	}
	writeJSON(w, http.StatusAccepted, account)
}

// DeleteCloudLink handles DELETE /api/cloud/link.
func (h *Handlers) DeleteCloudLink(w http.ResponseWriter, r *http.Request) {
	cloud, ok := h.cloudService(w)
	if !ok {
		return
	}
	if err := cloud.Unlink(r.Context()); err != nil {
		writeCloudError(w, "unlink cloud account", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CreateCloudJob handles POST /api/cloud/jobs.
func (h *Handlers) CreateCloudJob(w http.ResponseWriter, r *http.Request) {
	cloud, ok := h.cloudService(w)
	if !ok {
		return
	}
	var in cloudclient.SubmitInput
	// The edit document can be as large as the one generate accepts.
	if err := decodeRenderJSONBody(w, r, &in, false); err != nil {
		writeError(w, http.StatusBadRequest, "invalid cloud job JSON")
		return
	}
	created, err := cloud.Submit(r.Context(), in)
	if err != nil {
		writeCloudError(w, "submit cloud job", err)
		return
	}
	writeJSON(w, http.StatusAccepted, created)
}

// ListCloudJobs handles GET /api/cloud/jobs.
func (h *Handlers) ListCloudJobs(w http.ResponseWriter, _ *http.Request) {
	cloud, ok := h.cloudService(w)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": cloud.Jobs()})
}

// CancelCloudJob handles POST /api/cloud/jobs/{id}/cancel.
func (h *Handlers) CancelCloudJob(w http.ResponseWriter, r *http.Request) {
	cloud, ok := h.cloudService(w)
	if !ok {
		return
	}
	canceled, err := cloud.Cancel(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeCloudError(w, "cancel cloud job", err)
		return
	}
	writeJSON(w, http.StatusAccepted, canceled)
}

// DeleteCloudJob handles DELETE /api/cloud/jobs/{id}.
func (h *Handlers) DeleteCloudJob(w http.ResponseWriter, r *http.Request) {
	cloud, ok := h.cloudService(w)
	if !ok {
		return
	}
	if err := cloud.Remove(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeCloudError(w, "remove cloud job", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetCloudJobVideo handles GET /api/cloud/jobs/{id}/videos/{name}. The name
// only selects one of the job's known videos; the file path comes from the
// client's own record, never from the request.
func (h *Handlers) GetCloudJobVideo(w http.ResponseWriter, r *http.Request) {
	cloud, ok := h.cloudService(w)
	if !ok {
		return
	}
	path, err := cloud.VideoPath(chi.URLParam(r, "id"), chi.URLParam(r, "name"))
	if err != nil {
		writeCloudError(w, "locate cloud video", err)
		return
	}
	file, err := os.Open(path) //nolint:gosec // path comes from the cloud client's record of its own downloads
	if err != nil {
		writeError(w, http.StatusNotFound, "video not found")
		return
	}
	// The file behind this URL never changes, but the job can be removed, so
	// the browser revalidates. ServeContent behind the helper serves Range.
	serveArtifact(w, r, "video/mp4", file)
}
