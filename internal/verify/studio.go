package verify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const forbiddenBareHLAE = `C:\HLAE\HLAE.exe`

// StudioPorts accepts the current single-port contract and the former two-port
// document so doctor can inspect Studio during an upgrade.
type StudioPorts struct {
	Orchestrator int `json:"orchestrator,omitempty"`
	Web          int `json:"web"`
}

// StudioSurface is the live ClipHub Studio layout doctor inspects.
type StudioSurface struct {
	UserDataDir     string       `json:"user_data_dir"`
	PortsPath       string       `json:"ports_path"`
	JobsDBPath      string       `json:"jobs_db_path"`
	PortsPresent    bool         `json:"ports_present"`
	JobsDBPresent   bool         `json:"jobs_db_present"`
	Ports           *StudioPorts `json:"ports,omitempty"`
	OrchestratorURL string       `json:"orchestrator_url,omitempty"`
	WebURL          string       `json:"web_url,omitempty"`
	Healthz         HTTPReport   `json:"healthz"`
}

// ToolCheck is one detected executable. HLAE must never be C:\HLAE\HLAE.exe.
type ToolCheck struct {
	Detected     bool   `json:"detected"`
	Path         string `json:"path,omitempty"`
	RejectedBare bool   `json:"rejected_bare_c_hlae,omitempty"`
}

// ProcessCheck is a running-process probe. CS2 is never inferred from a file path.
type ProcessCheck struct {
	Image   string `json:"image"`
	Running bool   `json:"running"`
	Detail  string `json:"detail,omitempty"`
}

func defaultUserData(goos string) string {
	if override := strings.TrimSpace(os.Getenv("CLIPHUB_STUDIO_USERDATA")); override != "" {
		return override
	}
	if goos == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, StudioAppDir)
		}
	}
	return ""
}

func resolveUserData(goos, override string) string {
	if strings.TrimSpace(override) != "" {
		return override
	}
	return defaultUserData(goos)
}

func studioLayout(userData string) (portsPath, jobsDBPath string) {
	if strings.TrimSpace(userData) == "" {
		return "", ""
	}
	return filepath.Join(userData, StudioPortsFile), filepath.Join(userData, filepath.FromSlash(StudioJobsDBRel))
}

func parseStudioPorts(raw []byte) (StudioPorts, error) {
	var ports StudioPorts
	if err := json.Unmarshal(raw, &ports); err != nil {
		return StudioPorts{}, fmt.Errorf("ports.json: %w", err)
	}
	if !validTCPPort(ports.Web) {
		return StudioPorts{}, fmt.Errorf("ports.json must contain a web port")
	}
	if ports.Orchestrator != 0 && !validTCPPort(ports.Orchestrator) {
		return StudioPorts{}, fmt.Errorf("ports.json contains an invalid legacy orchestrator port")
	}
	return ports, nil
}

func validTCPPort(port int) bool {
	return port >= 1 && port <= 65535
}

func loopbackURL(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

func isForbiddenHLAE(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	normalized := filepath.Clean(path)
	normalized = strings.ReplaceAll(normalized, "/", `\`)
	return strings.EqualFold(normalized, forbiddenBareHLAE)
}

func inspectStudio(goos, userData string, probe StudioProbe, dryRun bool) StudioSurface {
	surface := StudioSurface{UserDataDir: userData}
	if userData == "" {
		return surface
	}
	surface.PortsPath, surface.JobsDBPath = studioLayout(userData)
	if surface.JobsDBPath != "" {
		surface.JobsDBPresent = probe.FileExists(surface.JobsDBPath)
	}
	if surface.PortsPath == "" {
		return surface
	}
	raw, err := probe.ReadFile(surface.PortsPath)
	if err != nil {
		return surface
	}
	ports, err := parseStudioPorts(raw)
	if err != nil {
		return surface
	}
	surface.PortsPresent = true
	surface.Ports = &ports
	servicePort := ports.Web
	if validTCPPort(ports.Orchestrator) {
		servicePort = ports.Orchestrator
	}
	surface.OrchestratorURL = loopbackURL(servicePort)
	surface.WebURL = loopbackURL(ports.Web)
	if dryRun {
		surface.Healthz = HTTPReport{
			Status: HTTPStatusSkipped,
			URL:    surface.OrchestratorURL + HealthzPath,
			Detail: "dry-run: no HTTP",
			OK:     true,
		}
		return surface
	}
	surface.Healthz = probe.Healthz(surface.OrchestratorURL)
	return surface
}

func runtimeGOOS(override string) string {
	if override != "" {
		return override
	}
	return runtime.GOOS
}

func runtimeGOARCH(override string) string {
	if override != "" {
		return override
	}
	return runtime.GOARCH
}
