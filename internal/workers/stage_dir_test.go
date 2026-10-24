package workers

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

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
