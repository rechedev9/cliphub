package workers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/streamclips"
)

type streamSourceRepairRepository interface {
	ListByStatus(ctx context.Context, status streamclips.Status) ([]streamclips.Job, error)
	Get(ctx context.Context, id uuid.UUID) (streamclips.Job, error)
	SetSourceProbe(ctx context.Context, id uuid.UUID, probe streamclips.SourceProbe, sha256 string) error
}

// StreamSourceRepairConfig configures RepairStreamSources.
type StreamSourceRepairConfig struct {
	Repo       streamSourceRepairRepository
	Storage    storage.Storage
	Prober     streamclips.Prober
	FFmpegPath string
	// Locks is shared with the stream handlers and workers, so a swap never races a delete, an acquire or a render claim.
	Locks  *streamclips.JobLocks
	runner commandRunner
	swap   sourceSwapRetry
}

// sourceSwapRetry bounds the wait for readers, such as the play scan's FFmpeg, to close the old file.
type sourceSwapRetry struct {
	attempts int
	wait     time.Duration
	rename   func(oldpath, newpath string) error
}

var defaultSourceSwapRetry = sourceSwapRetry{attempts: 60, wait: 15 * time.Second, rename: os.Rename}

// errSourceInUse is a failed swap: on Windows a reader that holds the old file open blocks the rename.
var errSourceInUse = errors.New("stream source is in use")

// Statuses whose source is on disk; queued and rendering jobs are left to their render.
var repairableStreamStatuses = []streamclips.Status{
	streamclips.StatusUploaded, streamclips.StatusReady, streamclips.StatusRendered, streamclips.StatusFailed,
}

// RepairStreamSources checks, once per job, the container of every stream source stored
// before SourceProbe.Container existed, and remuxes the MPEG-TS sources that older
// Studios stored as source.mp4. A job that fails is checked again on the next start.
func RepairStreamSources(ctx context.Context, cfg StreamSourceRepairConfig) (int, error) {
	resolver, ok := cfg.Storage.(interface{ ResolvePath(string) (string, error) })
	if !ok || cfg.Prober == nil {
		return 0, nil
	}
	if cfg.Locks == nil {
		return 0, errors.New("stream source repair needs the shared stream job locks")
	}
	r := streamSourceRepairer{cfg: cfg, resolve: resolver.ResolvePath, runner: cfg.runner, swap: cfg.swap}
	if r.runner == nil {
		r.runner = execCommandRunner{}
	}
	if r.swap.rename == nil {
		r.swap = defaultSourceSwapRetry
	}
	repaired := 0
	var errs []error
	for _, status := range repairableStreamStatuses {
		jobs, err := cfg.Repo.ListByStatus(ctx, status)
		if err != nil {
			errs = append(errs, fmt.Errorf("list %s stream jobs: %w", status, err))
			continue
		}
		for _, j := range jobs {
			if j.Probe.Container != "" || j.SourcePath == "" {
				continue
			}
			remuxed, err := r.repair(ctx, j)
			if err != nil {
				errs = append(errs, fmt.Errorf("stream %s: %w", j.ID, err))
				continue
			}
			if remuxed {
				repaired++
				log.Printf("startup: stream %s source remuxed to MP4", j.ID)
			}
		}
	}
	return repaired, errors.Join(errs...)
}

type streamSourceRepairer struct {
	cfg     StreamSourceRepairConfig
	resolve func(key string) (string, error)
	runner  commandRunner
	swap    sourceSwapRetry
}

// sourceSwap is what one repair records: the probe and hash, and the remuxed file that
// replaces the source; from is empty when the source only needs its container recorded.
type sourceSwap struct {
	job      streamclips.Job
	probe    streamclips.SourceProbe
	sha      string
	from, to string
}

// repair probes and remuxes without the job lock, so the editor keeps the old file
// meanwhile; only the swap and the row update in commit hold the lock.
func (r streamSourceRepairer) repair(ctx context.Context, j streamclips.Job) (bool, error) {
	path, err := r.resolve(j.SourcePath)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	probe, err := r.cfg.Prober.Probe(ctx, path)
	if err != nil {
		return false, fmt.Errorf("probe source: %w", err)
	}
	if probe.Container == "" {
		return false, nil
	}
	if !probe.NeedsRemux() {
		_, err := r.commit(ctx, sourceSwap{job: j, probe: probe, sha: j.SourceSHA256})
		return false, err
	}
	remuxed := path + ".remux.mp4"
	defer func() { _ = os.Remove(remuxed) }()
	if err := streamclips.RemuxToMP4(ctx, streamclips.RemuxOptions{FFmpeg: r.cfg.FFmpegPath, Input: path, Output: remuxed, Run: r.runner.Run}); err != nil {
		return false, err
	}
	if probe, err = r.cfg.Prober.Probe(ctx, remuxed); err != nil {
		return false, fmt.Errorf("probe remuxed source: %w", err)
	}
	if probe.NeedsRemux() {
		return false, fmt.Errorf("%w: still %s after remux", streamclips.ErrSourceNotPlayable, probe.Container)
	}
	sha, err := sha256File(remuxed)
	if err != nil {
		return false, fmt.Errorf("hash remuxed source: %w", err)
	}
	s := sourceSwap{job: j, probe: probe, sha: sha, from: remuxed, to: path}
	for attempt := 1; ; attempt++ {
		swapped, err := r.commit(ctx, s)
		if !errors.Is(err, errSourceInUse) || attempt >= r.swap.attempts {
			return swapped, err
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(r.swap.wait):
		}
	}
}

// commit swaps the file and records the probe under the job lock, unless the job
// moved on since it was listed: deleted, re-acquired, rendering or already checked.
func (r streamSourceRepairer) commit(ctx context.Context, s sourceSwap) (bool, error) {
	release := r.cfg.Locks.Lock(s.job.ID)
	defer release()
	current, err := r.cfg.Repo.Get(ctx, s.job.ID)
	if errors.Is(err, streamclips.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if current.Probe.Container != "" || current.SourcePath != s.job.SourcePath || !slices.Contains(repairableStreamStatuses, current.Status) {
		return false, nil
	}
	if s.from != "" {
		if err := r.swap.rename(s.from, s.to); err != nil {
			return false, fmt.Errorf("%w: %w", errSourceInUse, err)
		}
	}
	return s.from != "", r.cfg.Repo.SetSourceProbe(ctx, s.job.ID, s.probe, s.sha)
}
