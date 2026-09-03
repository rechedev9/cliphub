package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Every error body carries a stable `code`; the Studio proxy reserves
// `service_unavailable` for an unreachable orchestrator, so no Go status may
// default to it.
func TestWriteErrorAlwaysCarriesACode(t *testing.T) {
	cases := []struct {
		status int
		want   string
	}{
		{http.StatusBadRequest, "invalid_request"},
		{http.StatusNotFound, "not_found"},
		{http.StatusConflict, "conflict"},
		{http.StatusServiceUnavailable, "not_configured"},
		{http.StatusInternalServerError, "internal_error"},
		{http.StatusTeapot, "error"},
	}
	for _, tc := range cases {
		rw := httptest.NewRecorder()
		writeError(rw, tc.status, "x")
		var body struct {
			Code  string `json:"code"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(rw.Body.Bytes(), &body); err != nil {
			t.Fatalf("%d: %v", tc.status, err)
		}
		if body.Code != tc.want || body.Error != "x" {
			t.Fatalf("%d: body = %+v, want code %q", tc.status, body, tc.want)
		}
	}
	for status, code := range defaultErrorCodes {
		if code == "service_unavailable" {
			t.Fatalf("status %d defaults to the proxy-reserved service_unavailable code", status)
		}
	}
}
