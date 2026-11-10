package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/teamvoice"
	"github.com/rechedev9/cliphub/internal/voicecomms"
)

// maxVoiceUploadBytes is one import request: 2 GiB of audio plus 1 MiB of multipart framing.
const maxVoiceUploadBytes = 2<<30 + 1<<20

// voiceImportResult is the outcome of one uploaded file.
type voiceImportResult struct {
	FileName  string `json:"file_name"`
	Status    string `json:"status"`
	Tracks    int    `json:"tracks"`
	ErrorCode string `json:"error_code,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ImportLibraryMatchVoice handles POST /api/library/matches/{id}/voice: "audio" parts, one track per
// channel past two. The first import of a match places the recording on the demo on its own.
func (h *Handlers) ImportLibraryMatchVoice(w http.ResponseWriter, r *http.Request) {
	m, ok := h.voiceMatch(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxVoiceUploadBytes)
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "body must be multipart/form-data")
		return
	}
	existing, err := h.libraryRepo.GetMatchVoice(r.Context(), m.ID)
	isNew := errors.Is(err, library.ErrNotFound)
	if err != nil && !isNew {
		internalError(w, "get match voice", err)
		return
	}
	room := library.MaxVoiceTracks - len(existing.Tracks)
	results := []voiceImportResult{}
	var tracks []library.VoiceTrack
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			h.discardVoiceTracks(tracks)
			writeError(w, http.StatusBadRequest, "reading multipart body: "+err.Error())
			return
		}
		if part.FormName() != "audio" {
			_ = part.Close()
			continue
		}
		res, made, err := h.importVoiceFile(r.Context(), voiceUpload{matchID: m.ID, src: part, name: part.FileName(), room: room - len(tracks)})
		_ = part.Close()
		if err != nil {
			h.discardVoiceTracks(append(tracks, made...))
			internalError(w, "import voice file", err)
			return
		}
		if res.Status != libraryImportImported {
			log.Printf("httpapi: refuse voice file %q of match %s code=%s: %s", res.FileName, m.ID, res.ErrorCode, res.Error)
		}
		tracks = append(tracks, made...)
		results = append(results, res)
	}
	if len(tracks) == 0 {
		code, msg := codeNotAudio, "no audio file in the request"
		if len(results) > 0 {
			code, msg = results[0].ErrorCode, results[0].Error
		}
		writeCodedError(w, http.StatusBadRequest, code, msg)
		return
	}
	add := library.VoiceAdd{MatchID: m.ID, Tracks: tracks}
	if isNew {
		res, err := h.estimateVoiceSync(r.Context(), voiceSyncSubject{match: m, tracks: tracks})
		if err != nil {
			h.discardVoiceTracks(tracks)
			internalError(w, "estimate voice sync", err)
			return
		}
		add.Sync = res.Sync
	}
	if _, err := h.libraryRepo.AddVoiceTracks(r.Context(), add); err != nil {
		h.discardVoiceTracks(tracks)
		if errors.Is(err, library.ErrConflict) {
			writeCodedError(w, http.StatusConflict, codeTooManyTracks, fmt.Sprintf("a match holds at most %d voice tracks", library.MaxVoiceTracks))
			return
		}
		writeLibraryError(w, "add voice tracks", err)
		return
	}
	v, err := h.libraryRepo.GetMatchVoice(r.Context(), m.ID)
	if err != nil {
		internalError(w, "get match voice", err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		voiceStatus
		Files []voiceImportResult `json:"files"`
	}{h.voiceStatusOf(m.ID, &v), results})
}

type voiceUpload struct {
	matchID uuid.UUID
	src     io.Reader
	name    string
	room    int
}

// importVoiceFile stores one upload, probes it and encodes its tracks. A file that is not audio is a
// rejected result; only a failure of Studio itself is an error, with the tracks made so far.
func (h *Handlers) importVoiceFile(ctx context.Context, up voiceUpload) (voiceImportResult, []library.VoiceTrack, error) {
	name := sanitizeVoiceFileName(up.name)
	res := voiceImportResult{FileName: name, Status: libraryImportRejected}
	dir := h.voice.matchDir(up.matchID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return res, nil, err
	}
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return res, nil, err
	}
	defer os.Remove(tmp.Name())
	_, copyErr := io.Copy(tmp, up.src)
	if err := errors.Join(copyErr, tmp.Close()); err != nil {
		res.ErrorCode, res.Error = codeNotAudio, "could not read the uploaded file"
		return res, nil, nil
	}
	probe, err := h.voice.deps.Media.Probe(ctx, tmp.Name())
	if err != nil {
		res.ErrorCode, res.Error = codeNotAudio, "the file is not an audio recording FFmpeg can read"
		return res, nil, nil
	}
	channels := []int{-1}
	if probe.Channels > 2 {
		channels = channels[:0]
		for c := range probe.Channels {
			channels = append(channels, c)
		}
	}
	if len(channels) > up.room {
		res.ErrorCode, res.Error = codeTooManyTracks, fmt.Sprintf("a match holds at most %d voice tracks", library.MaxVoiceTracks)
		return res, nil, nil
	}
	base := strings.TrimSuffix(name, filepath.Ext(name))
	stored := make([]string, len(channels))
	dsts := make([]string, len(channels))
	for i := range channels {
		stored[i] = uuid.NewString() + ".ogg"
		dsts[i] = filepath.Join(dir, stored[i])
	}
	removeAll := func() {
		for _, d := range dsts {
			_ = os.Remove(d)
		}
	}
	if err := h.voice.deps.Media.Encode(ctx, teamvoice.EncodeJob{Src: tmp.Name(), Dsts: dsts}); err != nil {
		removeAll()
		return res, nil, err
	}
	speech, err := h.trackSpeech(ctx, dsts)
	if err != nil {
		removeAll()
		return res, nil, err
	}
	made := make([]library.VoiceTrack, 0, len(channels))
	for i, c := range channels {
		label := base
		if c >= 0 {
			label = fmt.Sprintf("Pista %d", i+1)
		}
		var encoded int64
		if info, err := os.Stat(dsts[i]); err == nil {
			encoded = info.Size()
		}
		made = append(made, library.VoiceTrack{Label: truncateRunes(label, library.MaxVoiceLabelRunes), FileName: name, Channel: c,
			StoredKey: path.Join(up.matchID.String(), stored[i]), DurationMS: max(speech[i].durationMS, probe.DurationMS), SizeBytes: encoded,
			Source: library.VoiceSourceUpload, Speech: speech[i].spans})
	}
	res.Status, res.Tracks = libraryImportImported, len(made)
	return res, made, nil
}

type trackSpeech struct {
	spans      []library.VoiceSpan
	durationMS int64
}

// trackSpeech finds the speech of each encoded track at once; the first failure cancels the rest.
func (h *Handlers) trackSpeech(ctx context.Context, paths []string) ([]trackSpeech, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	out := make([]trackSpeech, len(paths))
	errs := make([]error, len(paths))
	var wg sync.WaitGroup
	for i, p := range paths {
		wg.Go(func() {
			spans, duration, err := h.voice.deps.Media.Speech(ctx, p)
			out[i], errs[i] = trackSpeech{spans: spans, durationMS: duration}, err
			if err != nil {
				cancel()
			}
		})
	}
	wg.Wait()
	return out, errors.Join(errs...)
}

func (h *Handlers) discardVoiceTracks(tracks []library.VoiceTrack) {
	for _, t := range tracks {
		if p, err := h.voice.trackPath(t); err == nil {
			_ = os.Remove(p)
		}
	}
}

func sanitizeVoiceFileName(raw string) string {
	name := library.CleanName(filepath.Base(strings.ReplaceAll(raw, "\\", "/")))
	if name == "" || name == "." || name == "/" {
		name = "voz"
	}
	return truncateRunes(name, 120)
}

func truncateRunes(s string, n int) string {
	if runes := []rune(s); len(runes) > n {
		return strings.TrimSpace(string(runes[:n]))
	}
	return s
}

// voiceSyncSubject is the recording to place and its match.
type voiceSyncSubject struct {
	match  library.Match
	tracks []library.VoiceTrack
}

// estimateVoiceSync places tracks on a match's demo and logs a recording that does not fit it.
func (h *Handlers) estimateVoiceSync(ctx context.Context, s voiceSyncSubject) (teamvoice.SyncResult, error) {
	m, tracks := s.match, s.tracks
	doc, err := h.readLibraryDocument(m.DemoID)
	if err != nil {
		return teamvoice.SyncResult{}, fmt.Errorf("read the match analysis: %w", err)
	}
	spans := make([][]library.VoiceSpan, 0, len(tracks))
	var audioMS int64
	for _, t := range tracks {
		spans = append(spans, t.Speech)
		audioMS = max(audioMS, t.DurationMS)
	}
	timeline := teamvoice.TimelineOf(doc)
	in := teamvoice.SyncInput{Speech: teamvoice.UnionSpans(spans...), AudioMS: audioMS, Timeline: timeline, DemoVoice: h.demoTeamSpeech(ctx, m)}
	res := teamvoice.Estimate(in)
	if res.Mismatch != "" {
		log.Printf("httpapi: voice of match %s does not fit its demo (audio %s, demo %s): %s", m.ID,
			teamvoice.Clock(audioMS), teamvoice.Clock(timeline.DurationMS), res.Mismatch)
	} else {
		log.Printf("httpapi: voice of match %s placed at %s by %s (state %s, confidence %.2f)", m.ID,
			teamvoice.Clock(res.Sync.OffsetMS), res.Sync.Method, res.Sync.State, res.Sync.Confidence)
	}
	return res, nil
}

// demoTeamSpeech reads each team's in-game voice from the match's demo, in demo time;
// a demo without voice, or one Studio cannot read, gives none.
func (h *Handlers) demoTeamSpeech(ctx context.Context, m library.Match) map[string][]library.VoiceSpan {
	out := map[string][]library.VoiceSpan{}
	demoPath, ok := h.matchDemoPath(ctx, m)
	if !ok || len(m.Players) == 0 {
		return out
	}
	_, packets, _, err := voicecomms.CollectDemo(ctx, voicecomms.DemoRequest{Path: demoPath, Target: m.Players[0].SteamID64})
	if err != nil {
		log.Printf("httpapi: read the in-game voice of match %s: %v", m.ID, err)
		return out
	}
	team := map[string]string{}
	for _, p := range m.Players {
		team[p.SteamID64] = p.TeamKey
	}
	rate := m.Tickrate
	if rate <= 0 {
		rate = 64
	}
	toMS := func(tick int) int64 { return int64(float64(tick) * 1000 / rate) }
	for steamID, ranges := range voicecomms.SpeechByPlayer(packets, int(rate+0.5)) {
		key := team[steamID]
		if key == "" {
			continue
		}
		for _, rg := range ranges {
			out[key] = append(out[key], library.VoiceSpan{StartMS: toMS(rg.StartTick), EndMS: toMS(rg.EndTick)})
		}
	}
	for key, spans := range out {
		out[key] = teamvoice.UnionSpans(spans)
	}
	return out
}
