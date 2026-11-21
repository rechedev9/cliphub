package httpapi

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/teamvoice"
)

type streamTranscriber struct {
	mu    sync.Mutex
	state string
	jobs  []teamvoice.TranscribeJob
	hold  chan struct{}
	err   error
}

func (f *streamTranscriber) Status() teamvoice.TranscriberStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	return teamvoice.TranscriberStatus{State: f.state}
}

func (f *streamTranscriber) Install(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state = teamvoice.TranscriberInstalled
	return nil
}

func (f *streamTranscriber) Transcribe(ctx context.Context, job teamvoice.TranscribeJob) ([]teamvoice.Line, error) {
	f.mu.Lock()
	hold, err := f.hold, f.err
	f.mu.Unlock()
	if hold != nil {
		select {
		case <-hold:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobs = append(f.jobs, job)
	at := job.Clip.StartMS + 1000
	return []teamvoice.Line{{StartMS: at, EndMS: at + 1000, Text: "vaya tiro", Words: []teamvoice.Word{
		{Text: "vaya", StartMS: at, EndMS: at + 400}, {Text: "tiro", StartMS: at + 500, EndMS: at + 1000},
	}}}, nil
}

func (f *streamTranscriber) clips() []library.VoiceSpan {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []library.VoiceSpan{}
	for _, job := range f.jobs {
		out = append(out, *job.Clip)
	}
	return out
}

type streamTranscriptEnv struct {
	libraryTestEnv
	streams     *store.MemoryStreamJobRepository
	transcriber *streamTranscriber
}

func newStreamTranscriptEnv(t *testing.T) streamTranscriptEnv {
	t.Helper()
	return streamTranscriptEnvOver(t, store.NewMemoryStreamJobRepository(), &streamTranscriber{state: teamvoice.TranscriberMissing})
}

func streamTranscriptEnvOver(t *testing.T, streams *store.MemoryStreamJobRepository, tr *streamTranscriber) streamTranscriptEnv {
	t.Helper()
	files := localFakeStorage{fakeStorage: newFakeStorage(), root: t.TempDir()}
	repo := store.NewMemoryLibraryRepository()
	h := NewHandlers(newFakeRepo(), files, &fakeQueue{}, WithLibrary(repo), WithStreamRepository(streams),
		WithVoice(VoiceDeps{Dir: t.TempDir(), Media: fakeVoiceMedia{}, Transcriber: tr}))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	h.StartVoice(ctx)
	return streamTranscriptEnv{libraryTestEnv: libraryTestEnv{t: t, repo: repo, storage: files.fakeStorage, queue: &fakeQueue{}, h: h, router: Routes(h)},
		streams: streams, transcriber: tr}
}

func (e streamTranscriptEnv) stream(probe streamclips.SourceProbe) string {
	e.t.Helper()
	probe.DurationSeconds = 600
	job := &streamclips.Job{Status: streamclips.StatusReady, SourcePath: "streams/source.mkv", SourceSHA256: uuid.NewString(), Probe: probe}
	if err := e.streams.Create(context.Background(), job); err != nil {
		e.t.Fatal(err)
	}
	return "/api/stream-jobs/" + job.ID.String()
}

func (e streamTranscriptEnv) await(base string, done func(streamTranscript) bool) streamTranscript {
	e.t.Helper()
	var got streamTranscript
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		e.json(http.MethodGet, base+"/transcript", nil, http.StatusOK, &got)
		if done(got) {
			return got
		}
		if time.Now().After(deadline) {
			e.t.Fatalf("transcript never got there: %+v", got)
		}
	}
}

type streamSpans struct {
	Spans []streamSpan `json:"spans"`
	Force bool         `json:"force,omitempty"`
}

var micStream = streamclips.SourceProbe{AudioCodec: "aac", AudioStreams: 2, AudioTitles: []string{"Escritorio", "Mic"}}

func TestStreamTranscriptLivesWithItsVideos(t *testing.T) {
	env := newStreamTranscriptEnv(t)
	base := env.stream(micStream)
	moment := streamSpans{Spans: []streamSpan{{StartSeconds: 62, EndSeconds: 75}}}

	var first streamTranscript
	env.json(http.MethodGet, base+"/transcript", nil, http.StatusOK, &first)
	if first.Availability != streamVoiceAvailable || first.State != library.TranscriptNone || first.Transcriber.State != teamvoice.TranscriberMissing || len(first.Lines) != 0 {
		t.Fatalf("transcript before anything = %+v; want a mic stream with no frases and the transcriber to install", first)
	}
	env.json(http.MethodPost, base+"/transcript", moment, http.StatusConflict, nil)
	env.transcriber.state = teamvoice.TranscriberInstalled

	env.json(http.MethodPost, base+"/transcript", moment, http.StatusAccepted, nil)
	got := env.await(base, func(s streamTranscript) bool { return s.State == library.TranscriptReady })
	padded := library.VoiceSpan{StartMS: 60_000, EndMS: 80_000}
	if clips := env.transcriber.clips(); len(clips) != 1 || clips[0] != padded || env.transcriber.jobs[0].Track != 1 {
		t.Fatalf("transcribed %+v, want only the stretch of the momento on the mic track", env.transcriber.jobs)
	}
	if len(got.Lines) != 1 || got.Lines[0].Text != "vaya tiro" || got.Lines[0].StartMS != 61_000 || len(got.Spans) != 1 || got.Spans[0] != padded {
		t.Fatalf("transcript = %+v; want the frase on the clock of the stream", got)
	}

	env.json(http.MethodPost, base+"/transcript", moment, http.StatusOK, nil)
	two := streamSpans{Spans: append([]streamSpan{{StartSeconds: 300, EndSeconds: 310}}, moment.Spans...)}
	env.json(http.MethodPost, base+"/transcript", two, http.StatusAccepted, nil)
	got = env.await(base, func(s streamTranscript) bool { return s.State == library.TranscriptReady && len(s.Lines) == 2 })
	if clips := env.transcriber.clips(); len(clips) != 2 || clips[1] != (library.VoiceSpan{StartMS: 290_000, EndMS: 320_000}) {
		t.Fatalf("transcribed %+v, want the new stretch alone", clips)
	}

	line := base + "/transcript/lines/" + got.Lines[0].ID.String()
	env.json(http.MethodPatch, line, map[string]string{"text": " "}, http.StatusBadRequest, nil)
	env.json(http.MethodPatch, line, map[string]any{}, http.StatusBadRequest, nil)
	var fixed streamclips.VoiceLine
	env.json(http.MethodPatch, line, map[string]string{"text": "vaya tiro, tío"}, http.StatusOK, &fixed)
	if fixed.Text != "vaya tiro, tío" || !fixed.Corrected || fixed.OriginalText != "vaya tiro" {
		t.Fatalf("correction = %+v, want the original kept", fixed)
	}
	other := base + "/transcript/lines/" + got.Lines[1].ID.String()
	env.json(http.MethodPatch, other, map[string]bool{"discarded": true}, http.StatusOK, &fixed)
	if !fixed.Discarded || fixed.Text != "vaya tiro" {
		t.Fatalf("discard = %+v", fixed)
	}
	env.json(http.MethodPatch, base+"/transcript/lines/"+uuid.NewString(), map[string]bool{"discarded": true}, http.StatusNotFound, nil)

	var subs streamclips.Subtitles
	env.json(http.MethodPatch, base+"/subtitles", map[string]bool{"shorts": true}, http.StatusOK, &subs)
	hide := map[string]any{"video": "clip-1", "line_id": got.Lines[0].ID, "hidden": true}
	env.json(http.MethodPatch, base+"/subtitles", hide, http.StatusOK, &subs)
	if !subs.Shorts || subs.Summary || len(subs.HiddenIn("clip-1")) != 1 || subs.HiddenIn("clip-1")[0] != got.Lines[0].ID || len(subs.HiddenIn(streamclips.SummaryVideo)) != 0 {
		t.Fatalf("subtitles = %+v; want Shorts on and the frase hidden in its video only", subs)
	}
	env.json(http.MethodPatch, base+"/subtitles", map[string]any{"video": "clip-1"}, http.StatusBadRequest, nil)

	moment.Force = true
	env.json(http.MethodPost, base+"/transcript", moment, http.StatusAccepted, nil)
	again := env.await(base, func(s streamTranscript) bool {
		return s.State == library.TranscriptReady && len(env.transcriber.clips()) == 3
	})
	if len(again.Lines) != 2 || again.Lines[0].ID != got.Lines[0].ID || again.Lines[0].Text != "vaya tiro, tío" || again.Lines[0].OriginalText != "vaya tiro" || !again.Lines[1].Discarded {
		t.Fatalf("lines after transcribing again = %+v; want the correction and the discard kept", again.Lines)
	}
	if !again.Subtitles.Shorts || len(again.Subtitles.HiddenIn("clip-1")) != 1 {
		t.Fatalf("subtitles after transcribing again = %+v; want them kept", again.Subtitles)
	}
	var shown streamclips.Subtitles
	env.json(http.MethodPatch, base+"/subtitles", map[string]any{"video": "clip-1", "line_id": got.Lines[0].ID, "hidden": false}, http.StatusOK, &shown)
	if !shown.Shorts || len(shown.HiddenIn("clip-1")) != 0 {
		t.Fatalf("subtitles after showing the frase again = %+v", shown)
	}
}

func TestStreamWithOnlyAMixedTrackIsNotTranscribed(t *testing.T) {
	env := newStreamTranscriptEnv(t)
	env.transcriber.state = teamvoice.TranscriberInstalled
	base := env.stream(streamclips.SourceProbe{AudioCodec: "aac", AudioStreams: 1})
	var got streamTranscript
	env.json(http.MethodGet, base+"/transcript", nil, http.StatusOK, &got)
	if got.Availability != streamVoiceMixedTrack {
		t.Fatalf("availability = %q, want %q", got.Availability, streamVoiceMixedTrack)
	}
	var refused struct {
		Code string `json:"code"`
	}
	env.json(http.MethodPost, base+"/transcript", streamSpans{Spans: []streamSpan{{StartSeconds: 62, EndSeconds: 75}}}, http.StatusConflict, &refused)
	if refused.Code != codeMixedTrack || len(env.transcriber.clips()) != 0 {
		t.Fatalf("a mixed track answered %q and ran %d transcriptions; want it refused", refused.Code, len(env.transcriber.clips()))
	}
	silent := env.stream(streamclips.SourceProbe{})
	env.json(http.MethodGet, silent+"/transcript", nil, http.StatusOK, &got)
	if got.Availability != streamVoiceNoAudio {
		t.Fatalf("availability of a stream without audio = %q, want %q", got.Availability, streamVoiceNoAudio)
	}
}

func TestStreamTranscriptIsQueuedAgainAfterARestart(t *testing.T) {
	env := newStreamTranscriptEnv(t)
	env.transcriber.state = teamvoice.TranscriberInstalled
	base := env.stream(micStream)
	id := uuid.MustParse(base[len("/api/stream-jobs/"):])
	waiting := streamclips.VoiceTranscript{State: library.TranscriptRunning, Pending: []library.VoiceSpan{{StartMS: 100_000, EndMS: 120_000}}}
	if err := env.streams.SetStreamVoice(context.Background(), streamclips.VoiceUpdate{JobID: id, Transcript: waiting}); err != nil {
		t.Fatal(err)
	}
	restarted := streamTranscriptEnvOver(t, env.streams, &streamTranscriber{state: teamvoice.TranscriberInstalled})
	got := restarted.await(base, func(s streamTranscript) bool { return s.State == library.TranscriptReady })
	if len(got.Lines) != 1 || got.Lines[0].StartMS != 101_000 || len(got.Pending) != 0 {
		t.Fatalf("transcript after a restart = %+v; want the waiting stretch transcribed", got)
	}
}
