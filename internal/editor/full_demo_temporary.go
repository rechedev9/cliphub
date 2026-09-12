package editor

import (
	"fmt"
	"os"
	"path/filepath"
)

// Once every timeline item has been committed, its mixed PCM owns the audio.
// Keep the original assets and diagnostic files; release only generated buses.
func releaseFullDemoAudio(short ShortEdit) error {
	runtime := short.fullDemo
	paths := append([]string{}, runtime.voicePaths...)
	if runtime.playlist != "" {
		paths = append(paths, runtime.playlist)
		for i := range short.FullDemo.Effective.Options.Audio.Music.Assets {
			paths = append(paths, filepath.Join(runtime.workDir, fmt.Sprintf("music-%d.wav", i)))
		}
	}
	return removeFullDemoTemporaryFiles(runtime.workDir, paths)
}

// The committed program is the sole input to all mastering attempts. Retaining
// the prepared items here adds another full video's worth of disk usage.
func releaseFullDemoItems(short ShortEdit) error {
	return removeFullDemoTemporaryFiles(short.fullDemo.workDir, short.fullDemo.preparedInputs)
}

func removeFullDemoTemporaryFiles(dir string, paths []string) error {
	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	// Validate the complete set before removing anything. These are individual
	// generated files in one attempt directory, never durable input directories.
	for _, path := range paths {
		absolute, err := filepath.Abs(path)
		if err != nil || filepath.Dir(absolute) != root {
			return fmt.Errorf("Full Demo temporary file is outside its work directory: %s", path)
		}
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect Full Demo temporary file: %w", err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Full Demo temporary path is not a regular file: %s", path)
		}
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("release Full Demo temporary file: %w", err)
		}
	}
	return nil
}
