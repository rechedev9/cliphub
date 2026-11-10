package httpapi

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/voicecomms"
)

// localFakeStorage is the fake storage with its keys as files under a folder, as the PC storage is.
type localFakeStorage struct {
	*fakeStorage
	root string
}

func (s localFakeStorage) ResolvePath(key string) (string, error) {
	return filepath.Join(s.root, filepath.FromSlash(key)), nil
}

// fakeDemoVoice gives the target a voice track, and records each team asked for.
type fakeDemoVoice struct {
	mu      sync.Mutex
	targets []string
}

func (f *fakeDemoVoice) ExtractDemoVoice(_ context.Context, job DemoVoiceJob) ([]voicecomms.Track, error) {
	f.mu.Lock()
	f.targets = append(f.targets, job.Target)
	f.mu.Unlock()
	if err := os.MkdirAll(job.Dir, 0o750); err != nil {
		return nil, err
	}
	p := filepath.Join(job.Dir, job.Target+".ogg")
	if err := os.WriteFile(p, []byte("voz de "+job.Target), 0o600); err != nil {
		return nil, err
	}
	return []voicecomms.Track{{SteamID64: job.Target, Name: "p" + job.Target[len(job.Target)-1:], Path: p, Packets: 12}}, nil
}

func TestLibraryMatchDemoVoiceExtractsBothTeamsAndServesTheirTracks(t *testing.T) {
	repo := store.NewMemoryLibraryRepository()
	files := localFakeStorage{fakeStorage: newFakeStorage(), root: t.TempDir()}
	extractor := &fakeDemoVoice{}
	voiceDir := t.TempDir()
	h := NewHandlers(newFakeRepo(), files, &fakeQueue{}, WithLibrary(repo),
		WithVoice(VoiceDeps{Dir: voiceDir, Media: fakeVoiceMedia{}, Transcriber: &fakeTranscriber{}, DemoVoice: extractor}))
	env := libraryTestEnv{t: t, repo: repo, storage: files.fakeStorage, queue: &fakeQueue{}, h: h, router: Routes(h)}
	m := env.seedLibraryMatch("demo-voice", "Alpha", "Bravo")
	base := "/api/library/matches/" + m.ID.String() + "/voice/demo"

	var status demoVoiceStatus
	env.json(http.MethodGet, base, nil, http.StatusOK, &status)
	if status.State != demoVoiceUnreadable {
		t.Fatalf("status without the demo file = %+v", status)
	}
	env.json(http.MethodPost, base, nil, http.StatusConflict, nil)

	d, err := repo.GetDemo(context.Background(), m.DemoID)
	if err != nil {
		t.Fatal(err)
	}
	demoPath, _ := files.ResolvePath(d.Path)
	if err := os.MkdirAll(filepath.Dir(demoPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(demoPath, []byte("demo"), 0o600); err != nil {
		t.Fatal(err)
	}
	env.json(http.MethodGet, base, nil, http.StatusOK, &status)
	if status.State != demoVoiceNone {
		t.Fatalf("status before extracting = %+v", status)
	}
	env.json(http.MethodPost, base, nil, http.StatusAccepted, &status)
	for deadline := time.Now().Add(5 * time.Second); status.State != demoVoiceReady; {
		if time.Now().After(deadline) {
			t.Fatalf("extraction never finished: %+v", status)
		}
		time.Sleep(10 * time.Millisecond)
		env.json(http.MethodGet, base, nil, http.StatusOK, &status)
	}
	if len(status.Tracks) != 2 || status.Tracks[0].TeamKey != "ct-start" || status.Tracks[1].SteamID64 != "76561198000000002" {
		t.Fatalf("demo voice = %+v", status)
	}
	audio := env.do(http.MethodGet, base+"/76561198000000002/audio", nil, "")
	if audio.Code != http.StatusOK || audio.Body.String() != "voz de 76561198000000002" || audio.Header().Get("Content-Type") != "audio/ogg" {
		t.Fatalf("audio = %d %q", audio.Code, audio.Body.String())
	}
	if rw := env.do(http.MethodGet, base+"/76561198000000009/audio", nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("audio of a player without voice = %d", rw.Code)
	}
	if rw := env.do(http.MethodGet, base+"/..%2F..%2Fx/audio", nil, ""); rw.Code != http.StatusBadRequest {
		t.Fatalf("audio of a path = %d", rw.Code)
	}
	env.json(http.MethodPost, base, nil, http.StatusOK, &status)
	if len(extractor.targets) != 2 {
		t.Fatalf("extracted %v; a second request must reuse the first extraction", extractor.targets)
	}

	extracted := filepath.Join(voiceDir, "demo", m.DemoID.String())
	if _, err := os.Stat(extracted); err != nil {
		t.Fatalf("extracted voice folder: %v", err)
	}
	if rw := env.do(http.MethodDelete, "/api/library/demos/"+m.DemoID.String(), nil, ""); rw.Code != http.StatusNoContent {
		t.Fatalf("delete demo = %d", rw.Code)
	}
	if _, err := os.Stat(extracted); !os.IsNotExist(err) {
		t.Fatalf("the in-game voice outlived its demo: %v", err)
	}
}
