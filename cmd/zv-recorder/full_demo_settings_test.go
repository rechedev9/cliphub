package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/recording"
)

func TestFullDemoWindowSettingsRequireSnapshotAndReadback(t *testing.T) {
	for _, name := range []string{"patched and restored", "already windowed", "changed after snapshot", "missing after snapshot", "nonregular after snapshot"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "cs2_video.txt")
			original := `"VideoConfig" { "setting.fullscreen" "1" "setting.nowindowborder" "1" }`
			if name == "already windowed" {
				original = strings.ReplaceAll(original, `"1"`, `"0"`)
			}
			if err := os.WriteFile(path, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			journal, err := recording.BeginSettingsJournal(filepath.Join(t.TempDir(), "journal.json"), []string{root}, nil)
			if err != nil {
				t.Fatal(err)
			}
			switch name {
			case "changed after snapshot":
				if err := os.WriteFile(path, []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
			case "missing after snapshot", "nonregular after snapshot":
				if err := os.Rename(path, path+".saved"); err != nil {
					t.Fatal(err)
				}
				if name == "nonregular after snapshot" {
					if err := os.Mkdir(path, 0700); err != nil {
						t.Fatal(err)
					}
				}
			}
			err = forceWindowedJournalConfigs(journal)
			wantError := strings.Contains(name, "after snapshot")
			if (err != nil) != wantError {
				t.Fatalf("patch error: %v", err)
			}
			if wantError {
				return
			}
			current, err := os.ReadFile(path)
			if err != nil || strings.Contains(string(current), `"1"`) {
				t.Fatalf("window settings: %q %v", current, err)
			}
			if err := journal.Restore(); err != nil {
				t.Fatal(err)
			}
			current, err = os.ReadFile(path)
			if err != nil || string(current) != original {
				t.Fatalf("restoration: %q %v", current, err)
			}
		})
	}
}
