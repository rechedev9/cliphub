package verify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbeOrchestratorAbsentIsHonest(t *testing.T) {
	report := ProbeOrchestrator("http://127.0.0.1:1")
	if !report.OK || report.Status != HTTPStatusAbsent {
		t.Fatalf("absent probe = %#v, want ok absent", report)
	}
}

func TestProbeOrchestratorContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != HealthzPath {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"service": "cliphub", "status": "ok"})
	}))
	defer server.Close()

	report := ProbeOrchestrator(server.URL)
	if !report.OK || report.Status != HTTPStatusOK {
		t.Fatalf("contract probe = %#v", report)
	}
	if report.Service != "cliphub" || report.BodyStatus != "ok" {
		t.Fatalf("decoded = %#v", report)
	}
}

func TestProbeOrchestratorRejectsMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"service": "other", "status": "ok"})
	}))
	defer server.Close()

	report := ProbeOrchestrator(server.URL)
	if report.OK || report.Status != HTTPStatusMismatch {
		t.Fatalf("mismatch = %#v, want not-ok mismatch", report)
	}
}

func TestProbeOrchestratorRejectsNonLoopback(t *testing.T) {
	report := ProbeOrchestrator("https://example.com/healthz")
	if report.OK || report.Status != HTTPStatusRejected {
		t.Fatalf("non-loopback = %#v, want rejected", report)
	}
	if !strings.Contains(report.Detail, "loopback") {
		t.Fatalf("detail = %q, want loopback", report.Detail)
	}
}

func TestGatesCatalogStaysCheap(t *testing.T) {
	report := InspectGates(true)
	if !report.OK || !report.DryRun || report.Playwright || report.HLAE {
		t.Fatalf("gates = %#v, want cheap dry-run catalog", report)
	}
	if len(report.Gates) != 3 {
		t.Fatalf("gates = %d, want 3 hosted checks", len(report.Gates))
	}
	for _, gate := range report.Gates {
		if gate.Playwright || gate.HLAE {
			t.Fatalf("gate %s added Playwright or HLAE to hosted CI", gate.ID)
		}
		if !gate.Hosted {
			t.Fatalf("gate %s should be hosted", gate.ID)
		}
	}
}
