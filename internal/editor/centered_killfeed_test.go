package editor

import (
	"os"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/recording"
)

func TestBuildManifestDrawsCenteredKillfeedOncePerShort(t *testing.T) {
	for _, compile := range []bool{false, true} {
		dir := t.TempDir()
		opts := testManifestOptions(dir, nil)
		opts.KillfeedOverlay = true
		opts.CenteredKillfeed = true
		opts.CompileSegments = compile

		manifest := mustBuildManifest(t, testRecordingResult(dir), opts)
		short := manifest.Shorts[0]
		for _, effect := range short.Effects {
			if effect.Type == EffectKillfeed {
				t.Fatalf("compile=%v: frozen crop overlay %#v alongside the centered killfeed", compile, effect)
			}
		}
		script, err := os.ReadFile(short.KillfeedASS)
		if err != nil {
			t.Fatalf("compile=%v: killfeed script: %v", compile, err)
		}
		for _, kill := range short.Kills {
			if !strings.Contains(string(script), kill.Victim) {
				t.Fatalf("compile=%v: script lacks victim %q", compile, kill.Victim)
			}
		}
		// The test kills carry no killer name; the short's player stands in.
		if !strings.Contains(string(script), "MartinezSa") {
			t.Fatalf("compile=%v: script lacks the attacker fallback", compile)
		}
		// Per-part filters would restart the script's clock at every cut.
		if got := strings.Count(strings.Join(short.FFmpegCommand, " "), "ass=filename="); got != 1 {
			t.Fatalf("compile=%v: ass filters = %d, want 1", compile, got)
		}
	}
}

func TestBuildManifestSkipsCenteredKillfeedWhenCaptureShowsNotices(t *testing.T) {
	dir := t.TempDir()
	result := testRecordingResult(dir)
	result.Plan.Stream.HUDMode = recording.HUDModeDeathnotices
	result.Plan.Stream.PortraitSafeKillfeed = true
	opts := testManifestOptions(dir, nil)
	opts.CenteredKillfeed = true

	manifest := mustBuildManifest(t, result, opts)
	if manifest.Shorts[0].KillfeedASS != "" {
		t.Fatal("drew a centered killfeed over notices the capture already moved into the frame")
	}
	if len(manifest.Warnings) != 1 || !strings.Contains(manifest.Warnings[0], "centered killfeed skipped") {
		t.Fatalf("warnings = %v", manifest.Warnings)
	}
}

func TestBuildManifestLandscapeIgnoresCenteredKillfeed(t *testing.T) {
	dir := t.TempDir()
	opts := testManifestOptions(dir, nil)
	opts.OutputFormat = OutputFormatLandscape16x9
	opts.CenteredKillfeed = true

	manifest := mustBuildManifest(t, testRecordingResult(dir), opts)
	if short := manifest.Shorts[0]; short.CenteredKillfeed || short.KillfeedASS != "" {
		t.Fatalf("landscape short drew a centered killfeed: %+v", short)
	}
}
