package filecommit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitReplacesDestinationAndCleansAttempt(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "final.mp4")
	if err := os.WriteFile(destination, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	attempt, cleanup, err := Attempt(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if filepath.Ext(attempt) != ".mp4" {
		t.Fatalf("attempt extension = %q, want .mp4", filepath.Ext(attempt))
	}
	if err := os.WriteFile(attempt, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Commit(attempt, destination); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "new" {
		t.Fatalf("destination = %q, want new", body)
	}
	if _, err := os.Stat(attempt); !os.IsNotExist(err) {
		t.Fatalf("attempt remains after commit: %v", err)
	}
}

func TestCommitRejectsEmptyAttemptWithoutChangingDestination(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "final.mp4")
	if err := os.WriteFile(destination, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	attempt, cleanup, err := Attempt(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := os.WriteFile(attempt, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Commit(attempt, destination); err == nil {
		t.Fatal("Commit error = nil, want empty attempt rejected")
	}
	body, _ := os.ReadFile(destination)
	if string(body) != "old" {
		t.Fatalf("destination = %q, want old", body)
	}
}

func TestCommitRejectsNonRegularAttemptWithoutChangingDestination(t *testing.T) {
	for _, tt := range []struct {
		name    string
		prepare func(*testing.T, string)
	}{
		{
			name: "directory",
			prepare: func(t *testing.T, attempt string) {
				t.Helper()
				if err := os.Mkdir(attempt, 0o750); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "symlink",
			prepare: func(t *testing.T, attempt string) {
				t.Helper()
				target := filepath.Join(filepath.Dir(attempt), "target.mp4")
				if err := os.WriteFile(target, []byte("complete"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, attempt); err != nil {
					t.Skipf("symlink creation is unavailable: %v", err)
				}
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			destination := filepath.Join(dir, "final.mp4")
			if err := os.WriteFile(destination, []byte("old"), 0o600); err != nil {
				t.Fatal(err)
			}
			attempt := filepath.Join(dir, "attempt.mp4")
			tt.prepare(t, attempt)

			err := Commit(attempt, destination)
			if err == nil || !strings.Contains(err.Error(), "not a regular file") {
				t.Fatalf("Commit error = %v, want non-regular attempt rejection", err)
			}
			body, readErr := os.ReadFile(destination)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(body) != "old" {
				t.Fatalf("destination = %q, want old", body)
			}
		})
	}
}
