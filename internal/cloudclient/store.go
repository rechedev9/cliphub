package cloudclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/rechedev9/cliphub/internal/filecommit"
)

const (
	storeSchemaVersion = "cliphub.cloud-client.v1"
	damagedSuffix      = ".damaged"
)

// StoredUser is the account this device is linked to, kept for display while
// the portal is unreachable.
type StoredUser struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// StoredVideo is one result video of a submission and where it lives on disk.
type StoredVideo struct {
	ArtifactID string `json:"artifact_id"`
	Name       string `json:"name"`
	Variant    string `json:"variant"`
	Size       int64  `json:"size"`
	SHA256     string `json:"sha256"`
	Path       string `json:"path,omitempty"`
	// Ready is true once the file is on disk and its sha256 matched.
	Ready bool `json:"ready"`
	// Received is true once the portal knows this Studio has the file.
	Received bool `json:"received,omitempty"`
}

// Submission is one cloud job this Studio submitted.
type Submission struct {
	CloudJobID   string    `json:"cloud_job_id"`
	LocalJobID   string    `json:"local_job_id"`
	Title        string    `json:"title"`
	Kind         string    `json:"kind"`
	CreatedAt    time.Time `json:"created_at"`
	DemoUploaded bool      `json:"demo_uploaded"`
	// UploadPercent is the demo upload progress at the last save.
	UploadPercent int `json:"upload_percent"`
	// LocalFailure is a failure the portal does not know about, such as a
	// demo that could not be uploaded. Empty while the portal owns the status.
	LocalFailure string `json:"local_failure,omitempty"`
	// DownloadMismatches counts result downloads that arrived with the wrong
	// sha256, so restarts do not make the limit start over.
	DownloadMismatches int           `json:"download_mismatches,omitempty"`
	LastPortalView     *PortalJob    `json:"last_portal_view,omitempty"`
	Videos             []StoredVideo `json:"videos"`
}

func (s Submission) clone() Submission {
	s.Videos = slices.Clone(s.Videos)
	if s.LastPortalView != nil {
		view := *s.LastPortalView
		view.Artifacts = slices.Clone(view.Artifacts)
		s.LastPortalView = &view
	}
	return s
}

// AbandonedJob is a portal job this Studio gave up on while the portal still
// counts it as active. It is canceled there as soon as the portal answers.
type AbandonedJob struct {
	CloudJobID string    `json:"cloud_job_id"`
	Since      time.Time `json:"since"`
}

type storeFile struct {
	SchemaVersion string         `json:"schema_version"`
	DeviceToken   string         `json:"device_token,omitempty"`
	User          *StoredUser    `json:"user,omitempty"`
	Submissions   []Submission   `json:"submissions"`
	Abandoned     []AbandonedJob `json:"abandoned_jobs,omitempty"`
}

// Store is <DataDir>/cloud/client.json: the device token and the jobs this
// Studio submitted. It is written with mode 0600 and replaced atomically,
// like the Steam account file.
type Store struct {
	mu   sync.Mutex
	path string
	file storeFile
}

// OpenStore reads the store at path. A missing file is an empty store, and so
// is a damaged one: a file cut short by a power loss must not keep the cloud
// off at every start. The damaged file is kept beside it.
func OpenStore(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path) //nolint:gosec // path is built from the configured data dir
	switch {
	case errors.Is(err, os.ErrNotExist):
		return s, nil
	case err != nil:
		return nil, fmt.Errorf("read cloud client store: %w", err)
	}
	if err := json.Unmarshal(data, &s.file); err != nil {
		s.file = storeFile{}
		aside := path + damagedSuffix
		log.Printf("cloudclient: %s cannot be read (%v); starting with an empty store, this PC has to be linked again", path, err)
		// The next save replaces the file either way, so a failed move is not fatal.
		if moveErr := os.Rename(path, aside); moveErr != nil {
			log.Printf("cloudclient: keep the damaged store as %s: %v", aside, moveErr)
		}
		return s, nil
	}
	if s.file.SchemaVersion != "" && s.file.SchemaVersion != storeSchemaVersion {
		return nil, fmt.Errorf("cloud client store schema %q is not supported", s.file.SchemaVersion)
	}
	return s, nil
}

// Token returns the device token, or "" when this PC is not linked.
func (s *Store) Token() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.file.DeviceToken
}

// User returns the linked account as last seen.
func (s *Store) User() *StoredUser {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file.User == nil {
		return nil
	}
	user := *s.file.User
	return &user
}

// Link stores a new device token and the account it belongs to.
func (s *Store) Link(token string, user StoredUser) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.file.DeviceToken = token
	s.file.User = &user
	return s.saveLocked()
}

// Unlink forgets the device token. Submissions stay: their downloaded videos
// are still on this PC.
func (s *Store) Unlink() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.file.DeviceToken = ""
	s.file.User = nil
	return s.saveLocked()
}

// Submissions returns a copy of every submission, newest first.
func (s *Store) Submissions() []Submission {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Submission, 0, len(s.file.Submissions))
	for _, submission := range s.file.Submissions {
		out = append(out, submission.clone())
	}
	slices.SortStableFunc(out, func(a, b Submission) int { return b.CreatedAt.Compare(a.CreatedAt) })
	return out
}

// Submission returns a copy of one submission.
func (s *Store) Submission(cloudJobID string) (Submission, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := s.indexLocked(cloudJobID)
	if index < 0 {
		return Submission{}, false
	}
	return s.file.Submissions[index].clone(), true
}

// Add records a new submission.
func (s *Store) Add(submission Submission) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if submission.Videos == nil {
		submission.Videos = []StoredVideo{}
	}
	s.file.Submissions = append(s.file.Submissions, submission.clone())
	return s.saveLocked()
}

// Update applies mutate to one submission and saves. It reports false when
// the submission is gone, which a background task must treat as "stop".
func (s *Store) Update(cloudJobID string, mutate func(*Submission)) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := s.indexLocked(cloudJobID)
	if index < 0 {
		return false, nil
	}
	mutate(&s.file.Submissions[index])
	return true, s.saveLocked()
}

// Refresh applies mutate to one submission in memory only. It is for values
// the portal resends on every poll (progress, queue estimate): the next save
// writes them, and after a restart the first poll brings them back.
func (s *Store) Refresh(cloudJobID string, mutate func(*Submission)) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := s.indexLocked(cloudJobID)
	if index < 0 {
		return false
	}
	mutate(&s.file.Submissions[index])
	return true
}

// Remove deletes one submission record.
func (s *Store) Remove(cloudJobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := s.indexLocked(cloudJobID)
	if index < 0 {
		return nil
	}
	s.file.Submissions = slices.Delete(s.file.Submissions, index, index+1)
	return s.saveLocked()
}

// Abandon records a portal job to cancel once the portal can be reached.
func (s *Store) Abandon(cloudJobID string, since time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.abandonedIndexLocked(cloudJobID) >= 0 {
		return nil
	}
	s.file.Abandoned = append(s.file.Abandoned, AbandonedJob{CloudJobID: cloudJobID, Since: since.UTC()})
	return s.saveLocked()
}

// Abandoned returns the portal jobs still waiting for their cancel.
func (s *Store) Abandoned() []AbandonedJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.file.Abandoned)
}

// IsAbandoned reports whether a portal job still waits for its cancel.
func (s *Store) IsAbandoned(cloudJobID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.abandonedIndexLocked(cloudJobID) >= 0
}

// Settle forgets an abandoned job: it is canceled, or gone from the portal.
func (s *Store) Settle(cloudJobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := s.abandonedIndexLocked(cloudJobID)
	if index < 0 {
		return nil
	}
	s.file.Abandoned = slices.Delete(s.file.Abandoned, index, index+1)
	return s.saveLocked()
}

func (s *Store) abandonedIndexLocked(cloudJobID string) int {
	return slices.IndexFunc(s.file.Abandoned, func(abandoned AbandonedJob) bool {
		return abandoned.CloudJobID == cloudJobID
	})
}

func (s *Store) indexLocked(cloudJobID string) int {
	return slices.IndexFunc(s.file.Submissions, func(submission Submission) bool {
		return submission.CloudJobID == cloudJobID
	})
}

func (s *Store) saveLocked() error {
	s.file.SchemaVersion = storeSchemaVersion
	if s.file.Submissions == nil {
		s.file.Submissions = []Submission{}
	}
	// #nosec G117 -- client.json intentionally persists the revocable device
	// token; it is written with 0600 permissions.
	data, err := json.MarshalIndent(s.file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode cloud client store: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return fmt.Errorf("create cloud client directory: %w", err)
	}
	attempt, cleanup, err := filecommit.Attempt(s.path)
	if err != nil {
		return fmt.Errorf("stage cloud client store: %w", err)
	}
	defer cleanup()
	if err := writeFlushed(attempt, data); err != nil {
		return fmt.Errorf("write cloud client store: %w", err)
	}
	if err := filecommit.Commit(attempt, s.path); err != nil {
		return fmt.Errorf("commit cloud client store: %w", err)
	}
	return nil
}

// writeFlushed writes data and flushes it to disk before returning, so the
// rename that follows can never publish a file whose content is still only
// in the page cache.
func writeFlushed(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec // path is the attempt file beside the store
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
