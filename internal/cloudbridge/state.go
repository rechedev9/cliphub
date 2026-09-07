// Package cloudbridge polls a ClipHub Portal deployment for approved demo
// submissions, admits them into the local job pipeline exactly as a manual
// upload so the owner can drive them through Studio unchanged, and sends the
// resulting renders back. It only ever makes outbound requests; no inbound
// route is ever opened for it.
package cloudbridge

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rechedev9/cliphub/internal/storage"
)

// ErrNotTracked is returned by Update for a cloud request the state file does
// not hold.
var ErrNotTracked = errors.New("cloudbridge: request not tracked")

// TrackedRequest is one cloud request the bridge has admitted locally.
// Persisted so a crash between creating the local Job and confirming that
// back to the portal does not admit the same demo a second time on restart.
type TrackedRequest struct {
	CloudRequestID string `json:"cloud_request_id"`
	LocalJobID     string `json:"local_job_id"`
	// LocalJobReported is false only in the narrow window between the local
	// Job existing and the portal's local-job report succeeding; reconcile
	// retries the report until this is true before claiming new work.
	LocalJobReported bool `json:"local_job_reported"`
	// UploadedArtifacts holds "<variant>/<video name>" for every reel already
	// sent to the portal, so a reel is uploaded once no matter how many times
	// the watcher revisits a finished job.
	UploadedArtifacts []string `json:"uploaded_artifacts,omitempty"`
	// Completed marks a request the watcher has reported a terminal local
	// outcome for and will not revisit.
	Completed bool `json:"completed,omitempty"`
}

// State is the bridge's small local tracking file. It is a crash-recovery
// aid, not a source of truth: the portal's own request rows remain
// authoritative for status.
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

// All returns a snapshot of every tracked request. The copies are safe to
// read after the lock is released, but must not be written back directly —
// use Update, so a concurrent writer's fields are never clobbered by a stale
// copy.
func (s *State) All() []TrackedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]TrackedRequest, 0, len(s.tracked))
	for _, tr := range s.tracked {
		out = append(out, *tr)
	}
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
	return *entry, true
}

// Insert records a newly admitted request and durably persists the state.
func (s *State) Insert(tr TrackedRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := tr
	s.tracked[tr.CloudRequestID] = &entry
	return s.saveLocked()
}

// Update applies mutate to the live entry under the lock and persists the
// result, so the poller and the watcher can each own their own fields of the
// same request without a read-modify-write race between them.
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

func (s *State) saveLocked() error {
	list := make([]*TrackedRequest, 0, len(s.tracked))
	for _, tr := range s.tracked {
		list = append(list, tr)
	}
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
	if err := storage.ReplaceFile(tempPath, s.path); err != nil {
		return fmt.Errorf("cloudbridge: replace state file: %w", err)
	}
	keepTemp = false
	return nil
}
