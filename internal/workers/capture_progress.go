package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/tickcut/internal/artifacts"
	"github.com/rechedev9/tickcut/internal/recording"
	"github.com/rechedev9/tickcut/internal/storage"
)

const captureProgressPollInterval = 250 * time.Millisecond

type captureProgressReporter struct {
	store      storage.Storage
	jobID      uuid.UUID
	attemptID  uuid.UUID
	segmentDir string
	segmentIDs []string
}

func newCaptureProgressReporter(store storage.Storage, jobID, attemptID uuid.UUID, segmentDir string, segmentIDs []string) *captureProgressReporter {
	return &captureProgressReporter{
		store:      store,
		jobID:      jobID,
		attemptID:  attemptID,
		segmentDir: segmentDir,
		segmentIDs: append([]string(nil), segmentIDs...),
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
	progress, err := recording.NewCaptureProgress(r.attemptID, r.segmentIDs, completed, time.Now())
	if err != nil {
		return err
	}
	body, err := json.Marshal(progress)
	if err != nil {
		return err
	}
	return r.store.Put(artifacts.CaptureProgressKey(r.jobID), bytes.NewReader(body))
}
