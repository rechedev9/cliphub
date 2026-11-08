package workers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/streamclips"
)

type streamSourceRepairRepository interface {
	ListByStatus(ctx context.Context, status streamclips.Status) ([]streamclips.Job, error)
	SetSourceProbe(ctx context.Context, id uuid.UUID, probe streamclips.SourceProbe, sha256 string) error
}

// StreamSourceRepairConfig configures RepairStreamSources.
type StreamSourceRepairConfig struct {
	Repo       streamSourceRepairRepository
	Storage    storage.Storage
	Prober     streamclips.Prober
	FFmpegPath string
	// Locks is shared with the stream render worker, so a render never reads a source mid-remux.
	Locks  *streamclips.JobLocks
	runner commandRunner
}

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
	r := streamSourceRepairer{cfg: cfg, resolve: resolver.ResolvePath, runner: cfg.runner}
	if r.runner == nil {
		r.runner = execCommandRunner{}
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
}

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
	release := r.cfg.Locks.Lock(j.ID)
	defer release()
	probe, err := r.cfg.Prober.Probe(ctx, path)
	if err != nil {
		return false, fmt.Errorf("probe source: %w", err)
	}
	if probe.Container == "" {
		return false, nil
	}
	sha := j.SourceSHA256
	remuxed := probe.NeedsRemux()
	if remuxed {
		if err := streamclips.RemuxToMP4(ctx, streamclips.RemuxOptions{FFmpeg: r.cfg.FFmpegPath, Input: path, Run: r.runner.Run}); err != nil {
			return false, err
		}
		if probe, err = r.cfg.Prober.Probe(ctx, path); err != nil {
			return false, fmt.Errorf("probe remuxed source: %w", err)
		}
		if sha, err = sha256File(path); err != nil {
			return false, fmt.Errorf("hash remuxed source: %w", err)
		}
	}
	return remuxed, r.cfg.Repo.SetSourceProbe(ctx, j.ID, probe, sha)
}
