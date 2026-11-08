package workers

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/vodfetch"
)

const mp4Container = "mov,mp4,m4a,3gp,3g2,mj2"

// sequenceProber returns probes in order, then repeats the last one.
type sequenceProber struct {
	probes []streamclips.SourceProbe
	calls  int
}

func (p *sequenceProber) Probe(context.Context, string) (streamclips.SourceProbe, error) {
	i := min(p.calls, len(p.probes)-1)
	p.calls++
	return p.probes[i], nil
}

func TestAcquireWorkerRemuxesMPEGTSSourceToMP4(t *testing.T) {
	id := uuid.New()
	repo := newFakeStreamRepo(streamclips.Job{ID: id, Status: streamclips.StatusAcquiring, SourceURL: "https://www.twitch.tv/videos/2923183027"})
	store := newFakeStorage()
	ffmpeg := &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		return nil, os.WriteFile(args[len(args)-1], []byte("mp4-bytes"), 0o600)
	}}

	w := NewAcquireWorker(repo, store, AcquireWorkerConfig{WorkDir: t.TempDir(), FFmpegPath: "ffmpeg.exe"})
	w.fetcher = vodfetch.Fetcher{Runner: &fakeVodfetchRunner{content: "ts-bytes"}}
	w.prober = &sequenceProber{probes: []streamclips.SourceProbe{
		{Container: "mpegts", Width: 1920, Height: 1080},
		{Container: mp4Container, Width: 1920, Height: 1080},
	}}
	w.runner = ffmpeg

	if err := w.HandleStreamAcquire(context.Background(), streamAcquireTask(t, id)); err != nil {
		t.Fatalf("HandleStreamAcquire error = %v", err)
	}

	if got := repo.jobs[id].Status; got != streamclips.StatusReady {
		t.Fatalf("status = %s, want ready", got)
	}
	if got := repo.jobs[id].Probe.Container; got != mp4Container {
		t.Fatalf("stored probe container = %q, want the remuxed MP4", got)
	}
	if got := string(store.files[streamclips.SourceKey(id)]); got != "mp4-bytes" {
		t.Fatalf("stored source = %q, want the remuxed MP4", got)
	}
	if len(ffmpeg.calls) != 1 {
		t.Fatalf("ffmpeg calls = %d, want 1", len(ffmpeg.calls))
	}
	if args := ffmpeg.calls[0].args; argValue(args, "-c") != "copy" || !slices.Contains(args, "0:a?") {
		t.Fatalf("remux args = %q, want a stream copy that keeps optional audio", args)
	}
}

func TestAcquireWorkerFailsMPEGTSSourceWithoutFFmpeg(t *testing.T) {
	id := uuid.New()
	repo := newFakeStreamRepo(streamclips.Job{ID: id, Status: streamclips.StatusAcquiring, SourceURL: "https://www.twitch.tv/videos/2923183027"})
	store := newFakeStorage()

	w := NewAcquireWorker(repo, store, AcquireWorkerConfig{WorkDir: t.TempDir()})
	w.fetcher = vodfetch.Fetcher{Runner: &fakeVodfetchRunner{content: "ts-bytes"}}
	w.prober = &sequenceProber{probes: []streamclips.SourceProbe{{Container: "mpegts"}}}

	err := w.HandleStreamAcquire(context.Background(), streamAcquireTask(t, id))
	if !errors.Is(err, errSourceNotMP4) {
		t.Fatalf("HandleStreamAcquire error = %v, want errSourceNotMP4", err)
	}
	if got := repo.jobs[id].Status; got != streamclips.StatusFailed {
		t.Fatalf("status = %s, want failed so the editor never opens an unplayable source", got)
	}
}
