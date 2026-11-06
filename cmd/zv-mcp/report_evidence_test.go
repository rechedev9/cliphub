package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReportEvidenceReadsTheStudioProfile(t *testing.T) {
	dir := t.TempDir()
	s := &studio{sessionPath: filepath.Join(dir, "agent-session.json")}
	if got := s.reportEvidence(); got != (reportEvidence{}) {
		t.Fatalf("empty profile: got %+v, want no evidence", got)
	}

	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("agent-session.json", `{"version":1,"enabled":true,"studioVersion":"5.10.0"}`)
	write("telemetry.json", `{"supportCode":"CH-1A2B-3C4D-5E6F-7A8B-9C0D","installationId":"x"}`)
	write("studio.log", "boot\n")

	want := reportEvidence{studioVersion: "5.10.0", supportCode: "CH-1A2B-3C4D-5E6F-7A8B-9C0D", logPath: filepath.Join(dir, "studio.log")}
	if got := s.reportEvidence(); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	write("telemetry.json", `{"supportCode":"not a code"}`)
	if got := s.reportEvidence().supportCode; got != "" {
		t.Fatalf("malformed support code: got %q, want empty", got)
	}
}
