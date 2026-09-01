package verify

const (
	CaptureUnavailable = "unavailable"
	CaptureStudioLive  = "studio_live"
)

// Host is what this machine can actually recertify.
// Capture recertification is studio_live only: Windows + live Studio + HLAE + running CS2.
type Host struct {
	GOOS                   string `json:"goos"`
	GOARCH                 string `json:"goarch"`
	WindowsStudio          bool   `json:"windows_studio"`
	HLAE                   bool   `json:"hlae"`
	CS2                    bool   `json:"cs2"`
	CaptureRecertification string `json:"capture_recertification"`
}

// HostFacts are the raw probes used to classify recertification.
type HostFacts struct {
	GOOS, GOARCH string
	StudioUp     bool
	HLAE         bool
	CS2Running   bool
}

// InspectHost resolves the live Studio surface on this machine.
// Linux never claims capture recertification, even if stubs exist.
func InspectHost() Host {
	return InspectHostWith(DoctorOptions{})
}

// InspectHostWith applies GOOS/probe overrides used by tests and the CLI.
func InspectHostWith(opts DoctorOptions) Host {
	goos := runtimeGOOS(opts.GOOS)
	goarch := runtimeGOARCH(opts.GOARCH)
	probe := opts.Probe
	if probe == nil {
		probe = liveProbe{}
	}
	userData := resolveUserData(goos, opts.UserData)
	studio := inspectStudio(goos, userData, probe, opts.DryRun)
	hlaePath, hlaeOK := probe.DetectHLAE()
	if isForbiddenHLAE(hlaePath) {
		hlaeOK = false
	}
	cs2, _ := probe.CS2Running()
	return ClassifyHost(HostFacts{
		GOOS:       goos,
		GOARCH:     goarch,
		StudioUp:   studioIsUp(studio),
		HLAE:       hlaeOK,
		CS2Running: cs2,
	})
}

func studioIsUp(studio StudioSurface) bool {
	return studio.PortsPresent && studio.JobsDBPresent && studio.Healthz.Status == HTTPStatusOK
}

// ClassifyHost is the fail-closed rule: recertification needs Windows plus a
// live Studio, detected HLAE, and a running cs2.exe. Tools on disk are not enough.
func ClassifyHost(facts HostFacts) Host {
	windows := facts.GOOS == "windows"
	live := windows && facts.StudioUp && facts.HLAE && facts.CS2Running
	recert := CaptureUnavailable
	if live {
		recert = CaptureStudioLive
	}
	return Host{
		GOOS:                   facts.GOOS,
		GOARCH:                 facts.GOARCH,
		WindowsStudio:          windows && facts.StudioUp,
		HLAE:                   windows && facts.HLAE,
		CS2:                    windows && facts.CS2Running,
		CaptureRecertification: recert,
	}
}

// CanRecertifyCapture is true only when Studio, HLAE, and CS2 are actually up
// on Windows. Cloud Linux never returns true.
func (h Host) CanRecertifyCapture() bool {
	return h.CaptureRecertification == CaptureStudioLive && h.GOOS == "windows"
}
