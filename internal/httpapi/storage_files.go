package httpapi

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/rechedev9/cliphub/internal/diskfiles"
)

// storageFileIndex remembers the last list: delete and reveal accept only a path
// it listed. The PC demo scan runs in the background; a whole drive takes seconds.
type storageFileIndex struct {
	mu        sync.Mutex
	files     map[string]diskfiles.File
	demos     []diskfiles.File
	scanning  bool
	scannedAt time.Time
}

// startScan launches a PC demo scan unless one is running.
func (idx *storageFileIndex) startScan(dataDir string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if idx.scanning {
		return
	}
	idx.scanning = true
	go idx.runScan(diskfiles.Roots(), dataDir)
}

func (idx *storageFileIndex) runScan(roots []string, dataDir string) {
	demos := diskfiles.ScanDemos(roots, []string{dataDir})
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.demos = demos
	idx.scanning = false
	idx.scannedAt = time.Now().UTC()
}

const studioDemoManagedCode = "studio_demo_managed"

type storageFileRequest struct {
	Path string `json:"path"`
}

// ListStorageFiles handles GET /api/storage/files: Studio's demo copies and
// rendered MP4s, plus the .dem files of the last PC scan. The first call starts that scan.
func (h *Handlers) ListStorageFiles(w http.ResponseWriter, r *http.Request) {
	dataDir, ok := h.storageDataDir(w)
	if !ok {
		return
	}
	studio, err := diskfiles.ScanStudio(dataDir)
	if err != nil {
		internalError(w, "scan studio files", err)
		return
	}
	studioDemos := []diskfiles.File{}
	videos := []diskfiles.File{}
	for _, file := range studio {
		if file.Kind == diskfiles.KindStudioDemo {
			studioDemos = append(studioDemos, file)
		} else {
			videos = append(videos, file)
		}
	}
	idx := &h.storageFiles
	idx.mu.Lock()
	neverScanned := !idx.scanning && idx.scannedAt.IsZero()
	idx.mu.Unlock()
	if neverScanned {
		idx.startScan(dataDir)
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	demos := append([]diskfiles.File{}, idx.demos...)
	listed := map[string]diskfiles.File{}
	for _, file := range append(demos, studio...) {
		listed[file.Path] = file
	}
	idx.files = listed
	var scannedAt *time.Time
	if !idx.scannedAt.IsZero() {
		scannedAt = &idx.scannedAt
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data_dir":     dataDir,
		"scanning":     idx.scanning,
		"scanned_at":   scannedAt,
		"demos":        demos,
		"studio_demos": studioDemos,
		"videos":       videos,
	})
}

// ScanStorageFiles handles POST /api/storage/files/scan: it searches the PC for
// demos again; 202 whether it started one or one was already running.
func (h *Handlers) ScanStorageFiles(w http.ResponseWriter, r *http.Request) {
	dataDir, ok := h.storageDataDir(w)
	if !ok {
		return
	}
	h.storageFiles.startScan(dataDir)
	w.WriteHeader(http.StatusAccepted)
}

// DeleteStorageFile handles POST /api/storage/files/delete {path}: it removes a
// listed demo or video for good. Studio's own demo copies go with their match.
func (h *Handlers) DeleteStorageFile(w http.ResponseWriter, r *http.Request) {
	file, ok := h.listedStorageFile(w, r)
	if !ok {
		return
	}
	if file.Kind == diskfiles.KindStudioDemo {
		writeCodedError(w, http.StatusConflict, studioDemoManagedCode, "Studio's demo copy is deleted with its match")
		return
	}
	paths := append([]string{file.Path}, diskfiles.Siblings(file.Path)...)
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			internalError(w, "delete storage file", err)
			return
		}
	}
	idx := &h.storageFiles
	idx.mu.Lock()
	delete(idx.files, file.Path)
	idx.demos = slices.DeleteFunc(idx.demos, func(demo diskfiles.File) bool { return demo.Path == file.Path })
	idx.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"freed_bytes": file.Size})
}

// RevealStorageFile handles POST /api/storage/files/reveal {path}: it opens File
// Explorer on a listed file.
func (h *Handlers) RevealStorageFile(w http.ResponseWriter, r *http.Request) {
	file, ok := h.listedStorageFile(w, r)
	if !ok {
		return
	}
	if err := diskfiles.Reveal(file.Path); err != nil {
		writeError(w, http.StatusNotImplemented, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) listedStorageFile(w http.ResponseWriter, r *http.Request) (diskfiles.File, bool) {
	var request storageFileRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&request); err != nil || request.Path == "" {
		writeError(w, http.StatusBadRequest, "body must be {\"path\": \"...\"}")
		return diskfiles.File{}, false
	}
	h.storageFiles.mu.Lock()
	file, ok := h.storageFiles.files[filepath.Clean(request.Path)]
	h.storageFiles.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "file is not in the storage list; refresh it")
		return diskfiles.File{}, false
	}
	return file, true
}

func (h *Handlers) storageDataDir(w http.ResponseWriter) (string, bool) {
	resolver, ok := h.storage.(storagePathResolver)
	if !ok {
		writeError(w, http.StatusNotImplemented, "storage backend does not report disk use")
		return "", false
	}
	jobs, err := resolver.ResolvePath("jobs")
	if err != nil {
		internalError(w, "resolve data dir", err)
		return "", false
	}
	return filepath.Dir(jobs), true
}
