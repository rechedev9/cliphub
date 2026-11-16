package workers

import (
	"cmp"
	"errors"
	"io"
	"log"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/storage"
)

// scannerBuild names the running Studio build: the commit it was built from, or
// its module version when the binary carries no VCS stamp.
var scannerBuild = sync.OnceValue(func() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "devel"
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && s.Value != "" {
			return s.Value
		}
	}
	return cmp.Or(info.Main.Version, "devel")
})

// libraryFailureVersionDependent reports a failure that says more about the
// Studio that read the demo than about the demo, so a newer build may not repeat it.
func libraryFailureVersionDependent(d library.Demo) bool {
	class := d.ErrorCode
	// Builds before the classes were split stored these failures as unreadable.
	if class == tacticalClassDemoUnreadable {
		class = tacticalFailureClass(errors.New(d.Error))
	}
	switch class {
	case obs.ClassDemoIncompatible, tacticalClassInternal, tacticalClassMapUncalibrated:
		return true
	}
	return false
}

// ClaimLibraryRetryAfterUpgrade reports whether a demo an older build could not read is due one
// retry at startup; it records the running build first, so a build never retries a demo twice.
func ClaimLibraryRetryAfterUpgrade(files storage.Storage, d library.Demo) bool {
	if d.State != library.ParseFailed || !libraryFailureVersionDependent(d) {
		return false
	}
	key := artifacts.LibraryScanFailedBuildKey(d.ID)
	recorded := ""
	rc, err := files.Open(key)
	switch {
	case err == nil:
		b, readErr := io.ReadAll(io.LimitReader(rc, 256))
		_ = rc.Close()
		if readErr != nil {
			log.Printf("worker demo=%s read %s: %v", d.ID, key, readErr)
			return false
		}
		recorded = strings.TrimSpace(string(b))
	case !storage.IsNotExist(err):
		log.Printf("worker demo=%s open %s: %v", d.ID, key, err)
		return false
	}
	if recorded == scannerBuild() {
		return false
	}
	if err := files.Put(key, strings.NewReader(scannerBuild())); err != nil {
		log.Printf("worker demo=%s write %s: %v", d.ID, key, err)
		return false
	}
	return true
}
