package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveRetiredWhisperKeepsTheTranscription(t *testing.T) {
	dir := t.TempDir()
	for _, tool := range []string{"whisper", "parakeet"} {
		if err := os.MkdirAll(filepath.Join(dir, "tools", tool, "bin"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "tools", tool, "bin", "model.bin"), []byte(tool), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	removeRetiredWhisper(dir)
	if _, err := os.Stat(filepath.Join(dir, "tools", "whisper")); !os.IsNotExist(err) {
		t.Fatalf("tools/whisper after startup: %v; want it gone", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "tools", "parakeet", "bin", "model.bin")); err != nil {
		t.Fatalf("the Parakeet model went too: %v", err)
	}
	removeRetiredWhisper(dir)
}
