package workers

import (
	"os"
	"testing"
)

// TestMain redirects observability output to a temp dir so the best-effort obs
// recording in recordTaskFailure never writes data/obs into the source tree.
// It also pins the studio encoder probes to "already probed, disabled" so no
// test result depends on the host machine's ffmpeg/GPU; tests that need NVENC
// opt in through setStudioCaptureEncoderForTest.
func TestMain(m *testing.M) {
	studioEncoders.captureProbed = true
	studioEncoders.renderProbed = true
	obsDir, _ := os.MkdirTemp("", "zv-workers-test-obs-")
	if obsDir != "" {
		os.Setenv("ZV_DATA_DIR", obsDir)
	}
	code := m.Run()
	if obsDir != "" {
		_ = os.RemoveAll(obsDir)
	}
	os.Exit(code)
}
