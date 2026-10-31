package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// codeJobDemoMissing is the error code of a job whose demo file is gone, so
// the library has nothing to keep.
const codeJobDemoMissing = "job_demo_missing"

// unlinkedJobScanLimit is how many recent jobs the library looks at for demos
// that came in through Crear before the library existed.
const unlinkedJobScanLimit = 100

var errJobDemoMissing = errors.New("the job's demo is no longer stored")

// storageLinker is the storage capability that gives a stored file a second
// key without copying it (storage.Local.Link).
type storageLinker interface {
	Link(srcKey, dstKey string) error
}

// placeLibraryDemo stores the demo of a job under a library key. A hard link
// costs no disk; a storage without links gets a copy.
func (h *Handlers) placeLibraryDemo(srcKey, dstKey string) error {
	if linker, ok := h.storage.(storageLinker); ok {
		return linker.Link(srcKey, dstKey)
	}
	src, err := h.storage.Open(srcKey)
	if err != nil {
		return err
	}
	defer src.Close()
	return h.storage.Put(dstKey, src)
}

func (h *Handlers) storedSize(key string) int64 {
	resolver, ok := h.storage.(storagePathResolver)
	if !ok {
		return 0
	}
	path, err := resolver.ResolvePath(key)
	if err != nil {
		return 0
	}
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// registerJobDemo puts the demo of a Crear job in the library and links the
// job to it. A demo already in the library (same hash) is linked as is; a new
// one gets its own copy of the file, so deleting the job later does not take
// the library's demo with it, and is queued for its scan.
func (h *Handlers) registerJobDemo(ctx context.Context, j job.Job) (library.Demo, error) {
	if d, err := h.libraryRepo.DemoForJob(ctx, j.ID); err == nil {
		return d, nil
	} else if !errors.Is(err, library.ErrNotFound) {
		return library.Demo{}, err
	}
	if j.DemoSHA256 == "" || j.DemoPath == "" {
		return library.Demo{}, errJobDemoMissing
	}
	if exists, err := h.storage.Exists(j.DemoPath); err != nil || !exists {
		return library.Demo{}, errJobDemoMissing
	}
	name := j.DemoFileName
	if name == "" {
		name = j.ID.String() + ".dem"
	}
	id := uuid.New()
	key := artifacts.LibraryDemoKey(id)
	demo := library.Demo{
		ID:        id,
		SHA256:    j.DemoSHA256,
		FileName:  name,
		Path:      key,
		SizeBytes: h.storedSize(j.DemoPath),
		Source:    library.DetectSource("", name),
	}
	created, err := h.libraryRepo.CreateDemo(ctx, &demo)
	if err != nil {
		return library.Demo{}, err
	}
	if created {
		if err := h.placeLibraryDemo(j.DemoPath, key); err != nil {
			if delErr := h.libraryRepo.DeleteDemo(ctx, demo.ID); delErr != nil {
				log.Printf("httpapi: drop library demo %s after a failed store: %v", demo.ID, delErr)
			}
			h.discardLibraryDemo(demo.ID)
			return library.Demo{}, fmt.Errorf("store library copy of job %s: %w", j.ID, err)
		}
	}
	if err := h.libraryRepo.LinkDemoJob(ctx, demo.ID, j.ID); err != nil {
		return library.Demo{}, err
	}
	if created {
		if err := h.enqueueLibraryScan(ctx, demo.ID); err != nil {
			log.Printf("httpapi: %v", err)
		}
	}
	return h.libraryRepo.GetDemo(ctx, demo.ID)
}

// linkJobToLibrary is the Crear side of the library: every demo a job admits
// also lands in Partidas. It never fails the job; a miss is logged and the
// job can be linked later from the library.
func (h *Handlers) linkJobToLibrary(ctx context.Context, j job.Job) {
	if h.libraryRepo == nil {
		return
	}
	if _, err := h.registerJobDemo(ctx, j); err != nil {
		log.Printf("httpapi: add the demo of job %s to the library: %v", j.ID, err)
	}
}

// unlinkedJobs returns the recent jobs whose stored demo is not in the library
// yet, newest first.
func (h *Handlers) unlinkedJobs(ctx context.Context) ([]job.Job, error) {
	jobs, err := h.repo.List(ctx, unlinkedJobScanLimit)
	if err != nil {
		return nil, err
	}
	out := []job.Job{}
	for _, j := range jobs {
		if j.DemoSHA256 == "" || j.DemoPath == "" {
			continue
		}
		if _, err := h.libraryRepo.DemoForJob(ctx, j.ID); err == nil {
			continue
		} else if !errors.Is(err, library.ErrNotFound) {
			return nil, err
		}
		if exists, err := h.storage.Exists(j.DemoPath); err != nil || !exists {
			continue
		}
		out = append(out, j)
	}
	return out, nil
}

// ListUnlinkedLibraryJobs handles GET /api/library/jobs: the ids of the recent
// Crear jobs whose demo is not in the library yet.
func (h *Handlers) ListUnlinkedLibraryJobs(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	jobs, err := h.unlinkedJobs(r.Context())
	if err != nil {
		internalError(w, "list unlinked jobs", err)
		return
	}
	ids := make([]uuid.UUID, len(jobs))
	for i, j := range jobs {
		ids[i] = j.ID
	}
	writeJSON(w, http.StatusOK, map[string]any{"unlinked_job_ids": ids})
}

// LinkUnlinkedLibraryJobs handles POST /api/library/jobs: every unlinked job
// joins the library. Each new demo is queued for its scan.
func (h *Handlers) LinkUnlinkedLibraryJobs(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	jobs, err := h.unlinkedJobs(r.Context())
	if err != nil {
		internalError(w, "list unlinked jobs", err)
		return
	}
	demos := []library.Demo{}
	seen := map[uuid.UUID]bool{}
	for _, j := range jobs {
		d, err := h.registerJobDemo(r.Context(), j)
		if errors.Is(err, errJobDemoMissing) {
			continue
		}
		if err != nil {
			internalError(w, "link job demo", err)
			return
		}
		if !seen[d.ID] {
			seen[d.ID] = true
			demos = append(demos, d)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"demos": demos})
}

func (h *Handlers) libraryJobParam(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	if !h.libraryReady(w) {
		return uuid.Nil, false
	}
	return libraryUUIDParam(w, r, "jobId")
}

// GetLibraryJobDemo handles GET /api/library/jobs/{jobId}: the library demo a
// Crear job was made from, so a job link can open its partida.
func (h *Handlers) GetLibraryJobDemo(w http.ResponseWriter, r *http.Request) {
	jobID, ok := h.libraryJobParam(w, r)
	if !ok {
		return
	}
	d, err := h.libraryRepo.DemoForJob(r.Context(), jobID)
	if err != nil {
		writeLibraryError(w, "get job demo", err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// LinkLibraryJobDemo handles POST /api/library/jobs/{jobId}: it adds that
// job's demo to the library, or returns the demo it is already linked to.
func (h *Handlers) LinkLibraryJobDemo(w http.ResponseWriter, r *http.Request) {
	jobID, ok := h.libraryJobParam(w, r)
	if !ok {
		return
	}
	j, err := h.repo.Get(r.Context(), jobID)
	if errors.Is(err, job.ErrNotFound) {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		internalError(w, "get job", err)
		return
	}
	d, err := h.registerJobDemo(r.Context(), j)
	if errors.Is(err, errJobDemoMissing) {
		writeCodedError(w, http.StatusConflict, codeJobDemoMissing, err.Error())
		return
	}
	if err != nil {
		internalError(w, "link job demo", err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// createLibraryJobRequest is the optional body of POST
// /api/library/demos/{id}/jobs. A target skips the scan:roster task and parses
// that player's POV right away, as the analysis area's video actions need; the
// parse scans the roster inline first, since the Full Demo render reads it.
type createLibraryJobRequest struct {
	TargetSteamID string `json:"target_steamid"`
}

// reusableLibraryJob returns a linked job that already parses or parsed the
// target and has not made a video yet, so a second click does not parse the
// demo again. A job that recorded keeps its videos; the next video gets a new job.
func (h *Handlers) reusableLibraryJob(ctx context.Context, d library.Demo, target string) (job.Job, bool) {
	for i := len(d.JobIDs) - 1; i >= 0; i-- {
		j, err := h.repo.Get(ctx, d.JobIDs[i])
		if err != nil || j.TargetSteamID != target {
			continue
		}
		switch j.Status {
		case job.StatusQueued, job.StatusParsing, job.StatusParsed:
			if exists, err := h.storage.Exists(j.DemoPath); err == nil && exists {
				return j, true
			}
		}
	}
	return job.Job{}, false
}

// CreateJobFromLibraryDemo handles POST /api/library/demos/{id}/jobs: a Crear
// job over a demo already in the library, so a partida becomes a Short or a
// long video without loading the file again. The job's demo is a hard link
// of the library copy. Without a target the job starts with the roster scan.
func (h *Handlers) CreateJobFromLibraryDemo(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req createLibraryJobRequest
	if err := decodeSingleJSONBody(w, r, &req, false); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid job JSON")
		return
	}
	if req.TargetSteamID != "" && !validSteamID(req.TargetSteamID) {
		writeError(w, http.StatusBadRequest, "target_steamid must be a 64-bit unsigned integer")
		return
	}
	d, err := h.libraryRepo.GetDemo(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get demo", err)
		return
	}
	if req.TargetSteamID != "" {
		if j, ok := h.reusableLibraryJob(r.Context(), d, req.TargetSteamID); ok {
			writeJSON(w, http.StatusOK, map[string]any{"job_id": j.ID, "status": j.Status, "reused": true})
			return
		}
	}
	if exists, err := h.storage.Exists(d.Path); err != nil || !exists {
		writeCodedError(w, http.StatusConflict, codeJobDemoMissing, "the library demo file is no longer stored")
		return
	}
	j := newDemoJob(d.FileName, req.TargetSteamID, "", rules.Default())
	if err := h.placeLibraryDemo(d.Path, j.DemoPath); err != nil {
		internalError(w, "store job demo", err)
		return
	}
	j.DemoSHA256 = d.SHA256
	created, err := h.admitDemoJob(r.Context(), j)
	if err != nil {
		internalError(w, "create job from library demo", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"job_id": created.ID, "status": created.Status, "reused": false})
}

// roleMatchLimit is how many of a player's most recent matches the role is
// read from.
const roleMatchLimit = library.MaxListLimit

// GetLibraryPlayerRole handles GET /api/library/players/{steamid}/role: the
// role read from the player's tactical documents, with the counts and the
// rounds behind it.
func (h *Handlers) GetLibraryPlayerRole(w http.ResponseWriter, r *http.Request) {
	steamID, ok := h.libraryPlayerParam(w, r)
	if !ok {
		return
	}
	if _, err := h.libraryRepo.GetPlayer(r.Context(), steamID); err != nil {
		writeLibraryError(w, "get player", err)
		return
	}
	matches, err := h.libraryRepo.ListMatches(r.Context(), library.MatchFilter{SteamID: steamID, Limit: roleMatchLimit})
	if err != nil {
		internalError(w, "list player matches", err)
		return
	}
	docs := make([]library.RoleDocument, 0, len(matches))
	for _, m := range matches {
		doc, err := h.readLibraryDocument(m.DemoID)
		if err != nil {
			// A match whose document is gone or unreadable is left out, not guessed.
			log.Printf("httpapi: role of %s skips match %s: %v", steamID, m.ID, err)
			continue
		}
		docs = append(docs, library.RoleDocument{MatchID: m.ID, Document: doc})
	}
	writeJSON(w, http.StatusOK, library.InferPlayerRole(steamID, docs))
}

func (h *Handlers) readLibraryDocument(demoID uuid.UUID) (tacticalplan.Document, error) {
	rc, err := h.storage.Open(artifacts.LibraryTacticalIndexKey(demoID))
	if err != nil {
		return tacticalplan.Document{}, err
	}
	defer rc.Close()
	var doc tacticalplan.Document
	if err := json.NewDecoder(rc).Decode(&doc); err != nil {
		return tacticalplan.Document{}, fmt.Errorf("decode tactical document: %w", err)
	}
	doc.DropRepeatedUtility()
	return doc, nil
}
