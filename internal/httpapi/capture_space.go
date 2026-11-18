package httpapi

import (
	"math"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/rechedev9/cliphub/internal/diskfiles"
)

// Capture runs near 40 Mb/s (docs/incidents.md); a render of the same footage
// is assumed at half that.
const (
	captureBytesPerSecond = 40_000_000 / 8
	renderBytesPerSecond  = 20_000_000 / 8
	maxEstimateSeconds    = 24 * 60 * 60
)

type spaceEstimate struct {
	CaptureSeconds float64
	RenderSeconds  float64
}

// spaceCheck backs the low disk space warning before a capture or render; it never blocks one.
type spaceCheck struct {
	FreeBytes   uint64 `json:"free_bytes"`
	NeededBytes uint64 `json:"needed_bytes"`
	Short       bool   `json:"short"`
}

// checkSpace is nil when the free space is unknown: no warning beats a wrong one.
func checkSpace(work spaceEstimate, free func() (uint64, error)) *spaceCheck {
	available, err := free()
	if err != nil {
		return nil
	}
	needed := uint64(max(work.CaptureSeconds, 0)*captureBytesPerSecond + max(work.RenderSeconds, 0)*renderBytesPerSecond)
	return &spaceCheck{FreeBytes: available, NeededBytes: needed, Short: needed > available}
}

// dataDiskFree probes the disk that holds the job trees.
func (h *Handlers) dataDiskFree() (uint64, error) {
	resolver, ok := h.storage.(storagePathResolver)
	if !ok {
		return 0, errNoLocalDisk
	}
	jobs, err := resolver.ResolvePath("jobs")
	if err != nil {
		return 0, err
	}
	return diskfiles.FreeBytes(filepath.Dir(jobs))
}

// GetStorageSpace handles GET /api/storage/space?capture_seconds=&render_seconds=,
// the warning of the Short producer; space is null when the disk cannot be probed.
func (h *Handlers) GetStorageSpace(w http.ResponseWriter, r *http.Request) {
	var work spaceEstimate
	for name, into := range map[string]*float64{"capture_seconds": &work.CaptureSeconds, "render_seconds": &work.RenderSeconds} {
		raw := r.URL.Query().Get(name)
		if raw == "" {
			continue
		}
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || math.IsNaN(v) || v < 0 || v > maxEstimateSeconds {
			writeError(w, http.StatusBadRequest, name+" must be a number of seconds")
			return
		}
		*into = v
	}
	writeJSON(w, http.StatusOK, map[string]any{"space": checkSpace(work, h.dataDiskFree)})
}
