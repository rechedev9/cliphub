package main

import (
	"strings"
	"testing"
)

// Perf / capture flags added by the performance audit must be part of the
// documented value-flag surface for record, short, and shorts render so the
// unified CLI passes them through (zv record / zv shorts render) or forwards
// them onto the recorder/editor stages (zv short).

func TestRecordValueFlagsIncludePerfFlags(t *testing.T) {
	flags := commandValueFlags(`"record"`, []string{"--killplan", "--demo", "--out"})
	for _, want := range []string{"--encoder", "--gap-timescale", "--settle-seconds"} {
		if !containsString(flags, want) {
			t.Errorf("record value flags = %v, missing %s", flags, want)
		}
	}
}

func TestShortValueFlagsIncludePerfFlags(t *testing.T) {
	flags := commandValueFlags(`"short"`, []string{"--prompt"})
	for _, want := range []string{"--encoder", "--gap-timescale", "--settle-seconds", "--threads"} {
		if !containsString(flags, want) {
			t.Errorf("short value flags = %v, missing %s", flags, want)
		}
	}
}

func TestShortsRenderValueFlagsIncludeThreads(t *testing.T) {
	flags := commandValueFlags(`"shorts render"`, []string{"--recording-result", "--out"})
	if !containsString(flags, "--threads") {
		t.Errorf("shorts render value flags = %v, missing --threads", flags)
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