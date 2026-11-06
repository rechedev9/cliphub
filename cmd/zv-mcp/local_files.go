package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// checkNewLocalFile accepts an absolute local-drive path with the given
// extension, in an existing folder, for a file that does not exist yet.
func checkNewLocalFile(path, ext string) error {
	if strings.HasPrefix(path, `\`) || strings.HasPrefix(path, "//") {
		return errors.New("network (UNC) paths are not accepted; use a folder on a local drive")
	}
	if runtime.GOOS == "windows" && !localDrivePath(path) {
		return errors.New(`path must start with a local drive letter, such as C:\`)
	}
	if !filepath.IsAbs(path) {
		return errors.New("path must be an absolute path")
	}
	if !strings.EqualFold(filepath.Ext(path), ext) {
		return fmt.Errorf("path must end in %s", ext)
	}
	dir, err := os.Stat(filepath.Dir(path))
	if err != nil || !dir.IsDir() {
		return errors.New("the folder of that path does not exist")
	}
	if _, err := os.Lstat(path); err == nil {
		return errors.New("a file with that name already exists; pick another name")
	}
	return nil
}

// writeNewFile writes data to a file that must not exist yet.
func writeNewFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return errors.New("cannot create that file; check the folder and the name")
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return errors.New("cannot write that file")
	}
	return f.Close()
}

// readJSONFile reads a file checkLocalFile accepted and requires it to be JSON.
func readJSONFile(path string) (json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("cannot read that file; check the path")
	}
	if !json.Valid(data) {
		return nil, errors.New("the file is not valid JSON")
	}
	return data, nil
}
