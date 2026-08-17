package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateDemoProbeFlags(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing demo", args: []string{"demo", "probe", "--out", "p.json"}, wantErr: `missing required flag --demo for "demo probe"`},
		{name: "missing out", args: []string{"demo", "probe", "--demo", "x.dem"}, wantErr: `missing required flag --out for "demo probe"`},
		{name: "ok", args: []string{"demo", "probe", "--demo", "x.dem", "--out", "p.json"}},
		{name: "ok dry-run", args: []string{"demo", "probe", "--demo", "x.dem", "--out", "p.json", "--dry-run"}},
		{name: "unknown flag", args: []string{"demo", "probe", "--demo", "x.dem", "--out", "p.json", "--bogus"}, wantErr: `unknown flag --bogus for "demo probe"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := validateSkillCommand(tc.args)
			if tc.wantErr == "" {
				if got != "" {
					t.Fatalf("validateSkillCommand() = %q, want none", got)
				}
				return
			}
			if got != tc.wantErr {
				t.Fatalf("validateSkillCommand() = %q, want %q", got, tc.wantErr)
			}
		})
	}
}

func TestRunDemoProbeMissingFileJSON(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "playability.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"zv", "demo", "probe", "--demo", filepath.Join(dir, "missing.dem"), "--out", out, "--format", "json"}, &stdout, &stderr, nil, &fakeRunner{})
	if code == exitSuccess {
		t.Fatalf("code = %d, want failure; stdout=%s", code, stdout.String())
	}
	if !strings.Contains(stderr.String()+stdout.String(), "stat demo") && !strings.Contains(stderr.String()+stdout.String(), "open demo") {
		t.Fatalf("output missing file error: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err == nil {
		if envelope["ok"] == true {
			t.Fatalf("json envelope ok=true on missing file: %s", stdout.String())
		}
	}
}
