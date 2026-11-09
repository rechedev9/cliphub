package httpapi

import (
	"cmp"
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// libraryAnalysisStale reports whether a ready demo's stored analysis is
// missing, unreadable or older than tacticalplan.AnalysisVersion, so a scan
// would name its zones and sites differently.
func (h *Handlers) libraryAnalysisStale(id uuid.UUID) bool {
	rc, err := h.storage.Open(artifacts.LibraryTacticalIndexKey(id))
	if err != nil {
		return storage.IsNotExist(err)
	}
	defer rc.Close()
	version, ok := leadingAnalysisVersion(json.NewDecoder(rc))
	return !ok || version < tacticalplan.AnalysisVersion
}

// leadingAnalysisVersion reads analysis_version without decoding the whole
// document, which runs to megabytes. Document marshals schema_version first
// and analysis_version right after it, so any other key there means version 0.
func leadingAnalysisVersion(dec *json.Decoder) (int, bool) {
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return 0, false
	}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return 0, false
		}
		switch key {
		case "schema_version":
			var skip string
			if err := dec.Decode(&skip); err != nil {
				return 0, false
			}
		case "analysis_version":
			var version int
			if err := dec.Decode(&version); err != nil {
				return 0, false
			}
			return version, true
		default:
			return 0, true
		}
	}
	return 0, true
}

// ResumeLibraryScans re-queues every demo a previous process left queued or
// parsing, as the desktop queue lives in memory, and every ready demo whose
// analysis an older scanner made.
func (h *Handlers) ResumeLibraryScans(ctx context.Context) (int, error) {
	if h.libraryRepo == nil {
		return 0, nil
	}
	var pending []library.Demo
	for offset := 0; ; offset += library.MaxListLimit {
		page, err := h.libraryRepo.ListDemos(ctx, library.DemoFilter{
			States: []library.ParseState{library.ParseQueued, library.ParseParsing, library.ParseReady},
			Limit:  library.MaxListLimit, Offset: offset,
		})
		if err != nil {
			return 0, err
		}
		for _, d := range page {
			// A shared match has no demo file to scan; its analysis came with it.
			if d.Source.WithoutDemo() {
				continue
			}
			if d.State != library.ParseReady || h.libraryAnalysisStale(d.ID) {
				pending = append(pending, d)
			}
		}
		if len(page) < library.MaxListLimit {
			break
		}
	}
	// One demo the queue refuses (it is full) does not strand the rest; a ready
	// demo it refused goes back to ready, keeping its older analysis usable.
	resumed := 0
	var firstErr error
	for _, d := range pending {
		if err := h.libraryRepo.SetDemoState(ctx, d.ID, library.ParseQueued, "", ""); err != nil {
			firstErr = cmp.Or(firstErr, err)
			continue
		}
		if err := h.enqueueLibraryScan(ctx, d.ID); err != nil {
			firstErr = cmp.Or(firstErr, err)
			if d.State == library.ParseReady {
				if err := h.libraryRepo.SetDemoState(ctx, d.ID, library.ParseReady, "", ""); err != nil {
					log.Printf("httpapi: restore library demo %s to ready: %v", d.ID, err)
				}
			}
			continue
		}
		resumed++
	}
	return resumed, firstErr
}

// AnalyzeLibraryDemo handles POST /api/library/demos/{id}/analyze, which
// queues the scan again: after a failure, or to refresh a ready match with a
// newer scanner. A demo already queued or parsing is left alone.
func (h *Handlers) AnalyzeLibraryDemo(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	d, err := h.libraryRepo.GetDemo(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get demo", err)
		return
	}
	if d.Source.WithoutDemo() {
		writeCodedError(w, http.StatusConflict, codeSharedMatch, errSharedMatch)
		return
	}
	if d.State == library.ParseQueued || d.State == library.ParseParsing {
		writeJSON(w, http.StatusAccepted, d)
		return
	}
	if err := h.libraryRepo.SetDemoState(r.Context(), id, library.ParseQueued, "", ""); err != nil {
		writeLibraryError(w, "queue demo", err)
		return
	}
	if err := h.enqueueLibraryScan(r.Context(), id); err != nil {
		internalError(w, "enqueue library scan", err)
		return
	}
	d, err = h.libraryRepo.GetDemo(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get demo", err)
		return
	}
	writeJSON(w, http.StatusAccepted, d)
}

// sharedDemo answers 409 when the demo is a shared match, whose source and
// file are fixed until its .dem is imported.
func (h *Handlers) sharedDemo(w http.ResponseWriter, ctx context.Context, id uuid.UUID) bool {
	d, err := h.libraryRepo.GetDemo(ctx, id)
	if err != nil || !d.Source.WithoutDemo() {
		return false
	}
	writeCodedError(w, http.StatusConflict, codeSharedMatch, errSharedMatch)
	return true
}
