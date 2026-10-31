package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/demozstd"
	"github.com/rechedev9/cliphub/internal/faceit"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tasks"
)

// LibraryRepository is the shared demo library the /api/library routes read
// and correct. store.LibraryRepository satisfies it.
type LibraryRepository interface {
	CreateDemo(ctx context.Context, d *library.Demo) (bool, error)
	GetDemo(ctx context.Context, id uuid.UUID) (library.Demo, error)
	ListDemos(ctx context.Context, filter library.DemoFilter) ([]library.Demo, error)
	UpdateDemo(ctx context.Context, id uuid.UUID, patch library.DemoPatch) (library.Demo, error)
	SetDemoState(ctx context.Context, id uuid.UUID, state library.ParseState, errorCode, message string) error
	DeleteDemo(ctx context.Context, id uuid.UUID) error
	LinkDemoJob(ctx context.Context, demoID, jobID uuid.UUID) error
	DemoForJob(ctx context.Context, jobID uuid.UUID) (library.Demo, error)
	GetMatch(ctx context.Context, id uuid.UUID) (library.Match, error)
	ListMatches(ctx context.Context, filter library.MatchFilter) ([]library.Match, error)
	UpdateMatch(ctx context.Context, id uuid.UUID, patch library.MatchPatch) (library.Match, error)
	AssignMatchTeam(ctx context.Context, assignment library.TeamAssignment) (library.Match, error)
	CreateTeam(ctx context.Context, t *library.Team) error
	GetTeam(ctx context.Context, id uuid.UUID) (library.Team, error)
	ListTeams(ctx context.Context, filter library.TeamFilter) ([]library.Team, error)
	UpdateTeam(ctx context.Context, id uuid.UUID, patch library.TeamPatch) (library.Team, error)
	MergeTeams(ctx context.Context, from, into uuid.UUID) (library.Team, error)
	GetPlayer(ctx context.Context, steamID string) (library.Player, error)
	ListPlayers(ctx context.Context, filter library.PlayerFilter) ([]library.Player, error)
	SetPlayerFaceit(ctx context.Context, steamID, faceitPlayerID, faceitNickname string) (library.Player, error)
	CreateSeries(ctx context.Context, s *library.Series) error
	GetSeries(ctx context.Context, id uuid.UUID) (library.Series, error)
	ListSeries(ctx context.Context, filter library.SeriesFilter) ([]library.Series, error)
	UpdateSeries(ctx context.Context, id uuid.UUID, s library.Series) (library.Series, error)
	DeleteSeries(ctx context.Context, id uuid.UUID) error
	CreateReport(ctx context.Context, r *library.Report) error
	GetReport(ctx context.Context, id uuid.UUID) (library.Report, error)
	ListReports(ctx context.Context, filter library.ReportFilter) ([]library.Report, error)
	UpdateReport(ctx context.Context, id uuid.UUID, patch library.ReportPatch) (library.Report, error)
	DeleteReport(ctx context.Context, id uuid.UUID) error
	CreateAnnotation(ctx context.Context, a *library.Annotation) error
	GetAnnotation(ctx context.Context, id uuid.UUID) (library.Annotation, error)
	ListAnnotations(ctx context.Context, filter library.AnnotationFilter) ([]library.Annotation, error)
	UpdateAnnotation(ctx context.Context, id uuid.UUID, patch library.AnnotationPatch) (library.Annotation, error)
	DeleteAnnotation(ctx context.Context, id uuid.UUID) error
	CreatePlay(ctx context.Context, p *library.Play) error
	GetPlay(ctx context.Context, id uuid.UUID) (library.Play, error)
	ListPlays(ctx context.Context, filter library.PlayFilter) ([]library.Play, error)
	UpdatePlay(ctx context.Context, id uuid.UUID, patch library.PlayPatch) (library.Play, error)
	DeletePlay(ctx context.Context, id uuid.UUID) error
}

// WithLibrary enables the /api/library routes over repo.
func WithLibrary(repo LibraryRepository) Option {
	return func(h *Handlers) {
		h.libraryRepo = repo
	}
}

// Import limits. One request carries at most maxLibraryImportFiles demos, each
// capped like a job upload; a client importing a large folder sends batches.
const (
	maxLibraryImportFiles = 50
	maxLibraryImportBytes = maxLibraryImportFiles * maxMultipartBytes
	maxLibraryFieldBytes  = 256
	maxLibraryTextRunes   = 120
	libraryUniqueTTL      = 24 * time.Hour
)

// Import outcomes of one file.
const (
	libraryImportImported  = "imported"
	libraryImportDuplicate = "duplicate"
	libraryImportRejected  = "rejected"
)

// libraryImportResult is the outcome of one file of an import request. Demo
// is set for imported and duplicate files; a duplicate returns the demo that
// was already in the library.
type libraryImportResult struct {
	FileName  string        `json:"file_name"`
	Status    string        `json:"status"`
	Demo      *library.Demo `json:"demo,omitempty"`
	ErrorCode string        `json:"error_code,omitempty"`
	Error     string        `json:"error,omitempty"`
}

func (h *Handlers) libraryReady(w http.ResponseWriter) bool {
	if h.libraryRepo == nil {
		writeError(w, http.StatusNotImplemented, "library is not configured")
		return false
	}
	return true
}

// writeLibraryError maps the repository's sentinel errors onto HTTP statuses.
func writeLibraryError(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, library.ErrNotFound):
		writeError(w, http.StatusNotFound, op+": not found")
	case errors.Is(err, library.ErrConflict):
		writeError(w, http.StatusConflict, op+": conflict")
	default:
		internalError(w, op, err)
	}
}

func libraryUUIDParam(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		writeError(w, http.StatusBadRequest, name+" must be a valid UUID")
		return uuid.Nil, false
	}
	return id, true
}

func validSteamID(raw string) bool {
	_, err := strconv.ParseUint(raw, 10, 64)
	return err == nil
}

// libraryPaging reads ?limit and ?offset.
func libraryPaging(q url.Values) (limit, offset int, err error) {
	if raw := q.Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > library.MaxListLimit {
			return 0, 0, fmt.Errorf("limit must be an integer from 1 to %d", library.MaxListLimit)
		}
	}
	if raw := q.Get("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 {
			return 0, 0, errors.New("offset must be a non-negative integer")
		}
	}
	return limit, offset, nil
}

func optionalUUIDQuery(q url.Values, name string) (*uuid.UUID, error) {
	raw := q.Get(name)
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be a valid UUID", name)
	}
	return &id, nil
}

func optionalTimeQuery(q url.Values, name string) (*time.Time, error) {
	raw := q.Get(name)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be an RFC 3339 time", name)
	}
	return &t, nil
}

func cleanLibraryText(raw string) string {
	text := library.CleanName(raw)
	if runes := []rune(text); len(runes) > maxLibraryTextRunes {
		text = strings.TrimSpace(string(runes[:maxLibraryTextRunes]))
	}
	return text
}

// ImportLibraryDemos handles POST /api/library/demos: a multipart body with
// one or more "demo" file parts, streamed one at a time so no file is held in
// memory. A "played_at" field (RFC 3339) sets the date of the next demo only;
// each "tag" field applies to every demo after it. Every file gets its own
// result, so one bad file does not undo the others.
func (h *Handlers) ImportLibraryDemos(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxLibraryImportBytes)
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "body must be multipart/form-data")
		return
	}
	results := []libraryImportResult{}
	var playedAt *time.Time
	var tags []string
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "reading multipart body: "+err.Error())
			return
		}
		switch part.FormName() {
		case "played_at":
			value, err := readLibraryField(part)
			if err == nil {
				var t time.Time
				t, err = time.Parse(time.RFC3339, value)
				playedAt = &t
			}
			if err != nil {
				_ = part.Close()
				writeError(w, http.StatusBadRequest, "played_at must be an RFC 3339 time")
				return
			}
		case "tag":
			value, err := readLibraryField(part)
			if err != nil {
				_ = part.Close()
				writeError(w, http.StatusBadRequest, "tag is too long")
				return
			}
			tags = append(tags, value)
		case "demo":
			if len(results) == maxLibraryImportFiles {
				_ = part.Close()
				writeError(w, http.StatusBadRequest, fmt.Sprintf("at most %d demos per request", maxLibraryImportFiles))
				return
			}
			result, err := h.importLibraryDemo(r.Context(), part, part.FileName(), playedAt, tags)
			if err != nil {
				_ = part.Close()
				internalError(w, "import library demo", err)
				return
			}
			// importLibraryDemo already logs a truncated demo with its byte counts.
			if result.Status == libraryImportRejected && result.ErrorCode != codeTruncatedDemo {
				log.Printf("httpapi: refuse library demo %q code=%s: %s", result.FileName, result.ErrorCode, result.Error)
			}
			results = append(results, result)
			playedAt = nil
		}
		_ = part.Close()
	}
	if len(results) == 0 {
		writeError(w, http.StatusBadRequest, "missing demo file")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"demos": results})
}

func readLibraryField(part io.Reader) (string, error) {
	b, err := io.ReadAll(io.LimitReader(part, maxLibraryFieldBytes+1))
	if err != nil {
		return "", err
	}
	if len(b) > maxLibraryFieldBytes {
		return "", errors.New("field too long")
	}
	return strings.TrimSpace(string(b)), nil
}

// importLibraryDemo validates, stores and registers one demo. A file that is
// not a CS2 demo is a rejected result, not an error; only a failure of the
// library itself is returned as an error.
func (h *Handlers) importLibraryDemo(ctx context.Context, src io.Reader, rawName string, playedAt *time.Time, tags []string) (libraryImportResult, error) {
	name := sanitizeDemoFileName(rawName)
	result := libraryImportResult{FileName: name, Status: libraryImportRejected}
	opened, name, err := demozstd.Open(src, name, maxDemoBytes)
	if err != nil {
		result.ErrorCode, result.Error = codeUnreadableDemo, "could not read the demo"
		return result, nil
	}
	defer opened.Close()
	result.FileName = name
	var header [8]byte
	n, err := io.ReadFull(opened, header[:])
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		result.ErrorCode, result.Error = codeUnreadableDemo, "could not read the demo"
		return result, nil
	}
	switch {
	case isDemoHeader(header[:n]):
	case isCSGODemoHeader(header[:n]):
		result.ErrorCode, result.Error = codeCSGODemo, "the file is a CS:GO demo; only CS2 demos are supported"
		return result, nil
	default:
		result.ErrorCode, result.Error = codeNotADemo, "the file is not a CS2 demo"
		return result, nil
	}

	id := uuid.New()
	key := artifacts.LibraryDemoKey(id)
	digest := sha256.New()
	counted := &storedDemo{r: io.MultiReader(bytes.NewReader(header[:n]), opened)}
	if err := h.storage.Put(key, io.TeeReader(counted, digest)); err != nil {
		h.discardLibraryDemo(id)
		log.Printf("httpapi: store library demo %q: %v", name, err)
		result.ErrorCode, result.Error = codeUnreadableDemo, "could not store the demo"
		return result, nil
	}
	if err := counted.checkLength(); err != nil {
		h.discardLibraryDemo(id)
		log.Printf("httpapi: refuse library demo code=%s: %v", codeTruncatedDemo, err)
		result.ErrorCode, result.Error = codeTruncatedDemo, "the demo file is incomplete"
		return result, nil
	}
	demo := library.Demo{
		ID:        id,
		SHA256:    hex.EncodeToString(digest.Sum(nil)),
		FileName:  name,
		Path:      key,
		SizeBytes: counted.n,
		Source:    library.DetectSource("", name),
		PlayedAt:  playedAt,
		Tags:      tags,
	}
	created, err := h.libraryRepo.CreateDemo(ctx, &demo)
	if err != nil {
		h.discardLibraryDemo(id)
		return libraryImportResult{}, err
	}
	if !created {
		h.discardLibraryDemo(id)
		result.Status, result.Demo = libraryImportDuplicate, &demo
		return result, nil
	}
	// A refused enqueue leaves the demo failed in the library, where a retry
	// can queue it again; the import itself still succeeded.
	if err := h.enqueueLibraryScan(ctx, demo.ID); err != nil {
		log.Printf("httpapi: %v", err)
	}
	if current, err := h.libraryRepo.GetDemo(ctx, demo.ID); err == nil {
		demo = current
	}
	result.Status, result.Demo = libraryImportImported, &demo
	return result, nil
}

// discardLibraryDemo removes the stored folder of a demo the library does not
// keep: a duplicate, a failed store, or a deleted demo.
func (h *Handlers) discardLibraryDemo(id uuid.UUID) {
	deleter, ok := h.storage.(jobArtifactDeleter)
	if !ok {
		return
	}
	if err := deleter.DeleteTree(artifacts.LibraryDemoPrefix(id)); err != nil {
		log.Printf("httpapi: remove library demo files %s: %v", id, err)
	}
}

// enqueueLibraryScan queues the scan of one demo. A demo whose task is
// dropped at shutdown stays queued, and ResumeLibraryScans picks it up on the
// next start; an enqueue the queue refuses marks the demo failed instead.
func (h *Handlers) enqueueLibraryScan(ctx context.Context, id uuid.UUID) error {
	task, err := tasks.NewAnalyzeLibraryDemoTask(id)
	if err != nil {
		return fmt.Errorf("build library scan task: %w", err)
	}
	_, err = h.queue.Enqueue(task, asynq.Unique(libraryUniqueTTL))
	if err == nil || errors.Is(err, asynq.ErrDuplicateTask) {
		return nil
	}
	if stateErr := h.libraryRepo.SetDemoState(ctx, id, library.ParseFailed, "enqueue_failed", "enqueue library scan: "+err.Error()); stateErr != nil {
		return fmt.Errorf("enqueue library scan: %w (mark failed: %v)", err, stateErr)
	}
	return fmt.Errorf("enqueue library scan: %w", err)
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
			if d.State != library.ParseReady || h.libraryAnalysisStale(d.ID) {
				pending = append(pending, d)
			}
		}
		if len(page) < library.MaxListLimit {
			break
		}
	}
	for _, d := range pending {
		if err := h.libraryRepo.SetDemoState(ctx, d.ID, library.ParseQueued, "", ""); err != nil {
			return 0, err
		}
		if err := h.enqueueLibraryScan(ctx, d.ID); err != nil {
			return 0, err
		}
	}
	return len(pending), nil
}

// ListLibraryDemos handles GET /api/library/demos.
func (h *Handlers) ListLibraryDemos(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	q := r.URL.Query()
	limit, offset, err := libraryPaging(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	filter := library.DemoFilter{Tag: q.Get("tag"), Limit: limit, Offset: offset}
	for _, raw := range q["state"] {
		state := library.ParseState(raw)
		if !library.ValidParseState(state) {
			writeError(w, http.StatusBadRequest, "state must be queued, parsing, ready or failed")
			return
		}
		filter.States = append(filter.States, state)
	}
	if raw := q.Get("source"); raw != "" {
		filter.Source = library.Source(raw)
		if !library.ValidSource(filter.Source) {
			writeError(w, http.StatusBadRequest, "source must be valve, faceit, hltv, scrim or unknown")
			return
		}
	}
	demos, err := h.libraryRepo.ListDemos(r.Context(), filter)
	if err != nil {
		internalError(w, "list library demos", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"demos": demos})
}

// GetLibraryDemo handles GET /api/library/demos/{id}.
func (h *Handlers) GetLibraryDemo(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, d)
}

type patchLibraryDemoRequest struct {
	Source *library.Source `json:"source"`
	Tags   *[]string       `json:"tags"`
}

// PatchLibraryDemo handles PATCH /api/library/demos/{id}: a manual source
// (it then survives every re-analysis) and the full tag set.
func (h *Handlers) PatchLibraryDemo(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req patchLibraryDemoRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid demo JSON")
		return
	}
	if req.Source != nil && !library.ValidSource(*req.Source) {
		writeError(w, http.StatusBadRequest, "source must be valve, faceit, hltv, scrim or unknown")
		return
	}
	d, err := h.libraryRepo.UpdateDemo(r.Context(), id, library.DemoPatch{Source: req.Source, Tags: req.Tags})
	if err != nil {
		writeLibraryError(w, "update demo", err)
		return
	}
	writeJSON(w, http.StatusOK, d)
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

// DeleteLibraryDemo handles DELETE /api/library/demos/{id}: the demo, its
// match and its files. A scan still running drops its result.
func (h *Handlers) DeleteLibraryDemo(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.libraryRepo.DeleteDemo(r.Context(), id); err != nil {
		writeLibraryError(w, "delete demo", err)
		return
	}
	h.discardLibraryDemo(id)
	w.WriteHeader(http.StatusNoContent)
}

// ListLibraryMatches handles GET /api/library/matches.
func (h *Handlers) ListLibraryMatches(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	q := r.URL.Query()
	filter, err := libraryMatchFilter(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	matches, err := h.libraryRepo.ListMatches(r.Context(), filter)
	if err != nil {
		internalError(w, "list library matches", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"matches": matches})
}

func libraryMatchFilter(q url.Values) (library.MatchFilter, error) {
	var filter library.MatchFilter
	var err error
	if filter.Limit, filter.Offset, err = libraryPaging(q); err != nil {
		return filter, err
	}
	if raw := q.Get("map"); raw != "" {
		if !mapNamePattern.MatchString(raw) {
			return filter, errors.New("map must match ^[a-z0-9_]+$")
		}
		filter.Map = raw
	}
	if filter.TeamID, err = optionalUUIDQuery(q, "team_id"); err != nil {
		return filter, err
	}
	if filter.SeriesID, err = optionalUUIDQuery(q, "series_id"); err != nil {
		return filter, err
	}
	if raw := q.Get("steamid"); raw != "" {
		if !validSteamID(raw) {
			return filter, errors.New("steamid must be a 64-bit unsigned integer")
		}
		filter.SteamID = raw
	}
	if raw := q.Get("source"); raw != "" {
		filter.Source = library.Source(raw)
		if !library.ValidSource(filter.Source) {
			return filter, errors.New("source must be valve, faceit, hltv, scrim or unknown")
		}
	}
	filter.Tag = q.Get("tag")
	if filter.From, err = optionalTimeQuery(q, "from"); err != nil {
		return filter, err
	}
	if filter.To, err = optionalTimeQuery(q, "to"); err != nil {
		return filter, err
	}
	return filter, nil
}

// GetLibraryMatch handles GET /api/library/matches/{id}.
func (h *Handlers) GetLibraryMatch(w http.ResponseWriter, r *http.Request) {
	m, ok := h.loadLibraryMatch(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (h *Handlers) loadLibraryMatch(w http.ResponseWriter, r *http.Request) (library.Match, bool) {
	if !h.libraryReady(w) {
		return library.Match{}, false
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return library.Match{}, false
	}
	m, err := h.libraryRepo.GetMatch(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get match", err)
		return library.Match{}, false
	}
	return m, true
}

type patchLibraryMatchRequest struct {
	PlayedAt *time.Time      `json:"played_at"`
	SeriesID json.RawMessage `json:"series_id"`
}

// PatchLibraryMatch handles PATCH /api/library/matches/{id}: the match date,
// and the series (a UUID attaches it, null detaches it).
func (h *Handlers) PatchLibraryMatch(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req patchLibraryMatchRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid match JSON")
		return
	}
	patch := library.MatchPatch{PlayedAt: req.PlayedAt}
	switch raw := strings.TrimSpace(string(req.SeriesID)); raw {
	case "":
	case "null":
		patch.ClearSeries = true
	default:
		var seriesID uuid.UUID
		if err := json.Unmarshal(req.SeriesID, &seriesID); err != nil {
			writeError(w, http.StatusBadRequest, "series_id must be a UUID or null")
			return
		}
		if _, err := h.libraryRepo.GetSeries(r.Context(), seriesID); err != nil {
			writeLibraryError(w, "get series", err)
			return
		}
		patch.SeriesID = &seriesID
	}
	m, err := h.libraryRepo.UpdateMatch(r.Context(), id, patch)
	if err != nil {
		writeLibraryError(w, "update match", err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

type assignLibraryTeamRequest struct {
	TeamID        *uuid.UUID `json:"team_id"`
	TeamName      string     `json:"team_name"`
	RememberAlias bool       `json:"remember_alias"`
}

// AssignLibraryMatchTeam handles PUT /api/library/matches/{id}/teams/{key},
// the manual correction of one side. team_id picks an existing team;
// team_name picks the team with that name or alias, creating it if none has
// it; neither clears the side. A manual side survives every re-analysis.
func (h *Handlers) AssignLibraryMatchTeam(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req assignLibraryTeamRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid team assignment JSON")
		return
	}
	teamID := req.TeamID
	if teamID == nil && library.NormalizeName(req.TeamName) != "" {
		team, err := h.teamByName(r.Context(), req.TeamName)
		if err != nil {
			writeLibraryError(w, "resolve team", err)
			return
		}
		teamID = &team.ID
	}
	m, err := h.libraryRepo.AssignMatchTeam(r.Context(), library.TeamAssignment{
		MatchID: id, TeamKey: chi.URLParam(r, "key"), TeamID: teamID, RememberAlias: req.RememberAlias,
	})
	if err != nil {
		writeLibraryError(w, "assign match team", err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// teamByName returns the team whose name or alias matches name, creating it
// when no team has it.
func (h *Handlers) teamByName(ctx context.Context, name string) (library.Team, error) {
	key := library.NormalizeName(name)
	for offset := 0; ; offset += library.MaxListLimit {
		teams, err := h.libraryRepo.ListTeams(ctx, library.TeamFilter{Query: name, Limit: library.MaxListLimit, Offset: offset})
		if err != nil {
			return library.Team{}, err
		}
		for _, t := range teams {
			for _, alias := range append([]string{t.Name}, t.Aliases...) {
				if library.NormalizeName(alias) == key {
					return t, nil
				}
			}
		}
		if len(teams) < library.MaxListLimit {
			break
		}
	}
	team := library.Team{Name: cleanLibraryText(name)}
	if err := h.libraryRepo.CreateTeam(ctx, &team); err != nil {
		return library.Team{}, err
	}
	return team, nil
}

func (h *Handlers) libraryMatchKeys(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	m, ok := h.loadLibraryMatch(w, r)
	if !ok {
		return "", "", false
	}
	return artifacts.LibraryTacticalIndexKey(m.DemoID), artifacts.LibraryTacticalPositionsKey(m.DemoID), true
}

// GetLibraryMatchTactical handles GET /api/library/matches/{id}/tactical: the
// tactical document every number of the match traces back to.
func (h *Handlers) GetLibraryMatchTactical(w http.ResponseWriter, r *http.Request) {
	indexKey, _, ok := h.libraryMatchKeys(w, r)
	if ok {
		h.serveTacticalDocument(w, indexKey)
	}
}

// GetLibraryMatchTacticalRound handles GET /api/library/matches/{id}/tactical/rounds/{round}.
func (h *Handlers) GetLibraryMatchTacticalRound(w http.ResponseWriter, r *http.Request) {
	indexKey, positionsKey, ok := h.libraryMatchKeys(w, r)
	if ok {
		h.serveTacticalRound(w, r, indexKey, positionsKey)
	}
}

// GetLibraryMatchTacticalPositions handles GET /api/library/matches/{id}/tactical/positions.
func (h *Handlers) GetLibraryMatchTacticalPositions(w http.ResponseWriter, r *http.Request) {
	_, positionsKey, ok := h.libraryMatchKeys(w, r)
	if ok {
		h.serveTacticalPositions(w, r, positionsKey)
	}
}

// GetLibraryMatchTacticalAggregate handles GET /api/library/matches/{id}/tactical/aggregate.
func (h *Handlers) GetLibraryMatchTacticalAggregate(w http.ResponseWriter, r *http.Request) {
	indexKey, _, ok := h.libraryMatchKeys(w, r)
	if ok {
		h.serveTacticalAggregate(w, r, indexKey)
	}
}

// ListLibraryTeams handles GET /api/library/teams.
func (h *Handlers) ListLibraryTeams(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	q := r.URL.Query()
	limit, offset, err := libraryPaging(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	teams, err := h.libraryRepo.ListTeams(r.Context(), library.TeamFilter{Query: q.Get("q"), Limit: limit, Offset: offset})
	if err != nil {
		internalError(w, "list library teams", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"teams": teams})
}

type libraryTeamRequest struct {
	Name    *string   `json:"name"`
	Aliases *[]string `json:"aliases"`
}

func (req libraryTeamRequest) clean() (libraryTeamRequest, error) {
	if req.Name != nil {
		name := cleanLibraryText(*req.Name)
		if library.NormalizeName(name) == "" {
			return req, errors.New("name must not be empty")
		}
		req.Name = &name
	}
	if req.Aliases != nil {
		aliases := make([]string, 0, len(*req.Aliases))
		for _, a := range *req.Aliases {
			aliases = append(aliases, cleanLibraryText(a))
		}
		req.Aliases = &aliases
	}
	return req, nil
}

// CreateLibraryTeam handles POST /api/library/teams. The name is also an
// alias; a name or alias another team already owns is a 409.
func (h *Handlers) CreateLibraryTeam(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	var req libraryTeamRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid team JSON")
		return
	}
	req, err := req.clean()
	if err == nil && req.Name == nil {
		err = errors.New("name is required")
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	team := library.Team{Name: *req.Name}
	if req.Aliases != nil {
		team.Aliases = *req.Aliases
	}
	if err := h.libraryRepo.CreateTeam(r.Context(), &team); err != nil {
		writeLibraryError(w, "create team", err)
		return
	}
	writeJSON(w, http.StatusCreated, team)
}

// GetLibraryTeam handles GET /api/library/teams/{id}, with the roster by date.
func (h *Handlers) GetLibraryTeam(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	team, err := h.libraryRepo.GetTeam(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get team", err)
		return
	}
	writeJSON(w, http.StatusOK, team)
}

// PatchLibraryTeam handles PATCH /api/library/teams/{id}: rename, or replace
// the alias set. Automatic sides whose clan name is a new alias follow it.
func (h *Handlers) PatchLibraryTeam(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req libraryTeamRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid team JSON")
		return
	}
	req, err := req.clean()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	team, err := h.libraryRepo.UpdateTeam(r.Context(), id, library.TeamPatch{Name: req.Name, Aliases: req.Aliases})
	if err != nil {
		writeLibraryError(w, "update team", err)
		return
	}
	writeJSON(w, http.StatusOK, team)
}

type mergeLibraryTeamRequest struct {
	IntoTeamID uuid.UUID `json:"into_team_id"`
}

// MergeLibraryTeam handles POST /api/library/teams/{id}/merge: the team in the
// path is folded into into_team_id, which is returned.
func (h *Handlers) MergeLibraryTeam(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req mergeLibraryTeamRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil || req.IntoTeamID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "into_team_id must be a valid UUID")
		return
	}
	team, err := h.libraryRepo.MergeTeams(r.Context(), id, req.IntoTeamID)
	if err != nil {
		writeLibraryError(w, "merge teams", err)
		return
	}
	writeJSON(w, http.StatusOK, team)
}

// ListLibraryPlayers handles GET /api/library/players.
func (h *Handlers) ListLibraryPlayers(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	q := r.URL.Query()
	limit, offset, err := libraryPaging(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	teamID, err := optionalUUIDQuery(q, "team_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	players, err := h.libraryRepo.ListPlayers(r.Context(), library.PlayerFilter{Query: q.Get("q"), TeamID: teamID, Limit: limit, Offset: offset})
	if err != nil {
		internalError(w, "list library players", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"players": players})
}

func (h *Handlers) libraryPlayerParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	if !h.libraryReady(w) {
		return "", false
	}
	steamID := chi.URLParam(r, "steamid")
	if !validSteamID(steamID) {
		writeError(w, http.StatusBadRequest, "steamid must be a 64-bit unsigned integer")
		return "", false
	}
	return steamID, true
}

// GetLibraryPlayer handles GET /api/library/players/{steamid}, with the nick
// history and the teams by date.
func (h *Handlers) GetLibraryPlayer(w http.ResponseWriter, r *http.Request) {
	steamID, ok := h.libraryPlayerParam(w, r)
	if !ok {
		return
	}
	p, err := h.libraryRepo.GetPlayer(r.Context(), steamID)
	if err != nil {
		writeLibraryError(w, "get player", err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// LinkLibraryPlayerFaceit handles POST /api/library/players/{steamid}/faceit:
// it looks the SteamID64 up on FACEIT and stores the FACEIT id and nickname.
func (h *Handlers) LinkLibraryPlayerFaceit(w http.ResponseWriter, r *http.Request) {
	steamID, ok := h.libraryPlayerParam(w, r)
	if !ok {
		return
	}
	if _, err := h.libraryRepo.GetPlayer(r.Context(), steamID); err != nil {
		writeLibraryError(w, "get player", err)
		return
	}
	account, err := h.faceit.LookupBySteamID(r.Context(), steamID)
	switch {
	case errors.Is(err, faceit.ErrNotConfigured):
		writeError(w, http.StatusServiceUnavailable, "FACEIT is not configured")
		return
	case errors.Is(err, faceit.ErrPlayerNotFound):
		writeCodedError(w, http.StatusNotFound, "faceit_player_not_found", "this SteamID64 has no FACEIT CS2 account")
		return
	case err != nil:
		log.Printf("httpapi: FACEIT lookup for library player: %v", err)
		writeError(w, http.StatusBadGateway, "FACEIT lookup failed")
		return
	}
	p, err := h.libraryRepo.SetPlayerFaceit(r.Context(), steamID, account.ID, account.Nickname)
	if err != nil {
		writeLibraryError(w, "link player FACEIT", err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}
