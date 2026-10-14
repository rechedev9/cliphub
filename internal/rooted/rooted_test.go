package rooted

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWithinRejectsEscape(t *testing.T) {
	dir, err := NewDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dir.Within("../outside"); err == nil {
		t.Fatal("expected escape to fail")
	}
	if _, err := dir.Within("/abs"); err == nil {
		t.Fatal("expected absolute key to fail")
	}
	got, err := dir.Within("a/b")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir.Root(), "a", "b")
	if got.String() != want {
		t.Fatalf("path = %q, want %q", got.String(), want)
	}
}

func TestLocalReadWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := WriteFile(path, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	body, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Fatalf("body = %q", body)
	}
	if _, err := Local(""); err == nil {
		t.Fatal("expected empty path to fail")
	}
	f, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
