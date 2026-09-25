package main

import (
	"strings"
	"testing"
)

// Perf / capture flags added by the performance audit must be part of the
// documented value-flag surface for record, short, and shorts render so the
// unified CLI passes them through (zv record / zv shorts render) or forwards
// them onto the recorder/editor stages (zv short).

func TestValidateSkillCommandAcceptsPerfFlagValues(t *testing.T) {
	for _, command := range [][]string{
		{"record", "--killplan", "plan.json", "--demo", "match.dem", "--out", "recording", "--encoder", "x", "--gap-timescale", "1", "--settle-seconds", "1"},
		{"short", "match.dem", "--prompt", "all kills", "--encoder", "x", "--gap-timescale", "1", "--settle-seconds", "1", "--threads", "4"},
		{"shorts", "render", "--recording-result", "recording-result.json", "--out", "shorts", "--threads", "4"},
	} {
		if issue := validateSkillCommand(command); issue != "" {
			t.Errorf("validateSkillCommand(%q) = %q, want none", command, issue)
		}
	}
}

func TestParseShortArgsAcceptsPerfFlags(t *testing.T) {
	opts, err := parseShortArgs([]string{
		"--from-recording", "recording.json",
		"--prompt", "todas las kills",
		"--encoder", "nvenc-h264",
		"--gap-timescale", "12",
		"--settle-seconds", "1",
		"--threads", "4",
	})
	if err != nil {
		t.Fatalf("parseShortArgs error = %v", err)
	}
	if opts.Encoder != "nvenc-h264" {
		t.Errorf("encoder = %q, want nvenc-h264", opts.Encoder)
	}
	if opts.GapTimescale != 12 {
		t.Errorf("gap timescale = %v, want 12", opts.GapTimescale)
	}
	if opts.SettleSeconds != 1 {
		t.Errorf("settle seconds = %v, want 1", opts.SettleSeconds)
	}
	if opts.Threads != 4 {
		t.Errorf("threads = %d, want 4", opts.Threads)
	}
}

func TestParseShortArgsRejectsNegativePerfFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--from-recording", "recording.json", "--prompt", "x", "--gap-timescale", "-1"},
		{"--from-recording", "recording.json", "--prompt", "x", "--settle-seconds", "-0.5"},
		{"--from-recording", "recording.json", "--prompt", "x", "--threads", "-2"},
	} {
		if _, err := parseShortArgs(args); err == nil {
			t.Errorf("parseShortArgs(%v) succeeded, want negative-value rejection", args)
		} else if !strings.Contains(err.Error(), "must be >= 0") {
			t.Errorf("parseShortArgs(%v) error = %v, want >= 0 message", args, err)
		}
	}
}
