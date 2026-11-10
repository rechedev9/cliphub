package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/teamvoice"
)

// fakeVoiceMedia reads the channel count from the file's first word: "mono" or "quad".
type fakeVoiceMedia struct{}

func (fakeVoiceMedia) Probe(_ context.Context, path string) (teamvoice.Probe, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return teamvoice.Probe{}, err
	}
	switch {
	case bytes.HasPrefix(b, []byte("mono")):
		return teamvoice.Probe{Channels: 1, DurationMS: 600_000, Codec: "pcm_s16le"}, nil
	case bytes.HasPrefix(b, []byte("quad")):
		return teamvoice.Probe{Channels: 4, DurationMS: 600_000, Codec: "pcm_s16le"}, nil
	}
	return teamvoice.Probe{}, teamvoice.ErrNotAudio
}

func (fakeVoiceMedia) Encode(_ context.Context, job teamvoice.EncodeJob) error {
	b, err := os.ReadFile(job.Src)
	if err != nil {
		return err
	}
	return os.WriteFile(job.Dst, b, 0o600)
}

func (fakeVoiceMedia) Speech(context.Context, string) ([]library.VoiceSpan, int64, error) {
	return []library.VoiceSpan{{StartMS: 1000, EndMS: 2500}}, 600_000, nil
}

type fakeTranscriber struct{ state string }

func (f *fakeTranscriber) Status() teamvoice.TranscriberStatus {
	return teamvoice.TranscriberStatus{State: f.state}
}

func (f *fakeTranscriber) Install(context.Context) error {
	f.state = teamvoice.TranscriberInstalled
	return nil
}

func (f *fakeTranscriber) Transcribe(_ context.Context, job teamvoice.TranscribeJob) ([]teamvoice.Line, error) {
	if job.Language != "es" {
		return nil, errors.New("unexpected language")
	}
	return []teamvoice.Line{{StartMS: 1000, EndMS: 2000, Text: "Rotamos a B ya"}}, nil
}

type voiceTestEnv struct {
	libraryTestEnv
	dir         string
	transcriber *fakeTranscriber
}

func newVoiceTestEnv(t *testing.T) voiceTestEnv {
	t.Helper()
	repo := store.NewMemoryLibraryRepository()
	files := newFakeStorage()
	queue := &fakeQueue{}
	dir := t.TempDir()
	tr := &fakeTranscriber{state: teamvoice.TranscriberMissing}
	h := NewHandlers(newFakeRepo(), files, queue, WithLibrary(repo),
		WithVoice(VoiceDeps{Dir: dir, Media: fakeVoiceMedia{}, Transcriber: tr}))
	env := libraryTestEnv{t: t, repo: repo, storage: files, queue: queue, h: h, router: Routes(h)}
	return voiceTestEnv{libraryTestEnv: env, dir: dir, transcriber: tr}
}

func (e voiceTestEnv) upload(path string, files map[string]string) *http.Response {
	e.t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	for name, content := range files {
		part, _ := mw.CreateFormFile("audio", name)
		_, _ = part.Write([]byte(content))
	}
	_ = mw.Close()
	return e.do(http.MethodPost, path, body.Bytes(), mw.FormDataContentType()).Result()
}

func TestLibraryMatchVoiceImportSyncTranscribeAndSearch(t *testing.T) {
	env := newVoiceTestEnv(t)
	m := env.seedLibraryMatch("voice-sha", "Alpha", "Bravo")
	base := "/api/library/matches/" + m.ID.String() + "/voice"

	var status voiceStatus
	env.json(http.MethodGet, base, nil, http.StatusOK, &status)
	if status.Present || status.Sync.State != library.VoiceUnsynced {
		t.Fatalf("empty voice = %+v", status)
	}

	res := env.upload(base, map[string]string{"ts3 partida.wav": "mono recording"})
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("import = %d: %s", res.StatusCode, b)
	}
	env.json(http.MethodGet, base, nil, http.StatusOK, &status)
	if !status.Present || len(status.Tracks) != 1 || status.Tracks[0].Label != "ts3 partida" || status.SpeechMS != 1500 {
		t.Fatalf("imported voice = %+v", status)
	}

	env.json(http.MethodPatch, base, map[string]int64{"offset_ms": 5000}, http.StatusOK, &status)
	if status.Sync.State != library.VoiceManual || status.Sync.OffsetMS != 5000 {
		t.Fatalf("manual sync = %+v", status.Sync)
	}
	env.json(http.MethodPatch, base, map[string]any{"offset_ms": 1, "auto": true}, http.StatusBadRequest, nil)
	env.json(http.MethodPatch, base+"/tracks/1", map[string]string{"label": "IGL", "steamid64": "76561198000000001"}, http.StatusOK, &status)
	if status.Tracks[0].Label != "IGL" || status.Tracks[0].SteamID64 != "76561198000000001" {
		t.Fatalf("track patch = %+v", status.Tracks[0])
	}
	env.json(http.MethodPatch, base+"/tracks/1", map[string]string{"steamid64": "999"}, http.StatusBadRequest, nil)

	audio := env.do(http.MethodGet, base+"/tracks/1/audio", nil, "")
	if audio.Code != http.StatusOK || audio.Body.String() != "mono recording" || audio.Header().Get("Content-Type") != "audio/ogg" {
		t.Fatalf("audio = %d %q", audio.Code, audio.Body.String())
	}

	env.json(http.MethodPost, base+"/transcribe", nil, http.StatusConflict, nil)
	env.json(http.MethodPost, "/api/library/voice/transcriber/install", nil, http.StatusAccepted, nil)
	env.json(http.MethodPost, base+"/transcribe", map[string]string{"language": "es"}, http.StatusAccepted, &status)
	if status.Transcript.State != library.TranscriptQueued {
		t.Fatalf("transcript = %+v", status.Transcript)
	}
	env.h.transcribeMatch(context.Background(), <-env.h.voice.queue)

	var comms voiceComms
	env.json(http.MethodGet, base+"/comms", nil, http.StatusOK, &comms)
	if comms.Transcript.State != library.TranscriptReady || len(comms.Lines) != 1 || len(comms.Rounds) == 0 {
		t.Fatalf("comms = %+v", comms)
	}
	if line := comms.Lines[0]; line.Round != 1 || line.Label != "IGL" || line.Tick <= 0 {
		t.Fatalf("placed line = %+v", line)
	}
	env.json(http.MethodGet, base+"/comms?round=99", nil, http.StatusBadRequest, nil)

	var search struct {
		Hits []voiceSearchHit `json:"hits"`
	}
	env.json(http.MethodGet, "/api/library/voice/search?q=rotamos%20a%20b", nil, http.StatusOK, &search)
	if len(search.Hits) != 1 || search.Hits[0].MatchID != m.ID || search.Hits[0].Map != m.Map {
		t.Fatalf("search = %+v", search.Hits)
	}
	env.json(http.MethodGet, "/api/library/voice/search?q=eco", nil, http.StatusOK, &search)
	if len(search.Hits) != 0 {
		t.Fatalf("search miss = %+v", search.Hits)
	}

	if rw := env.do(http.MethodDelete, base, nil, ""); rw.Code != http.StatusNoContent {
		t.Fatalf("delete = %d", rw.Code)
	}
	if _, err := os.Stat(filepath.Join(env.dir, m.ID.String())); !os.IsNotExist(err) {
		t.Fatalf("voice files kept after delete: %v", err)
	}
	env.json(http.MethodGet, base, nil, http.StatusOK, &status)
	if status.Present {
		t.Fatalf("voice after delete = %+v", status)
	}
}

func TestLibraryMatchVoiceImportSplitsChannelsAndRejectsNonAudio(t *testing.T) {
	env := newVoiceTestEnv(t)
	m := env.seedLibraryMatch("voice-split", "Alpha", "Bravo")
	base := "/api/library/matches/" + m.ID.String() + "/voice"

	res := env.upload(base, map[string]string{"notas.txt": "hello"})
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(string(b), codeNotAudio) {
		t.Fatalf("non-audio import = %d: %s", res.StatusCode, b)
	}

	res = env.upload(base, map[string]string{"multitrack.wav": "quad recording"})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("multi-track import = %d", res.StatusCode)
	}
	var status voiceStatus
	env.json(http.MethodGet, base, nil, http.StatusOK, &status)
	if len(status.Tracks) != 4 || status.Tracks[3].Label != "Pista 4" || status.Tracks[0].Channel != 0 {
		t.Fatalf("tracks = %+v", status.Tracks)
	}

	for range 3 {
		if res := env.upload(base, map[string]string{"more.wav": "quad recording"}); res.StatusCode != http.StatusOK {
			t.Fatalf("filling tracks = %d", res.StatusCode)
		}
	}
	res = env.upload(base, map[string]string{"extra.wav": "mono recording"})
	b, _ = io.ReadAll(res.Body)
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(string(b), codeTooManyTracks) {
		t.Fatalf("over the track cap = %d: %s", res.StatusCode, b)
	}
}

func TestLibraryMatchVoiceUnavailableWithoutFFmpeg(t *testing.T) {
	env := newLibraryTestEnv(t)
	m := env.seedLibraryMatch("no-ffmpeg", "Alpha", "Bravo")
	rw := env.do(http.MethodGet, "/api/library/matches/"+m.ID.String()+"/voice", nil, "")
	if rw.Code != http.StatusServiceUnavailable || !strings.Contains(rw.Body.String(), codeVoiceUnavailable) {
		t.Fatalf("voice without deps = %d: %s", rw.Code, rw.Body.String())
	}
	var tr teamvoice.TranscriberStatus
	env.json(http.MethodGet, "/api/library/voice/transcriber", nil, http.StatusOK, &tr)
	if tr.State != teamvoice.TranscriberUnsupported {
		t.Fatalf("transcriber = %+v", tr)
	}
}
