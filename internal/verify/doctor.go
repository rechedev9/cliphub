package verify

// Check is one inspectable capability on this host.
type Check struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// Gap is a named closed failure. Doctor never converts these into a Pass.
type Gap struct {
	ID      string `json:"id"`
	Class   string `json:"class"`
	Stage   string `json:"stage"`
	Message string `json:"message"`
}

// DoctorOptions drives the Windows-first Studio inspection.
type DoctorOptions struct {
	Root     string
	DryRun   bool
	UserData string
	GOOS     string
	GOARCH   string
	Probe    StudioProbe
}

// DoctorReport is the agent-facing inspection of what this host can prove.
type DoctorReport struct {
	SchemaVersion int                `json:"schema_version"`
	OK            bool               `json:"ok"`
	Closed        bool               `json:"closed"`
	DryRun        bool               `json:"dry_run,omitempty"`
	Host          Host               `json:"host"`
	Studio        StudioSurface      `json:"studio"`
	HLAE          ToolCheck          `json:"hlae"`
	CS2           ProcessCheck       `json:"cs2"`
	Gaps          []Gap              `json:"gaps"`
	Remaining     []Gap              `json:"remaining,omitempty"`
	Available     []Check            `json:"available"`
	Features      []FeatureMapStatus `json:"features"`
	Orchestrator  HTTPReport         `json:"orchestrator"`
	Gates         GateReport         `json:"gates"`
}

// Doctor inspects the repo and the live Studio surface. It Passes only on
// Windows when Studio, HLAE, and cs2.exe are actually up. Linux always
// fail-closes with hlae_cs2_windows_studio.
func Doctor(opts DoctorOptions) DoctorReport {
	probe := opts.Probe
	if probe == nil {
		probe = liveProbe{}
	}
	goos := runtimeGOOS(opts.GOOS)
	goarch := runtimeGOARCH(opts.GOARCH)
	userData := resolveUserData(goos, opts.UserData)
	studio := inspectStudio(goos, userData, probe, opts.DryRun)
	hlaePath, hlaeOK := probe.DetectHLAE()
	hlae := ToolCheck{Path: hlaePath, Detected: hlaeOK}
	if isForbiddenHLAE(hlaePath) {
		hlae.Detected = false
		hlae.RejectedBare = true
	}
	cs2Running, cs2Err := probe.CS2Running()
	cs2 := ProcessCheck{Image: CS2ImageName, Running: cs2Running}
	if cs2Err != nil {
		cs2.Detail = cs2Err.Error()
		cs2.Running = false
	}

	host := ClassifyHost(HostFacts{
		GOOS:       goos,
		GOARCH:     goarch,
		StudioUp:   studioIsUp(studio),
		HLAE:       hlae.Detected,
		CS2Running: cs2.Running,
	})

	features := InspectFeatureMap(opts.Root)
	gates := InspectGates(true)
	report := DoctorReport{
		SchemaVersion: SchemaVersion,
		DryRun:        opts.DryRun,
		Host:          host,
		Studio:        studio,
		HLAE:          hlae,
		CS2:           cs2,
		Features:      features.Features,
		Orchestrator:  studio.Healthz,
		Gates:         gates,
	}
	if report.Orchestrator.URL == "" && studio.OrchestratorURL != "" {
		report.Orchestrator.URL = studio.OrchestratorURL + HealthzPath
	}

	report.Available = append(report.Available,
		Check{ID: "feature_catalog", Status: statusFromOK(features.OK), Detail: FeatureCatalogSource},
		Check{ID: "hosted_gates", Status: "cataloged", Detail: "CI frontend/backend/infra; no Playwright; no HLAE"},
		Check{ID: "studio_ports", Status: statusFromOK(studio.PortsPresent), Detail: studio.PortsPath},
		Check{ID: "studio_jobs_db", Status: statusFromOK(studio.JobsDBPresent), Detail: studio.JobsDBPath},
		Check{ID: "hlae", Status: statusFromOK(hlae.Detected), Detail: hlae.Path},
		Check{ID: "cs2_running", Status: statusFromOK(cs2.Running), Detail: cs2.Image},
	)
	orchStatus := studio.Healthz.Status
	if orchStatus == "" {
		orchStatus = HTTPStatusAbsent
	}
	report.Available = append(report.Available, Check{
		ID:     "orchestrator_http",
		Status: orchStatus,
		Detail: studio.OrchestratorURL,
	})

	if goos != "windows" {
		report.Gaps = append(report.Gaps, closedGap(ClosedCaptureGapID, "capture", ClosedCaptureGap))
	} else {
		report.Gaps = append(report.Gaps, windowsStudioGaps(studio, hlae, cs2)...)
	}

	report.Remaining = append(report.Remaining, closedGap(OverlayWalkGapID, "studio_walk", OverlayWalkGap))
	report.Closed = len(report.Gaps) > 0
	report.OK = host.CanRecertifyCapture() && features.OK && !opts.DryRun
	if !host.CanRecertifyCapture() {
		report.OK = false
		report.Closed = true
	}
	return report
}

func windowsStudioGaps(studio StudioSurface, hlae ToolCheck, cs2 ProcessCheck) []Gap {
	var gaps []Gap
	if !studio.PortsPresent {
		gaps = append(gaps, closedGap(StudioPortsGapID, "studio", StudioPortsGap))
	}
	if !studio.JobsDBPresent {
		gaps = append(gaps, closedGap(StudioJobsDBGapID, "studio", StudioJobsDBGap))
	}
	if studio.PortsPresent && studio.Healthz.Status != HTTPStatusOK && studio.Healthz.Status != HTTPStatusSkipped {
		gaps = append(gaps, closedGap(StudioDownGapID, "studio", StudioDownGap))
	}
	if !hlae.Detected {
		gaps = append(gaps, closedGap(HLAEMissingGapID, "capture", HLAEMissingGap))
	}
	if !cs2.Running {
		gaps = append(gaps, closedGap(CS2NotRunningGapID, "capture", CS2NotRunningGap))
	}
	return gaps
}

func closedGap(id, stage, message string) Gap {
	return Gap{ID: id, Class: "closed", Stage: stage, Message: message}
}

func statusFromOK(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}
