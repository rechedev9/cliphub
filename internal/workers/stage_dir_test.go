package workers

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSweepStageDirsRemovesOnlyWhatPrepareStageDirCreates(t *testing.T) {
	base := t.TempDir()
	var stale []string
	for _, stage := range []string{"record", "render", "stream-render"} {
		// With a root the stage directory is kept, like one whose deferred
		// removal never ran because the process was killed.
		dir, _, err := prepareStageDir(base, uuid.New(), stage)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "take0000.mp4"), []byte("raw capture"), 0o600); err != nil {
			t.Fatal(err)
		}
		stale = append(stale, dir)
	}
	keep := []string{
		filepath.Join(base, "zv-workers-test-obs-123"),
		filepath.Join(base, "zv-record-not-a-job-123"),
		filepath.Join(base, "someone-elses-temp"),
	}
	for _, dir := range keep {
		if err := os.Mkdir(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	fileWithStageName := filepath.Join(base, "zv-record-"+uuid.NewString()+"-1")
	if err := os.WriteFile(fileWithStageName, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	removed, err := sweepStageDirs(base)
	if err != nil || removed != len(stale) {
		t.Fatalf("sweepStageDirs = %d, %v, want %d removed", removed, err, len(stale))
	}
	for _, dir := range stale {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("stale stage directory %s is still there", dir)
		}
	}
	for _, path := range append(keep, fileWithStageName) {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s was removed, but no stage created it: %v", path, err)
		}
	}
}

// A capture that was just stopped can hold a file open for a moment. The
// stage directory is several gigabytes, so its removal must outlast that.
func TestStageDirCleanupOutlastsAFileThatIsStillOpen(t *testing.T) {
	dir, cleanup, err := prepareStageDir("", uuid.New(), "record")
	if err != nil {
		t.Fatal(err)
	}
	held, err := os.Create(filepath.Join(dir, "take0000.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	released := make(chan struct{})
	go func() {
		time.Sleep(700 * time.Millisecond)
		_ = held.Close()
		close(released)
	}()

	cleanup()

	<-released
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("the stage directory is still there after its cleanup (stat error %v)", err)
	}
}
