// Package captureretention keeps the round captures of the newest rendered
// Full Demos on disk, so editing one of them only renders again.
package captureretention

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/storage"
)

const (
	// DefaultKeep is how many rendered Full Demos keep their captures until the user changes it.
	DefaultKeep = 3
	// MaxKeep bounds the setting: a full match is about 5 GB of captures.
	MaxKeep    = 20
	settingKey = "settings/full-demo-capture-retention.json"
)

type setting struct {
	Keep int `json:"keep"`
}

type pathResolver interface {
	ResolvePath(key string) (string, error)
}

// Load reads the retention set in Almacenamiento, DefaultKeep when never set.
func Load(store storage.Storage) (int, error) {
	rc, err := store.Open(settingKey)
	if storage.IsNotExist(err) {
		return DefaultKeep, nil
	}
	if err != nil {
		return 0, err
	}
	defer rc.Close()
	var s setting
	if err := json.NewDecoder(io.LimitReader(rc, 4096)).Decode(&s); err != nil || s.Keep < 0 || s.Keep > MaxKeep {
		return DefaultKeep, nil
	}
	return s.Keep, nil
}

func Save(store storage.Storage, keep int) error {
	if keep < 0 || keep > MaxKeep {
		return fmt.Errorf("capture retention must be 0 to %d", MaxKeep)
	}
	b, err := json.Marshal(setting{Keep: keep})
	if err != nil {
		return err
	}
	return store.Put(settingKey, bytes.NewReader(b))
}

// ApplyOptions: Busy reports a job whose capture or render may still need its clips.
type ApplyOptions struct {
	Store storage.Storage
	Keep  int
	Busy  func(uuid.UUID) bool
}

// Apply frees the round clips of every rendered Full Demo outside the Keep
// most recently rendered ones. A job it cannot read is skipped, not fatal.
func Apply(opts ApplyOptions) (int64, error) {
	resolver, ok := opts.Store.(pathResolver)
	if !ok {
		return 0, nil
	}
	root, err := resolver.ResolvePath("jobs")
	if err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	type rendered struct {
		id uuid.UUID
		at time.Time
	}
	var jobs []rendered
	var errs []error
	for _, entry := range entries {
		id, err := uuid.Parse(entry.Name())
		if err != nil || !entry.IsDir() {
			continue
		}
		at, ok, err := renderedAt(opts.Store, id)
		if err != nil {
			errs = append(errs, fmt.Errorf("job %s: %w", id, err))
		} else if ok {
			jobs = append(jobs, rendered{id, at})
		}
	}
	slices.SortFunc(jobs, func(a, b rendered) int { return b.at.Compare(a.at) })
	var freed int64
	for _, j := range jobs[min(max(opts.Keep, 0), len(jobs)):] {
		if opts.Busy != nil && opts.Busy(j.id) {
			continue
		}
		n, err := Free(opts.Store, j.id)
		if err != nil && !errors.Is(err, ErrNotCovered) {
			errs = append(errs, fmt.Errorf("job %s: %w", j.id, err))
		}
		freed += n
	}
	return freed, errors.Join(errs...)
}

// InFlight reports a job status whose capture or render may still read its clips.
func InFlight(s job.Status) bool {
	switch s {
	case job.StatusQueued, job.StatusScanning, job.StatusParsing, job.StatusRecording, job.StatusComposing:
		return true
	}
	return false
}

// ErrNotCovered: no ready long video covers the clips, so they are still needed.
var ErrNotCovered = errors.New("no ready Full Demo render covers the captures")

// Free deletes the round clips of every capture attempt of one rendered Full
// Demo; the recording result stays, so a later edit records them again.
func Free(store storage.Storage, id uuid.UUID) (int64, error) {
	resolver, ok := store.(pathResolver)
	if !ok {
		return 0, nil
	}
	dir, err := resolver.ResolvePath(artifacts.FullDemoCaptureRevisionsPrefix(id))
	if err != nil {
		return 0, err
	}
	clips, err := recording.FullDemoCaptureClips(dir)
	if err != nil || len(clips) == 0 {
		return 0, err
	}
	var newest time.Time
	for _, clip := range clips {
		if clip.ModTime.After(newest) {
			newest = clip.ModTime
		}
	}
	if covered, err := Covers(store, id, newest); err != nil {
		return 0, err
	} else if !covered {
		return 0, ErrNotCovered
	}
	return recording.DeleteCaptureClips(clips)
}

// Covers reports whether the Full Demo render is ready, still has its MP4 and
// is newer than every round clip; a later capture whose render never ran still needs its clips.
func Covers(store storage.Storage, id uuid.UUID, newestClip time.Time) (bool, error) {
	at, ok, err := renderedAt(store, id)
	if err != nil || !ok {
		return false, err
	}
	return !newestClip.After(at), nil
}

// renderedAt is when the job's long video became ready, ok only while its MP4 is on disk.
func renderedAt(store storage.Storage, id uuid.UUID) (time.Time, bool, error) {
	key, err := renderplan.RenderVariantStateKey(id, editor.PresetGameplayPOV60)
	if err != nil {
		return time.Time{}, false, err
	}
	rc, err := store.Open(key)
	if storage.IsNotExist(err) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	defer rc.Close()
	var state renderplan.RenderVariantState
	if err := json.NewDecoder(rc).Decode(&state); err != nil {
		return time.Time{}, false, err
	}
	if state.JobID != id || state.Variant != editor.PresetGameplayPOV60 {
		return time.Time{}, false, fmt.Errorf("render state identity does not match its job")
	}
	if state.Status != renderplan.RenderVariantStatusReady {
		return time.Time{}, false, nil
	}
	onDisk, err := videoOnDisk(store, state.ArtifactPrefix)
	return state.UpdatedAt, onDisk, err
}

// videoOnDisk: Almacenamiento can delete the MP4 while status.json keeps saying ready.
func videoOnDisk(store storage.Storage, artifactPrefix string) (bool, error) {
	resolver, ok := store.(pathResolver)
	if !ok || artifactPrefix == "" {
		return false, nil
	}
	dir, err := resolver.ResolvePath(artifactPrefix + "/videos")
	if err != nil {
		return false, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(entries, func(e os.DirEntry) bool {
		return !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".mp4")
	}), nil
}
