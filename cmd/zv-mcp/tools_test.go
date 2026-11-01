package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestParseAgentSession(t *testing.T) {
	token := strings.Repeat("ab", 32)
	cases := []struct {
		name string
		raw  string
		want error
	}{
		{"valid", `{"version":1,"enabled":true,"orchestratorUrl":"http://127.0.0.1:4100","token":"` + token + `"}`, nil},
		{"switch off", `{"version":1,"enabled":false}`, errAgentOff},
		{"unknown version", `{"version":2,"enabled":true}`, errStudioClosed},
		{"not json", `{`, errStudioClosed},
	}
	for _, tc := range cases {
		_, err := parseAgentSession([]byte(tc.raw))
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, err, tc.want)
		}
	}
	for _, raw := range []string{
		`{"version":1,"enabled":true,"orchestratorUrl":"http://evil.example:80","token":"` + token + `"}`,
		`{"version":1,"enabled":true,"orchestratorUrl":"http://127.0.0.1:4100","token":"short"}`,
	} {
		if _, err := parseAgentSession([]byte(raw)); err == nil {
			t.Errorf("accepted invalid session %s", raw)
		}
	}
}

func TestCleanNameStripsControlAndCaps(t *testing.T) {
	if got := cleanName(" s1mple‮\n\x00 "); got != "s1mple" {
		t.Errorf("got %q", got)
	}
	long := strings.Repeat("x", 200)
	if got := []rune(cleanName(long)); len(got) != maxNameRunes+1 || got[maxNameRunes] != '…' {
		t.Errorf("not capped: %d runes", len(got))
	}
}

func TestCheckDemoPath(t *testing.T) {
	dir := t.TempDir()
	demo := filepath.Join(dir, "match.dem")
	if err := os.WriteFile(demo, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkDemoPath(demo); err != nil {
		t.Errorf("valid demo refused: %v", err)
	}
	for _, p := range []string{"match.dem", filepath.Join(dir, "notes.txt"), filepath.Join(dir, "missing.dem")} {
		if checkDemoPath(p) == nil {
			t.Errorf("accepted %s", p)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "folder.dem"), 0o700); err != nil {
		t.Fatal(err)
	}
	if checkDemoPath(filepath.Join(dir, "folder.dem")) == nil {
		t.Error("accepted a directory")
	}
}

func TestJobRunning(t *testing.T) {
	for _, s := range []string{"queued", "parsing", "recording", "composing"} {
		if !jobRunning(s, nil) {
			t.Errorf("%s should be running", s)
		}
	}
	for _, s := range []string{"parsed", "done", "failed", "review_required", "recorded"} {
		if jobRunning(s, nil) {
			t.Errorf("%s should be settled", s)
		}
	}
	if !jobRunning("recorded", &tuiclient.JobProgress{Percent: 40}) {
		t.Error("recorded with render progress should be running")
	}
}

func TestApprovalCodeBindsJobPresetAndSegments(t *testing.T) {
	base := approvalCode("job", "viral-60-clean", []string{"seg-1", "seg-2"})
	for _, other := range []string{
		approvalCode("job2", "viral-60-clean", []string{"seg-1", "seg-2"}),
		approvalCode("job", "other", []string{"seg-1", "seg-2"}),
		approvalCode("job", "viral-60-clean", []string{"seg-1"}),
	} {
		if other == base {
			t.Error("approval code did not change with its inputs")
		}
	}
}
