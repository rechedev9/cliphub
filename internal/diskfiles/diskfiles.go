// Package diskfiles finds the files the Storage screen lists: every CS2 demo on
// the PC and every MP4 that Studio rendered into its data dir.
package diskfiles

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Kind tells the UI where a file came from and how deleting it behaves.
type Kind string

const (
	KindDemo       Kind = "demo"
	KindStudioDemo Kind = "studio_demo"
	KindShort      Kind = "short"
	KindFullDemo   Kind = "full_demo"
	KindStream     Kind = "stream"
	KindEditor     Kind = "editor"
	KindCloud      Kind = "cloud"
)

// File is one listed file. Path is absolute; JobID names the Studio job that owns it, if any.
type File struct {
	Path    string    `json:"path"`
	Size    int64     `json:"size_bytes"`
	ModTime time.Time `json:"modified_at"`
	Kind    Kind      `json:"kind"`
	JobID   string    `json:"job_id,omitempty"`
}

// fullDemoVariant is editor.PresetGameplayPOV60, kept here so this package stays a leaf.
const fullDemoVariant = "gameplay-pov-60"

// skippedDirNames are system or tooling folders that never hold a user's demos
// and would make a whole-drive walk slow.
var skippedDirNames = map[string]bool{
	"$recycle.bin":              true,
	"system volume information": true,
	"$winreagent":               true,
	"windows":                   true,
	"recovery":                  true,
	"node_modules":              true,
	".git":                      true,
}

// ScanDemos walks roots for *.dem files, skipping excluded dirs, system folders and links.
// Top-level folders are walked in parallel: a whole drive takes over a minute serially.
func ScanDemos(roots []string, excluded []string) []File {
	skip := map[string]bool{}
	for _, dir := range excluded {
		skip[pathKey(dir)] = true
	}
	var (
		mu    sync.Mutex
		files = []File{}
		wg    sync.WaitGroup
		slots = make(chan struct{}, 8)
	)
	walk := func(dir string) {
		found := walkDemos(dir, skip)
		mu.Lock()
		files = append(files, found...)
		mu.Unlock()
	}
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			path := filepath.Join(root, entry.Name())
			if !entry.IsDir() {
				if file, ok := demoFile(path, entry); ok {
					files = append(files, file)
				}
				continue
			}
			if skipDir(path, entry, skip) {
				continue
			}
			wg.Add(1)
			slots <- struct{}{}
			go func() {
				defer func() { <-slots; wg.Done() }()
				walk(path)
			}()
		}
	}
	wg.Wait()
	sortBySize(files)
	return files
}

func walkDemos(dir string, skip map[string]bool) []File {
	files := []File{}
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if path != dir && skipDir(path, entry, skip) {
				return filepath.SkipDir
			}
			return nil
		}
		if file, ok := demoFile(path, entry); ok {
			files = append(files, file)
		}
		return nil
	})
	return files
}

func skipDir(path string, entry fs.DirEntry, skip map[string]bool) bool {
	return entry.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0 ||
		skippedDirNames[strings.ToLower(entry.Name())] || skip[pathKey(path)]
}

func demoFile(path string, entry fs.DirEntry) (File, bool) {
	if entry.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0 || !strings.EqualFold(filepath.Ext(path), ".dem") {
		return File{}, false
	}
	return statFile(path, KindDemo)
}

// ScanStudio lists the demo copies and rendered MP4s inside the data dir.
// Round captures and imported sources are left out: they are inputs, not results.
func ScanStudio(dataDir string) ([]File, error) {
	files := []File{}
	err := filepath.WalkDir(dataDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dataDir, path)
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if isCaptureDir(filepath.ToSlash(rel)) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
			return nil
		}
		kind, jobID, ok := classifyStudioFile(filepath.ToSlash(rel))
		if !ok {
			return nil
		}
		if file, ok := statFile(path, kind); ok {
			file.JobID = jobID
			files = append(files, file)
		}
		return nil
	})
	sortBySize(files)
	return files, err
}

// isCaptureDir matches jobs/<id>/recording: round captures are large and never listed.
func isCaptureDir(rel string) bool {
	parts := strings.Split(rel, "/")
	return len(parts) == 3 && parts[0] == "jobs" && parts[2] == "recording"
}

// classifyStudioFile maps a data-dir relative path to its kind and owning job.
func classifyStudioFile(rel string) (Kind, string, bool) {
	parts := strings.Split(rel, "/")
	ext := strings.ToLower(filepath.Ext(rel))
	switch {
	case ext == ".dem" && len(parts) == 2 && parts[0] == "demos":
		return KindStudioDemo, strings.TrimSuffix(parts[1], filepath.Ext(parts[1])), true
	case ext == ".dem" && len(parts) == 3 && parts[0] == "library" && parts[2] == "demo.dem":
		return KindStudioDemo, "", true
	case ext != ".mp4":
		return "", "", false
	case parts[0] == "jobs" && len(parts) >= 6 && parts[2] == "renders" && parts[len(parts)-2] == "videos":
		if parts[3] == fullDemoVariant {
			return KindFullDemo, parts[1], true
		}
		return KindShort, parts[1], true
	case parts[0] == "jobs" && len(parts) == 4 && parts[2] == "composition":
		return KindShort, parts[1], true
	case parts[0] == "stream-jobs" && len(parts) == 3 && parts[2] == "summary.mp4":
		return KindStream, parts[1], true
	case parts[0] == "stream-jobs" && len(parts) >= 6 && parts[2] == "renders" && parts[len(parts)-2] == "videos":
		return KindStream, parts[1], true
	case parts[0] == "editor-jobs" && len(parts) >= 4 && parts[2] == "renders":
		return KindEditor, parts[1], true
	case parts[0] == "cloud" && len(parts) >= 3 && parts[1] == "results":
		return KindCloud, "", true
	}
	return "", "", false
}

// Siblings are the cover and caption a render writes next to videos/<name>.mp4;
// deleting the video removes them too so the reel leaves no orphan files.
func Siblings(videoPath string) []string {
	dir := filepath.Dir(videoPath)
	if filepath.Base(dir) != "videos" {
		return nil
	}
	name := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	parent := filepath.Dir(dir)
	return []string{
		filepath.Join(parent, "covers", name+".jpg"),
		filepath.Join(parent, "captions", name+".caption.txt"),
	}
}

func statFile(path string, kind Kind) (File, bool) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return File{}, false
	}
	return File{Path: path, Size: info.Size(), ModTime: info.ModTime().UTC(), Kind: kind}, true
}

func sortBySize(files []File) {
	sort.SliceStable(files, func(i, j int) bool { return files[i].Size > files[j].Size })
}

// pathKey compares paths the way Windows does: cleaned and case-insensitive.
func pathKey(path string) string {
	return strings.ToLower(filepath.Clean(path))
}
