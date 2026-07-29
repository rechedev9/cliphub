package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDataDirLeaseIsExclusiveAndReleased(t *testing.T) {
	dataDir := t.TempDir()
	first, err := acquireDataDirLease(dataDir)
	if err != nil {
		t.Fatalf("first acquireDataDirLease: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })

	_, err = acquireDataDirLease(dataDir)
	if err == nil || !strings.Contains(err.Error(), "already owned") {
		t.Fatalf("second acquireDataDirLease error = %v, want exclusive-owner rejection", err)
	}

	if _, err := first.file.Seek(0, 0); err != nil {
		t.Fatalf("seek lease owner: %v", err)
	}
	body, err := io.ReadAll(first.file)
	if err != nil {
		t.Fatalf("read lease owner: %v", err)
	}
	var owner dataDirLeaseOwner
	if err := json.Unmarshal(body, &owner); err != nil {
		t.Fatalf("decode lease owner: %v", err)
	}
	if owner.PID != os.Getpid() || owner.StartedAt.IsZero() {
		t.Fatalf("lease owner = %#v, want current process and timestamp", owner)
	}

	if err := first.Close(); err != nil {
		t.Fatalf("close first lease: %v", err)
	}
	second, err := acquireDataDirLease(dataDir)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("close second lease: %v", err)
	}
}

func TestDataDirLeaseRejectsSymlinkWithoutTouchingTarget(t *testing.T) {
	dataDir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	const original = "do not replace"
	if err := os.WriteFile(outside, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dataDir, dataDirLeaseFilename)); err != nil {
		t.Skipf("symlink creation is unavailable: %v", err)
	}

	if lease, err := acquireDataDirLease(dataDir); err == nil {
		_ = lease.Close()
		t.Fatal("acquireDataDirLease accepted a symlink lease entry")
	}
	body, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != original {
		t.Fatalf("outside target = %q, want %q", body, original)
	}
}

func TestDataDirLeaseRejectsHardLinkWithoutTouchingTarget(t *testing.T) {
	dataDir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	const original = "do not replace"
	if err := os.WriteFile(outside, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(outside, filepath.Join(dataDir, dataDirLeaseFilename)); err != nil {
		t.Skipf("hard-link creation is unavailable: %v", err)
	}

	if lease, err := acquireDataDirLease(dataDir); err == nil {
		_ = lease.Close()
		t.Fatal("acquireDataDirLease accepted a hard-linked lease entry")
	}
	body, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != original {
		t.Fatalf("outside target = %q, want %q", body, original)
	}
}
