package workers

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/streamclips"
)

// containerProber reports MPEG-TS until the file holds the remuxed bytes.
type containerProber struct{ calls int }

func (p *containerProber) Probe(_ context.Context, path string) (streamclips.SourceProbe, error) {
	p.calls++
	b, err := os.ReadFile(path)
	if err != nil {
		return streamclips.SourceProbe{}, err
	}
	if string(b) == "mp4-bytes" {
		return streamclips.SourceProbe{Container: mp4Container, Width: 1920}, nil
	}
	return streamclips.SourceProbe{Container: "mpegts", Width: 1920}, nil
}

func TestRepairStreamSourcesRemuxesOldMPEGTSSourcesOnce(t *testing.T) {
	ctx := context.Background()
	files, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := store.NewMemoryStreamJobRepository()
	broken := streamclips.Job{ID: uuid.New(), Status: streamclips.StatusRendered, SourceSHA256: "ts-sha"}
	checked := streamclips.Job{ID: uuid.New(), Status: streamclips.StatusReady, SourceSHA256: "mp4-sha", Probe: streamclips.SourceProbe{Container: mp4Container}}
	acquiring := streamclips.Job{ID: uuid.New(), Status: streamclips.StatusAcquiring}
	for _, j := range []streamclips.Job{broken, checked, acquiring} {
		j.SourcePath = streamclips.SourceKey(j.ID)
		if err := repo.Create(ctx, &j); err != nil {
			t.Fatal(err)
		}
		if j.Status != streamclips.StatusAcquiring {
			if err := files.Put(j.SourcePath, strings.NewReader("ts-bytes")); err != nil {
				t.Fatal(err)
			}
		}
	}
	broken.SourcePath = streamclips.SourceKey(broken.ID)
	prober := &containerProber{}
	cfg := StreamSourceRepairConfig{Repo: repo, Storage: files, Prober: prober, FFmpegPath: "ffmpeg.exe", Locks: streamclips.NewJobLocks(), runner: remuxingFFmpeg()}

	repaired, err := RepairStreamSources(ctx, cfg)
	if err != nil || repaired != 1 {
		t.Fatalf("RepairStreamSources = %d, %v; want 1 repaired", repaired, err)
	}
	got, err := repo.Get(ctx, broken.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Probe.Container != mp4Container || got.SourceSHA256 == "ts-sha" || got.Status != streamclips.StatusRendered {
		t.Fatalf("repaired job = container %q sha %q status %s, want MP4, a new hash and the status kept", got.Probe.Container, got.SourceSHA256, got.Status)
	}
	path, err := files.ResolvePath(broken.SourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "mp4-bytes" {
		t.Fatalf("stored source = %q, %v; want the remuxed MP4", b, err)
	}

	// The container is now recorded, so a second start probes nothing.
	prober.calls = 0
	if repaired, err := RepairStreamSources(ctx, cfg); err != nil || repaired != 0 || prober.calls != 0 {
		t.Fatalf("second run = %d repaired, %v, %d probes; want nothing to do", repaired, err, prober.calls)
	}
}
