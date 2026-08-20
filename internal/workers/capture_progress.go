package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/storage"
)

// captureProgressPollInterval bounds how often the watcher re-stats segment
// clips and writes the capture-progress artifact while a capture is running.
// 1s is frequent enough for UI-visible progress without competing with the
// single cs2.exe process and the encoder for I/O during the capture.
const captureProgressPollInterval = time.Second

type captureProgressReporter struct {
	store      storage.Storage
	jobID      uuid.UUID
	attemptID  uuid.UUID
	segmentDir string
	segmentIDs []string
	outDir     string
	tickrate   int
	ticks      []int
	takeSeen   map[string]time.Time
	now        func() time.Time
}

func newCaptureProgressReporter(store storage.Storage, jobID, attemptID uuid.UUID, segmentDir string, segmentIDs []string) *captureProgressReporter {
	return &captureProgressReporter{
		store:      store,
		jobID:      jobID,
		attemptID:  attemptID,
		segmentDir: segmentDir,
		segmentIDs: append([]string(nil), segmentIDs...),
		takeSeen:   map[string]time.Time{},
		now:        time.Now,
	}
}

func startCaptureProgressAttempt(store storage.Storage, jobID uuid.UUID, segmentIDs []string) (uuid.UUID, error) {
	attemptID := uuid.New()
	reporter := newCaptureProgressReporter(store, jobID, attemptID, "", segmentIDs)
	if err := reporter.write(nil); err != nil {
		return uuid.Nil, err
	}
	return attemptID, nil
}

func (r *captureProgressReporter) watch(ctx context.Context) {
	_ = r.report()
	ticker := time.NewTicker(captureProgressPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = r.report()
			return
		case <-ticker.C:
			_ = r.report()
		}
	}
}

func (r *captureProgressReporter) report() error {
	completed := make([]string, 0, len(r.segmentIDs))
	for _, segmentID := range r.segmentIDs {
		path := filepath.Join(r.segmentDir, segmentID+".mp4")
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() && info.Size() > 0 {
			completed = append(completed, segmentID)
		}
	}
	return r.write(completed)
}

func (r *captureProgressReporter) write(completed []string) error {
	progress, err := recording.NewCaptureProgress(r.attemptID, r.segmentIDs, completed, r.clock())
	if err != nil {
		return err
	}
	progress.Percent = r.livePercent(completed)
	body, err := json.Marshal(progress)
	if err != nil {
		return err
	}
	return r.store.Put(artifacts.CaptureProgressKey(r.jobID), bytes.NewReader(body))
}

func (r *captureProgressReporter) clock() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

func (r *captureProgressReporter) livePercent(completed []string) int {
	n := len(r.segmentIDs)
	finished := len(completed)
	frac := 0.0
	if r.outDir != "" {
		takes := recording.LiveTakeNames(r.outDir)
		if len(takes) > 0 {
			newest := takes[len(takes)-1]
			if idx, ok := recording.TakeIndex(newest); ok && idx >= 0 && idx < n {
				video := filepath.Join(r.outDir, newest, "video.mp4")
				info, err := os.Stat(video)
				if err == nil && !info.IsDir() && info.Size() > 0 {
					if finished < idx {
						finished = idx
					}
					frac = r.takeFraction(newest, idx)
				}
			}
		}
	}
	return recording.CaptureWorkPercent(n, finished, r.ticks, frac)
}

func (r *captureProgressReporter) takeFraction(take string, index int) float64 {
	if r.tickrate <= 0 || index >= len(r.ticks) || r.ticks[index] <= 0 {
		return 0
	}
	expected := float64(r.ticks[index]) / float64(r.tickrate)
	if expected <= 0 {
		return 0
	}
	started, ok := r.takeSeen[take]
	if !ok {
		started = r.clock()
		if dirInfo, err := os.Stat(filepath.Join(r.outDir, take)); err == nil {
			if mt := dirInfo.ModTime(); !mt.IsZero() && mt.Before(started) {
				started = mt
			}
		}
		r.takeSeen[take] = started
	}
	frac := r.clock().Sub(started).Seconds() / expected
	if frac < 0 {
		return 0
	}
	if frac > 0.99 {
		return 0.99
	}
	return frac
}
