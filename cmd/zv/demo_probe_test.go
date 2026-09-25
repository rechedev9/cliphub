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
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want JSON mode to keep the reason on stdout", stderr.String())
	}
	var envelope struct {
		OK       bool   `json:"ok"`
		Executed bool   `json:"executed"`
		Error    string `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v\n%s", err, stdout.String())
	}
	if envelope.OK || envelope.Executed || !strings.HasPrefix(envelope.Error, "stat demo: ") {
		t.Fatalf("envelope = %#v, want unexecuted failure with a stat demo reason", envelope)
	}
}
