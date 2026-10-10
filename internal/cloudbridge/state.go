// Package cloudbridge is the unattended ClipHub cloud worker. It claims one
// job at a time from a ClipHub Portal deployment, runs it through the local
// pipeline exactly as a user in Studio would (download, parse, capture,
// render), and uploads the finished videos. It only ever makes outbound
// requests; no inbound route is ever opened for it.
package cloudbridge

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/rechedev9/cliphub/internal/storage"
)

// ErrNotTracked is returned by Update for a cloud request the state file does
// not hold.
var ErrNotTracked = errors.New("cloudbridge: request not tracked")

const (
	stateReplaceAttempts = 5
	stateReplaceDelay    = 20 * time.Millisecond
)

// The two phases a claimed job goes through on this machine. The portal uses
// the same words for the job's status.
const (
	phaseRunning   = "running"
	phaseUploading = "uploading"
)

// TrackedArtifact is one result video of a job and how far its upload got.
type TrackedArtifact struct {
	Variant    string `json:"variant"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	SHA256     string `json:"sha256"`
	ArtifactID string `json:"artifact_id,omitempty"`
	Done       bool   `json:"done,omitempty"`
}

// TrackedRequest is one cloud job this worker holds. It is persisted from
// the claim on, so a restart reports an interrupted capture instead of
// losing it, and resumes an upload instead of capturing again.
type TrackedRequest struct {
	CloudRequestID string `json:"cloud_request_id"`
	LocalJobID     string `json:"local_job_id"`
	// Phase is empty only for entries written by the manual bridge that
	// preceded the worker. They are kept, and so are their local jobs.
	Phase     string    `json:"phase,omitempty"`
	Attempt   int       `json:"attempt,omitempty"`
	ClaimedAt time.Time `json:"claimed_at,omitzero"`
	DemoPath  string    `json:"demo_path,omitempty"`
	// MachineSeconds is the time the attempt had used when it was last
	// saved; a restart reports it with the interrupted capture.
	MachineSeconds int               `json:"machine_seconds,omitempty"`
	Artifacts      []TrackedArtifact `json:"artifacts,omitempty"`
	// PhaseReported is true once the portal has acknowledged the move to
	// uploading; until then the job still holds the portal's capture slot.
	PhaseReported   bool      `json:"phase_reported,omitempty"`
	UploadStartedAt time.Time `json:"upload_started_at,omitzero"`

	// Fields of the manual bridge. Nothing reads them any more; they are
	// declared so an old state file round-trips without losing them.
	LocalJobReported  bool     `json:"local_job_reported,omitempty"`
	UploadedArtifacts []string `json:"uploaded_artifacts,omitempty"`
	Completed         bool     `json:"completed,omitempty"`
}

func (tr TrackedRequest) clone() TrackedRequest {
	tr.Artifacts = slices.Clone(tr.Artifacts)
	tr.UploadedArtifacts = slices.Clone(tr.UploadedArtifacts)
	return tr
}

// State is the worker's small local tracking file. It is a crash-recovery
// aid, not a source of truth: the portal's own job rows remain authoritative
// for status.
type State struct {
	mu      sync.Mutex
	path    string
	tracked map[string]*TrackedRequest
}

// LoadState reads the state file at path, treating a missing or empty file
// as an empty, freshly-initialized state.
func LoadState(path string) (*State, error) {
	s := &State{path: path, tracked: map[string]*TrackedRequest{}}
	data, err := os.ReadFile(path) //nolint:gosec // path is operator-configured, not user input
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cloudbridge: read state: %w", err)
	}
	if len(data) == 0 {
		return s, nil
	}
	var list []*TrackedRequest
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("cloudbridge: parse state: %w", err)
	}
	for _, tr := range list {
		s.tracked[tr.CloudRequestID] = tr
	}
	return s, nil
}

// All returns a snapshot of every tracked request, oldest claim first. The
// copies are safe to read after the lock is released, but must not be
// written back directly: use Update, so a concurrent writer's fields are
// never clobbered by a stale copy.
func (s *State) All() []TrackedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]TrackedRequest, 0, len(s.tracked))
	for _, tr := range s.tracked {
		out = append(out, tr.clone())
	}
	slices.SortFunc(out, func(a, b TrackedRequest) int {
		if order := a.ClaimedAt.Compare(b.ClaimedAt); order != 0 {
			return order
		}
		return cmp.Compare(a.CloudRequestID, b.CloudRequestID)
	})
	return out
}

// Get returns a copy of one tracked request.
func (s *State) Get(cloudRequestID string) (TrackedRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.tracked[cloudRequestID]
	if !ok {
		return TrackedRequest{}, false
	}
	return entry.clone(), true
}

// Insert records a newly claimed request and durably persists the state. It
// replaces an entry a previous attempt of the same job left behind.
func (s *State) Insert(tr TrackedRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := tr.clone()
	s.tracked[tr.CloudRequestID] = &entry
	return s.saveLocked()
}

// Update applies mutate to the live entry under the lock and persists the
// result, so the runner and the uploader can each own their own fields of
// the same request without a read-modify-write race between them.
func (s *State) Update(cloudRequestID string, mutate func(*TrackedRequest)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.tracked[cloudRequestID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrNotTracked, cloudRequestID)
	}
	mutate(entry)
	return s.saveLocked()
}

// Remove forgets a request this worker is done with. Removing one that is
// not tracked is not an error.
func (s *State) Remove(cloudRequestID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tracked[cloudRequestID]; !ok {
		return nil
	}
	delete(s.tracked, cloudRequestID)
	return s.saveLocked()
}

func (s *State) saveLocked() error {
	list := make([]*TrackedRequest, 0, len(s.tracked))
	for _, tr := range s.tracked {
		list = append(list, tr)
	}
	slices.SortFunc(list, func(a, b *TrackedRequest) int {
		return cmp.Compare(a.CloudRequestID, b.CloudRequestID)
	})
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("cloudbridge: marshal state: %w", err)
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("cloudbridge: create state directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, "."+filepath.Base(s.path)+"-*")
	if err != nil {
		return fmt.Errorf("cloudbridge: create temporary state file: %w", err)
	}
	tempPath := temp.Name()
	keepTemp := true
	defer func() {
		if keepTemp {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := temp.Write(data); err != nil {
		return fmt.Errorf("cloudbridge: write temporary state file: %w", errors.Join(err, temp.Close()))
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("cloudbridge: sync temporary state file: %w", errors.Join(err, temp.Close()))
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("cloudbridge: close temporary state file: %w", err)
	}
	// On Windows a scanner that has the file open for an instant makes the
	// replace fail, so it gets a few tries before the save is given up.
	for try := 1; ; try++ {
		err := storage.ReplaceFile(tempPath, s.path)
		if err == nil {
			break
		}
		if try == stateReplaceAttempts {
			return fmt.Errorf("cloudbridge: replace state file: %w", err)
		}
		time.Sleep(stateReplaceDelay)
	}
	keepTemp = false
	return nil
}
