package main

import (
	"strings"
	"testing"
	"time"
)

func TestLoadConfigEnforcesLoopbackAndDisclosedRetention(t *testing.T) {
	setBaseTelemetryEnvironment(t)
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr string
	}{
		{name: "valid"},
		{name: "public wildcard", key: "CLIPHUB_TELEMETRY_PUBLIC_ADDR", value: "0.0.0.0:8120", wantErr: "numeric loopback"},
		{name: "admin public", key: "CLIPHUB_TELEMETRY_ADMIN_ADDR", value: "37.27.20.69:8121", wantErr: "numeric loopback"},
		{name: "hostname rejected", key: "CLIPHUB_TELEMETRY_ADMIN_ADDR", value: "localhost:8121", wantErr: "numeric loopback"},
		{name: "retention mismatch", key: "CLIPHUB_TELEMETRY_RETENTION_DAYS", value: "31", wantErr: "remain 30"},
		{name: "invalid proxy protocol", key: "CLIPHUB_TELEMETRY_PROXY_PROTOCOL", value: "v1", wantErr: "empty or v2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.key != "" {
				t.Setenv(tt.key, tt.value)
			}
			cfg, err := loadConfig()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("loadConfig: %v", err)
				}
				if cfg.retention != 30*24*time.Hour {
					t.Fatalf("retention = %v", cfg.retention)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("loadConfig error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func setBaseTelemetryEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("CLIPHUB_TELEMETRY_PUBLIC_ADDR", "127.0.0.1:8120")
	t.Setenv("CLIPHUB_TELEMETRY_ADMIN_ADDR", "127.0.0.1:8121")
	t.Setenv("CLIPHUB_TELEMETRY_DATABASE", t.TempDir()+"/telemetry.db")
	t.Setenv("CLIPHUB_TELEMETRY_INGEST_KEY", "ingest-key-with-at-least-24-characters")
	t.Setenv("CLIPHUB_TELEMETRY_ADMIN_TOKEN", "admin-token-with-at-least-32-characters-long")
	t.Setenv("CLIPHUB_TELEMETRY_RETENTION_DAYS", "30")
	t.Setenv("CLIPHUB_TELEMETRY_PROXY_PROTOCOL", "")
}
