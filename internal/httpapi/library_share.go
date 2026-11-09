package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/sharepkg"
)

// Error codes of the share package routes.
const (
	codeShareEmpty       = "share_empty"
	codeShareUnavailable = "share_match_unavailable"
	codeSharePackage     = "share_package_invalid"
	codeShareDemoPending = "share_demo_pending"
	codeSharedMatch      = "shared_match"
)

// errSharedMatch is the reason every action that needs the .dem gives on a
// shared match.
const errSharedMatch = "this match came without its demo file (a share package or the pro match catalog)"

// errShareUnavailable marks a match whose analysis cannot be packaged.
var errShareUnavailable = errors.New("match is not analyzed")

type shareExportRequest struct {
	Notes string `json:"notes"`
}

// readShareNotes reads the optional message the sender adds to a package.
func readShareNotes(w http.ResponseWriter, r *http.Request) (string, bool) {
	var req shareExportRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid share JSON")
		return "", false
	}
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > sharepkg.MaxNotesRunes {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("notes must be at most %d characters", sharepkg.MaxNotesRunes))
		return "", false
	}
	return notes, true
}

// ExportLibraryReport handles POST /api/library/reports/{id}/export: the
// report and every match it reads as a .cliphub package.
func (h *Handlers) ExportLibraryReport(w http.ResponseWriter, r *http.Request) {
	report, ok := h.loadLibraryReport(w, r)
	if !ok {
		return
	}
	notes, ok := readShareNotes(w, r)
	if !ok {
		return
	}
	team, err := h.libraryRepo.GetTeam(r.Context(), report.TeamID)
	if err != nil {
		writeLibraryError(w, "get team", err)
		return
	}
	matches, err := h.shareMatches(r.Context(), report.MatchIDs)
	if err != nil {
		writeShareExportError(w, err)
		return
	}
	shared := report
	shared.TeamName = team.Name
	shared.MatchIDs = make([]uuid.UUID, 0, len(matches))
	for _, m := range matches {
		shared.MatchIDs = append(shared.MatchIDs, m.Match.ID)
	}
	notesOf, err := h.shareAnnotations(r.Context(), matches, func(library.Annotation) bool { return true })
	if err != nil {
		internalError(w, "list report notes", err)
		return
	}
	h.writeSharePackage(w, sharepkg.Manifest{
		Kind: sharepkg.KindReport, Title: report.Title, Notes: notes, Report: &shared,
		Matches: matches, Annotations: notesOf,
	})
}

func writeShareExportError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errShareUnavailable):
		writeCodedError(w, http.StatusConflict, codeShareUnavailable, err.Error())
	case errors.Is(err, library.ErrNotFound):
		writeCodedError(w, http.StatusConflict, codeShareEmpty, "none of its matches is in the library")
	default:
		internalError(w, "build share package", err)
	}
}

// shareMatches reads the records of the given matches, skipping deleted ones.
// A match whose analysis is missing makes the whole export fail, since its
// moments could not be replayed.
func (h *Handlers) shareMatches(ctx context.Context, ids []uuid.UUID) ([]sharepkg.Match, error) {
	out := make([]sharepkg.Match, 0, len(ids))
	for _, id := range ids {
		m, err := h.libraryRepo.GetMatch(ctx, id)
		if errors.Is(err, library.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		d, err := h.libraryRepo.GetDemo(ctx, m.DemoID)
		if err != nil {
			return nil, err
		}
		for _, key := range []string{artifacts.LibraryTacticalIndexKey(d.ID), artifacts.LibraryTacticalPositionsKey(d.ID)} {
			if exists, err := h.storage.Exists(key); err != nil || !exists {
				return nil, fmt.Errorf("%w: %s %s", errShareUnavailable, m.Map, m.PlayedAt.Format(time.DateOnly))
			}
		}
		m.SeriesID = nil
		out = append(out, sharepkg.Match{Match: m, Demo: sharepkg.Demo{
			SHA256: d.SHA256, FileName: d.FileName, CS2Build: d.CS2Build, PatchVersion: d.PatchVersion,
		}})
	}
	if len(out) == 0 {
		return nil, library.ErrNotFound
	}
	if len(out) > sharepkg.MaxMatches {
		out = out[:sharepkg.MaxMatches]
	}
	return out, nil
}

func (h *Handlers) shareAnnotations(ctx context.Context, matches []sharepkg.Match, keep func(library.Annotation) bool) ([]library.Annotation, error) {
	ids := make([]uuid.UUID, 0, len(matches))
	for _, m := range matches {
		ids = append(ids, m.Match.ID)
	}
	all, err := h.libraryRepo.ListAnnotations(ctx, library.AnnotationFilter{MatchIDs: ids})
	if err != nil {
		return nil, err
	}
	out := make([]library.Annotation, 0, len(all))
	for _, a := range all {
		if keep(a) && len(out) < sharepkg.MaxAnnotations {
			out = append(out, a)
		}
	}
	return out, nil
}

// writeSharePackage builds the package in a temporary file first, so a
// storage error is a clean 500 and the download carries its exact length.
func (h *Handlers) writeSharePackage(w http.ResponseWriter, m sharepkg.Manifest) {
	m.Format, m.Version, m.CreatedAt = sharepkg.Format, sharepkg.Version, time.Now().UTC().Truncate(time.Second)
	tmp, err := os.CreateTemp("", "zv-share-*"+sharepkg.Extension)
	if err != nil {
		internalError(w, "create share package", err)
		return
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()
	demoOf := map[uuid.UUID]uuid.UUID{}
	for _, entry := range m.Matches {
		demoOf[entry.Match.ID] = entry.Match.DemoID
	}
	err = sharepkg.Write(tmp, m, func(id uuid.UUID) (io.ReadCloser, io.ReadCloser, error) {
		doc, err := h.storage.Open(artifacts.LibraryTacticalIndexKey(demoOf[id]))
		if err != nil {
			return nil, nil, err
		}
		pos, err := h.storage.Open(artifacts.LibraryTacticalPositionsKey(demoOf[id]))
		if err != nil {
			_ = doc.Close()
			return nil, nil, err
		}
		return doc, pos, nil
	})
	if err != nil {
		internalError(w, "write share package", err)
		return
	}
	size, err := tmp.Seek(0, io.SeekCurrent)
	if err == nil {
		_, err = tmp.Seek(0, io.SeekStart)
	}
	if err != nil {
		internalError(w, "read share package", err)
		return
	}
	w.Header().Set("Content-Type", sharepkg.MediaType)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s%s"`, shareFileStem(m.Title), sharepkg.Extension))
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, tmp); err != nil {
		log.Printf("httpapi: send share package: %v", err)
	}
}

// shareFileStem is an ASCII file name for the package, from its title.
func shareFileStem(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
		if b.Len() >= 60 {
			break
		}
	}
	stem := strings.Trim(b.String(), "-")
	if stem == "" {
		return "cliphub"
	}
	return stem
}
