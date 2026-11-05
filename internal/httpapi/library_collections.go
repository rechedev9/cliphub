package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// libraryCollectionRepository is the saved-moments part of LibraryRepository.
type libraryCollectionRepository interface {
	CreateCollection(ctx context.Context, c *library.Collection) error
	GetCollection(ctx context.Context, id uuid.UUID) (library.Collection, error)
	ListCollections(ctx context.Context) ([]library.Collection, error)
	UpdateCollection(ctx context.Context, id uuid.UUID, patch library.CollectionPatch) (library.Collection, error)
	DeleteCollection(ctx context.Context, id uuid.UUID) error
}

type libraryCollectionRequest struct {
	Name  *string                   `json:"name"`
	Items *[]library.CollectionItem `json:"items"`
}

// collectionResponse is one collection with the matches its moments name that
// are no longer in the library.
type collectionResponse struct {
	library.Collection
	MissingMatchIDs []uuid.UUID `json:"missing_match_ids"`
}

// checkCollectionItems checks that every new moment names a stored match and
// one of its rounds, like checkPlayExamples. Moments already saved are not
// checked again, so a list that holds a deleted match can still be edited.
func (h *Handlers) checkCollectionItems(ctx context.Context, items, saved []library.CollectionItem) (string, error) {
	known := map[library.CollectionItem]bool{}
	for _, item := range saved {
		known[library.CollectionItem{MatchID: item.MatchID, Round: item.Round, Tick: item.Tick}] = true
	}
	matches := map[uuid.UUID]library.Match{}
	for _, item := range items {
		if known[library.CollectionItem{MatchID: item.MatchID, Round: item.Round, Tick: item.Tick}] {
			continue
		}
		m, ok := matches[item.MatchID]
		if !ok {
			var err error
			m, err = h.libraryRepo.GetMatch(ctx, item.MatchID)
			if errors.Is(err, library.ErrNotFound) {
				return codeUnknownMatch, fmt.Errorf("match %s does not exist", item.MatchID)
			}
			if err != nil {
				return "", err
			}
			matches[item.MatchID] = m
		}
		if err := checkAnnotationRound(m, item.Round); err != nil {
			return codeUnknownMatch, err
		}
	}
	return "", nil
}

// writeCollectionItemsError answers a failed moment check.
func writeCollectionItemsError(w http.ResponseWriter, code string, err error) {
	if code == "" {
		internalError(w, "check collection moments", err)
		return
	}
	writePlayError(w, code, err)
}

// ListLibraryCollections handles GET /api/library/collections.
func (h *Handlers) ListLibraryCollections(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	collections, err := h.libraryRepo.ListCollections(r.Context())
	if err != nil {
		internalError(w, "list collections", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"collections": collections})
}

// CreateLibraryCollection handles POST /api/library/collections.
func (h *Handlers) CreateLibraryCollection(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	var req libraryCollectionRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid collection JSON")
		return
	}
	c := library.Collection{}
	if req.Name != nil {
		c.Name = *req.Name
	}
	if req.Items != nil {
		c.Items = *req.Items
	}
	c, err := library.CleanCollection(c)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if code, err := h.checkCollectionItems(r.Context(), c.Items, nil); err != nil {
		writeCollectionItemsError(w, code, err)
		return
	}
	if err := h.libraryRepo.CreateCollection(r.Context(), &c); err != nil {
		writeLibraryError(w, "create collection", err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (h *Handlers) loadLibraryCollection(w http.ResponseWriter, r *http.Request) (library.Collection, bool) {
	if !h.libraryReady(w) {
		return library.Collection{}, false
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return library.Collection{}, false
	}
	c, err := h.libraryRepo.GetCollection(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get collection", err)
		return library.Collection{}, false
	}
	return c, true
}

// GetLibraryCollection handles GET /api/library/collections/{id}.
func (h *Handlers) GetLibraryCollection(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadLibraryCollection(w, r)
	if !ok {
		return
	}
	resp := collectionResponse{Collection: c, MissingMatchIDs: []uuid.UUID{}}
	seen := map[uuid.UUID]bool{}
	for _, item := range c.Items {
		if seen[item.MatchID] {
			continue
		}
		seen[item.MatchID] = true
		if _, err := h.libraryRepo.GetMatch(r.Context(), item.MatchID); errors.Is(err, library.ErrNotFound) {
			resp.MissingMatchIDs = append(resp.MissingMatchIDs, item.MatchID)
		} else if err != nil {
			internalError(w, "get collection match", err)
			return
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// PatchLibraryCollection handles PATCH /api/library/collections/{id}. Items
// replace the whole list, in the order given.
func (h *Handlers) PatchLibraryCollection(w http.ResponseWriter, r *http.Request) {
	current, ok := h.loadLibraryCollection(w, r)
	if !ok {
		return
	}
	var req libraryCollectionRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid collection JSON")
		return
	}
	next := current
	if req.Name != nil {
		next.Name = *req.Name
	}
	if req.Items != nil {
		next.Items = *req.Items
	}
	next, err := library.CleanCollection(next)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	patch := library.CollectionPatch{}
	if req.Name != nil {
		patch.Name = &next.Name
	}
	if req.Items != nil {
		if code, err := h.checkCollectionItems(r.Context(), next.Items, current.Items); err != nil {
			writeCollectionItemsError(w, code, err)
			return
		}
		patch.Items = &next.Items
	}
	updated, err := h.libraryRepo.UpdateCollection(r.Context(), current.ID, patch)
	if err != nil {
		writeLibraryError(w, "update collection", err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// DeleteLibraryCollection handles DELETE /api/library/collections/{id}.
func (h *Handlers) DeleteLibraryCollection(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.libraryRepo.DeleteCollection(r.Context(), id); err != nil {
		writeLibraryError(w, "delete collection", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
