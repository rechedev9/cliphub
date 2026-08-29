package telemetry

import (
	"strings"
	"testing"
	"time"
)

func TestValidateBatch(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	valid := Event{
		SchemaVersion: SchemaVersion,
		ID:            "b838ff59-f6a1-49fc-b11d-7478550238d1",
		OccurredAt:    now,
		Kind:          KindError,
		SupportCode:   "CH-ABCD-1234-5678-90AB-CDEF",
		SessionID:     "a20a070d-c99f-4744-a613-5759d6ecc74c",
		Release:       "2.4.35",
		Component:     "orchestrator",
		Name:          "pipeline.error",
		Stage:         "render",
		Class:         "render:variant",
		OS:            "win32",
		Arch:          "x64",
	}
	tests := []struct {
		name    string
		mutate  func(*Event)
		wantErr string
	}{
		{name: "valid"},
		{name: "wrong schema", mutate: func(event *Event) { event.SchemaVersion = 2 }, wantErr: "schema_version"},
		{name: "bad support code", mutate: func(event *Event) { event.SupportCode = "person" }, wantErr: "support_code"},
		{name: "missing class", mutate: func(event *Event) { event.Class = "" }, wantErr: "class"},
		{name: "future", mutate: func(event *Event) { event.OccurredAt = now.Add(6 * time.Minute) }, wantErr: "future"},
		{name: "span without duration", mutate: func(event *Event) { event.Kind = KindSpan; event.Class = "" }, wantErr: "duration_ms"},
		{name: "client fingerprint", mutate: func(event *Event) { event.Fingerprint = strings.Repeat("a", 64) }, wantErr: "server generated"},
		{name: "steam id label", mutate: func(event *Event) { event.Component = "76561198000000000" }, wantErr: "allowlisted"},
		{name: "path label", mutate: func(event *Event) { event.Name = "C:/Users/Luis/demo.dem" }, wantErr: "allowlisted"},
		{name: "prompt label", mutate: func(event *Event) { event.Class = "make_all_kills_viral" }, wantErr: "allowlisted"},
		{name: "error outcome", mutate: func(event *Event) { event.Outcome = "ok" }, wantErr: "outcome must be empty"},
		{name: "unstable release label", mutate: func(event *Event) { event.Release = "Luis-build" }, wantErr: "release"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := valid
			if tt.mutate != nil {
				tt.mutate(&event)
			}
			events, err := ValidateBatch(Batch{Events: []Event{event}}, now)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateBatch: %v", err)
				}
				if got := events[0].Fingerprint; len(got) != 64 {
					t.Fatalf("server fingerprint = %q", got)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateBatch error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestCanonicalFingerprintUsesOnlyBoundedLabels(t *testing.T) {
	t.Parallel()
	event := Event{
		Kind: "error", Release: "2.4.35", Component: "renderer",
		Name: "route.error", Stage: "renderer", Class: "exception",
	}
	first := canonicalFingerprint(event)
	second := canonicalFingerprint(event)
	if first != second || len(first) != 64 {
		t.Fatalf("fingerprints = %q and %q", first, second)
	}
	event.Class = "process_gone"
	if first == canonicalFingerprint(event) {
		t.Fatal("different canonical labels produced the same fingerprint")
	}
}
