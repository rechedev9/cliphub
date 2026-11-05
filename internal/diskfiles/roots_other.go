//go:build !windows

package diskfiles

import (
	"errors"
	"os"
)

// Roots returns the home dir: outside Windows Studio only runs for development.
func Roots() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{home}
}

// Reveal is only supported on Windows.
func Reveal(string) error {
	return errors.New("reveal in folder is only supported on Windows")
}
