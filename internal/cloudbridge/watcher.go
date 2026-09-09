package cloudbridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

// DefaultWatchInterval is how often the watcher re-checks tracked jobs for
// newly finished renders. Slower than the poller: a render takes minutes, and
// nothing is waiting on the difference.
const DefaultWatchInterval = 60 * time.Second

// JobLookup reads the local job the bridge admitted. store.JobRepository
// satisfies it.
type JobLookup interface {
	GetMeta(ctx context.Context, id uuid.UUID) (job.Job, error)
}

// ArtifactStore reads render artifacts off local disk. *storage.Local
// satisfies it.
type ArtifactStore interface {
	Open(key string) (io.ReadCloser, error)
	ResolvePath(key string) (string, error)
}

// Watcher sends finished work back: it follows every tracked local job and
// uploads each newly finished reel to the portal as a candidate deliverable.
// It deliberately does not decide which candidate is the real one — a job can
// render several variants and several reels per variant, so the owner picks
// in /admin.
type Watcher struct {
	client   *client
	jobs     JobLookup
	files    ArtifactStore
	state    *State
	interval time.Duration
}

// NewWatcher builds a Watcher against a running ClipHub Portal deployment.
func NewWatcher(baseURL, token string, jobs JobLookup, files ArtifactStore, state *State) *Watcher {
	return &Watcher{
		client:   newClient(baseURL, token),
		jobs:     jobs,
		files:    files,
		state:    state,
		interval: DefaultWatchInterval,
	}
}

// Run watches until ctx is canceled. It sweeps once immediately, so renders
// that finished while the orchestrator was shut down are sent on startup
// rather than an interval later.
func (w *Watcher) Run(ctx context.Context) {
	w.tick(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *Watcher) tick(ctx context.Context) {
	for _, tr := range w.state.All() {
		if tr.Completed || !tr.LocalJobReported {
			// Either already reported terminal, or the poller has not
			// finished handing this one over yet.
			continue
		}
		if err := w.check(ctx, tr); err != nil {
			log.Printf("cloudbridge: watch %s: %v", tr.CloudRequestID, err)
		}
	}
}

// terminalPortalStatuses are the states the portal considers closed. Once a
// request reaches one, nothing the local job does afterwards matters.
var terminalPortalStatuses = []string{"done", "failed", "rejected"}

func (w *Watcher) check(ctx context.Context, tr TrackedRequest) error {
	status, err := w.client.requestStatus(ctx, tr.CloudRequestID)
	if err != nil {
		return fmt.Errorf("read portal status: %w", err)
	}
	if slices.Contains(terminalPortalStatuses, status) {
		return w.state.Update(tr.CloudRequestID, func(entry *TrackedRequest) {
			entry.Completed = true
		})
	}

	jobID, err := uuid.Parse(tr.LocalJobID)
	if err != nil {
		return fmt.Errorf("parse local job id %q: %w", tr.LocalJobID, err)
	}

	j, err := w.jobs.GetMeta(ctx, jobID)
	if errors.Is(err, job.ErrNotFound) {
		// The owner deleted the job in Studio. Nothing will ever finish it,
		// so tell the submitter rather than leaving them on "processing".
		return w.complete(ctx, tr.CloudRequestID, "the operator removed this job before it finished")
	}
	if err != nil {
		return fmt.Errorf("load local job: %w", err)
	}
	if j.Status == job.StatusFailed {
		reason := j.FailureReason
		if reason == "" {
			reason = "the local render failed"
		}
		return w.complete(ctx, tr.CloudRequestID, reason)
	}

	for _, loadout := range renderplan.LoadoutCatalog() {
		if err := w.uploadVariant(ctx, tr.CloudRequestID, jobID, loadout.Variant); err != nil {
			log.Printf("cloudbridge: upload %s variant %s: %v", tr.CloudRequestID, loadout.Variant, err)
		}
	}
	return nil
}

// complete reports a terminal failure and stops tracking the request.
func (w *Watcher) complete(ctx context.Context, cloudRequestID, reason string) error {
	if err := w.client.reportFailure(ctx, cloudRequestID, reason); err != nil {
		return fmt.Errorf("report failure: %w", err)
	}
	return w.state.Update(cloudRequestID, func(entry *TrackedRequest) {
		entry.Completed = true
	})
}

func (w *Watcher) uploadVariant(ctx context.Context, cloudRequestID string, jobID uuid.UUID, variant string) error {
	state, ok, err := w.readVariantState(jobID, variant)
	if err != nil || !ok || !variantPublishable(state) {
		return err
	}

	manifest, ok, err := w.readPackManifest(jobID, variant)
	if err != nil || !ok {
		return err
	}

	for _, item := range manifest.Items {
		if item.Video == "" {
			continue
		}
		if err := w.uploadReel(ctx, cloudRequestID, jobID, variant, item.Video); err != nil {
			return err
		}
	}
	return nil
}

func (w *Watcher) uploadReel(ctx context.Context, cloudRequestID string, jobID uuid.UUID, variant, name string) error {
	marker := variant + "/" + name
	if tracked, ok := w.state.Get(cloudRequestID); ok && slices.Contains(tracked.UploadedArtifacts, marker) {
		return nil
	}

	key, err := artifacts.RenderVariantVideoKey(jobID, variant, name)
	if err != nil {
		return fmt.Errorf("resolve video key: %w", err)
	}
	path, err := w.files.ResolvePath(key)
	if err != nil {
		return fmt.Errorf("resolve video path: %w", err)
	}
	file, err := os.Open(path) //nolint:gosec // path is resolved inside the local storage root
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open reel: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat reel: %w", err)
	}

	log.Printf("cloudbridge: uploading %s reel %s (%d bytes)", cloudRequestID, marker, info.Size())
	if err := w.client.uploadArtifact(ctx, cloudRequestID, variant, name, file, info.Size()); err != nil {
		return fmt.Errorf("upload reel %s: %w", marker, err)
	}
	return w.state.Update(cloudRequestID, func(entry *TrackedRequest) {
		if !slices.Contains(entry.UploadedArtifacts, marker) {
			entry.UploadedArtifacts = append(entry.UploadedArtifacts, marker)
		}
	})
}

func (w *Watcher) readVariantState(jobID uuid.UUID, variant string) (*renderplan.RenderVariantState, bool, error) {
	key, err := artifacts.RenderVariantStatusKey(jobID, variant)
	if err != nil {
		return nil, false, fmt.Errorf("resolve state key: %w", err)
	}
	var state renderplan.RenderVariantState
	ok, err := w.readJSON(key, &state)
	if err != nil || !ok {
		return nil, false, err
	}
	return &state, true, nil
}

func (w *Watcher) readPackManifest(jobID uuid.UUID, variant string) (*editor.PackManifest, bool, error) {
	key, err := artifacts.RenderVariantPackManifestKey(jobID, variant)
	if err != nil {
		return nil, false, fmt.Errorf("resolve manifest key: %w", err)
	}
	var manifest editor.PackManifest
	ok, err := w.readJSON(key, &manifest)
	if err != nil || !ok {
		return nil, false, err
	}
	return &manifest, true, nil
}

// readJSON decodes one storage document, reporting a missing one as absent
// rather than as an error: a variant the owner never rendered simply has no
// documents yet.
func (w *Watcher) readJSON(key string, target any) (bool, error) {
	rc, err := w.files.Open(key)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("open %s: %w", key, err)
	}
	defer rc.Close()
	if err := json.NewDecoder(rc).Decode(target); err != nil {
		return false, fmt.Errorf("decode %s: %w", key, err)
	}
	return true, nil
}

// variantPublishable reports whether a render variant is finished and safe to
// send to the submitter: rendered successfully, and either warning-free or
// with its warnings explicitly reviewed and accepted on this exact revision.
// A variant still queued, rendering, failed, or awaiting review is skipped —
// it may still change.
func variantPublishable(state *renderplan.RenderVariantState) bool {
	if state == nil || state.Status != renderplan.RenderVariantStatusReady {
		return false
	}
	if len(state.Warnings) == 0 {
		return true
	}
	return state.ReviewResolvedFor(state.Warnings)
}
