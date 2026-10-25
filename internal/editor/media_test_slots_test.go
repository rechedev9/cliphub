package editor

import (
	"runtime"
	"testing"
)

// Each FFmpeg encode is already multithreaded, so a few slots keep the CPU
// busy; more only stretch every test toward its own context deadline.
var mediaTestSlots = make(chan struct{}, max(2, runtime.NumCPU()/4))

// mediaTest marks a test that encodes real media: `go test -short` skips it,
// otherwise it runs beside other media tests. Call it before creating any
// context deadline, because the test waits here for a slot. Keep its subtests
// serial: waiting tests hold the -parallel slots a parallel subtest needs.
func mediaTest(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("encodes real media with FFmpeg; runs without -short")
	}
	t.Parallel()
	mediaTestSlots <- struct{}{}
	t.Cleanup(func() { <-mediaTestSlots })
}
