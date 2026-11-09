package workers

import (
	"context"
	"errors"
	"fmt"
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

// remuxingFFmpeg writes "mp4-bytes" to the output path, the last argument.
func remuxingFFmpeg() *fakeRunner {
	return &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		return nil, os.WriteFile(args[len(args)-1], []byte("mp4-bytes"), 0o600)
	}}
}

func TestAcquireWorkerRemuxesNonMP4SourceAndStoresItOnce(t *testing.T) {
	for _, container := range []string{"mpegts", "flv"} {
		t.Run(container, func(t *testing.T) {
			id := uuid.New()
			repo := newFakeStreamRepo(streamclips.Job{ID: id, Status: streamclips.StatusAcquiring, SourceURL: "https://www.twitch.tv/videos/2923183027"})
			store := newFakeStorage()
			counted := &countingStorage{Storage: store}
			ffmpeg := remuxingFFmpeg()

			w := NewAcquireWorker(repo, counted, AcquireWorkerConfig{WorkDir: t.TempDir(), FFmpegPath: "ffmpeg.exe"})
			w.fetcher = vodfetch.Fetcher{Runner: &fakeVodfetchRunner{content: "ts-bytes"}}
			w.prober = &sequenceProber{probes: []streamclips.SourceProbe{
				{Container: container, Width: 1920, Height: 1080},
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
			// Metadata, source and default edit plan: the source is stored once, after the remux.
			if counted.puts != 3 {
				t.Fatalf("storage puts = %d, want 3 with the source stored once", counted.puts)
			}
			if args := ffmpeg.calls[0].args; argValue(args, "-c") != "copy" || !slices.Contains(args, "0:a?") {
				t.Fatalf("remux args = %q, want a stream copy that keeps optional audio", args)
			}
		})
	}
}

func TestAcquireWorkerReportsUnplayableSource(t *testing.T) {
	tests := []struct {
		name    string
		ffmpeg  string
		fetcher vodfetch.Fetcher
		runner  *fakeRunner
	}{
		{name: "no ffmpeg to remux", fetcher: vodfetch.Fetcher{Runner: &fakeVodfetchRunner{content: "ts-bytes"}}},
		{name: "remux fails", ffmpeg: "ffmpeg.exe", fetcher: vodfetch.Fetcher{Runner: &fakeVodfetchRunner{content: "ts-bytes"}},
			runner: &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) { return nil, fmt.Errorf("exit status 1") }}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := uuid.New()
			repo := newFakeStreamRepo(streamclips.Job{ID: id, Status: streamclips.StatusAcquiring, SourceURL: "https://www.twitch.tv/videos/2923183027"})
			store := newFakeStorage()
			w := NewAcquireWorker(repo, store, AcquireWorkerConfig{WorkDir: t.TempDir(), FFmpegPath: tt.ffmpeg})
			w.fetcher = tt.fetcher
			w.prober = &sequenceProber{probes: []streamclips.SourceProbe{{Container: "mpegts"}}}
			if tt.runner != nil {
				w.runner = tt.runner
			}

			err := w.HandleStreamAcquire(context.Background(), streamAcquireTask(t, id))
			if !errors.Is(err, streamclips.ErrSourceNotPlayable) {
				t.Fatalf("HandleStreamAcquire error = %v, want ErrSourceNotPlayable", err)
			}
			got := repo.jobs[id]
			if got.Status != streamclips.StatusFailed || got.FailureReason != streamclips.AcquireReasonNotPlayable {
				t.Fatalf("status/reason = %s/%q, want failed with the not-playable reason, not the wrong-link one", got.Status, got.FailureReason)
			}
			// The download is kept, so a retry checks it again instead of downloading the VOD twice.
			if got := string(store.files[streamclips.SourceKey(id)]); got != "ts-bytes" {
				t.Fatalf("stored source = %q, want the download kept for a retry", got)
			}
		})
	}
}

func TestAcquireWorkerCancelledRemuxIsNotReportedAsUnplayable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	id := uuid.New()
	repo := newFakeStreamRepo(streamclips.Job{ID: id, Status: streamclips.StatusAcquiring, SourceURL: "https://www.twitch.tv/videos/2923183027"})
	w := NewAcquireWorker(repo, newFakeStorage(), AcquireWorkerConfig{WorkDir: t.TempDir(), FFmpegPath: "ffmpeg.exe"})
	w.fetcher = vodfetch.Fetcher{Runner: &fakeVodfetchRunner{content: "ts-bytes"}}
	w.prober = &sequenceProber{probes: []streamclips.SourceProbe{{Container: "mpegts"}}}
	// Studio shuts down mid-remux: FFmpeg is killed and the context is done.
	w.runner = &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) {
		cancel()
		return nil, fmt.Errorf("exit status 1")
	}}

	err := w.HandleStreamAcquire(ctx, streamAcquireTask(t, id))
	if !errors.Is(err, context.Canceled) || errors.Is(err, streamclips.ErrSourceNotPlayable) {
		t.Fatalf("HandleStreamAcquire error = %v, want the cancellation, not ErrSourceNotPlayable", err)
	}
	if got := repo.jobs[id].FailureReason; got == streamclips.AcquireReasonNotPlayable {
		t.Fatalf("reason = %q, want a shutdown not to blame the video", got)
	}
}

func TestFriendlyAcquireReasonNamesYtdlpMPEGTS(t *testing.T) {
	err := fmt.Errorf("download https://www.twitch.tv/videos/1: %w", vodfetch.ErrMPEGTS)
	if got := friendlyAcquireReason(err); got != streamclips.AcquireReasonNotPlayable {
		t.Fatalf("reason = %q, want the not-playable reason", got)
	}
	if got := acquireFailureClass(err); got != streamclips.AcquireCodeNotPlayable {
		t.Fatalf("class = %q, want %q", got, streamclips.AcquireCodeNotPlayable)
	}
}
