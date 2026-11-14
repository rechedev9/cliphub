package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/rechedev9/cliphub/internal/syncclient"
)

// clipmindOptions holds what the ClipMind editions switch on in Handlers.
type clipmindOptions struct {
	syncHub        bool
	libraryUploads bool
	// uploadWorkspace is the workspace a ClipMind tenant serves; an upload naming another is refused.
	uploadWorkspace string
	sync            ClipMindSync
}

// ClipMindSync is the link between this Studio and a ClipMind account.
// *syncclient.Client satisfies it.
type ClipMindSync interface {
	Status(ctx context.Context) syncclient.Status
	Connect(ctx context.Context, in syncclient.ConnectInput) (syncclient.Status, error)
	Sync(ctx context.Context) error
	Disconnect(ctx context.Context) error
}

// WithClipMindSync enables the /api/clipmind routes that link Studio to a ClipMind account.
func WithClipMindSync(c ClipMindSync) Option {
	return func(h *Handlers) {
		h.clipmind.sync = c
	}
}

// GetClipMindAccount handles GET /api/clipmind/account.
func (h *Handlers) GetClipMindAccount(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.clipmind.sync.Status(r.Context()))
}

// ConnectClipMindAccount handles POST /api/clipmind/account: sign in and sync once.
func (h *Handlers) ConnectClipMindAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerURL   string `json:"server_url"`
		Login       string `json:"login"`
		Password    string `json:"password"`
		WorkspaceID string `json:"workspace_id"`
	}
	if err := decodeJSONBody(w, r, &req, true, 1<<16); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if _, err := h.clipmind.sync.Connect(r.Context(), syncclient.ConnectInput{
		ServerURL: req.ServerURL, Login: req.Login, Password: req.Password, WorkspaceID: req.WorkspaceID,
	}); err != nil {
		writeError(w, http.StatusBadGateway, "could not sign in to ClipMind: "+err.Error())
		return
	}
	// The first sync can take a while for a big library: it runs on, and the UI reads the status.
	go func() { _ = h.clipmind.sync.Sync(context.WithoutCancel(r.Context())) }()
	writeJSON(w, http.StatusOK, h.clipmind.sync.Status(r.Context()))
}

// SyncClipMindNow handles POST /api/clipmind/sync.
func (h *Handlers) SyncClipMindNow(w http.ResponseWriter, r *http.Request) {
	if err := h.clipmind.sync.Sync(r.Context()); err != nil {
		if errors.Is(err, syncclient.ErrNotConnected) {
			writeError(w, http.StatusConflict, "sign in to ClipMind first")
			return
		}
		writeError(w, http.StatusBadGateway, "sync failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, h.clipmind.sync.Status(r.Context()))
}

// DisconnectClipMindAccount handles DELETE /api/clipmind/account.
func (h *Handlers) DisconnectClipMindAccount(w http.ResponseWriter, r *http.Request) {
	if err := h.clipmind.sync.Disconnect(r.Context()); err != nil {
		internalError(w, "disconnect ClipMind", err)
		return
	}
	writeJSON(w, http.StatusOK, h.clipmind.sync.Status(r.Context()))
}
