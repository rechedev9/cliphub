package vodfetch

import (
	"bytes"
	"math"
	"strconv"
	"strings"
	"sync"
)

// progressPrefix marks the lines yt-dlp prints through progressTemplate, so
// they never mix with the title Download reads from the last stdout line.
const progressPrefix = "cliphub-progress "

// progressTemplate prints one line per update: the format's vcodec ("none"
// for an audio-only track), bytes so far, exact total, estimated total,
// fragment index and count (HLS) and the ETA in seconds. Missing fields are NA.
const progressTemplate = "download:" + progressPrefix +
	"%(info.vcodec)s %(progress.downloaded_bytes)s %(progress.total_bytes)s " +
	"%(progress.total_bytes_estimate)s %(progress.fragment_index)s " +
	"%(progress.fragment_count)s %(progress.eta)s"

// Progress is one download update for the track yt-dlp is fetching now. A
// YouTube source downloads video and then audio, each from zero.
type Progress struct {
	AudioOnly       bool  `json:"audio_only,omitempty"`
	DownloadedBytes int64 `json:"downloaded_bytes"`
	// TotalBytes is 0 when unknown; TotalEstimated marks yt-dlp's estimate.
	TotalBytes     int64 `json:"total_bytes,omitempty"`
	TotalEstimated bool  `json:"total_estimated,omitempty"`
	// Percent is nil when neither a byte total nor a fragment count is known,
	// as on a live stream, so the UI keeps an indeterminate bar.
	Percent    *float64 `json:"percent,omitempty"`
	ETASeconds int      `json:"eta_seconds,omitempty"`
}

// ParseProgressLine reads one progressTemplate line. Other lines return false.
func ParseProgressLine(line string) (Progress, bool) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(line), progressPrefix)
	if !ok {
		return Progress{}, false
	}
	fields := strings.Fields(rest)
	if len(fields) != 7 {
		return Progress{}, false
	}
	downloaded, ok := parseCount(fields[1])
	if !ok {
		return Progress{}, false
	}
	p := Progress{AudioOnly: fields[0] == "none", DownloadedBytes: downloaded}
	if total, ok := parseCount(fields[2]); ok && total > 0 {
		p.TotalBytes = total
	} else if estimate, ok := parseCount(fields[3]); ok && estimate > 0 {
		p.TotalBytes, p.TotalEstimated = estimate, true
	}
	fragment, fragOK := parseCount(fields[4])
	fragments, countOK := parseCount(fields[5])
	// HLS sizes are estimates that drift; the fragment count is exact.
	switch {
	case p.TotalBytes > 0 && !p.TotalEstimated:
		p.Percent = percentOf(float64(p.DownloadedBytes), float64(p.TotalBytes))
	case fragOK && countOK && fragments > 0:
		p.Percent = percentOf(float64(fragment), float64(fragments))
	}
	if eta, ok := parseCount(fields[6]); ok {
		p.ETASeconds = int(min(eta, math.MaxInt32))
	}
	return p, true
}

// parseCount reads a non-negative yt-dlp number; templates can print floats.
func parseCount(s string) (int64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return int64(v), true
}

func percentOf(part, whole float64) *float64 {
	v := math.Min(100, math.Max(0, part/whole*100))
	return &v
}

// lineWriter buffers command output and calls onLine for each complete line.
type lineWriter struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	pending []byte
	onLine  func(string)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(p)
	w.pending = append(w.pending, p...)
	for {
		i := bytes.IndexByte(w.pending, '\n')
		if i < 0 {
			return len(p), nil
		}
		w.onLine(strings.TrimRight(string(w.pending[:i]), "\r"))
		w.pending = w.pending[i+1:]
	}
}

func (w *lineWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}
