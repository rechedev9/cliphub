package verify

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/rechedev9/cliphub/internal/capturetools"
)

// StudioProbe is the injectable live-Studio surface. Tests fake it.
// The live probe never writes jobs.db and never POSTs.
type StudioProbe interface {
	ReadFile(path string) ([]byte, error)
	FileExists(path string) bool
	DetectHLAE() (path string, ok bool)
	CS2Running() (bool, error)
	Healthz(baseURL string) HTTPReport
	GetJSON(url string) (status int, body []byte, err error)
}

type liveProbe struct{}

func (liveProbe) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (liveProbe) FileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}

func (liveProbe) DetectHLAE() (path string, ok bool) {
	paths, sources := capturetools.Detect(capturetools.FromEnvironment())
	tool := capturetools.ResolveTool("ZV_HLAE_PATH", paths.HLAE, sources)
	if !tool.Accessible {
		return "", false
	}
	if isForbiddenHLAE(tool.Path) {
		return tool.Path, false
	}
	return tool.Path, true
}

func (liveProbe) CS2Running() (bool, error) {
	return cs2ProcessRunning()
}

func (liveProbe) Healthz(baseURL string) HTTPReport {
	return ProbeOrchestrator(baseURL)
}

func (liveProbe) GetJSON(rawURL string) (int, []byte, error) {
	resolved, err := resolveLoopbackGET(rawURL)
	if err != nil {
		return 0, nil, err
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(resolved)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

func cs2ProcessRunning() (bool, error) {
	if runtime.GOOS != "windows" {
		return false, nil
	}
	return windowsImageRunning(CS2ImageName)
}

func windowsImageRunning(image string) (bool, error) {
	// #nosec G204 -- tasklist executable is fixed and image is a constant.
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq "+image, "/FO", "CSV", "/NH").Output()
	if err != nil {
		return false, fmt.Errorf("tasklist %s: %w", image, err)
	}
	text := strings.TrimSpace(string(out))
	return strings.Contains(strings.ToLower(text), strings.ToLower(image)), nil
}

func resolveLoopbackGET(rawURL string) (string, error) {
	return resolveOrchestratorURL(rawURL)
}
