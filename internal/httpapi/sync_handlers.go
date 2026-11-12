package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/rechedev9/cliphub/internal/store"
)

const maxSyncBodyBytes = 8 << 20 // 8 MiB

// WithSyncLog enables the device sync change endpoints.
func WithSyncLog(log *store.SyncLog) Option {
	return func(h *Handlers) {
		h.syncLog = log
	}
}

// GetSyncChanges handles GET /api/sync/changes.
func (h *Handlers) GetSyncChanges(w http.ResponseWriter, r *http.Request) {
	after, err := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "after must be an integer")
		return
	}
	limit := 1000
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 1000 {
			writeError(w, http.StatusBadRequest, "limit must be an integer from 1 to 1000")
			return
		}
		limit = parsed
	}
	batch, err := h.syncLog.Changes(r.Context(), after, limit)
	if err != nil {
		internalError(w, "list sync changes", err)
		return
	}
	writeJSON(w, http.StatusOK, batch)
}

// ApplySyncChanges handles POST /api/sync/changes.
func (h *Handlers) ApplySyncChanges(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Changes []store.SyncChange `json:"changes"`
	}
	if err := decodeSyncChangesBody(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid sync changes JSON")
		return
	}
	if err := h.syncLog.Apply(r.Context(), req.Changes); err != nil {
		var input *store.SyncInputError
		if errors.As(err, &input) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		internalError(w, "apply sync changes", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// decodeSyncChangesBody bounds the body and keeps JSON numbers exact, so
// integer keys survive the round trip without floating point loss.
func decodeSyncChangesBody(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxSyncBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var trailing any
	switch err := dec.Decode(&trailing); {
	case errors.Is(err, io.EOF):
		return nil
	case err == nil:
		return errMultipleJSONValues
	default:
		return fmt.Errorf("invalid trailing JSON data: %w", err)
	}
}
