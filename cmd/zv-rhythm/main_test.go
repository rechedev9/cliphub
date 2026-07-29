package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunAnalyzeRejectsOutputAliasingInput(t *testing.T) {
	input := filepath.Join(t.TempDir(), "source.wav")
	if err := os.WriteFile(input, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := runAnalyze([]string{"--input", input, "--out", input})
	if err == nil || !strings.Contains(err.Error(), "--input") {
		t.Fatalf("runAnalyze error = %v, want input alias rejection", err)
	}
	if got, err := os.ReadFile(input); err != nil || string(got) != "source" {
		t.Fatalf("source after rejection = %q, %v", got, err)
	}
}

func TestRunAnalyzeRejectsOutputAliasingKillPlan(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "source.wav")
	killPlan := filepath.Join(root, "plan.json")
	if err := os.WriteFile(input, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(killPlan, []byte("plan"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := runAnalyze([]string{"--input", input, "--killplan", killPlan, "--out", killPlan})
	if err == nil || !strings.Contains(err.Error(), "--killplan") {
		t.Fatalf("runAnalyze error = %v, want kill-plan alias rejection", err)
	}
}

func TestRunAnalyzeRejectsOutputAliasingRecordingResult(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "source.wav")
	result := filepath.Join(root, "recording-result.json")
	if err := os.WriteFile(input, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(result, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := runAnalyze([]string{"--input", input, "--recording-result", result, "--out", result})
	if err == nil || !strings.Contains(err.Error(), "--recording-result") {
		t.Fatalf("runAnalyze error = %v, want recording-result alias rejection", err)
	}
}

func TestRunAnalyzeRejectsAmbiguousTimingPlansBeforeFFmpeg(t *testing.T) {
	err := runAnalyze([]string{
		"--input", "source.wav",
		"--killplan", "plan.json",
		"--recording-result", "recording-result.json",
		"--out", "rhythm.json",
	})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("runAnalyze error = %v, want mutually exclusive timing sources", err)
	}
}

func TestRunAnalyzeRejectsLimitWithoutRankingBeforeFFmpeg(t *testing.T) {
	err := runAnalyze([]string{
		"--input", "source.wav",
		"--out", "rhythm.json",
		"--limit", "5",
	})
	if err == nil || !strings.Contains(err.Error(), "limit requires ranked moments") {
		t.Fatalf("runAnalyze error = %v, want ranking contract", err)
	}
}
