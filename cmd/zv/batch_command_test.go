package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunBatchRejectsReportAliasingDemoBeforeParsing(t *testing.T) {
	root := t.TempDir()
	demo := filepath.Join(root, "source.dem")
	original := []byte("not a valid demo")
	if err := os.WriteFile(demo, original, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runBatch([]string{root, "--report", demo, "--obs-dir", filepath.Join(root, "obs")}, &stdout, &stderr)
	if code != exitInvalidArgs {
		t.Fatalf("code = %d, want %d; stderr=%q", code, exitInvalidArgs, stderr.String())
	}
	if !strings.Contains(stderr.String(), "batch demo") {
		t.Fatalf("stderr = %q, want input alias rejection", stderr.String())
	}
	got, err := os.ReadFile(demo)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("demo changed to %q", got)
	}
}

func TestRunBatchJSONKeepsProgressOffStdout(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bad.dem"), []byte("not a demo"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runBatch([]string{root, "--format", "json", "--obs-dir", filepath.Join(root, "obs")}, &stdout, &stderr)
	if code != exitUnexpected {
		t.Fatalf("code = %d, want %d; stderr=%q", code, exitUnexpected, stderr.String())
	}
	var summary batchJSONSummary
	if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
		t.Fatalf("stdout is not one JSON document: %q: %v", stdout.String(), err)
	}
	if summary.Failed != 1 {
		t.Fatalf("summary = %+v, want one failed demo", summary)
	}
}

type batchJSONSummary struct {
	Failed int `json:"failed"`
}

func TestRunBatchRejectsUnknownFormat(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bad.dem"), []byte("not a demo"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runBatch([]string{root, "--format", "yaml"}, &stdout, &stderr)
	if code != exitInvalidArgs {
		t.Fatalf("code = %d, want %d", code, exitInvalidArgs)
	}
}
