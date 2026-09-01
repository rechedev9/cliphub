package verify

import (
	"runtime"

	"github.com/rechedev9/cliphub/internal/capturetools"
)

const (
	CaptureUnavailable  = "unavailable"
	CaptureToolsPresent = "tools_present"
)

// Host is what this machine can actually recertify.
type Host struct {
	GOOS                   string `json:"goos"`
	GOARCH                 string `json:"goarch"`
	WindowsStudio          bool   `json:"windows_studio"`
	HLAE                   bool   `json:"hlae"`
	CS2                    bool   `json:"cs2"`
	CaptureRecertification string `json:"capture_recertification"`
}

// InspectHost resolves HLAE/CS2 the same way zv capabilities does, then applies
// the Windows Studio gate. A stub HLAE path on Linux is still unavailable.
func InspectHost() Host {
	paths, sources := capturetools.Detect(capturetools.FromEnvironment())
	hlae := capturetools.ResolveTool("ZV_HLAE_PATH", paths.HLAE, sources)
	cs2 := capturetools.ResolveTool("ZV_CS2_PATH", paths.CS2, sources)
	return ClassifyHost(runtime.GOOS, runtime.GOARCH, hlae.Accessible, cs2.Accessible)
}

// ClassifyHost is the fail-closed rule: capture recertification needs Windows
// plus accessible HLAE and CS2. Anything less is unavailable.
func ClassifyHost(goos, goarch string, hlae, cs2 bool) Host {
	windows := goos == "windows"
	tools := windows && hlae && cs2
	recert := CaptureUnavailable
	if tools {
		recert = CaptureToolsPresent
	}
	return Host{
		GOOS:                   goos,
		GOARCH:                 goarch,
		WindowsStudio:          windows,
		HLAE:                   windows && hlae,
		CS2:                    windows && cs2,
		CaptureRecertification: recert,
	}
}

// CanRecertifyCapture is true only when HLAE and CS2 are present on Windows.
func (h Host) CanRecertifyCapture() bool {
	return h.CaptureRecertification == CaptureToolsPresent
}
