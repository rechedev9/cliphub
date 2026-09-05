package recording

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

const maxSettingsJournalBytes = 32 << 20

type SavedSettingsFile struct {
	Directory int    `json:"directory"`
	Name      string `json:"name"`
	Contents  []byte `json:"contents"`
	Mode      uint32 `json:"mode"`
}

// SettingsJournal is written and synced before CS2 can modify archived user
// settings. Runtime cvar readback handles normal exits; this journal handles a
// killed recorder or game and is recovered before the next capture starts.
type SettingsJournal struct {
	SchemaVersion string              `json:"schema_version"`
	OwnerPID      int                 `json:"owner_pid"`
	StartedAt     time.Time           `json:"started_at"`
	Restored      bool                `json:"restored"`
	Directories   []string            `json:"directories"`
	Files         []SavedSettingsFile `json:"files"`
	path          string
}

func BeginSettingsJournal(journalPath string, directories []string, ownerAlive func(int) (bool, error)) (*SettingsJournal, error) {
	roots := []string{}
	for _, dir := range directories {
		resolved, err := filepath.EvalSymlinks(dir)
		if err != nil {
			return nil, fmt.Errorf("resolve capture settings directory: %w", err)
		}
		absolute, err := filepath.Abs(resolved)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(roots, absolute) {
			roots = append(roots, absolute)
		}
	}
	slices.Sort(roots)
	if len(roots) > 256 {
		return nil, fmt.Errorf("capture settings exceed directory resource limit")
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("pov_contract_failed: no Steam CS2 settings directory could be isolated")
	}
	if err := os.MkdirAll(filepath.Dir(journalPath), 0700); err != nil {
		return nil, err
	}
	if f, err := os.Open(journalPath); err == nil {
		var previous SettingsJournal
		dec := json.NewDecoder(io.LimitReader(f, maxSettingsJournalBytes+1))
		dec.DisallowUnknownFields()
		decodeErr := dec.Decode(&previous)
		if decodeErr == nil && dec.Decode(new(any)) != io.EOF {
			decodeErr = fmt.Errorf("trailing recovery journal data")
		}
		closeErr := f.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("read settings recovery journal: %w", decodeErr)
		}
		if closeErr != nil {
			return nil, closeErr
		}
		previous.path = journalPath
		if previous.SchemaVersion != "1.0" || !slices.Equal(previous.Directories, roots) {
			return nil, fmt.Errorf("settings recovery journal does not match the current CS2 configuration directories")
		}
		if !previous.Restored {
			if ownerAlive == nil {
				return nil, fmt.Errorf("capture recovery requires checking the prior recorder process")
			}
			alive, err := ownerAlive(previous.OwnerPID)
			if err != nil {
				return nil, err
			}
			if alive {
				return nil, fmt.Errorf("another recorder still owns the settings journal")
			}
			if err := previous.Restore(); err != nil {
				return nil, err
			}
		}
		if err := os.Rename(journalPath, journalPath+".recovered-"+uuid.NewString()); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	j := &SettingsJournal{SchemaVersion: "1.0", OwnerPID: os.Getpid(), StartedAt: time.Now().UTC(), Directories: roots, Files: []SavedSettingsFile{}, path: journalPath}
	var size int64
	for index, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !settingsFileName(entry.Name()) {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				return nil, err
			}
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("capture settings must be regular files")
			}
			size += info.Size()
			if info.Size() > 4<<20 || size > 16<<20 || len(j.Files) >= 4096 {
				return nil, fmt.Errorf("capture settings exceed journal resource limit")
			}
			body, err := os.ReadFile(filepath.Join(root, entry.Name()))
			if err != nil {
				return nil, err
			}
			j.Files = append(j.Files, SavedSettingsFile{Directory: index, Name: entry.Name(), Contents: body, Mode: uint32(info.Mode().Perm())})
		}
	}
	b, err := json.Marshal(j)
	if err != nil {
		return nil, err
	}
	if len(b) > maxSettingsJournalBytes {
		return nil, fmt.Errorf("capture settings exceed encoded journal resource limit")
	}
	f, err := os.OpenFile(journalPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	_, writeErr := f.Write(b)
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil {
		return nil, writeErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return j, nil
}

func settingsFileName(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".cfg", ".vcfg", ".txt":
		return true
	}
	return false
}

func (j *SettingsJournal) Path() string { return j.path }

func (j *SettingsJournal) Restore() error {
	if j.Restored {
		return nil
	}
	if j.SchemaVersion != "1.0" || j.path == "" || len(j.Files) > 4096 || len(j.Directories) == 0 || len(j.Directories) > 256 {
		return fmt.Errorf("invalid settings journal")
	}
	for _, root := range j.Directories {
		actual, err := filepath.EvalSymlinks(root)
		if err != nil || !filepath.IsAbs(root) || actual != root {
			return fmt.Errorf("settings recovery directory changed since the snapshot")
		}
	}
	known := map[string]bool{}
	total := 0
	for _, file := range j.Files {
		total += len(file.Contents)
		if total > 16<<20 {
			return fmt.Errorf("settings recovery exceeds journal resource limit")
		}
		if file.Directory < 0 || file.Directory >= len(j.Directories) || file.Name == "" || filepath.Base(file.Name) != file.Name || strings.ContainsAny(file.Name, "/\\:") || !settingsFileName(file.Name) || len(file.Contents) > 4<<20 {
			return fmt.Errorf("invalid settings recovery file")
		}
		path := filepath.Join(j.Directories[file.Directory], file.Name)
		if known[path] {
			return fmt.Errorf("duplicate settings recovery file")
		}
		known[path] = true
		if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
			return fmt.Errorf("settings recovery target became a non-regular file")
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	for _, file := range j.Files {
		path := filepath.Join(j.Directories[file.Directory], file.Name)
		if err := atomicSettingsWrite(path, file.Contents, os.FileMode(file.Mode)&0777); err != nil {
			return fmt.Errorf("restore capture settings: %w", err)
		}
	}
	for index, root := range j.Directories {
		entries, err := os.ReadDir(root)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			path := filepath.Join(root, entry.Name())
			if !settingsFileName(entry.Name()) || known[path] {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("new capture settings must be regular files")
			}
			// New capture-created configs are kept for recovery, never deleted.
			quarantine := filepath.Join(filepath.Dir(j.path), "created-settings-"+uuid.NewString(), fmt.Sprintf("%d", index))
			if err := os.MkdirAll(quarantine, 0700); err != nil {
				return err
			}
			if err := os.Rename(path, filepath.Join(quarantine, entry.Name())); err != nil {
				return err
			}
		}
	}
	j.Restored = true
	b, err := json.Marshal(j)
	if err != nil {
		j.Restored = false
		return err
	}
	if err := atomicSettingsWrite(j.path, b, 0600); err != nil {
		j.Restored = false
		return err
	}
	return nil
}

func atomicSettingsWrite(path string, contents []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cliphub-settings-")
	if err != nil {
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	_, writeErr := tmp.Write(contents)
	if writeErr == nil {
		writeErr = tmp.Sync()
	}
	closeErr := tmp.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp.Name(), path)
}
