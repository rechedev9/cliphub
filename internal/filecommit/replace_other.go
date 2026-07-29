//go:build !windows

package filecommit

import "os"

func replace(attempt, destination string) error {
	return os.Rename(attempt, destination)
}
