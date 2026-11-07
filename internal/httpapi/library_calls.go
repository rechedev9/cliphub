package httpapi

import (
	"net/http"
	"net/url"

	"github.com/rechedev9/cliphub/internal/library"
)

// ListLibraryCalls handles GET /api/library/calls.
func (h *Handlers) ListLibraryCalls(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	filter, err := callFilterQuery(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	calls, err := h.libraryRepo.ListCalls(r.Context(), filter)
	if err != nil {
		internalError(w, "list calls", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"calls": calls})
}

func callFilterQuery(q url.Values) (library.CallFilter, error) {
	var f library.CallFilter
	side, err := parseScoutSide(q.Get("side"))
	if err != nil {
		return f, err
	}
	f.Side = string(side)
	list, err := libretaListQuery(q, library.MaxCallList)
	if err != nil {
		return f, err
	}
	f.Status, f.Tag, f.ExcludeArchived, f.Limit, f.Offset = list.status, list.tag, list.excludeArchived, list.limit, list.offset
	return f, nil
}

// callRequest is a call as a client sends it; nil fields are not set.
type callRequest struct {
	Name    *string   `json:"name"`
	Side    *string   `json:"side"`
	Meaning *string   `json:"meaning"`
	Status  *string   `json:"status"`
	Tags    *[]string `json:"tags"`
}

func (req callRequest) patch() library.CallPatch {
	return library.CallPatch{Name: req.Name, Side: req.Side, Meaning: req.Meaning, Status: req.Status, Tags: req.Tags}
}

// CreateLibraryCall handles POST /api/library/calls.
func (h *Handlers) CreateLibraryCall(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	var req callRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid call JSON")
		return
	}
	c, err := library.CleanCall(library.ApplyCallPatch(library.Call{}, req.patch()))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.libraryRepo.CreateCall(r.Context(), &c); err != nil {
		writeLibraryError(w, "create call", err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (h *Handlers) loadLibraryCall(w http.ResponseWriter, r *http.Request) (library.Call, bool) {
	if !h.libraryReady(w) {
		return library.Call{}, false
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return library.Call{}, false
	}
	c, err := h.libraryRepo.GetCall(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get call", err)
		return library.Call{}, false
	}
	return c, true
}

// GetLibraryCall handles GET /api/library/calls/{id}.
func (h *Handlers) GetLibraryCall(w http.ResponseWriter, r *http.Request) {
	if c, ok := h.loadLibraryCall(w, r); ok {
		writeJSON(w, http.StatusOK, c)
	}
}

// PatchLibraryCall handles PATCH /api/library/calls/{id}.
func (h *Handlers) PatchLibraryCall(w http.ResponseWriter, r *http.Request) {
	current, ok := h.loadLibraryCall(w, r)
	if !ok {
		return
	}
	var req callRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid call JSON")
		return
	}
	next, err := library.CleanCall(library.ApplyCallPatch(current, req.patch()))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := h.libraryRepo.UpdateCall(r.Context(), current.ID, library.CallPatch{
		Name: &next.Name, Side: &next.Side, Meaning: &next.Meaning, Status: &next.Status, Tags: &next.Tags})
	if err != nil {
		writeLibraryError(w, "update call", err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// DeleteLibraryCall handles DELETE /api/library/calls/{id}.
func (h *Handlers) DeleteLibraryCall(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.libraryRepo.DeleteCall(r.Context(), id); err != nil {
		writeLibraryError(w, "delete call", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
