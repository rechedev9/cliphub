// Package storage owns the local artifact boundary used by the desktop
// orchestrator for demos, plans, render state, and generated media.
package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rechedev9/cliphub/internal/rooted"
)

// IsNotExist reports whether err means the requested storage key is absent.
func IsNotExist(err error) bool {
	return os.IsNotExist(err)
}

// Storage is the minimal contract for reading and writing artifact blobs.
type Storage interface {
	Put(key string, r io.Reader) error
	Open(key string) (io.ReadCloser, error)
	Exists(key string) (bool, error)
}

// Local implements Storage backed by the local filesystem under a root dir.
type Local struct {
	root rooted.Dir
}

// NewLocal returns a Local rooted at the given absolute or relative path.
// The root directory is created if it does not exist.
func NewLocal(root string) (*Local, error) {
	dir, err := rooted.NewDir(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir.Root(), 0o750); err != nil {
		return nil, err
	}
	return &Local{root: dir}, nil
}

// Put atomically replaces the file at key with r's contents. Readers see either
// the previous complete artifact or the new complete artifact, never a partial
// write.
func (l *Local) Put(key string, r io.Reader) error {
	path, err := l.resolve(key)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path.String())
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("storage: create artifact directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, "."+filepath.Base(path.String())+"-*")
	if err != nil {
		return fmt.Errorf("storage: create temporary artifact: %w", err)
	}
	tempPath := temp.Name()
	keepTemp := true
	defer func() {
		if keepTemp {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := io.Copy(temp, r); err != nil {
		return fmt.Errorf("storage: copy temporary artifact: %w", errors.Join(err, temp.Close()))
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("storage: sync temporary artifact: %w", errors.Join(err, temp.Close()))
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("storage: close temporary artifact: %w", err)
	}
	if err := replaceLocalFile(tempPath, path.String()); err != nil {
		return fmt.Errorf("storage: replace artifact: %w", err)
	}
	keepTemp = false
	return nil
}

// ReplaceFile atomically replaces destinationPath with tempPath's contents,
// using the platform-appropriate primitive (ReplaceFileW on Windows, which is
// more robust than MoveFileEx under concurrent readers or AV scanning than a
// plain rename). Exposed so other packages needing the same
// write-to-temp-then-publish pattern Put uses internally do not have to
// reimplement the Windows-specific half of it.
func ReplaceFile(tempPath, destinationPath string) error {
	return replaceLocalFile(tempPath, destinationPath)
}

// Open returns a ReadCloser for the file at key.
func (l *Local) Open(key string) (io.ReadCloser, error) {
	path, err := l.resolve(key)
	if err != nil {
		return nil, err
	}
	return openLocalFile(path)
}

// ResolvePath returns the absolute filesystem path of the artifact at key,
// whether or not it exists yet. Callers that hand artifacts to external tools
// (ffmpeg reading a multi-gigabyte stream source) use it to avoid copying the
// whole file through Open.
func (l *Local) ResolvePath(key string) (string, error) {
	path, err := l.resolve(key)
	if err != nil {
		return "", err
	}
	return path.String(), nil
}

// List returns the base file names present directly under the directory at key,
// non-recursively. A missing directory yields no names (not an error), so
// callers can list an artifact dir a stage has not written yet.
func (l *Local) List(key string) ([]string, error) {
	dir, err := l.resolve(key)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir.String())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// Delete removes the file at key inside the storage root. A missing key is
// not an error, so deletes are idempotent and safe to retry.
func (l *Local) Delete(key string) error {
	path, err := l.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path.String()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// DeleteTree removes the directory at key and everything under it inside the
// storage root. A missing key is not an error, so tree deletes are idempotent
// and safe to retry. The key is validated exactly like every other operation,
// so an empty or root key ("" resolves to ".") and any traversal key are
// rejected before removal, ensuring the storage root itself is never removed.
func (l *Local) DeleteTree(key string) error {
	path, err := l.resolve(key)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(path.String()); err != nil {
		return err
	}
	return nil
}

// Exists reports whether key exists inside the storage root.
func (l *Local) Exists(key string) (bool, error) {
	path, err := l.resolve(key)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(path.String()); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (l *Local) resolve(key string) (rooted.Path, error) {
	path, err := l.root.Within(key)
	if err == nil {
		return path, nil
	}
	if strings.HasSuffix(err.Error(), "escapes root") {
		return rooted.Path{}, errors.New("storage: key escapes root")
	}
	if strings.Contains(err.Error(), "invalid key") {
		return rooted.Path{}, fmt.Errorf("storage: invalid key %q", key)
	}
	return rooted.Path{}, err
}
