package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/vodplays"
)

const (
	playScanIdle    = "idle"
	playScanRunning = "running"
	playScanDone    = "done"
	playScanFailed  = "failed"
)

// streamPlayScan is the state of the kill-feed scan of one stream source.
type streamPlayScan struct {
	Status    string          `json:"status"`
	Progress  float64         `json:"progress"`
	Plays     []vodplays.Play `json:"plays"`
	Error     string          `json:"error,omitempty"`
	ScannedAt *time.Time      `json:"scanned_at,omitempty"`
	// Voice is what the voice peak scan did; absent on a scan from before it existed.
	Voice *voiceScan `json:"voice,omitempty"`
}

// Voice scan outcomes.
const (
	voiceOff     = "off"
	voiceDone    = "done"
	voiceNoAudio = "no_audio"
	voiceFailed  = "failed"
)

type voiceScan struct {
	vodplays.VoiceSettings
	Status string `json:"status"`
	// Peaks is how many voice peaks the scan found, in plays or alone.
	Peaks int `json:"peaks"`
}

// startScanRequest is the optional body of POST /plays; a missing field keeps its default.
type startScanRequest struct {
	Voice            *bool `json:"voice"`
	VoiceSensitivity int   `json:"voice_sensitivity"`
}

func (r startScanRequest) settings() vodplays.VoiceSettings {
	s := vodplays.DefaultVoiceSettings()
	if r.Voice != nil {
		s.Enabled = *r.Voice
	}
	if r.VoiceSensitivity != 0 {
		s.Sensitivity = r.VoiceSensitivity
	}
	return s.Normalized()
}

// streamPlayScans holds the scans that are running or failed since start;
// a finished scan lives in the job's plays artifact instead.
type streamPlayScans struct {
	mu    sync.Mutex
	scans map[uuid.UUID]streamPlayScan
}

func (s *streamPlayScans) get(id uuid.UUID) (streamPlayScan, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	scan, ok := s.scans[id]
	return scan, ok
}

func (s *streamPlayScans) set(id uuid.UUID, scan streamPlayScan) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scans == nil {
		s.scans = map[uuid.UUID]streamPlayScan{}
	}
	s.scans[id] = scan
}

func (s *streamPlayScans) clear(id uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.scans, id)
}

// startIfIdle marks a scan as running unless one already is.
func (s *streamPlayScans) startIfIdle(id uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scans[id].Status == playScanRunning {
		return false
	}
	if s.scans == nil {
		s.scans = map[uuid.UUID]streamPlayScan{}
	}
	s.scans[id] = streamPlayScan{Status: playScanRunning, Plays: []vodplays.Play{}}
	return true
}

// WithStreamFFmpeg sets the FFmpeg for the stream play scan, summaries and upload remuxes.
func WithStreamFFmpeg(ffmpeg string) Option {
	return func(h *Handlers) {
		h.streamFFmpeg = ffmpeg
	}
}

// GetStreamPlays handles GET /api/stream-jobs/{id}/plays.
func (h *Handlers) GetStreamPlays(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadStreamJob(w, r)
	if !ok {
		return
	}
	scan, err := h.currentStreamPlayScan(j.ID)
	if err != nil {
		internalError(w, "read stream plays", err)
		return
	}
	writeJSON(w, http.StatusOK, scan)
}

// StartStreamPlayScan handles POST /api/stream-jobs/{id}/plays: it scans the
// source in the background and answers at once with the running state.
func (h *Handlers) StartStreamPlayScan(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadStreamJob(w, r)
	if !ok {
		return
	}
	if !streamSourceIsReady(j.Status) {
		writeError(w, http.StatusConflict, "el vídeo todavía no está listo")
		return
	}
	if h.streamFFmpeg == "" {
		writeError(w, http.StatusServiceUnavailable, "FFmpeg no está disponible")
		return
	}
	resolver, ok := h.storage.(storagePathResolver)
	if !ok {
		writeError(w, http.StatusNotImplemented, "stream play scan needs local storage")
		return
	}
	source, err := resolver.ResolvePath(j.SourcePath)
	if err != nil {
		internalError(w, "resolve stream source", err)
		return
	}
	var body startScanRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "el cuerpo no es un JSON válido")
		return
	}
	if h.playScans.startIfIdle(j.ID) {
		go h.runStreamPlayScan(streamPlayScanRun{id: j.ID, source: source, probe: j.Probe, voice: body.settings()})
	}
	scan, _ := h.playScans.get(j.ID)
	writeJSON(w, http.StatusAccepted, scan)
}

func streamSourceIsReady(status streamclips.Status) bool {
	switch status {
	case streamclips.StatusUploaded, streamclips.StatusReady, streamclips.StatusQueued, streamclips.StatusRendering, streamclips.StatusRendered:
		return true
	default:
		return false
	}
}

type streamPlayScanRun struct {
	id     uuid.UUID
	source string
	probe  streamclips.SourceProbe
	voice  vodplays.VoiceSettings
}

// scanVoice measures the streamer's voice and returns its peaks. A video
// without audio or an audio failure only costs the voice moments.
func (h *Handlers) scanVoice(run streamPlayScanRun) ([]vodplays.VoicePeak, voiceScan) {
	out := voiceScan{VoiceSettings: run.voice, Status: voiceDone}
	switch {
	case !run.voice.Enabled:
		out.Status = voiceOff
		return nil, out
	case run.probe.AudioCodec == "" && run.probe.AudioStreams == 0:
		out.Status = voiceNoAudio
		return nil, out
	}
	levels, err := vodplays.MeasureLoudness(context.Background(), vodplays.LoudnessOptions{
		FFmpeg: h.streamFFmpeg, Input: run.source, Track: vodplays.VoiceTrack(run.probe.AudioStreams),
	})
	if err != nil {
		log.Printf("httpapi: stream %s voice scan: %v", run.id, err)
		out.Status = voiceFailed
		return nil, out
	}
	peaks := vodplays.DetectVoicePeaks(levels, run.voice.Sensitivity)
	out.Peaks = len(peaks)
	return peaks, out
}

func (h *Handlers) runStreamPlayScan(run streamPlayScanRun) {
	id, source, duration := run.id, run.source, run.probe.DurationSeconds
	peaks, voice := h.scanVoice(run)
	lastReport := 0.0
	kills, err := vodplays.FindKills(context.Background(), vodplays.FindOptions{
		FFmpeg:          h.streamFFmpeg,
		Input:           source,
		DurationSeconds: duration,
		OnProgress: func(share float64) {
			if share-lastReport >= 0.01 {
				lastReport = share
				h.playScans.set(id, streamPlayScan{Status: playScanRunning, Progress: share, Plays: []vodplays.Play{}})
			}
		},
	})
	if err != nil {
		log.Printf("httpapi: stream %s play scan: %v", id, err)
		h.playScans.set(id, streamPlayScan{Status: playScanFailed, Plays: []vodplays.Play{}, Error: "No pudimos leer el vídeo para buscar jugadas."})
		return
	}
	now := time.Now().UTC()
	plays := vodplays.AddVoicePeaks(vodplays.GroupPlays(kills, duration), peaks, duration)
	if plays == nil {
		plays = []vodplays.Play{}
	}
	done := streamPlayScan{Status: playScanDone, Progress: 1, Plays: plays, ScannedAt: &now, Voice: &voice}
	if err := h.writeStreamPlays(id, done); err != nil {
		log.Printf("httpapi: stream %s write plays: %v", id, err)
		h.playScans.set(id, streamPlayScan{Status: playScanFailed, Plays: []vodplays.Play{}, Error: "No pudimos guardar las jugadas encontradas."})
		return
	}
	h.playScans.clear(id)
}

func (h *Handlers) writeStreamPlays(id uuid.UUID, scan streamPlayScan) error {
	// The job may have been deleted while the scan ran; do not recreate it.
	if _, err := h.streamRepo.Get(context.Background(), id); err != nil {
		return err
	}
	body, err := json.MarshalIndent(scan, "", "  ")
	if err != nil {
		return err
	}
	return h.storage.Put(streamclips.PlaysKey(id), bytes.NewReader(append(body, '\n')))
}

func (h *Handlers) currentStreamPlayScan(id uuid.UUID) (streamPlayScan, error) {
	if scan, ok := h.playScans.get(id); ok && scan.Status == playScanRunning {
		return scan, nil
	}
	rc, err := h.storage.Open(streamclips.PlaysKey(id))
	if err == nil {
		defer rc.Close()
		var scan streamPlayScan
		if err := json.NewDecoder(rc).Decode(&scan); err != nil {
			return streamPlayScan{}, err
		}
		return scan, nil
	}
	if !storageNotExist(err) {
		return streamPlayScan{}, err
	}
	if scan, ok := h.playScans.get(id); ok {
		return scan, nil
	}
	return streamPlayScan{Status: playScanIdle, Plays: []vodplays.Play{}}, nil
}
