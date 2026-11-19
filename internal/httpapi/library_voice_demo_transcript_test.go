package httpapi

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/teamvoice"
)

// fakeTranslator answers every phrase with the same English and counts the tracks it read.
type fakeTranslator struct {
	mu     sync.Mutex
	state  string
	tracks int
}

func (f *fakeTranslator) Status() teamvoice.TranscriberStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	return teamvoice.TranscriberStatus{State: f.state}
}

func (f *fakeTranslator) Install(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state = teamvoice.TranscriberInstalled
	return nil
}

func (f *fakeTranslator) Translate(_ context.Context, job teamvoice.TranslateJob) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tracks++
	out := make([]string, len(job.Spans))
	for i := range out {
		out[i] = "we rotate to B now"
	}
	return out, nil
}

type demoTranscriptEnv struct {
	libraryTestEnv
	jobs        *fakeRepo
	match       library.Match
	base        string
	transcriber *fakeTranscriber
	translator  *fakeTranslator
}

func newDemoTranscriptEnv(t *testing.T) demoTranscriptEnv {
	t.Helper()
	repo := store.NewMemoryLibraryRepository()
	files := localFakeStorage{fakeStorage: newFakeStorage(), root: t.TempDir()}
	tr := &fakeTranscriber{state: teamvoice.TranscriberMissing}
	tl := &fakeTranslator{state: teamvoice.TranscriberMissing}
	jobs := newFakeRepo()
	h := NewHandlers(jobs, files, &fakeQueue{}, WithLibrary(repo),
		WithVoice(VoiceDeps{Dir: t.TempDir(), Media: fakeVoiceMedia{}, Transcriber: tr, Translator: tl, DemoVoice: &fakeDemoVoice{}}))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	h.StartVoice(ctx)
	env := libraryTestEnv{t: t, repo: repo, storage: files.fakeStorage, queue: &fakeQueue{}, h: h, router: Routes(h)}
	m := env.seedLibraryMatch("demo-transcript", "Alpha", "Bravo")
	d, err := repo.GetDemo(ctx, m.DemoID)
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
	return demoTranscriptEnv{libraryTestEnv: env, jobs: jobs, match: m, base: "/api/library/matches/" + m.ID.String() + "/voice/demo/transcript",
		transcriber: tr, translator: tl}
}

// await polls the transcript until done says so.
func (e demoTranscriptEnv) await(done func(demoTranscript) bool) demoTranscript {
	e.t.Helper()
	var got demoTranscript
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		e.json(http.MethodGet, e.base, nil, http.StatusOK, &got)
		if done(got) {
			return got
		}
		if time.Now().After(deadline) {
			e.t.Fatalf("transcript never got there: %+v", got)
		}
	}
}

func TestDemoVoiceTranscriptTranscribesOnceCorrectsAndTranslates(t *testing.T) {
	env := newDemoTranscriptEnv(t)
	env.json(http.MethodPost, env.base, nil, http.StatusConflict, nil)
	env.transcriber.state = teamvoice.TranscriberInstalled

	env.json(http.MethodPost, env.base, nil, http.StatusAccepted, nil)
	got := env.await(func(d demoTranscript) bool { return d.State == library.TranscriptReady })
	if len(got.Lines) != 2 || got.Lines[0].Text != "Rotamos a B ya" || got.Lines[0].Name == "" || got.Lines[0].TeamKey == "" || got.Lines[0].Tick <= 0 {
		t.Fatalf("lines = %+v; want one phrase per player, named and placed on the demo", got.Lines)
	}
	if env.transcriber.calls.Load() != 2 {
		t.Fatalf("transcribed %d tracks, want both players'", env.transcriber.calls.Load())
	}
	env.json(http.MethodPost, env.base, map[string]bool{}, http.StatusOK, nil)
	if env.transcriber.calls.Load() != 2 {
		t.Fatalf("a second request transcribed again: %d tracks", env.transcriber.calls.Load())
	}

	line := env.base + "/lines/" + got.Lines[0].ID.String()
	env.json(http.MethodPatch, line, map[string]string{"text": " "}, http.StatusBadRequest, nil)
	var fixed library.DemoVoiceLine
	env.json(http.MethodPatch, line, map[string]string{"text": "Rotamos a B, ya"}, http.StatusOK, &fixed)
	if fixed.Text != "Rotamos a B, ya" || !fixed.Corrected {
		t.Fatalf("correction = %+v", fixed)
	}

	env.json(http.MethodPost, env.base, map[string]bool{"english": true}, http.StatusConflict, nil)
	env.translator.state = teamvoice.TranscriberInstalled
	env.json(http.MethodPost, env.base, map[string]bool{"english": true}, http.StatusAccepted, nil)
	got = env.await(func(d demoTranscript) bool { return d.English == library.TranscriptReady })
	if got.Lines[0].English != "we rotate to B now" || got.Lines[0].Text != "Rotamos a B, ya" || env.transcriber.calls.Load() != 2 {
		t.Fatalf("after the translation: %+v, %d transcriptions; want the English beside the kept correction", got.Lines, env.transcriber.calls.Load())
	}

	var comms voiceComms
	env.json(http.MethodGet, "/api/library/matches/"+env.match.ID.String()+"/voice/comms", nil, http.StatusOK, &comms)
	if len(comms.Lines) != 2 || comms.Lines[0].Label == "" || len(comms.Totals) == 0 {
		t.Fatalf("comms of the demo voices = %+v", comms)
	}
	var found struct {
		Hits []voiceSearchHit `json:"hits"`
	}
	env.json(http.MethodGet, "/api/library/voice/search?q=rotamos", nil, http.StatusOK, &found)
	if len(found.Hits) != 2 || found.Hits[0].MatchID != env.match.ID {
		t.Fatalf("search of the demo voices = %+v", found.Hits)
	}
}

// A video hides phrases by id, so transcribing the demo again keeps the id of each phrase it hears again.
func TestDemoVoiceTranscriptKeepsPhraseIDsAcrossTranscriptions(t *testing.T) {
	env := newDemoTranscriptEnv(t)
	env.transcriber.state = teamvoice.TranscriberInstalled
	env.json(http.MethodPost, env.base, nil, http.StatusAccepted, nil)
	first := env.await(func(d demoTranscript) bool { return d.State == library.TranscriptReady })
	env.json(http.MethodPost, env.base, map[string]bool{"force": true}, http.StatusAccepted, nil)
	again := env.await(func(d demoTranscript) bool {
		return d.State == library.TranscriptReady && env.transcriber.calls.Load() == 4
	})
	if len(first.Lines) != 2 || len(again.Lines) != 2 {
		t.Fatalf("lines = %d then %d, want one phrase per player each time", len(first.Lines), len(again.Lines))
	}
	for i := range first.Lines {
		if again.Lines[i].ID != first.Lines[i].ID {
			t.Fatalf("phrase %d changed id from %s to %s", i, first.Lines[i].ID, again.Lines[i].ID)
		}
	}
	if first.Lines[0].ID == first.Lines[1].ID {
		t.Fatal("the same words of two players share an id")
	}
}
