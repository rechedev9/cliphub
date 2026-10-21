//go:build windows

package winproc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunningFindsThisProcessWithoutCase(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	running, err := Running(strings.ToUpper(filepath.Base(exe)))
	if err != nil || !running {
		t.Fatalf("Running(own image) = %v, %v; want true", running, err)
	}
	running, err = Running("no-such-image-for-winproc.exe")
	if err != nil || running {
		t.Fatalf("Running(missing image) = %v, %v; want false", running, err)
	}
}
