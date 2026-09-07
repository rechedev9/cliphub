package editor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFullDemoProgressStageChangesBypassThrottle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "progress.json")
	tracker := NewProgressTracker(path)
	fixedTime := time.Now()
	tracker.now = func() time.Time { return fixedTime }
	state := newEncodeProgressState(buildEncodeProgressPlan([]ShortEdit{{DurationSeconds: 700}}), tracker, "Montando cortes y ritmo")
	state.setStageFraction(0, "Preparando voces (1/5)", .1)
	state.setStageFraction(0, "Montando corte 1 de 14", .2)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var progress EditorProgress
	if err := json.Unmarshal(body, &progress); err != nil {
		t.Fatal(err)
	}
	if progress.Stage != "Montando corte 1 de 14" || progress.Percent <= progressPrepPercent {
		t.Fatalf("phase change was throttled: %+v", progress)
	}
	state.setStageFraction(0, "Verificando archivo final", .999)
	if percent := readProgressPercent(t, path); percent >= progressFinalizeStart {
		t.Fatalf("unverified render reported completion: %d", percent)
	}
}

func TestFFmpegProgressPreservesMeasurementAndErrors(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var fractions []float64
	command := []string{ffmpeg, "-v", "info", "-f", "lavfi", "-re", "-i", "sine=f=440:d=6", "-af", "loudnorm=print_format=json", "-f", "null", "-"}
	out, err := runFFmpegOutputProgress(ctx, command, "measurement progress canary", 6, func(f float64) { fractions = append(fractions, f) })
	if err != nil {
		t.Fatal(err)
	}
	measurement, err := parseLoudnessMeasurement(out)
	if err != nil || measurement.Status != "measured" {
		t.Fatalf("progress discarded measurement: %+v, %v", measurement, err)
	}
	if len(fractions) == 0 {
		t.Fatal("no media timestamp progress during real FFmpeg pass")
	}
	for _, f := range fractions {
		if f <= 0 || f >= 1 {
			t.Fatalf("FFmpeg pass reported premature completion: %f", f)
		}
	}
	out, err = runFFmpegOutputProgress(ctx, []string{ffmpeg, "-v", "error", "-i", filepath.Join(t.TempDir(), "missing.wav"), "-f", "null", "-"}, "failing progress canary", 2, func(f float64) {
		if f >= 1 {
			t.Error("failed FFmpeg pass reported completion")
		}
	})
	if err == nil || !strings.Contains(out, "missing.wav") || !strings.Contains(err.Error(), "missing.wav") {
		t.Fatalf("progress lost FFmpeg error diagnostics: %q, %v", out, err)
	}
}

func TestRenderProgressFinishesOnlyAfterResultIsSaved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "progress.json")
	renderer := shortPackRenderer{
		manifest: &Manifest{GalleryPath: filepath.Join(dir, "gallery.html")},
		result:   &Result{Executed: true},
		opts:     shortPackOptions{OutputDir: dir, PackPath: filepath.Join(dir, "pack.json"), ResultPath: dir, Progress: NewProgressTracker(path)},
	}
	if err := renderer.writeOutputs(); err == nil {
		t.Fatal("result path is a directory; expected write failure")
	}
	if percent := readProgressPercent(t, path); percent >= 100 {
		t.Fatalf("failed result write reported completion: %d", percent)
	}
	renderer.opts.ResultPath = filepath.Join(dir, "result.json")
	if err := renderer.writeOutputs(); err != nil {
		t.Fatal(err)
	}
	if percent := readProgressPercent(t, path); percent != 100 {
		t.Fatalf("saved result did not finish progress: %d", percent)
	}
}
