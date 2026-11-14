package httpapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

const (
	// maxTeamDocumentRequestBytes is one document plus its multipart envelope and text fields.
	maxTeamDocumentRequestBytes = library.MaxTeamDocumentBytes + 64<<10
	maxTeamDocumentFieldBytes   = 4 << 10
	// TeamDocumentPolicy runs a document in an opaque origin, so its scripts can
	// never read the signed-in session or call the API as the person viewing it.
	TeamDocumentPolicy = "sandbox allow-scripts allow-popups allow-popups-to-escape-sandbox; default-src 'none'; " +
		"script-src 'unsafe-inline' 'unsafe-eval' https:; style-src 'unsafe-inline' https:; font-src https: data:; " +
		"img-src https: data: blob:; media-src https: data: blob:; connect-src 'none'; form-action 'none'; " +
		"base-uri 'none'; frame-ancestors 'self'"
	// documentFragmentPrelude gives a fragment (no doctype) standards mode and UTF-8.
	documentFragmentPrelude = "<!doctype html>\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n"
)

// libraryTeamDocumentRepository is the documents part of LibraryRepository.
type libraryTeamDocumentRepository interface {
	CreateTeamDocument(ctx context.Context, d *library.TeamDocument, content []byte) error
	GetTeamDocument(ctx context.Context, id uuid.UUID) (library.TeamDocument, error)
	TeamDocumentContent(ctx context.Context, id uuid.UUID) ([]byte, error)
	ListTeamDocuments(ctx context.Context, filter library.TeamDocumentFilter) ([]library.TeamDocument, error)
	DeleteTeamDocument(ctx context.Context, id uuid.UUID) error
}

// ListLibraryTeamDocuments handles GET /api/library/documents.
func (h *Handlers) ListLibraryTeamDocuments(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	q := r.URL.Query()
	var filter library.TeamDocumentFilter
	var err error
	if filter.Limit, filter.Offset, err = libraryPaging(q); err == nil {
		filter.TeamID, err = optionalUUIDQuery(q, "team_id")
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	docs, err := h.libraryRepo.ListTeamDocuments(r.Context(), filter)
	if err != nil {
		internalError(w, "list team documents", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": docs})
}

// UploadLibraryTeamDocument handles POST /api/library/document-uploads: a
// multipart form with team_id, an optional title and the file.
func (h *Handlers) UploadLibraryTeamDocument(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxTeamDocumentRequestBytes)
	form, err := readTeamDocumentForm(r)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeCodedError(w, http.StatusRequestEntityTooLarge, "document_too_large", fmt.Sprintf("a document is at most %d MiB", library.MaxTeamDocumentBytes>>20))
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	teamID, err := uuid.Parse(form.teamID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "team_id must be a valid UUID")
		return
	}
	doc, err := library.NewTeamDocument(library.NewTeamDocumentInput{TeamID: teamID, Title: form.title, FileName: form.fileName, Content: form.content})
	if err != nil {
		writeCodedError(w, http.StatusBadRequest, "invalid_document", err.Error())
		return
	}
	if err := h.libraryRepo.CreateTeamDocument(r.Context(), &doc, form.content); err != nil {
		writeLibraryError(w, "create team document", err)
		return
	}
	writeJSON(w, http.StatusCreated, doc)
}

type teamDocumentForm struct {
	teamID, title, fileName string
	content                 []byte
	gotFile                 bool
}

func readTeamDocumentForm(r *http.Request) (teamDocumentForm, error) {
	var form teamDocumentForm
	reader, err := r.MultipartReader()
	if err != nil {
		return form, errors.New("multipart/form-data required")
	}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return form, err
		}
		if err := form.read(part); err != nil {
			return form, err
		}
	}
	if !form.gotFile {
		return form, errors.New("file is required")
	}
	return form, nil
}

func (form *teamDocumentForm) read(part *multipart.Part) error {
	defer part.Close()
	switch part.FormName() {
	case "file":
		if form.gotFile {
			return errors.New("send one file per request")
		}
		form.gotFile = true
		form.fileName = part.FileName()
		var buf bytes.Buffer
		n, err := io.Copy(&buf, io.LimitReader(part, library.MaxTeamDocumentBytes+1))
		if err != nil {
			return err
		}
		if n > library.MaxTeamDocumentBytes {
			return &http.MaxBytesError{Limit: library.MaxTeamDocumentBytes}
		}
		form.content = buf.Bytes()
	case "team_id", "title":
		raw, err := io.ReadAll(io.LimitReader(part, maxTeamDocumentFieldBytes+1))
		if err != nil {
			return err
		}
		if len(raw) > maxTeamDocumentFieldBytes {
			return fmt.Errorf("%s is too long", part.FormName())
		}
		if part.FormName() == "team_id" {
			form.teamID = strings.TrimSpace(string(raw))
		} else {
			form.title = string(raw)
		}
	default:
		return fmt.Errorf("unknown field %q", part.FormName())
	}
	return nil
}

func (h *Handlers) loadTeamDocument(w http.ResponseWriter, r *http.Request) (library.TeamDocument, bool) {
	if !h.libraryReady(w) {
		return library.TeamDocument{}, false
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return library.TeamDocument{}, false
	}
	doc, err := h.libraryRepo.GetTeamDocument(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get team document", err)
		return library.TeamDocument{}, false
	}
	return doc, true
}

// GetLibraryTeamDocument handles GET /api/library/documents/{id}.
func (h *Handlers) GetLibraryTeamDocument(w http.ResponseWriter, r *http.Request) {
	if doc, ok := h.loadTeamDocument(w, r); ok {
		writeJSON(w, http.StatusOK, doc)
	}
}

// GetLibraryTeamDocumentContent handles GET /api/library/documents/{id}/content: the HTML
// itself, under a policy that keeps it in a sandbox even when opened on its own tab.
func (h *Handlers) GetLibraryTeamDocumentContent(w http.ResponseWriter, r *http.Request) {
	doc, ok := h.loadTeamDocument(w, r)
	if !ok {
		return
	}
	content, err := h.libraryRepo.TeamDocumentContent(r.Context(), doc.ID)
	if err != nil {
		writeLibraryError(w, "read team document", err)
		return
	}
	if !library.HasDoctype(content) {
		content = append([]byte(documentFragmentPrelude), content...)
	}
	header := w.Header()
	header.Set("Content-Type", "text/html; charset=utf-8")
	header.Set("Content-Security-Policy", TeamDocumentPolicy)
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "SAMEORIGIN")
	header.Set("Cache-Control", "no-store")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("Cross-Origin-Resource-Policy", "same-origin")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

// DeleteLibraryTeamDocument handles DELETE /api/library/documents/{id}.
func (h *Handlers) DeleteLibraryTeamDocument(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.libraryRepo.DeleteTeamDocument(r.Context(), id); err != nil {
		writeLibraryError(w, "delete team document", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
