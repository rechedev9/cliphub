package verify

import (
	"net/http"
	"testing"
)

func TestInspectLiveAPI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		path     string
		status   int
		body     []byte
		err      error
		wantOK   bool
		wantN    *int
		wantCode string
	}{
		{
			name:   "empty jobs list",
			path:   "/api/jobs",
			status: http.StatusOK,
			body:   []byte(`{"jobs":[]}`),
			wantOK: true,
			wantN:  intPtr(0),
		},
		{
			name:   "jobs present",
			path:   "/api/jobs",
			status: http.StatusOK,
			body:   []byte(`{"jobs":[{"id":"1"},{"id":"2"}]}`),
			wantOK: true,
			wantN:  intPtr(2),
		},
		{
			name:   "capabilities object",
			path:   "/api/capabilities",
			status: http.StatusOK,
			body:   []byte(`{"record":{"enabled":true},"steam":{"enabled":false}}`),
			wantOK: true,
		},
		{
			name:     "faceit gated 503",
			path:     "/api/faceit/followed",
			status:   http.StatusServiceUnavailable,
			body:     []byte(`{"code":"faceit_not_configured","error":"FACEIT follow list is not configured"}`),
			wantOK:   true,
			wantCode: "faceit_not_configured",
		},
		{
			name:   "steam account object with matches",
			path:   "/api/steam/account",
			status: http.StatusOK,
			body:   []byte(`{"steamId":"","authCodeSet":false,"matches":[]}`),
			wantOK: true,
			wantN:  intPtr(0),
		},
		{
			name:   "500 is not ok",
			path:   "/api/jobs",
			status: http.StatusInternalServerError,
			body:   []byte(`{"error":"internal error"}`),
		},
		{
			name:   "503 without code is not ok",
			path:   "/api/jobs",
			status: http.StatusServiceUnavailable,
			body:   []byte(`{"error":"nope"}`),
		},
		{
			name:   "not json",
			path:   "/api/jobs",
			status: http.StatusOK,
			body:   []byte(`<html>`),
		},
		{
			name: "get error",
			path: "/api/jobs",
			err:  errLiveTimeout,
		},
		{
			name: "relative path rejected",
			path: "api/jobs",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			probe := &fakeProbe{jsonStatus: tt.status, jsonBody: tt.body, jsonErr: tt.err}
			got := inspectLiveAPI(probe, "http://127.0.0.1:41001", tt.path)
			if got.OK != tt.wantOK {
				t.Fatalf("ok = %t, want %t; detail=%q", got.OK, tt.wantOK, got.Detail)
			}
			if tt.wantOK && got.Detail != "" && containsFullDemoPass(got.Detail) {
				t.Fatalf("detail leaked Full Demo Pass: %q", got.Detail)
			}
			if tt.wantCode != "" && got.Code != tt.wantCode {
				t.Fatalf("code = %q, want %q", got.Code, tt.wantCode)
			}
			if tt.wantN == nil {
				if got.ItemCount != nil {
					t.Fatalf("item_count = %v, want nil", *got.ItemCount)
				}
				return
			}
			if got.ItemCount == nil || *got.ItemCount != *tt.wantN {
				t.Fatalf("item_count = %v, want %d", got.ItemCount, *tt.wantN)
			}
		})
	}
}

func intPtr(n int) *int { return &n }

type timeoutErr struct{}

func (timeoutErr) Error() string { return "timeout" }

var errLiveTimeout = timeoutErr{}
