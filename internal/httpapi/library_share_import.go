package httpapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/sharepkg"
)

type shareImportResponse struct {
	Kind            string     `json:"kind"`
	Title           string     `json:"title"`
	Notes           string     `json:"notes"`
	ReportID        *uuid.UUID `json:"report_id,omitempty"`
	Duplicate       bool       `json:"duplicate"`
	MatchesImported int        `json:"matches_imported"`
	MatchesExisting int        `json:"matches_existing"`
	Annotations     int        `json:"annotations"`
}

// errShareDemoPending reports a package match whose demo the library holds
// but has not analyzed, so it has no match to reuse yet.
var errShareDemoPending = errors.New("the demo of a shared match is in your library but not analyzed yet")

// ImportLibraryPackage handles POST /api/library/import-package: a multipart
// body with one "package" file. The package is checked whole before anything
// is stored; each match is reused when its demo is already in the library
// and otherwise stored as a shared, read-only match without a demo file.
func (h *Handlers) ImportLibraryPackage(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, sharepkg.MaxPackageBytes+1<<20)
	tmp, size, ok := receiveSharePackage(w, r)
	if !ok {
		return
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()
	pkg, err := sharepkg.Open(tmp, size)
	if err == nil {
		err = checkSharedMatches(pkg)
	}
	if err != nil {
		log.Printf("httpapi: refuse share package code=%s: %v", codeSharePackage, err)
		writeCodedError(w, http.StatusBadRequest, codeSharePackage, err.Error())
		return
	}
	resp, err := h.importSharePackage(r.Context(), pkg)
	switch {
	case errors.Is(err, errShareDemoPending):
		writeCodedError(w, http.StatusConflict, codeShareDemoPending, err.Error())
	case err != nil:
		internalError(w, "import share package", err)
	default:
		log.Printf("httpapi: imported share package kind=%s matches=%d reused=%d notes=%d duplicate=%t",
			resp.Kind, resp.MatchesImported, resp.MatchesExisting, resp.Annotations, resp.Duplicate)
		writeJSON(w, http.StatusOK, resp)
	}
}

// receiveSharePackage copies the "package" part to a temporary file, refusing
// one past the size cap without reading the rest.
func receiveSharePackage(w http.ResponseWriter, r *http.Request) (*os.File, int64, bool) {
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "body must be multipart/form-data")
		return nil, 0, false
	}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "missing package file")
			return nil, 0, false
		}
		if err != nil {
			writeCodedError(w, http.StatusRequestEntityTooLarge, codeSharePackage, "the package is too large or incomplete")
			return nil, 0, false
		}
		if part.FormName() != "package" {
			_ = part.Close()
			continue
		}
		tmp, err := os.CreateTemp("", "zv-share-import-*"+sharepkg.Extension)
		if err != nil {
			internalError(w, "create share import file", err)
			return nil, 0, false
		}
		n, err := io.Copy(tmp, io.LimitReader(part, sharepkg.MaxPackageBytes+1))
		_ = part.Close()
		if err == nil && n > sharepkg.MaxPackageBytes {
			err = fmt.Errorf("larger than %d MB", sharepkg.MaxPackageBytes>>20)
		}
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
			log.Printf("httpapi: refuse share package code=%s: %v", codeSharePackage, err)
			writeCodedError(w, http.StatusRequestEntityTooLarge, codeSharePackage, "the package is too large or incomplete")
			return nil, 0, false
		}
		return tmp, n, true
	}
}

// checkSharedMatches reads every carried match once, so a broken file is
// refused before the library changes.
func checkSharedMatches(pkg *sharepkg.Reader) error {
	for _, entry := range pkg.Manifest.Matches {
		if _, err := pkg.Match(entry.Match.ID); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handlers) importSharePackage(ctx context.Context, pkg *sharepkg.Reader) (shareImportResponse, error) {
	var stored []uuid.UUID
	resp, err := h.importSharePackageInto(ctx, pkg, &stored)
	if err != nil {
		// All or nothing: a note or report that fails drops the matches this import stored.
		h.dropSharedDemos(stored)
	}
	return resp, err
}

func (h *Handlers) importSharePackageInto(ctx context.Context, pkg *sharepkg.Reader, stored *[]uuid.UUID) (shareImportResponse, error) {
	m := pkg.Manifest
	resp := shareImportResponse{Kind: m.Kind, Title: m.Title, Notes: m.Notes}
	matchIDs := map[uuid.UUID]uuid.UUID{}
	for _, entry := range m.Matches {
		matchID, demoID, err := h.importSharedMatch(ctx, pkg, entry)
		if err != nil {
			return resp, err
		}
		matchIDs[entry.Match.ID] = matchID
		if demoID == uuid.Nil {
			resp.MatchesExisting++
			continue
		}
		*stored = append(*stored, demoID)
		resp.MatchesImported++
	}
	for _, a := range m.Annotations {
		if _, err := h.libraryRepo.GetAnnotation(ctx, a.ID); err == nil {
			continue
		}
		a.MatchID = matchIDs[a.MatchID]
		a, err := library.CleanAnnotation(a)
		if err != nil {
			continue
		}
		if err := h.libraryRepo.CreateAnnotation(ctx, &a); err != nil {
			return resp, fmt.Errorf("store note: %w", err)
		}
		resp.Annotations++
	}
	var err error
	if m.Report != nil {
		resp.ReportID, resp.Duplicate, err = h.importSharedReport(ctx, *m.Report, matchIDs)
	}
	return resp, err
}

// importSharedMatch returns the library match a carried match maps to, and
// the id of the demo it stored, nil when the demo was already in the library.
func (h *Handlers) importSharedMatch(ctx context.Context, pkg *sharepkg.Reader, entry sharepkg.Match) (uuid.UUID, uuid.UUID, error) {
	played := entry.Match.PlayedAt
	d := library.Demo{
		SHA256: entry.Demo.SHA256, FileName: sanitizeDemoFileName(entry.Demo.FileName),
		Source: library.SourceShared, SourceManual: true, State: library.ParseParsing,
		CS2Build: entry.Demo.CS2Build, PatchVersion: entry.Demo.PatchVersion, PlayedAt: &played,
	}
	created, err := h.libraryRepo.CreateDemo(ctx, &d)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if !created && d.MatchID != nil {
		return *d.MatchID, uuid.Nil, nil
	}
	if !created && !d.Source.WithoutDemo() {
		return uuid.Nil, uuid.Nil, fmt.Errorf("%w (%s)", errShareDemoPending, d.FileName)
	}
	data, err := pkg.Match(entry.Match.ID)
	if err == nil {
		err = h.storage.Put(artifacts.LibraryTacticalPositionsKey(d.ID), bytes.NewReader(data.Positions))
	}
	if err == nil {
		err = h.storage.Put(artifacts.LibraryTacticalIndexKey(d.ID), bytes.NewReader(data.DocumentJSON))
	}
	var match library.Match
	if err == nil {
		analysis := library.AnalysisFromDocument(d.ID, data.Document)
		analysis.CS2Build, analysis.PatchVersion = d.CS2Build, d.PatchVersion
		match, err = h.libraryRepo.SaveAnalysis(ctx, analysis)
	}
	if err != nil {
		h.dropSharedDemos([]uuid.UUID{d.ID})
		return uuid.Nil, uuid.Nil, fmt.Errorf("store shared match %s: %w", entry.Match.ID, err)
	}
	h.assignSharedTeams(ctx, match, entry.Match.Teams)
	return match.ID, d.ID, nil
}

// assignSharedTeams gives each side the team the sender named, so a report
// on that team reads the shared match.
func (h *Handlers) assignSharedTeams(ctx context.Context, match library.Match, sides []library.MatchTeam) {
	for _, side := range sides {
		name := cleanLibraryText(side.TeamName)
		if name == "" {
			continue
		}
		for _, current := range match.Teams {
			if current.Key == side.Key && current.TeamID != nil && library.NormalizeName(current.TeamName) == library.NormalizeName(name) {
				name = ""
			}
		}
		if name == "" {
			continue
		}
		team, err := h.teamByName(ctx, name)
		if err == nil {
			_, err = h.libraryRepo.AssignMatchTeam(ctx, library.TeamAssignment{MatchID: match.ID, TeamKey: side.Key, TeamID: &team.ID})
		}
		if err != nil {
			log.Printf("httpapi: shared match %s keeps side %s unassigned: %v", match.ID, side.Key, err)
		}
	}
}

func (h *Handlers) dropSharedDemos(ids []uuid.UUID) {
	for _, id := range ids {
		if err := h.libraryRepo.DeleteDemo(context.Background(), id); err != nil {
			log.Printf("httpapi: drop shared demo %s: %v", id, err)
		}
		h.discardLibraryDemo(id)
	}
}

func (h *Handlers) importSharedReport(ctx context.Context, rep library.Report, matchIDs map[uuid.UUID]uuid.UUID) (*uuid.UUID, bool, error) {
	if _, err := h.libraryRepo.GetReport(ctx, rep.ID); err == nil {
		return &rep.ID, true, nil
	} else if !errors.Is(err, library.ErrNotFound) {
		return nil, false, err
	}
	team, err := h.teamByName(ctx, rep.TeamName)
	if err != nil {
		return nil, false, fmt.Errorf("report team: %w", err)
	}
	ids := make([]uuid.UUID, 0, len(rep.MatchIDs))
	for _, id := range rep.MatchIDs {
		ids = append(ids, matchIDs[id])
	}
	report := library.Report{
		ID: rep.ID, Title: cleanLibraryText(rep.Title), TeamID: team.ID, MatchIDs: ids, Side: rep.Side, Notes: rep.Notes,
	}
	if err := h.libraryRepo.CreateReport(ctx, &report); err != nil {
		return nil, false, fmt.Errorf("store report: %w", err)
	}
	return &report.ID, false, nil
}

// attachSharedDemo gives a shared match the .dem just stored at upload.Path,
// so it can be analyzed again and recorded like any other match.
func (h *Handlers) attachSharedDemo(ctx context.Context, shared, upload library.Demo) (library.Demo, error) {
	key := artifacts.LibraryDemoKey(shared.ID)
	if err := h.placeLibraryDemo(upload.Path, key); err != nil {
		return library.Demo{}, fmt.Errorf("store demo of shared match: %w", err)
	}
	d, err := h.libraryRepo.AttachDemoFile(ctx, shared.ID, library.DemoFile{
		Path: key, FileName: upload.FileName, SizeBytes: upload.SizeBytes, Source: upload.Source,
	})
	if err != nil {
		return library.Demo{}, err
	}
	if err := h.libraryRepo.SetDemoState(ctx, d.ID, library.ParseQueued, "", ""); err != nil {
		return library.Demo{}, err
	}
	if err := h.enqueueLibraryScan(ctx, d.ID); err != nil {
		log.Printf("httpapi: %v", err)
	}
	return h.libraryRepo.GetDemo(ctx, d.ID)
}
