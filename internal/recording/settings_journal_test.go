package recording

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsJournalRecovery(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		crash, alive, checkError bool
	}{
		{name: "normal restoration"},
		{name: "killed recorder recovery", crash: true},
		{name: "live owner blocks recovery", crash: true, alive: true},
		{name: "unavailable owner blocks recovery", crash: true, checkError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			cfg := filepath.Join(root, "cfg")
			if err := os.Mkdir(cfg, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(cfg, "cs2_video.txt")
			write := func(path, body string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			write(path, "original settings")
			journalPath := filepath.Join(root, "recovery", "journal.json")
			j, err := BeginSettingsJournal(journalPath, []string{cfg}, nil)
			if err != nil {
				t.Fatal(err)
			}
			var durable SettingsJournal
			b, err := os.ReadFile(journalPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(b, &durable); err != nil || durable.Restored || len(durable.Files) != 1 {
				t.Fatalf("not durable before mutation: %s (%v)", b, err)
			}
			write(path, "capture settings")
			created := filepath.Join(cfg, "cs2_user_convars.vcfg")
			write(created, "created by capture")
			if tc.crash {
				j, err = BeginSettingsJournal(journalPath, []string{cfg}, func(pid int) (bool, error) {
					if pid != os.Getpid() {
						t.Fatalf("owner = %d", pid)
					}
					if tc.checkError {
						return false, errors.New("cannot inspect process")
					}
					return tc.alive, nil
				})
			} else {
				err = j.Restore()
			}
			blocked := tc.alive || tc.checkError
			if (err != nil) != blocked {
				t.Fatalf("restore: %v", err)
			}
			want := "original settings"
			if blocked {
				want = "capture settings"
			}
			b, err = os.ReadFile(path)
			if err != nil || string(b) != want {
				t.Fatalf("settings = %q, %v", b, err)
			}
			if !blocked {
				if _, err := os.Stat(created); !os.IsNotExist(err) {
					t.Fatalf("created config not quarantined: %v", err)
				}
				matches, err := filepath.Glob(filepath.Join(root, "recovery", "created-settings-*", "0", filepath.Base(created)))
				if err != nil || len(matches) != 1 {
					t.Fatalf("missing quarantined original: %v %v", matches, err)
				}
				if err := j.Restore(); err != nil {
					t.Fatalf("idempotent restore: %v", err)
				}
			}
		})
	}
}

func TestSettingsJournalRejectsInvalidRecoveryBeforeWriting(t *testing.T) {
	for _, mutation := range []string{"directory", "traversal", "duplicate", "target directory"} {
		t.Run(mutation, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "settings.cfg")
			if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			j, err := BeginSettingsJournal(filepath.Join(t.TempDir(), "journal.json"), []string{root}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("current"), 0600); err != nil {
				t.Fatal(err)
			}
			switch mutation {
			case "directory":
				j.Directories[0] = filepath.Join(root, "missing")
			case "traversal":
				j.Files = append(j.Files, SavedSettingsFile{Name: "../escape.cfg"})
			case "duplicate":
				j.Files = append(j.Files, j.Files[0])
			case "target directory":
				if err := os.Rename(path, path+".saved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := j.Restore(); err == nil {
				t.Fatal("invalid recovery accepted")
			}
			if mutation != "target directory" {
				b, err := os.ReadFile(path)
				if err != nil || string(b) != "current" {
					t.Fatalf("partial recovery changed bytes: %q %v", b, err)
				}
			}
		})
	}
}
