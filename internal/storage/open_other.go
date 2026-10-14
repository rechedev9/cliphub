//go:build !windows

package storage

import (
	"os"

	"github.com/rechedev9/cliphub/internal/rooted"
)

func openLocalFile(path rooted.Path) (*os.File, error) {
	return path.Open()
}

func replaceLocalFile(tempPath, destinationPath string) error {
	return os.Rename(tempPath, destinationPath)
}
