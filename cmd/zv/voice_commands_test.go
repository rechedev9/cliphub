package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDemoVoiceDryRunAndValidation(t *testing.T) {
	out := filepath.Join(t.TempDir(), "voice-probe.json")
	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantSub  string
	}{
		{
			name:     "missing demo",
			args:     []string{"--steamid", "76561198000000000", "--out", out, "--format", "json"},
			wantCode: exitInvalidArgs,
			wantSub:  "--demo",
		},
		{
			name:     "missing steamid",
			args:     []string{"--demo", "match.dem", "--out", out, "--format", "json"},
			wantCode: exitInvalidArgs,
			wantSub:  "--steamid",
		},
		{
			name:     "dry-run",
			args:     []string{"--demo", "match.dem", "--steamid", "76561198000000000", "--out", out, "--dry-run", "--format", "json"},
			wantCode: exitSuccess,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			code := runDemoVoice(tt.args, &stdout, &stderr)
			if code != tt.wantCode {
				t.Fatalf("code = %d, want %d; stdout=%s stderr=%s", code, tt.wantCode, stdout.String(), stderr.String())
			}
			if tt.wantSub != "" && !strings.Contains(stdout.String()+stderr.String(), tt.wantSub) {
				t.Fatalf("output missing %q: %s%s", tt.wantSub, stdout.String(), stderr.String())
			}
			if tt.wantCode == exitSuccess {
				var result demoVoiceResult
				if err := json.Unmarshal([]byte(stdout.String()), &result); err != nil {
					t.Fatalf("decode: %v\n%s", err, stdout.String())
				}
				if !result.OK || !result.DryRun || result.Executed {
					t.Fatalf("result = %+v", result)
				}
			}
		})
	}
}
