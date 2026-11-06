package community

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

const settingsFile = "settings.json"

// Settings is the rankings choice. InstallID is random and unrelated to the
// telemetry install id, so the two cannot be joined.
type Settings struct {
	RankingsEnabled bool      `json:"rankings_enabled"`
	InstallID       string    `json:"install_id"`
	LastSubmittedAt time.Time `json:"last_submitted_at,omitzero"`
}

// SettingsStore keeps Settings in one JSON file; rankings are off by default.
type SettingsStore struct {
	path string
	mu   sync.Mutex
}

// NewSettingsStore stores the settings in dir.
func NewSettingsStore(dir string) *SettingsStore {
	return &SettingsStore{path: filepath.Join(dir, settingsFile)}
}

// Load returns the settings, creating an install id the first time.
func (s *SettingsStore) Load() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *SettingsStore) loadLocked() (Settings, error) {
	var out Settings
	b, err := os.ReadFile(s.path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return out, err
	default:
		if err := json.Unmarshal(b, &out); err != nil {
			out = Settings{}
		}
	}
	if !InstallIDPattern.MatchString(out.InstallID) {
		out.InstallID = uuid.NewString()
		if err := s.saveLocked(out); err != nil {
			return out, err
		}
	}
	return out, nil
}

// Update loads, changes and saves the settings under one lock.
func (s *SettingsStore) Update(change func(*Settings)) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, err := s.loadLocked()
	if err != nil {
		return cur, err
	}
	change(&cur)
	return cur, s.saveLocked(cur)
}

func (s *SettingsStore) saveLocked(v Settings) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return writeFileAtomic(s.path, append(b, '\n'))
}
