package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

const (
	maxUploadDocumentBytes  = 64 << 20  // 64 MiB of JSON; real matches are under 2 MiB
	maxUploadPositionsBytes = 128 << 20 // 128 MiB; real matches are under 4 MiB
	maxUploadBodyBytes      = maxUploadDocumentBytes + maxUploadPositionsBytes + 1<<20
)

// WithLibraryUploads enables POST /api/library/uploads, the way the web
// edition stores a match the browser parsed. The demo itself never arrives.
func WithLibraryUploads() Option {
	return func(h *Handlers) {
		h.clipmind.libraryUploads = true
	}
}

// uploadResult is the answer to a library upload.
type uploadResult struct {
	Created bool          `json:"created"`
	Demo    library.Demo  `json:"demo"`
	Match   library.Match `json:"match"`
}

// UploadLibraryAnalysis handles POST /api/library/uploads: a multipart body
// with the tactical document ("document"), the position blob ("positions"),
// the demo's "file_name" and "size_bytes", and optionally "cs2_build". It does
// what the library worker does after a scan, for a scan done in the browser.
func (h *Handlers) UploadLibraryAnalysis(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBodyBytes)
	// #nosec G120 -- r.Body is capped with MaxBytesReader immediately above.
	if err := r.ParseMultipartForm(multipartMemBudget); err != nil {
		writeError(w, http.StatusBadRequest, "parsing multipart form: "+err.Error())
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	documentBytes, err := readUploadPart(r, "document", maxUploadDocumentBytes)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	positions, err := readUploadPart(r, "positions", maxUploadPositionsBytes)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	doc, err := validateUploadedAnalysis(documentBytes, positions)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	size, _ := strconv.ParseInt(r.FormValue("size_bytes"), 10, 64)
	build, _ := strconv.Atoi(r.FormValue("cs2_build"))
	fileName := path.Base(strings.ReplaceAll(strings.TrimSpace(r.FormValue("file_name")), `\`, "/"))
	if fileName == "." || fileName == "/" || fileName == "" {
		fileName = doc.Demo.SHA256[:12] + ".dem"
	}

	demo := library.Demo{
		SHA256:    doc.Demo.SHA256,
		FileName:  fileName,
		SizeBytes: max(size, 0),
		Source:    library.DetectSource("", fileName),
		State:     library.ParseQueued,
	}
	created, err := h.libraryRepo.CreateDemo(r.Context(), &demo)
	if err != nil {
		internalError(w, "create uploaded library demo", err)
		return
	}
	if !created && demo.State == library.ParseReady {
		match, err := h.matchOfDemo(r, demo)
		if err != nil {
			internalError(w, "load the existing match", err)
			return
		}
		writeJSON(w, http.StatusOK, uploadResult{Created: false, Demo: demo, Match: match})
		return
	}

	// The stored document names no path: the demo stays on the user's machine.
	doc.Demo.Path = ""
	stored, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		internalError(w, "encode the uploaded document", err)
		return
	}
	if err := h.storage.Put(artifacts.LibraryTacticalPositionsKey(demo.ID), bytes.NewReader(positions)); err != nil {
		internalError(w, "store uploaded positions", err)
		return
	}
	if err := h.storage.Put(artifacts.LibraryTacticalIndexKey(demo.ID), bytes.NewReader(append(stored, '\n'))); err != nil {
		internalError(w, "store uploaded document", err)
		return
	}
	analysis := library.AnalysisFromDocument(demo.ID, doc)
	analysis.Source = demo.Source
	analysis.CS2Build = build
	match, err := h.libraryRepo.SaveAnalysis(r.Context(), analysis)
	if err != nil {
		_ = h.libraryRepo.SetDemoState(r.Context(), demo.ID, library.ParseFailed, "write_library", err.Error())
		internalError(w, "save the uploaded analysis", err)
		return
	}
	demo, err = h.libraryRepo.GetDemo(r.Context(), demo.ID)
	if err != nil {
		internalError(w, "reload the uploaded demo", err)
		return
	}
	writeJSON(w, http.StatusCreated, uploadResult{Created: true, Demo: demo, Match: match})
}

func (h *Handlers) matchOfDemo(r *http.Request, demo library.Demo) (library.Match, error) {
	if demo.MatchID == nil {
		return library.Match{}, errors.New("ready demo without a match")
	}
	return h.libraryRepo.GetMatch(r.Context(), *demo.MatchID)
}

func readUploadPart(r *http.Request, name string, limit int64) ([]byte, error) {
	file, _, err := r.FormFile(name)
	if err != nil {
		return nil, fmt.Errorf("missing %s part", name)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("reading %s part: %w", name, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s part is too large", name)
	}
	return data, nil
}

// validateUploadedAnalysis refuses a document that is not a plausible scan and
// a position blob that does not match the descriptor inside it.
func validateUploadedAnalysis(documentBytes, positions []byte) (tacticalplan.Document, error) {
	var doc tacticalplan.Document
	if err := json.Unmarshal(documentBytes, &doc); err != nil {
		return doc, fmt.Errorf("the document is not valid JSON: %w", err)
	}
	if len(doc.Demo.SHA256) != 64 {
		return doc, errors.New("the document has no demo checksum")
	}
	if _, err := hex.DecodeString(doc.Demo.SHA256); err != nil {
		return doc, errors.New("the demo checksum is not hexadecimal")
	}
	if doc.Demo.Map == "" || len(doc.Rounds) == 0 {
		return doc, errors.New("the document has no map or no rounds")
	}
	if !doc.Geometry.Calibration.Valid() {
		return doc, fmt.Errorf("map %q has no usable radar calibration", doc.Demo.Map)
	}
	if doc.Positions.Format != tacticalplan.PositionsFormat {
		return doc, fmt.Errorf("unsupported positions format %q", doc.Positions.Format)
	}
	if int64(len(positions)) != doc.Positions.ByteLength {
		return doc, errors.New("the position blob does not match the document")
	}
	if sum := sha256.Sum256(positions); hex.EncodeToString(sum[:]) != doc.Positions.SHA256 {
		return doc, errors.New("the position blob does not match the document")
	}
	return doc, nil
}
