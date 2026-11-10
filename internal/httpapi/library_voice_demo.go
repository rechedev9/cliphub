package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/voicecomms"
)

// DemoVoiceJob extracts the in-game voice of Target's team from a demo into Dir.
type DemoVoiceJob struct {
	Demo   string
	Target string
	Dir    string
}

// DemoVoiceExtractor writes one Ogg per speaking player, starting at demo time 0; none is no voice.
type DemoVoiceExtractor interface {
	ExtractDemoVoice(ctx context.Context, job DemoVoiceJob) ([]voicecomms.Track, error)
}

// Demo voice states.
const (
	demoVoiceNone       = "none"
	demoVoiceExtracting = "extracting"
	demoVoiceReady      = "ready"
	demoVoiceFailed     = "failed"
	demoVoiceUnreadable = "unavailable"
)

// demoVoiceTrack is one player's in-game voice; its audio plays at demo time = audio time.
type demoVoiceTrack struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name,omitempty"`
	TeamKey   string `json:"team_key"`
	Packets   int    `json:"packets"`
	File      string `json:"-"`
}

type demoVoiceStatus struct {
	MatchID uuid.UUID        `json:"match_id"`
	State   string           `json:"state"`
	Tracks  []demoVoiceTrack `json:"tracks"`
}

// demoVoiceIndex is what an extraction leaves on disk; File is relative to the demo's folder.
type demoVoiceIndex struct {
	Tracks []demoVoiceTrack `json:"tracks"`
}

type demoVoiceFile struct {
	demoVoiceTrack
	File string `json:"file"`
}

const demoVoiceIndexName = "demo-voice.json"

func (v *voiceService) demoVoiceDir(demoID uuid.UUID) string {
	return filepath.Join(v.deps.Dir, "demo", demoID.String())
}

// localPathResolver is a storage whose keys are files on this PC.
type localPathResolver interface {
	ResolvePath(key string) (string, error)
}

// matchDemoPath is the match's demo file on this PC; a shared demo or one in remote storage has none.
func (h *Handlers) matchDemoPath(ctx context.Context, m library.Match) (string, bool) {
	resolver, ok := h.storage.(localPathResolver)
	if !ok {
		return "", false
	}
	d, err := h.libraryRepo.GetDemo(ctx, m.DemoID)
	if err != nil || d.Source == library.SourceShared {
		return "", false
	}
	p, err := resolver.ResolvePath(d.Path)
	if err != nil {
		return "", false
	}
	if _, err := os.Stat(p); err != nil {
		return "", false
	}
	return p, true
}

func (h *Handlers) readDemoVoice(demoID uuid.UUID) (demoVoiceIndex, bool) {
	body, err := os.ReadFile(filepath.Join(h.voice.demoVoiceDir(demoID), demoVoiceIndexName))
	if err != nil {
		return demoVoiceIndex{}, false
	}
	var files struct {
		Tracks []demoVoiceFile `json:"tracks"`
	}
	if err := json.Unmarshal(body, &files); err != nil {
		return demoVoiceIndex{}, false
	}
	out := demoVoiceIndex{Tracks: []demoVoiceTrack{}}
	for _, f := range files.Tracks {
		t := f.demoVoiceTrack
		t.File = f.File
		out.Tracks = append(out.Tracks, t)
	}
	return out, true
}

func (h *Handlers) demoVoiceStatusOf(ctx context.Context, m library.Match) demoVoiceStatus {
	out := demoVoiceStatus{MatchID: m.ID, State: demoVoiceNone, Tracks: []demoVoiceTrack{}}
	h.voice.mu.Lock()
	job := h.voice.demoJobs[m.DemoID]
	h.voice.mu.Unlock()
	switch {
	case job != "":
		out.State = job
	case h.voice.deps.DemoVoice == nil:
		out.State = demoVoiceUnreadable
	default:
		if idx, ok := h.readDemoVoice(m.DemoID); ok {
			out.State, out.Tracks = demoVoiceReady, idx.Tracks
		} else if _, ok := h.matchDemoPath(ctx, m); !ok {
			out.State = demoVoiceUnreadable
		}
	}
	return out
}

// GetLibraryMatchDemoVoice handles GET /api/library/matches/{id}/voice/demo: the demo's own voice.
func (h *Handlers) GetLibraryMatchDemoVoice(w http.ResponseWriter, r *http.Request) {
	m, ok := h.voiceMatch(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.demoVoiceStatusOf(r.Context(), m))
}

// ExtractLibraryMatchDemoVoice handles POST /api/library/matches/{id}/voice/demo: reads the
// in-game voice of both teams from the demo in the background, once per demo.
func (h *Handlers) ExtractLibraryMatchDemoVoice(w http.ResponseWriter, r *http.Request) {
	m, ok := h.voiceMatch(w, r)
	if !ok {
		return
	}
	status := h.demoVoiceStatusOf(r.Context(), m)
	if status.State == demoVoiceReady || status.State == demoVoiceExtracting {
		writeJSON(w, http.StatusOK, status)
		return
	}
	demoPath, ok := h.matchDemoPath(r.Context(), m)
	if !ok || h.voice.deps.DemoVoice == nil || len(m.Players) == 0 {
		writeCodedError(w, http.StatusConflict, codeNoDemoFile, "the demo file of this match is not on this PC")
		return
	}
	h.voice.mu.Lock()
	if h.voice.demoJobs[m.DemoID] == demoVoiceExtracting {
		h.voice.mu.Unlock()
		writeJSON(w, http.StatusAccepted, h.demoVoiceStatusOf(r.Context(), m))
		return
	}
	h.voice.demoJobs[m.DemoID] = demoVoiceExtracting
	h.voice.mu.Unlock()
	go h.extractDemoVoice(h.voice.lifetime(), demoVoiceSource{match: m, path: demoPath})
	writeJSON(w, http.StatusAccepted, h.demoVoiceStatusOf(r.Context(), m))
}

type demoVoiceSource struct {
	match library.Match
	path  string
}

// extractDemoVoice extracts each team's voice at once and writes the index only when both are done.
func (h *Handlers) extractDemoVoice(ctx context.Context, src demoVoiceSource) {
	m := src.match
	started := time.Now()
	dir := h.voice.demoVoiceDir(m.DemoID)
	targets := map[string]string{}
	for _, p := range m.Players {
		if p.TeamKey != "" && p.SteamID64 != "" && targets[p.TeamKey] == "" {
			targets[p.TeamKey] = p.SteamID64
		}
	}
	keys := make([]string, 0, len(targets))
	for k := range targets {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	found := make([][]voicecomms.Track, len(keys))
	errs := make([]error, len(keys))
	var wg sync.WaitGroup
	for i, key := range keys {
		wg.Go(func() {
			found[i], errs[i] = h.voice.deps.DemoVoice.ExtractDemoVoice(ctx, DemoVoiceJob{Demo: src.path, Target: targets[key], Dir: filepath.Join(dir, key)})
		})
	}
	wg.Wait()
	state := ""
	if err := errors.Join(errs...); err != nil {
		log.Printf("httpapi: extract the in-game voice of match %s: %v", m.ID, err)
		state = demoVoiceFailed
	} else if err := writeDemoVoiceIndex(dir, keys, found); err != nil {
		log.Printf("httpapi: store the in-game voice of match %s: %v", m.ID, err)
		state = demoVoiceFailed
	} else {
		log.Printf("httpapi: in-game voice of match %s: %d players in %s", m.ID, countTracks(found), time.Since(started).Round(time.Second))
	}
	h.voice.mu.Lock()
	defer h.voice.mu.Unlock()
	if state == "" || ctx.Err() != nil {
		delete(h.voice.demoJobs, m.DemoID)
		return
	}
	h.voice.demoJobs[m.DemoID] = state
}

func countTracks(found [][]voicecomms.Track) int {
	n := 0
	for _, f := range found {
		n += len(f)
	}
	return n
}

func writeDemoVoiceIndex(dir string, keys []string, found [][]voicecomms.Track) error {
	files := []demoVoiceFile{}
	for i, key := range keys {
		for _, t := range found[i] {
			rel, err := filepath.Rel(dir, t.Path)
			if err != nil || !filepath.IsLocal(rel) {
				return errors.New("an extracted voice track left its folder")
			}
			files = append(files, demoVoiceFile{demoVoiceTrack: demoVoiceTrack{SteamID64: t.SteamID64, Name: t.Name, TeamKey: key, Packets: t.Packets},
				File: filepath.ToSlash(rel)})
		}
	}
	body, err := json.Marshal(struct {
		Tracks []demoVoiceFile `json:"tracks"`
	}{files})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, demoVoiceIndexName), body, 0o600)
}

// GetLibraryMatchDemoVoiceAudio handles GET /api/library/matches/{id}/voice/demo/{steamid}/audio.
func (h *Handlers) GetLibraryMatchDemoVoiceAudio(w http.ResponseWriter, r *http.Request) {
	m, ok := h.voiceMatch(w, r)
	if !ok {
		return
	}
	steamID := chi.URLParam(r, "steamid")
	if _, err := strconv.ParseUint(steamID, 10, 64); err != nil {
		writeError(w, http.StatusBadRequest, "steamid must be a SteamID64")
		return
	}
	idx, _ := h.readDemoVoice(m.DemoID)
	for _, t := range idx.Tracks {
		if t.SteamID64 != steamID || !filepath.IsLocal(filepath.FromSlash(t.File)) {
			continue
		}
		f, err := os.Open(filepath.Join(h.voice.demoVoiceDir(m.DemoID), filepath.FromSlash(t.File)))
		if err != nil {
			break
		}
		defer f.Close()
		w.Header().Set("Content-Type", "audio/ogg")
		w.Header().Set("Cache-Control", "private, max-age=3600")
		http.ServeContent(w, r, "", time.Time{}, f)
		return
	}
	writeLibraryError(w, "open demo voice track", library.ErrNotFound)
}
