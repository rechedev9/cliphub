//go:build !windows

package storage

import "os"

func openLocalFile(path string) (*os.File, error) {
	// #nosec G304 -- path is resolved under Local.root by the only caller.
	return os.Open(path)
}

func replaceLocalFile(tempPath, destinationPath string) error {
	return os.Rename(tempPath, destinationPath)
}
