package clipmind

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// The derived files of a match travel beside the database rows: a device
// sends them gzip-compressed, the hub keeps them as the orchestrator reads them.
const (
	artifactTactical  = "tactical.json"
	artifactPositions = "positions.zvpos"

	maxArtifactBytes = 128 << 20
)

// withTenant runs fn on the principal's workspace once the role allows need.
func (s *Server) withTenant(need permission, fn func(w http.ResponseWriter, r *http.Request, tn *tenant)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := principalOf(r)
		if p.WorkspaceID == "" {
			writeError(w, http.StatusConflict, "no_workspace", "choose a workspace first")
			return
		}
		if !allows(p, need) {
			writeError(w, http.StatusForbidden, "forbidden", "your role cannot do this")
			return
		}
		tn, err := s.tenants.acquire(p.WorkspaceID)
		if err != nil {
			writeServerError(w, "open workspace", err)
			return
		}
		defer s.tenants.release(tn)
		fn(w, r, tn)
	}
}

func artifactKey(demo uuid.UUID, name string) (string, bool) {
	switch name {
	case artifactTactical:
		return artifacts.LibraryTacticalIndexKey(demo), true
	case artifactPositions:
		return artifacts.LibraryTacticalPositionsKey(demo), true
	}
	return "", false
}

type artifactState struct {
	ID         string `json:"id"`
	Tactical   bool   `json:"tactical"`
	Positions  bool   `json:"positions"`
	SHA256     string `json:"sha256"`
	DemoState  string `json:"state"`
	HasMatchID bool   `json:"has_match"`
}

// handleListArtifacts tells a device which demos already have their derived
// files on the hub, so it uploads only what is missing.
func (s *Server) handleListArtifacts(w http.ResponseWriter, r *http.Request, tn *tenant) {
	out := []artifactState{}
	for offset := 0; ; offset += library.MaxListLimit {
		page, err := tn.repos.Library.ListDemos(r.Context(), library.DemoFilter{Limit: library.MaxListLimit, Offset: offset})
		if err != nil {
			writeServerError(w, "list demos", err)
			return
		}
		for _, d := range page {
			tactical, _ := tn.files.Exists(artifacts.LibraryTacticalIndexKey(d.ID))
			positions, _ := tn.files.Exists(artifacts.LibraryTacticalPositionsKey(d.ID))
			out = append(out, artifactState{ID: d.ID.String(), Tactical: tactical, Positions: positions, SHA256: d.SHA256, DemoState: string(d.State), HasMatchID: d.MatchID != nil})
		}
		if len(page) < library.MaxListLimit {
			break
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"demos": out})
}

func (s *Server) handlePutArtifact(w http.ResponseWriter, r *http.Request, tn *tenant) {
	demoID, err := uuid.Parse(chi.URLParam(r, "demo"))
	name := chi.URLParam(r, "name")
	key, ok := artifactKey(demoID, name)
	if err != nil || !ok {
		writeError(w, http.StatusNotFound, "not_found", "unknown artifact")
		return
	}
	// The row must have synced first: nobody fills the disk with ids the library lacks.
	demo, err := tn.repos.Library.GetDemo(r.Context(), demoID)
	if err != nil {
		if errors.Is(err, library.ErrNotFound) {
			writeError(w, http.StatusConflict, "demo_not_synced", "sync the demo rows before its files")
			return
		}
		writeServerError(w, "get demo", err)
		return
	}
	body, err := readArtifactBody(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if name == artifactTactical {
		body, err = checkTacticalArtifact(body, demo.SHA256)
	} else {
		err = checkPositionsArtifact(tn, demo, body)
	}
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_artifact", err.Error())
		return
	}
	if err := tn.files.Put(key, bytes.NewReader(body)); err != nil {
		writeServerError(w, "store artifact", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleGetArtifact(w http.ResponseWriter, r *http.Request, tn *tenant) {
	demoID, err := uuid.Parse(chi.URLParam(r, "demo"))
	key, ok := artifactKey(demoID, chi.URLParam(r, "name"))
	if err != nil || !ok {
		writeError(w, http.StatusNotFound, "not_found", "unknown artifact")
		return
	}
	f, err := tn.files.Open(key)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "artifact not found")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		_, _ = io.Copy(gz, f)
		_ = gz.Close()
		return
	}
	_, _ = io.Copy(w, f)
}

// readArtifactBody reads a bounded body, undoing gzip when the sender used it.
func readArtifactBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	var src io.Reader = http.MaxBytesReader(w, r.Body, maxArtifactBytes)
	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(src)
		if err != nil {
			return nil, errors.New("the body is not valid gzip")
		}
		defer gz.Close()
		src = gz
	}
	data, err := io.ReadAll(io.LimitReader(src, maxArtifactBytes+1))
	if err != nil {
		return nil, errors.New("could not read the body")
	}
	if len(data) > maxArtifactBytes {
		return nil, errors.New("the file is too large")
	}
	return data, nil
}

// checkTacticalArtifact accepts a document of the demo the row names and
// returns it without the client's file path.
func checkTacticalArtifact(data []byte, sha string) ([]byte, error) {
	var doc tacticalplan.Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, errors.New("the document is not valid JSON")
	}
	if !strings.EqualFold(doc.Demo.SHA256, sha) {
		return nil, errors.New("the document belongs to another demo")
	}
	if doc.Demo.Path == "" {
		return data, nil
	}
	doc.Demo.Path = ""
	stored, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(stored, '\n'), nil
}

// checkPositionsArtifact binds the blob to the stored document's descriptor.
func checkPositionsArtifact(tn *tenant, demo library.Demo, data []byte) error {
	f, err := tn.files.Open(artifacts.LibraryTacticalIndexKey(demo.ID))
	if err != nil {
		return errors.New("send the document before the positions")
	}
	defer f.Close()
	var doc tacticalplan.Document
	if err := json.NewDecoder(f).Decode(&doc); err != nil {
		return errors.New("the stored document is unreadable")
	}
	if int64(len(data)) != doc.Positions.ByteLength {
		return errors.New("the position blob does not match the document")
	}
	if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != doc.Positions.SHA256 {
		return errors.New("the position blob does not match the document")
	}
	return nil
}
