package recording

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type CaptureClip struct {
	Path    string
	Size    int64
	ModTime time.Time
}

// IsFullDemoCaptureClip reports whether rel, a slash path inside a job dir, is a
// Full Demo round clip: recording/revisions/<attempt>/segments/<segment>.mp4.
func IsFullDemoCaptureClip(rel string) bool {
	parts := strings.Split(rel, "/")
	return len(parts) == 5 && parts[0] == "recording" && parts[1] == "revisions" && parts[3] == "segments" && strings.EqualFold(path.Ext(parts[4]), ".mp4")
}

// FullDemoCaptureClips lists the round clips of every capture attempt under
// revisionsDir, a job's local recording/revisions dir.
func FullDemoCaptureClips(revisionsDir string) ([]CaptureClip, error) {
	var clips []CaptureClip
	err := filepath.WalkDir(revisionsDir, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		rel, err := filepath.Rel(revisionsDir, file)
		if err != nil || entry.IsDir() || !IsFullDemoCaptureClip("recording/revisions/"+filepath.ToSlash(rel)) {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		clips = append(clips, CaptureClip{Path: file, Size: info.Size(), ModTime: info.ModTime()})
		return nil
	})
	return clips, err
}

func DeleteCaptureClips(clips []CaptureClip) (int64, error) {
	var freed int64
	for _, clip := range clips {
		if err := os.Remove(clip.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return freed, err
		}
		freed += clip.Size
	}
	return freed, nil
}
