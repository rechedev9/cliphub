package editor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFullDemoTemporaryCleanupRejectsForeignPathsBeforeDeleting(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "work")
	if err := os.Mkdir(work, 0700); err != nil {
		t.Fatal(err)
	}
	item, original := filepath.Join(work, "item-000.nut"), filepath.Join(root, "original.mp4")
	for _, path := range []string{item, original} {
		if err := os.WriteFile(path, []byte("retain"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := removeFullDemoTemporaryFiles(work, []string{item, original}); err == nil {
		t.Fatal("accepted cleanup outside the attempt directory")
	}
	for _, path := range []string{item, original} {
		if body, err := os.ReadFile(path); err != nil || string(body) != "retain" {
			t.Fatalf("changed file before validating complete cleanup set: %s, %v", path, err)
		}
	}
	if err := removeFullDemoTemporaryFiles(root, []string{work}); err == nil {
		t.Fatal("accepted a directory for temporary file cleanup")
	}
}
