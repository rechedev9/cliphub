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

// demoinfocsModule is the parser module whose version alone decides whether a demo parses.
const demoinfocsModule = "github.com/markus-wa/demoinfocs-golang/v5"

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

// parserVersion names the demoinfocs version the running build links.
var parserVersion = sync.OnceValue(func() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "devel"
	}
	for _, dep := range info.Deps {
		if dep.Path != demoinfocsModule {
			continue
		}
		if dep.Replace != nil {
			dep = dep.Replace
		}
		return cmp.Or(dep.Version, "devel")
	}
	return "devel"
})

// scanIdentity is what library/<id>/scan-failed-build records about the Studio that failed or retried a demo.
type scanIdentity struct {
	parser string
	build  string
}

func currentScanIdentity() scanIdentity {
	return scanIdentity{parser: parserVersion(), build: scannerBuild()}
}

func (s scanIdentity) String() string {
	return "parser=" + s.parser + ";build=" + s.build
}

// parseScanIdentity reads a recorded identity; a plain value from an older build names only that build.
func parseScanIdentity(text string) scanIdentity {
	head, build, ok := strings.Cut(text, ";build=")
	parser, tagged := strings.CutPrefix(head, "parser=")
	if !ok || !tagged {
		return scanIdentity{build: text}
	}
	return scanIdentity{parser: parser, build: build}
}

// libraryRetryClass returns the class of a failure that says more about the Studio
// that read the demo than about the demo, so a newer Studio may not repeat it.
func libraryRetryClass(d library.Demo) (string, bool) {
	class := d.ErrorCode
	// Builds before the classes were split stored these failures as unreadable.
	if class == tacticalClassDemoUnreadable {
		class = tacticalFailureClass(errors.New(d.Error))
	}
	switch class {
	case obs.ClassDemoIncompatible, tacticalClassInternal, tacticalClassMapUncalibrated:
		return class, true
	}
	return "", false
}

// libraryRetryDue reports whether what reads a demo of this class changed since the recorded failure:
// only the parser reads an incompatible demo, while the rest of Studio places and encodes the others.
func libraryRetryDue(class string, recorded, current scanIdentity) bool {
	if class == obs.ClassDemoIncompatible {
		return recorded.parser != current.parser
	}
	return recorded.build != current.build
}

// ClaimLibraryRetryAfterUpgrade reports whether a demo an older Studio could not read is due one
// retry at startup; it records the running identity first, so a Studio never retries a demo twice.
func ClaimLibraryRetryAfterUpgrade(files storage.Storage, d library.Demo) bool {
	if d.State != library.ParseFailed {
		return false
	}
	class, ok := libraryRetryClass(d)
	if !ok {
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
	current := currentScanIdentity()
	if !libraryRetryDue(class, parseScanIdentity(recorded), current) {
		return false
	}
	if err := files.Put(key, strings.NewReader(current.String())); err != nil {
		log.Printf("worker demo=%s write %s: %v", d.ID, key, err)
		return false
	}
	return true
}
