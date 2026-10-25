//go:build windows

package storage

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestOpenLocalFileRetriesUntilAnExclusiveHandleIsReleased(t *testing.T) {
	path := filepath.Join(t.TempDir(), "status.json")
	want := []byte(`{"status":"rendering","progress":42}`)
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatalf("write status document: %v", err)
	}
	release := holdFileExclusively(t, path)

	// The lock ends during the second backoff wait, so the open must have been
	// refused twice and still return the document; no timer decides the outcome.
	waits := 0
	file, err := openLocalFileWaiting(path, func(time.Duration) {
		waits++
		if waits == 2 {
			release()
		}
	})
	if err != nil {
		t.Fatalf("openLocalFile during a transient exclusive lock = %v, want success", err)
	}
	got, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if readErr != nil {
		t.Fatalf("read reopened status document: %v", readErr)
	}
	if closeErr != nil {
		t.Fatalf("close reopened status document: %v", closeErr)
	}
	if waits != 2 {
		t.Fatalf("openLocalFile waited %d times, want 2 (refused while locked, opened once released)", waits)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("openLocalFile returned %q, want %q", got, want)
	}
}

func TestOpenLocalFileReturnsPathErrorWhenTheLockOutlastsTheBudget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "status.json")
	if err := os.WriteFile(path, []byte(`{"status":"rendering"}`), 0o600); err != nil {
		t.Fatalf("write status document: %v", err)
	}
	holdFileExclusively(t, path)

	var waited time.Duration
	file, err := openLocalFileWaiting(path, func(d time.Duration) {
		waited += d
		time.Sleep(d)
	})
	if err == nil {
		_ = file.Close()
		t.Fatal("openLocalFile succeeded while the file stayed exclusively locked")
	}
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("openLocalFile error = %v (%T), want *os.PathError", err, err)
	}
	if pathErr.Op != "open" || pathErr.Path != path {
		t.Fatalf("openLocalFile error = %+v, want op \"open\" on %q", pathErr, path)
	}
	if waited == 0 || waited > openRetryBudget {
		t.Fatalf("openLocalFile backed off for %v, want retries within the %v budget", waited, openRetryBudget)
	}
}

// holdFileExclusively opens path with no sharing, the way a virus scanner or
// filesystem filter driver briefly does after a replace. Sharing is enforced
// per handle, so a handle in this process blocks openLocalFile like another
// process would. The returned release is idempotent and also runs at cleanup.
func holdFileExclusively(t *testing.T, path string) func() {
	t.Helper()
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatalf("encode path: %v", err)
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("hold %s exclusively: %v", path, err)
	}
	var once sync.Once
	release := func() { once.Do(func() { _ = syscall.CloseHandle(handle) }) }
	t.Cleanup(release)
	return release
}
