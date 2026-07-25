package httpapi

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rechedev9/fragforge/internal/obs"
)

func TestHealthHandler(t *testing.T) {
	h := &Handlers{mutationToken: "mutation-secret"}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/healthz", nil)
	req.RemoteAddr = "127.0.0.1:43210"
	h.Health(rr, req)
	if rr.Code != 200 {
		t.Fatalf("status: got %d want 200", rr.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if got, want := body["service"], "fragforge"; got != want {
		t.Errorf("service: got %q want %q", got, want)
	}
	if got, want := body["status"], "ok"; got != want {
		t.Errorf("status: got %q want %q", got, want)
	}
	if len(body) != 2 {
		t.Errorf("health response has extra fields: %q", rr.Body.String())
	}
}

// The discovery handshake is gone, so /healthz must not answer a challenge at
// all. It is served outside the auth middleware, and signing a proof there
// would turn an unauthenticated route into an oracle for whatever credential
// the handler happens to hold.
func TestHealthHandlerNeverProvesIdentityFromACredential(t *testing.T) {
	challenge := strings.Repeat("a", 64)
	for _, remoteAddr := range []string{"127.0.0.1:43210", "[::1]:43210", "203.0.113.20:43210"} {
		h := &Handlers{mutationToken: "mutation-secret"}
		req := httptest.NewRequest("GET", "/healthz?challenge="+challenge, nil)
		req.RemoteAddr = remoteAddr
		listener := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
		req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, listener))
		rr := httptest.NewRecorder()
		h.Health(rr, req)

		var body map[string]string
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode health response for %s: %v", remoteAddr, err)
		}
		if _, ok := body["proof"]; ok {
			t.Errorf("health response for %s carries a proof: %q", remoteAddr, rr.Body.String())
		}
		if _, ok := body["endpoint"]; ok {
			t.Errorf("health response for %s carries an endpoint: %q", remoteAddr, rr.Body.String())
		}
		if strings.Contains(rr.Body.String(), "mutation-secret") {
			t.Fatalf("health response for %s exposed a credential: %q", remoteAddr, rr.Body.String())
		}
	}
}

func TestMetricsHandlerServesCounters(t *testing.T) {
	t.Setenv("ZV_DATA_DIR", t.TempDir())
	rec, err := obs.New(obs.DefaultDir())
	if err != nil {
		t.Fatalf("obs.New: %v", err)
	}
	if err := rec.RecordError(obs.Event{Stage: obs.StageHTTP, Class: "boom", Message: "x"}); err != nil {
		t.Fatalf("RecordError: %v", err)
	}

	h := &Handlers{}
	rr := httptest.NewRecorder()
	h.Metrics(rr, httptest.NewRequest("GET", "/metrics", nil))
	if rr.Code != 200 {
		t.Fatalf("status: got %d want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `fragforge_errors_total{class="boom",stage="http"} 1`) {
		t.Errorf("metrics body missing seeded counter:\n%s", body)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("content-type: got %q", ct)
	}
}
