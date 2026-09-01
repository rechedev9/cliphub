package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/storage"
)

const renderProgressPollInterval = time.Second

type renderProgressReporter struct {
	store        storage.Storage
	jobID        uuid.UUID
	progressPath string
	now          func() time.Time
}

func newRenderProgressReporter(store storage.Storage, jobID uuid.UUID, progressPath string) *renderProgressReporter {
	return &renderProgressReporter{
		store:        store,
		jobID:        jobID,
		progressPath: progressPath,
		now:          time.Now,
	}
}

func (r *renderProgressReporter) watch(ctx context.Context) {
	_ = r.report()
	ticker := time.NewTicker(renderProgressPollInterval)
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

func (r *renderProgressReporter) report() error {
	progress, ok, err := readEditorProgressFile(r.progressPath)
	if err != nil || !ok {
		return err
	}
	body, err := json.Marshal(progress)
	if err != nil {
		return err
	}
	return r.store.Put(artifacts.RenderProgressKey(r.jobID), bytes.NewReader(body))
}

func readEditorProgressFile(path string) (editor.EditorProgress, bool, error) {
	if path == "" {
		return editor.EditorProgress{}, false, nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return editor.EditorProgress{}, false, nil
		}
		return editor.EditorProgress{}, false, err
	}
	var progress editor.EditorProgress
	if err := json.Unmarshal(body, &progress); err != nil {
		return editor.EditorProgress{}, false, err
	}
	if err := progress.Validate(); err != nil {
		return editor.EditorProgress{}, false, err
	}
	return progress, true, nil
}
