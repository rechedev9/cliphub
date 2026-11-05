package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/vodplays"
)

// streamSummary is the state of the landscape summary of a VOD's plays.
type streamSummary struct {
	Status          string     `json:"status"`
	Progress        float64    `json:"progress"`
	PlayCount       int        `json:"play_count,omitempty"`
	DurationSeconds float64    `json:"duration_seconds,omitempty"`
	Error           string     `json:"error,omitempty"`
	RenderedAt      *time.Time `json:"rendered_at,omitempty"`
}

// streamSummaries holds the summaries that are running or failed since
// start; a finished one lives in the job's summary artifacts instead.
type streamSummaries struct {
	mu    sync.Mutex
	items map[uuid.UUID]streamSummary
}

func (s *streamSummaries) get(id uuid.UUID) (streamSummary, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	return item, ok
}

func (s *streamSummaries) set(id uuid.UUID, item streamSummary) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.items == nil {
		s.items = map[uuid.UUID]streamSummary{}
	}
	s.items[id] = item
}

func (s *streamSummaries) clear(id uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, id)
}

func (s *streamSummaries) startIfIdle(id uuid.UUID, playCount int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.items[id].Status == playScanRunning {
		return false
	}
	if s.items == nil {
		s.items = map[uuid.UUID]streamSummary{}
	}
	s.items[id] = streamSummary{Status: playScanRunning, PlayCount: playCount}
	return true
}

// GetStreamSummary handles GET /api/stream-jobs/{id}/summary.
func (h *Handlers) GetStreamSummary(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadStreamJob(w, r)
	if !ok {
		return
	}
	summary, err := h.currentStreamSummary(j.ID)
	if err != nil {
		internalError(w, "read stream summary", err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// StartStreamSummary handles POST /api/stream-jobs/{id}/summary: it renders
// every detected play into one landscape video in the background.
func (h *Handlers) StartStreamSummary(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadStreamJob(w, r)
	if !ok {
		return
	}
	if !streamSourceIsReady(j.Status) {
		writeError(w, http.StatusConflict, "el vídeo todavía no está listo")
		return
	}
	if h.playScanFFmpeg == "" {
		writeError(w, http.StatusServiceUnavailable, "FFmpeg no está disponible")
		return
	}
	scan, err := h.currentStreamPlayScan(j.ID)
	if err != nil {
		internalError(w, "read stream plays", err)
		return
	}
	if scan.Status != playScanDone || len(scan.Plays) == 0 {
		writeError(w, http.StatusConflict, "busca las jugadas antes de crear el resumen")
		return
	}
	resolver, ok := h.storage.(storagePathResolver)
	if !ok {
		writeError(w, http.StatusNotImplemented, "stream summary needs local storage")
		return
	}
	source, err := resolver.ResolvePath(j.SourcePath)
	if err != nil {
		internalError(w, "resolve stream source", err)
		return
	}
	if h.summaries.startIfIdle(j.ID, len(scan.Plays)) {
		go h.runStreamSummary(j.ID, source, scan.Plays, j.Probe.AudioCodec != "")
	}
	summary, _ := h.summaries.get(j.ID)
	writeJSON(w, http.StatusAccepted, summary)
}

// GetStreamSummaryVideo handles GET /api/stream-jobs/{id}/summary/video.
func (h *Handlers) GetStreamSummaryVideo(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadStreamJob(w, r)
	if !ok {
		return
	}
	rc, err := h.storage.Open(streamclips.SummaryKey(j.ID))
	if err != nil {
		if storageNotExist(err) {
			writeError(w, http.StatusNotFound, "stream summary not found")
			return
		}
		internalError(w, "open stream summary", err)
		return
	}
	serveArtifact(w, r, "video/mp4", rc)
}

func (h *Handlers) runStreamSummary(id uuid.UUID, source string, plays []vodplays.Play, hasAudio bool) {
	fail := func(stage string, err error, message string) {
		log.Printf("httpapi: stream %s summary %s: %v", id, stage, err)
		h.summaries.set(id, streamSummary{Status: playScanFailed, PlayCount: len(plays), Error: message})
	}
	workDir, err := os.MkdirTemp("", "zv-stream-summary-*")
	if err != nil {
		fail("work dir", err, "No pudimos preparar el resumen.")
		return
	}
	defer func() { _ = os.RemoveAll(workDir) }()
	output := filepath.Join(workDir, "summary.mp4")
	lastReport := 0.0
	err = vodplays.RenderSummary(context.Background(), vodplays.SummaryOptions{
		FFmpeg:   h.playScanFFmpeg,
		Input:    source,
		Output:   output,
		WorkDir:  workDir,
		Plays:    plays,
		HasAudio: hasAudio,
		OnProgress: func(share float64) {
			if share-lastReport >= 0.01 {
				lastReport = share
				h.summaries.set(id, streamSummary{Status: playScanRunning, Progress: share, PlayCount: len(plays)})
			}
		},
	})
	if err != nil {
		fail("render", err, "No pudimos crear el resumen.")
		return
	}
	if err := h.publishStreamSummary(id, output, plays); err != nil {
		fail("publish", err, "No pudimos guardar el resumen.")
		return
	}
	h.summaries.clear(id)
}

func (h *Handlers) publishStreamSummary(id uuid.UUID, output string, plays []vodplays.Play) error {
	// The job may have been deleted while the summary rendered; do not recreate it.
	if _, err := h.streamRepo.Get(context.Background(), id); err != nil {
		return err
	}
	f, err := os.Open(output)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := h.storage.Put(streamclips.SummaryKey(id), f); err != nil {
		return err
	}
	duration := 0.0
	for _, p := range plays {
		duration += p.EndSeconds - p.StartSeconds
	}
	now := time.Now().UTC()
	body, err := json.MarshalIndent(streamSummary{
		Status: playScanDone, Progress: 1, PlayCount: len(plays), DurationSeconds: duration, RenderedAt: &now,
	}, "", "  ")
	if err != nil {
		return err
	}
	return h.storage.Put(streamclips.SummaryStateKey(id), bytes.NewReader(append(body, '\n')))
}

func (h *Handlers) currentStreamSummary(id uuid.UUID) (streamSummary, error) {
	// Memory holds only a running or failed render, and either is newer than
	// the stored summary.
	if item, ok := h.summaries.get(id); ok {
		return item, nil
	}
	rc, err := h.storage.Open(streamclips.SummaryStateKey(id))
	if err == nil {
		defer rc.Close()
		var item streamSummary
		if err := json.NewDecoder(rc).Decode(&item); err != nil {
			return streamSummary{}, err
		}
		return item, nil
	}
	if !storageNotExist(err) {
		return streamSummary{}, err
	}
	return streamSummary{Status: playScanIdle}, nil
}
