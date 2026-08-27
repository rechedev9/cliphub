package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthcheck(t *testing.T) {
	loopback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(loopback.Close)

	cases := []struct {
		name    string
		url     string
		wantErr string
	}{
		{name: "healthy loopback", url: loopback.URL},
		{name: "reject remote", url: "https://cliphub.example/healthz", wantErr: "HTTP loopback"},
		{name: "reject malformed", url: "://bad", wantErr: "HTTP loopback"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := healthcheck(tc.url)
			if tc.wantErr == "" && err != nil {
				t.Fatalf("healthcheck() error = %v, want nil", err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("healthcheck() error = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}
