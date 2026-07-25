package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCapabilitiesJSONReportsReadyTools(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"ZV_RECORDER_PATH", "ZV_HLAE_PATH", "ZV_CS2_PATH", "ZV_COMPOSER_PATH", "ZV_EDITOR_PATH", "ZV_FFMPEG_PATH", "ZV_FFPROBE_PATH"} {
		path := filepath.Join(dir, name+".exe")
		if err := os.WriteFile(path, []byte("stub"), 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv(name, path)
	}
	t.Setenv("FACEIT_API_KEY", "faceit_test_secret_not_for_output")

	var stdout, stderr strings.Builder
	code := runCapabilities([]string{"--format", "json"}, &stdout, &stderr)
	if got, want := code, exitSuccess; got != want {
		t.Fatalf("code = %d, want %d; stderr=%s", got, want, stderr.String())
	}
	var report localCapabilities
	if err := json.Unmarshal([]byte(stdout.String()), &report); err != nil {
		t.Fatalf("unmarshal stdout: %v\n%s", err, stdout.String())
	}
	if !report.LocalStudioReady || !report.Record.Ready || !report.Compose.Ready || !report.Render.Ready || !report.Stream.Ready {
		t.Fatalf("report = %#v, want all local stages ready", report)
	}
	if !report.Faceit.Ready || !report.Faceit.ManualDemoIndexReady || report.Faceit.AutomatedDownloadReady {
		t.Fatalf("faceit = %#v, want manual indexing ready and automated download pending", report.Faceit)
	}
	if strings.Contains(stdout.String(), "faceit_test_secret_not_for_output") {
		t.Fatal("capabilities output exposed FACEIT_API_KEY")
	}
	for _, group := range []localCapabilityGroup{report.Record, report.Compose, report.Render} {
		for _, tool := range group.Tools {
			if tool.Source != "env" || !tool.Configured || !tool.Accessible || tool.Path == "" {
				t.Fatalf("tool = %#v, want accessible explicit path", tool)
			}
		}
	}
}

func TestRunCapabilitiesJSONReportsMissingToolsWithoutFailing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.exe")
	for _, name := range []string{"ZV_RECORDER_PATH", "ZV_HLAE_PATH", "ZV_CS2_PATH", "ZV_COMPOSER_PATH", "ZV_EDITOR_PATH", "ZV_FFMPEG_PATH", "ZV_FFPROBE_PATH"} {
		t.Setenv(name, missing)
	}
	t.Setenv("FACEIT_API_KEY", "")

	var stdout, stderr strings.Builder
	code := runCapabilities([]string{"--format=json"}, &stdout, &stderr)
	if got, want := code, exitSuccess; got != want {
		t.Fatalf("code = %d, want %d; stderr=%s", got, want, stderr.String())
	}
	var report localCapabilities
	if err := json.Unmarshal([]byte(stdout.String()), &report); err != nil {
		t.Fatalf("unmarshal stdout: %v\n%s", err, stdout.String())
	}
	if report.LocalStudioReady || report.Record.Ready || report.Compose.Ready || report.Render.Ready || report.Stream.Ready {
		t.Fatalf("report = %#v, want unavailable tools reported as normal state", report)
	}
	if report.Faceit.Ready || report.Faceit.ManualDemoIndexReady || report.Faceit.AutomatedDownloadReady {
		t.Fatalf("faceit = %#v, want unavailable integration", report.Faceit)
	}
}

func TestRunCapabilitiesStreamReadinessRequiresFFprobe(t *testing.T) {
	dir := t.TempDir()
	ffmpeg := filepath.Join(dir, "ffmpeg.exe")
	if err := os.WriteFile(ffmpeg, []byte("stub"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZV_FFMPEG_PATH", ffmpeg)
	t.Setenv("ZV_FFPROBE_PATH", filepath.Join(dir, "missing-ffprobe.exe"))

	var stdout, stderr strings.Builder
	code := runCapabilities([]string{"--format=json"}, &stdout, &stderr)
	if got, want := code, exitSuccess; got != want {
		t.Fatalf("code = %d, want %d; stderr=%s", got, want, stderr.String())
	}
	var report localCapabilities
	if err := json.Unmarshal([]byte(stdout.String()), &report); err != nil {
		t.Fatalf("unmarshal stdout: %v\n%s", err, stdout.String())
	}
	if report.Stream.Ready {
		t.Fatalf("stream = %#v, stream readiness needs both ffmpeg and ffprobe", report.Stream)
	}
}

func TestRunCapabilitiesRejectsUnexpectedArgs(t *testing.T) {
	var stdout, stderr strings.Builder
	code := runCapabilities([]string{"extra"}, &stdout, &stderr)
	if got, want := code, exitInvalidArgs; got != want {
		t.Fatalf("code = %d, want %d", got, want)
	}
	if got, want := stderr.String(), "error: unexpected extra args for \"capabilities\"\n"+capabilitiesUsage; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}
