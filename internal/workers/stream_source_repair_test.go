package workers

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/streamclips"
)

// containerProber reports MPEG-TS until the file holds the remuxed bytes.
type containerProber struct {
	calls int
	// onRemuxed runs when the prober first sees remuxed bytes, to change the job mid-repair.
	onRemuxed func()
}

func (p *containerProber) Probe(_ context.Context, path string) (streamclips.SourceProbe, error) {
	p.calls++
	b, err := os.ReadFile(path)
	if err != nil {
		return streamclips.SourceProbe{}, err
	}
	if string(b) == "mp4-bytes" {
		if p.onRemuxed != nil {
			p.onRemuxed()
			p.onRemuxed = nil
		}
		return streamclips.SourceProbe{Container: mp4Container, Width: 1920}, nil
	}
	return streamclips.SourceProbe{Container: "mpegts", Width: 1920}, nil
}

type repairFixture struct {
	repo  *store.MemoryStreamJobRepository
	files *storage.Local
	cfg   StreamSourceRepairConfig
}

// newRepairFixture stores an MPEG-TS source for every job.
func newRepairFixture(t *testing.T, jobs ...streamclips.Job) repairFixture {
	t.Helper()
	files, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := store.NewMemoryStreamJobRepository()
	for _, j := range jobs {
		j.SourcePath = streamclips.SourceKey(j.ID)
		if err := repo.Create(context.Background(), &j); err != nil {
			t.Fatal(err)
		}
		if err := files.Put(j.SourcePath, strings.NewReader("ts-bytes")); err != nil {
			t.Fatal(err)
		}
	}
	cfg := StreamSourceRepairConfig{Repo: repo, Storage: files, Prober: &containerProber{}, FFmpegPath: "ffmpeg.exe",
		Locks: streamclips.NewJobLocks(), runner: remuxingFFmpeg(), swap: sourceSwapRetry{attempts: 3, rename: os.Rename}}
	return repairFixture{repo: repo, files: files, cfg: cfg}
}

// source returns the job's stored bytes and whether a remux temp file was left beside them.
func (f repairFixture) source(t *testing.T, id uuid.UUID) (string, bool) {
	t.Helper()
	path, err := f.files.ResolvePath(streamclips.SourceKey(id))
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, statErr := os.Stat(path + ".remux.mp4")
	return string(b), statErr == nil
}

func TestRepairStreamSourcesRemuxesOldMPEGTSSourcesOnce(t *testing.T) {
	ctx := context.Background()
	broken := streamclips.Job{ID: uuid.New(), Status: streamclips.StatusRendered, SourceSHA256: "ts-sha"}
	checked := streamclips.Job{ID: uuid.New(), Status: streamclips.StatusReady, SourceSHA256: "mp4-sha", Probe: streamclips.SourceProbe{Container: mp4Container}}
	acquiring := streamclips.Job{ID: uuid.New(), Status: streamclips.StatusAcquiring}
	f := newRepairFixture(t, broken, checked, acquiring)

	repaired, err := RepairStreamSources(ctx, f.cfg)
	if err != nil || repaired != 1 {
		t.Fatalf("RepairStreamSources = %d, %v; want 1 repaired", repaired, err)
	}
	got, err := f.repo.Get(ctx, broken.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Probe.Container != mp4Container || got.SourceSHA256 == "ts-sha" || got.Status != streamclips.StatusRendered {
		t.Fatalf("repaired job = container %q sha %q status %s, want MP4, a new hash and the status kept", got.Probe.Container, got.SourceSHA256, got.Status)
	}
	if b, leftover := f.source(t, broken.ID); b != "mp4-bytes" || leftover {
		t.Fatalf("repaired source = %q, temp left %v; want the remuxed MP4 and no temp", b, leftover)
	}
	for _, id := range []uuid.UUID{checked.ID, acquiring.ID} {
		if b, _ := f.source(t, id); b != "ts-bytes" {
			t.Fatalf("source of a job the repair must skip = %q, want it untouched", b)
		}
	}

	// The container is now recorded, so a second start probes nothing.
	prober := &containerProber{}
	f.cfg.Prober = prober
	if repaired, err := RepairStreamSources(ctx, f.cfg); err != nil || repaired != 0 || prober.calls != 0 {
		t.Fatalf("second run = %d repaired, %v, %d probes; want nothing to do", repaired, err, prober.calls)
	}
}

func TestRepairStreamSourcesWaitsForReadersToCloseTheSource(t *testing.T) {
	job := streamclips.Job{ID: uuid.New(), Status: streamclips.StatusReady}
	f := newRepairFixture(t, job)
	failures := 2
	f.cfg.swap.rename = func(oldpath, newpath string) error {
		if failures > 0 {
			failures--
			return errors.New("the process cannot access the file because it is being used by another process")
		}
		return os.Rename(oldpath, newpath)
	}

	if repaired, err := RepairStreamSources(context.Background(), f.cfg); err != nil || repaired != 1 {
		t.Fatalf("RepairStreamSources = %d, %v; want the swap to succeed once the reader is gone", repaired, err)
	}
	if b, _ := f.source(t, job.ID); b != "mp4-bytes" {
		t.Fatalf("source = %q, want the remuxed MP4", b)
	}
}

func TestRepairStreamSourcesGivesUpWhileTheSourceStaysOpen(t *testing.T) {
	job := streamclips.Job{ID: uuid.New(), Status: streamclips.StatusReady}
	f := newRepairFixture(t, job)
	f.cfg.swap.rename = func(string, string) error { return errors.New("sharing violation") }

	if _, err := RepairStreamSources(context.Background(), f.cfg); !errors.Is(err, errSourceInUse) {
		t.Fatalf("RepairStreamSources error = %v, want errSourceInUse", err)
	}
	got, err := f.repo.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if b, leftover := f.source(t, job.ID); b != "ts-bytes" || leftover || got.Probe.Container != "" {
		t.Fatalf("source %q, temp left %v, container %q; want the old source, no temp and the job left for the next start", b, leftover, got.Probe.Container)
	}
}

func TestRepairStreamSourcesLeavesAJobThatMovedOnDuringTheRemux(t *testing.T) {
	ctx := context.Background()
	job := streamclips.Job{ID: uuid.New(), Status: streamclips.StatusFailed}
	f := newRepairFixture(t, job)
	// The user retries the failed import while the repair remuxes its old source.
	f.cfg.Prober = &containerProber{onRemuxed: func() {
		if err := f.repo.UpdateStatus(ctx, job.ID, streamclips.StatusAcquiring, ""); err != nil {
			t.Error(err)
		}
	}}

	if repaired, err := RepairStreamSources(ctx, f.cfg); err != nil || repaired != 0 {
		t.Fatalf("RepairStreamSources = %d, %v; want nothing repaired", repaired, err)
	}
	if b, leftover := f.source(t, job.ID); b != "ts-bytes" || leftover {
		t.Fatalf("source = %q, temp left %v; want the acquire's file untouched and no temp", b, leftover)
	}
}

func TestRepairStreamSourcesNeedsTheSharedLocks(t *testing.T) {
	f := newRepairFixture(t, streamclips.Job{ID: uuid.New(), Status: streamclips.StatusReady})
	f.cfg.Locks = nil
	if _, err := RepairStreamSources(context.Background(), f.cfg); err == nil {
		t.Fatal("RepairStreamSources without locks succeeded, want an error")
	}
}
