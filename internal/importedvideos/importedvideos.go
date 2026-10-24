// Package importedvideos keeps the Shorts that older Studios recorded in the
// retired ClipHub cloud and downloaded to <DataDir>/cloud/results, so they stay
// in Resultados as ready local videos.
package importedvideos

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/filecommit"
)

const indexSchemaVersion = "cliphub.imported-videos.v1"

// fallbackTitle names a video whose cloud record was lost or unreadable.
const fallbackTitle = "Short grabado en la nube"

var videoNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,119}\.mp4$`)

// ErrNotFound means the id and name do not name a kept video.
var ErrNotFound = errors.New("imported video not found")

// Video is one kept MP4. Its file is always results/<CloudJobID>/<Name>.
type Video struct {
	CloudJobID string    `json:"cloud_job_id"`
	Name       string    `json:"name"`
	Title      string    `json:"title"`
	LocalJobID string    `json:"local_job_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type indexFile struct {
	SchemaVersion string  `json:"schema_version"`
	Videos        []Video `json:"videos"`
}

// The parts of the old cloud client store (cliphub.cloud-client.v1) the migration reads.
type legacyStore struct {
	Submissions []struct {
		CloudJobID string    `json:"cloud_job_id"`
		LocalJobID string    `json:"local_job_id"`
		Title      string    `json:"title"`
		CreatedAt  time.Time `json:"created_at"`
		Videos     []struct {
			Name  string `json:"name"`
			Ready bool   `json:"ready"`
		} `json:"videos"`
	} `json:"submissions"`
}

// Store is <DataDir>/cloud/results/index.json and the files beside it.
type Store struct {
	mu         sync.Mutex
	resultsDir string
	videos     []Video
}

// Open migrates the old cloud client store, if one is left, and loads the index.
// The client store holds the portal device token, so it is deleted once its
// videos are in the index.
func Open(dataDir string) (*Store, error) {
	cloudDir := filepath.Join(dataDir, "cloud")
	s := &Store{resultsDir: filepath.Join(cloudDir, "results")}
	if err := s.load(); err != nil {
		return nil, err
	}
	clientPath := filepath.Join(cloudDir, "client.json")
	if s.adopt(readLegacy(clientPath)) {
		if err := s.save(); err != nil {
			return nil, err
		}
	}
	// A locked client store is retried on the next start; the videos stay listed meanwhile.
	removeLegacy(clientPath)
	return s, nil
}

func (s *Store) indexPath() string {
	return filepath.Join(s.resultsDir, "index.json")
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.indexPath()) //nolint:gosec // path is built from the configured data dir
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read imported videos index: %w", err)
	}
	var file indexFile
	if err := json.Unmarshal(data, &file); err != nil {
		// The files are still on disk; adopt() lists them again under the fallback title.
		log.Printf("importedvideos: %s cannot be read (%v); listing the files again", s.indexPath(), err)
		return nil
	}
	s.videos = file.Videos
	return nil
}

func readLegacy(path string) legacyStore {
	var legacy legacyStore
	data, err := os.ReadFile(path) //nolint:gosec // path is built from the configured data dir
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("importedvideos: read %s: %v", path, err)
		}
		return legacy
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		log.Printf("importedvideos: %s cannot be read (%v); its videos keep a generic title", path, err)
	}
	return legacy
}

// adopt adds every kept MP4 that is on disk and not indexed yet, with its cloud
// record when there is one, and drops index entries whose file is gone. It
// reports whether the index changed.
func (s *Store) adopt(legacy legacyStore) bool {
	type key struct{ id, name string }
	known := map[key]bool{}
	kept := s.videos[:0]
	changed := false
	for _, v := range s.videos {
		if _, err := s.fileOf(v.CloudJobID, v.Name); err != nil || known[key{v.CloudJobID, v.Name}] {
			changed = true
			continue
		}
		known[key{v.CloudJobID, v.Name}] = true
		kept = append(kept, v)
	}
	s.videos = kept

	add := func(v Video) {
		k := key{v.CloudJobID, v.Name}
		if known[k] {
			return
		}
		info, err := s.stat(v.CloudJobID, v.Name)
		if err != nil {
			return
		}
		if v.CreatedAt.IsZero() {
			v.CreatedAt = info.ModTime().UTC()
		}
		known[k] = true
		s.videos = append(s.videos, v)
		changed = true
	}
	for _, sub := range legacy.Submissions {
		for _, video := range sub.Videos {
			if !video.Ready {
				continue
			}
			title := strings.TrimSpace(sub.Title)
			if title == "" {
				title = fallbackTitle
			}
			add(Video{CloudJobID: sub.CloudJobID, Name: video.Name, Title: title, LocalJobID: sub.LocalJobID, CreatedAt: sub.CreatedAt.UTC()})
		}
	}
	// Files whose record was lost, such as a damaged client store.
	dirs, err := os.ReadDir(s.resultsDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Printf("importedvideos: list %s: %v", s.resultsDir, err)
	}
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(s.resultsDir, dir.Name()))
		if err != nil {
			continue
		}
		for _, file := range files {
			add(Video{CloudJobID: dir.Name(), Name: file.Name(), Title: fallbackTitle})
		}
	}
	slices.SortStableFunc(s.videos, func(a, b Video) int { return b.CreatedAt.Compare(a.CreatedAt) })
	return changed
}

func removeLegacy(clientPath string) {
	for _, path := range []string{clientPath, clientPath + ".damaged"} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("importedvideos: remove old cloud client store: %v", err)
		}
	}
}

// fileOf builds the path of a kept video. The id must be a UUID and the name a
// plain MP4 name, so a request can never reach outside the results folder.
func (s *Store) fileOf(cloudJobID, name string) (string, error) {
	id, err := uuid.Parse(cloudJobID)
	if err != nil || id.String() != cloudJobID || !videoNamePattern.MatchString(name) {
		return "", ErrNotFound
	}
	return filepath.Join(s.resultsDir, cloudJobID, name), nil
}

func (s *Store) stat(cloudJobID, name string) (fs.FileInfo, error) {
	path, err := s.fileOf(cloudJobID, name)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return nil, ErrNotFound
	}
	return info, nil
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(indexFile{SchemaVersion: indexSchemaVersion, Videos: s.videos}, "", "  ")
	if err != nil {
		return err
	}
	attempt, cleanup, err := filecommit.Attempt(s.indexPath())
	if err != nil {
		return fmt.Errorf("write imported videos index: %w", err)
	}
	defer cleanup()
	if err := os.WriteFile(attempt, data, 0o600); err != nil {
		return fmt.Errorf("write imported videos index: %w", err)
	}
	return filecommit.Commit(attempt, s.indexPath())
}

// List returns the kept videos, newest first.
func (s *Store) List() []Video {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.videos)
}

// Open opens the file of one indexed video; any other id or name is ErrNotFound.
func (s *Store) Open(cloudJobID, name string) (*os.File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.indexOf(cloudJobID, name) < 0 {
		return nil, ErrNotFound
	}
	path, err := s.fileOf(cloudJobID, name)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path) //nolint:gosec // fileOf builds the path from a UUID and a validated name
	if err != nil {
		return nil, ErrNotFound
	}
	return file, nil
}

// Delete removes one kept video, its file and, once empty, its folder.
func (s *Store) Delete(cloudJobID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.indexOf(cloudJobID, name)
	if i < 0 {
		return ErrNotFound
	}
	path, err := s.fileOf(cloudJobID, name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove imported video: %w", err)
	}
	// Fails while other videos of the job are left, which is the point.
	_ = os.Remove(filepath.Dir(path))
	s.videos = slices.Delete(s.videos, i, i+1)
	return s.save()
}

func (s *Store) indexOf(cloudJobID, name string) int {
	return slices.IndexFunc(s.videos, func(v Video) bool { return v.CloudJobID == cloudJobID && v.Name == name })
}
