package verify

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultOrchestratorURL = "http://127.0.0.1:8080"
	HealthzPath            = "/healthz"
	HTTPStatusOK           = "ok"
	HTTPStatusAbsent       = "absent"
	HTTPStatusMismatch     = "mismatch"
	HTTPStatusRejected     = "rejected"
)

// HTTPReport is the /healthz contract check against a running orchestrator.
type HTTPReport struct {
	OK         bool              `json:"ok"`
	Status     string            `json:"status"`
	URL        string            `json:"url"`
	HTTPStatus int               `json:"http_status"`
	Service    string            `json:"service,omitempty"`
	BodyStatus string            `json:"body_status,omitempty"`
	Detail     string            `json:"detail,omitempty"`
	Contract   map[string]string `json:"contract,omitempty"`
}

// ProbeOrchestrator GETs /healthz. Connection refused is absent, not a fake pass
// of the Studio path. A live body must be {"service":"cliphub","status":"ok"}.
func ProbeOrchestrator(baseURL string) HTTPReport {
	report := HTTPReport{
		Contract: map[string]string{
			"service": "cliphub",
			"status":  "ok",
		},
	}
	resolved, err := resolveOrchestratorURL(baseURL)
	if err != nil {
		report.Status = HTTPStatusRejected
		report.Detail = err.Error()
		return report
	}
	report.URL = resolved
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(resolved)
	if err != nil {
		report.Status = HTTPStatusAbsent
		report.OK = true
		report.Detail = err.Error()
		return report
	}
	defer resp.Body.Close()
	report.HTTPStatus = resp.StatusCode
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		report.Status = HTTPStatusMismatch
		report.Detail = fmt.Sprintf("read body: %v", err)
		return report
	}
	var decoded map[string]string
	if err := json.Unmarshal(body, &decoded); err != nil {
		report.Status = HTTPStatusMismatch
		report.Detail = fmt.Sprintf("healthz is not the cliphub JSON contract: %v", err)
		return report
	}
	report.Service = decoded["service"]
	report.BodyStatus = decoded["status"]
	if resp.StatusCode != http.StatusOK || report.Service != "cliphub" || report.BodyStatus != "ok" || len(decoded) != 2 {
		report.Status = HTTPStatusMismatch
		report.Detail = fmt.Sprintf("want 200 {service:cliphub,status:ok} with exactly those keys; got %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
		return report
	}
	report.Status = HTTPStatusOK
	report.OK = true
	return report
}

func resolveOrchestratorURL(baseURL string) (string, error) {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultOrchestratorURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("orchestrator url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("orchestrator url must be http(s)")
	}
	host := parsed.Hostname()
	if !isLoopbackHost(host) {
		return "", fmt.Errorf("orchestrator url must be loopback (127.0.0.1, localhost, or ::1); got %s", host)
	}
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = HealthzPath
	}
	return parsed.String(), nil
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
