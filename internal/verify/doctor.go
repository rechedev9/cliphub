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

// DoctorReport is the agent-facing inspection of what this host can prove.
type DoctorReport struct {
	SchemaVersion int                `json:"schema_version"`
	OK            bool               `json:"ok"`
	Closed        bool               `json:"closed"`
	Host          Host               `json:"host"`
	Gaps          []Gap              `json:"gaps"`
	Available     []Check            `json:"available"`
	Skill         SkillReport        `json:"skill"`
	Features      []FeatureMapStatus `json:"features"`
	Orchestrator  HTTPReport         `json:"orchestrator"`
	Gates         GateReport         `json:"gates"`
}

// Doctor inspects the repo and host. It fails closed (OK=false, Closed=true)
// when capture or Full Demo recap cannot be recertified here.
func Doctor(root string, host Host, orchestrator HTTPReport) DoctorReport {
	skill := InspectSkill(root)
	features := InspectFeatureMap(root)
	gates := InspectGates(true)
	report := DoctorReport{
		SchemaVersion: SchemaVersion,
		Host:          host,
		Skill:         skill,
		Features:      features.Features,
		Orchestrator:  orchestrator,
		Gates:         gates,
	}

	report.Available = append(report.Available, Check{
		ID:     "skill",
		Status: statusFromOK(skill.OK),
		Detail: SkillRelPath,
	})
	report.Available = append(report.Available, Check{
		ID:     "feature_map",
		Status: statusFromOK(features.OK),
		Detail: FeatureMapRelDir,
	})
	report.Available = append(report.Available, Check{
		ID:     "hosted_gates",
		Status: "cataloged",
		Detail: "CI frontend/backend/infra; no Playwright; no HLAE",
	})
	orchStatus := orchestrator.Status
	if orchStatus == "" {
		orchStatus = HTTPStatusAbsent
	}
	report.Available = append(report.Available, Check{
		ID:     "orchestrator_http",
		Status: orchStatus,
		Detail: orchestrator.URL,
	})

	if !host.CanRecertifyCapture() {
		report.Gaps = append(report.Gaps, Gap{
			ID:      ClosedCaptureGapID,
			Class:   "closed",
			Stage:   "capture",
			Message: ClosedCaptureGap,
		})
	}
	report.Gaps = append(report.Gaps, Gap{
		ID:      OverlayWalkGapID,
		Class:   "closed",
		Stage:   "studio_walk",
		Message: OverlayWalkGap,
	})

	report.Closed = len(report.Gaps) > 0 && !host.CanRecertifyCapture()
	// Overlay walks stay unproven even when tools are present; that is named,
	// but doctor OK follows capture recertification plus cheap contracts.
	report.OK = host.CanRecertifyCapture() && skill.OK && features.OK
	if !host.CanRecertifyCapture() {
		report.OK = false
		report.Closed = true
	}
	return report
}

func statusFromOK(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}
