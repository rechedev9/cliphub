// Package cloudbridge polls a ClipHub Portal deployment for approved demo
// submissions and admits them into the local job pipeline exactly as a
// manual upload, so the owner can drive them through Studio unchanged. It
// only ever makes outbound requests; no inbound route is ever opened for it.
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

// All returns a snapshot of every tracked request.
func (s *State) All() []*TrackedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*TrackedRequest, 0, len(s.tracked))
	for _, tr := range s.tracked {
		cp := *tr
		out = append(out, &cp)
	}
	return out
}

// Upsert records tr and durably persists the whole state file.
func (s *State) Upsert(tr *TrackedRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *tr
	s.tracked[tr.CloudRequestID] = &cp
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
