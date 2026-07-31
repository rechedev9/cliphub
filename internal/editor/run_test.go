package editor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rechedev9/tickcut/internal/killplan"
	"github.com/rechedev9/tickcut/internal/recording"
)

func TestRunDryRunWritesManifestsPromptsAndDoesNotExecuteFFmpeg(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	recordingResultPath := writeRecordingResultFixture(t, dir)
	outDir := filepath.Join(dir, "shorts")
	publishDir := defaultPublishDir(outDir)
	missingFFmpeg := filepath.Join(dir, "missing-ffmpeg")

	result, err := Run(context.Background(), Config{
		RecordingResultPath: recordingResultPath,
		OutputDir:           outDir,
		FFmpegPath:          missingFFmpeg,
		DryRun:              true,
	})
	if err != nil {
		t.Fatalf("Run dry-run error = %v", err)
	}
	if !result.DryRun {
		t.Fatal("result.DryRun = false, want true")
	}
	if len(result.Shorts) != 2 {
		t.Fatalf("shorts len = %d, want 2", len(result.Shorts))
	}
	for _, path := range []string{
		filepath.Join(outDir, "edit-manifest.json"),
		filepath.Join(outDir, "shorts-result.json"),
		filepath.Join(outDir, "prompts", "short-001-seg-001-cover.md"),
		filepath.Join(publishDir, "pack-manifest.json"),
		filepath.Join(publishDir, "index.html"),
		filepath.Join(publishDir, "publish-summary.md"),
		filepath.Join(publishDir, "01_seg-001_martinezsa_de_ancient_2k_ak47.caption.txt"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected file missing %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(outDir, "short-001-seg-001.mp4")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not create video, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(publishDir, "01_seg-001_martinezsa_de_ancient_2k_ak47.mp4")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not publish video, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(publishDir, "01_seg-001_martinezsa_de_ancient_2k_ak47.cover.jpg")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not create cover, stat err = %v", err)
	}
	if result.Shorts[0].CoverPath == "" || result.Shorts[0].CoverTimeSeconds == 0 {
		t.Fatalf("dry-run missing planned cover: %#v", result.Shorts[0])
	}
}

func TestRunDryRunFiltersSegmentsAndLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	recordingResultPath := writeRecordingResultFixture(t, dir)
	outDir := filepath.Join(dir, "shorts")
	publishDir := defaultPublishDir(outDir)

	result, err := Run(context.Background(), Config{
		RecordingResultPath: recordingResultPath,
		OutputDir:           outDir,
		SegmentIDs:          []string{"seg-002"},
		Limit:               1,
		FFmpegPath:          filepath.Join(dir, "missing-ffmpeg"),
		DryRun:              true,
	})
	if err != nil {
		t.Fatalf("Run dry-run error = %v", err)
	}
	if result.Limit != 1 || len(result.SegmentFilter) != 1 || result.SegmentFilter[0] != "seg-002" {
		t.Fatalf("selection metadata missing: %#v", result)
	}
	if len(result.Shorts) != 1 || result.Shorts[0].SegmentID != "seg-002" {
		t.Fatalf("shorts = %#v", result.Shorts)
	}
	if _, err := os.Stat(filepath.Join(publishDir, "02_seg-002_martinezsa_de_ancient_1k_awp.caption.txt")); err != nil {
		t.Fatalf("filtered caption missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(publishDir, "01_seg-001_martinezsa_de_ancient_2k_ak47.caption.txt")); !os.IsNotExist(err) {
		t.Fatalf("unselected caption should not exist, stat err = %v", err)
	}
}

func TestRunDryRunPreservesEditorialSegmentOrder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fixture := testRecordingResult(dir)
	fixture.Plan.EditorialSegmentIDs = []string{"seg-002", "seg-001"}
	recordingResultPath := writeRecordingResult(t, dir, fixture)

	result, err := Run(context.Background(), Config{
		RecordingResultPath: recordingResultPath,
		OutputDir:           filepath.Join(dir, "shorts"),
		FFmpegPath:          filepath.Join(dir, "missing-ffmpeg"),
		DryRun:              true,
		CompileSegments:     true,
	})
	if err != nil {
		t.Fatalf("Run dry-run error = %v", err)
	}
	if len(result.Shorts) != 1 {
		t.Fatalf("shorts len = %d, want 1 compiled short", len(result.Shorts))
	}
	got := result.Shorts[0].Parts
	if len(got) != 2 || got[0].SegmentID != "seg-002" || got[1].SegmentID != "seg-001" {
		t.Fatalf("compiled parts = %#v, want editorial order [seg-002 seg-001]", got)
	}
}

func TestRankRecordingSegmentsUsesMomentScoreBeforePlanOrder(t *testing.T) {
	plan := recording.RecordingPlan{
		Tickrate: 64,
		Segments: []recording.RecordingSegment{
			{
				ID:        "seg-first",
				TickStart: 0,
				TickEnd:   640,
				Kills:     []killplan.Kill{{Tick: 100}},
			},
			{
				ID:        "seg-best",
				TickStart: 640,
				TickEnd:   1280,
				Kills: []killplan.Kill{
					{Tick: 700, Headshot: true},
					{Tick: 800, Headshot: true},
					{Tick: 900, Headshot: true},
				},
			},
		},
	}

	ranked := rankRecordingSegments(plan)
	if len(ranked) != 2 || ranked[0].ID != "seg-best" || ranked[1].ID != "seg-first" {
		t.Fatalf("ranked recording segments = %#v", ranked)
	}
}

func TestRunRankedCompilationKeepsRhythmAndManifestOrderAligned(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fixture := testRecordingResult(dir)
	// Start with the weaker segment first editorially so ranking must change
	// the order rather than accidentally confirming the persisted IDs.
	fixture.Plan.EditorialSegmentIDs = []string{"seg-002", "seg-001"}
	recordingResultPath := writeRecordingResult(t, dir, fixture)

	rankedPlan := fixture.Plan
	rankedPlan.Segments = rankRecordingSegments(rankedPlan)
	rankedPlan.EditorialSegmentIDs = []string{"seg-001", "seg-002"}
	musicPath := filepath.Join(dir, "music", "ranked.wav")
	rhythmPath := filepath.Join(dir, "ranked-rhythm.json")
	writeCanonicalRhythmFixture(
		t,
		rhythmPath,
		musicPath,
		rankedPlan.ToKillPlan(),
		[]float64{0.5, 1, 1.5, 2},
		0.1,
		0,
	)

	result, err := Run(context.Background(), Config{
		RecordingResultPath: recordingResultPath,
		OutputDir:           filepath.Join(dir, "shorts"),
		FFmpegPath:          filepath.Join(dir, "missing-ffmpeg"),
		MusicPath:           musicPath,
		RhythmPath:          rhythmPath,
		CompileSegments:     true,
		RankMoments:         true,
		DryRun:              true,
	})
	if err != nil {
		t.Fatalf("Run ranked rhythm dry-run error = %v", err)
	}
	if len(result.Shorts) != 1 {
		t.Fatalf("shorts len = %d, want 1 compiled short", len(result.Shorts))
	}
	parts := result.Shorts[0].Parts
	if len(parts) != 2 || parts[0].SegmentID != "seg-001" || parts[1].SegmentID != "seg-002" {
		t.Fatalf("compiled parts = %#v, want ranked order [seg-001 seg-002]", parts)
	}
}

func TestConfigRejectsRankMomentsWithExplicitSegmentIDs(t *testing.T) {
	err := (Config{
		RecordingResultPath: "recording-result.json",
		OutputDir:           "shorts",
		RankMoments:         true,
		SegmentIDs:          []string{"seg-001"},
	}).validate()
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("validate error = %v, want conflicting selection modes", err)
	}
}

func TestRunRejectsRecordingResultInsideOutputNamespace(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "shorts")
	recordingResultPath := filepath.Join(outDir, "shorts-result.json")
	writeJSONFile(t, recordingResultPath, testRecordingResult(dir))
	original, err := os.ReadFile(recordingResultPath)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Run(context.Background(), Config{
		RecordingResultPath: recordingResultPath,
		OutputDir:           outDir,
		DryRun:              true,
	})
	if err == nil {
		t.Fatal("Run error = nil, want source/output namespace conflict")
	}
	got, readErr := os.ReadFile(recordingResultPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("recording result changed to %q, want original bytes", got)
	}
}

func TestRunRejectsRelativeRecordingArtifactInsideOutputNamespace(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "shorts")
	sourcePath := filepath.Join(outDir, "short-001-seg-001.mp4")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("recording source")
	if err := os.WriteFile(sourcePath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	result := testRecordingResult(dir)
	result.Artifacts[0].Path = filepath.Join("..", "shorts", filepath.Base(sourcePath))
	recordingResultPath := writeRecordingResult(t, dir, result)

	_, err := Run(context.Background(), Config{
		RecordingResultPath: recordingResultPath,
		OutputDir:           outDir,
		DryRun:              true,
	})
	if err == nil || !strings.Contains(err.Error(), "recording artifact") {
		t.Fatalf("Run error = %v, want relative artifact conflict", err)
	}
	got, readErr := os.ReadFile(sourcePath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("recording source changed to %q", got)
	}
}

func TestRunWithFakeFFmpegWritesShortResults(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	recordingResultPath := writeRecordingResultFixture(t, dir)
	outDir := filepath.Join(dir, "shorts")
	publishDir := defaultPublishDir(outDir)
	ffmpegPath := fakeFFmpeg(t, dir)

	result, err := Run(context.Background(), Config{
		RecordingResultPath: recordingResultPath,
		OutputDir:           outDir,
		FFmpegPath:          ffmpegPath,
	})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	if len(result.Shorts) != 2 {
		t.Fatalf("shorts len = %d, want 2", len(result.Shorts))
	}
	first := result.Shorts[0]
	if first.OutputArtifact.SizeBytes == 0 {
		t.Fatalf("short output artifact missing size: %#v", first.OutputArtifact)
	}
	if _, err := os.Stat(filepath.Join(outDir, "short-001-seg-001.mp4")); err != nil {
		t.Fatalf("short output missing: %v", err)
	}
	if first.PublishArtifact.SizeBytes == 0 {
		t.Fatalf("publish artifact missing size: %#v", first.PublishArtifact)
	}
	if _, err := os.Stat(filepath.Join(publishDir, "01_seg-001_martinezsa_de_ancient_2k_ak47.mp4")); err != nil {
		t.Fatalf("publish output missing: %v", err)
	}
	if first.CoverArtifact.SizeBytes == 0 {
		t.Fatalf("cover artifact missing size: %#v", first.CoverArtifact)
	}
	if _, err := os.Stat(filepath.Join(publishDir, "01_seg-001_martinezsa_de_ancient_2k_ak47.cover.jpg")); err != nil {
		t.Fatalf("cover output missing: %v", err)
	}
	if got := argAfter(first.FFmpegCommand, "-vf"); !strings.Contains(got, "crop=1080:1920") {
		t.Fatalf("ffmpeg filter missing vertical crop:\n%s", got)
	}
	if got := argAfter(first.CoverCommand, "-ss"); got != "0.880" {
		t.Fatalf("cover -ss arg = %q, want 0.880", got)
	}
	if got := argAfter(first.FFmpegCommand, "-c:a"); got != "aac" {
		t.Fatalf("-c:a arg = %q, want aac", got)
	}

	var written Result
	b, err := os.ReadFile(filepath.Join(outDir, "shorts-result.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &written); err != nil {
		t.Fatal(err)
	}
	if len(written.Shorts) != 2 || written.Shorts[0].SegmentID != "seg-001" {
		t.Fatalf("written result = %#v", written.Shorts)
	}

	var pack PackManifest
	b, err = os.ReadFile(filepath.Join(publishDir, "pack-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &pack); err != nil {
		t.Fatal(err)
	}
	if len(pack.Items) != 2 || pack.Items[0].Video == "" || pack.Items[0].CoverPath == "" || !strings.Contains(pack.Items[0].Caption, "#CS2") {
		t.Fatalf("pack manifest = %#v", pack.Items)
	}
	if _, err := os.Stat(filepath.Join(publishDir, "index.html")); err != nil {
		t.Fatalf("publish gallery missing: %v", err)
	}
	indexHTML, err := os.ReadFile(filepath.Join(publishDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"../prompts/short-001-seg-001-cover.md",
		"preset <span>viral-60-clean</span>",
		"video <span>crf 16 / slow</span>",
		"id=\"search\"",
		"data-copy-target=\".caption\"",
		"All weapons",
		"source: h264 | 1920x1080 | 60fps | 8.0s",
		"output:",
	} {
		if !strings.Contains(string(indexHTML), want) {
			t.Fatalf("publish gallery missing %q:\n%s", want, indexHTML)
		}
	}
	summary, err := os.ReadFile(filepath.Join(publishDir, "publish-summary.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# TickCut Publish Summary", "Total kills: 3", "AK-47 x1", "AWP x1", "| 01 | seg-001 |"} {
		if !strings.Contains(string(summary), want) {
			t.Fatalf("publish summary missing %q:\n%s", want, summary)
		}
	}
}

func TestRunRejectsInvalidVideoEncodingOptions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	recordingResultPath := writeRecordingResultFixture(t, dir)

	_, err := Run(context.Background(), Config{
		RecordingResultPath: recordingResultPath,
		OutputDir:           filepath.Join(dir, "bad-crf"),
		VideoCRF:            99,
		FFmpegPath:          filepath.Join(dir, "missing-ffmpeg"),
		DryRun:              true,
	})
	if err == nil || !strings.Contains(err.Error(), "video crf") {
		t.Fatalf("Run error = %v, want video crf validation", err)
	}

	_, err = Run(context.Background(), Config{
		RecordingResultPath: recordingResultPath,
		OutputDir:           filepath.Join(dir, "bad-preset"),
		VideoPreset:         "cinema",
		FFmpegPath:          filepath.Join(dir, "missing-ffmpeg"),
		DryRun:              true,
	})
	if err == nil || !strings.Contains(err.Error(), "unknown video preset") {
		t.Fatalf("Run error = %v, want video preset validation", err)
	}
}

func TestRunSkipExistingReusesRenderedFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	recordingResultPath := writeRecordingResultFixture(t, dir)
	outDir := filepath.Join(dir, "shorts")
	publishDir := defaultPublishDir(outDir)
	prior, err := Run(context.Background(), Config{
		RecordingResultPath: recordingResultPath,
		OutputDir:           outDir,
		FFmpegPath:          filepath.Join(dir, "missing-ffmpeg"),
		SkipExisting:        true,
		DryRun:              true,
	})
	if err != nil {
		t.Fatalf("seed dry Run error = %v", err)
	}
	prior.DryRun = false
	prior.Executed = true
	for i := range prior.Shorts {
		short := &prior.Shorts[i]
		for _, path := range []string{short.Output, short.CoverPath, short.CoverSheetPath} {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("existing"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		short.OutputArtifact = recording.RecordingArtifact{
			Path:      short.Output,
			SizeBytes: int64(len("existing")),
		}
		short.CoverArtifact = recording.RecordingArtifact{
			Path:      short.CoverPath,
			SizeBytes: int64(len("existing")),
		}
		short.CoverSheetArtifact = recording.RecordingArtifact{
			Path:      short.CoverSheetPath,
			SizeBytes: int64(len("existing")),
		}
	}
	if err := WriteResult(filepath.Join(outDir, "shorts-result.json"), prior); err != nil {
		t.Fatal(err)
	}

	result, err := Run(context.Background(), Config{
		RecordingResultPath: recordingResultPath,
		OutputDir:           outDir,
		FFmpegPath:          filepath.Join(dir, "missing-ffmpeg"),
		SkipExisting:        true,
	})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	if !result.SkipExisting {
		t.Fatal("SkipExisting = false, want true")
	}
	if !result.Shorts[0].RenderSkipped || !result.Shorts[0].CoverSkipped {
		t.Fatalf("skip flags missing: %#v", result.Shorts[0])
	}
	if result.Shorts[0].OutputArtifact.SizeBytes == 0 || result.Shorts[0].CoverArtifact.SizeBytes == 0 {
		t.Fatalf("reused artifacts missing size: %#v", result.Shorts[0])
	}
	if _, err := os.Stat(filepath.Join(publishDir, "01_seg-001_martinezsa_de_ancient_2k_ak47.mp4")); err != nil {
		t.Fatalf("publish output missing after reuse: %v", err)
	}
}

func TestRunSkipExistingRerendersChangedCompiledProducerContract(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fixture := testRecordingResult(dir)
	fixture.Plan.EditorialSegmentIDs = []string{"seg-002", "seg-001"}
	recordingResultPath := writeRecordingResult(t, dir, fixture)
	outDir := filepath.Join(dir, "shorts")
	ffmpegPath := fakeFFmpeg(t, dir)

	previous, err := Run(context.Background(), Config{
		RecordingResultPath: recordingResultPath,
		OutputDir:           outDir,
		FFmpegPath:          ffmpegPath,
		CompileSegments:     true,
	})
	if err != nil {
		t.Fatalf("initial compiled Run error = %v", err)
	}
	if len(previous.Shorts) != 1 || len(previous.Shorts[0].Parts) != 2 ||
		previous.Shorts[0].Parts[0].SegmentID != "seg-002" {
		t.Fatalf("initial compilation = %#v, want editorial order", previous.Shorts)
	}

	current, err := Run(context.Background(), Config{
		RecordingResultPath: recordingResultPath,
		OutputDir:           outDir,
		FFmpegPath:          ffmpegPath,
		CompileSegments:     true,
		RankMoments:         true,
		SkipExisting:        true,
	})
	if err != nil {
		t.Fatalf("ranked compiled Run error = %v", err)
	}
	if len(current.Shorts) != 1 || len(current.Shorts[0].Parts) != 2 ||
		current.Shorts[0].Parts[0].SegmentID != "seg-001" {
		t.Fatalf("current compilation = %#v, want ranked order", current.Shorts)
	}
	oldShort, newShort := previous.Shorts[0], current.Shorts[0]
	if oldShort.Output != newShort.Output ||
		oldShort.CoverPath != newShort.CoverPath ||
		oldShort.CoverSheetPath != newShort.CoverSheetPath {
		t.Fatalf(
			"artifact paths changed; regression needs the same reuse targets:\nold=%#v\nnew=%#v",
			oldShort,
			newShort,
		)
	}
	if newShort.RenderSkipped || newShort.CoverSkipped || newShort.CoverSheetSkipped {
		t.Fatalf("changed compilation reused stale artifacts: %#v", newShort)
	}
}

func TestRunNoCoversSkipsCoverOutputs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	recordingResultPath := writeRecordingResultFixture(t, dir)
	outDir := filepath.Join(dir, "shorts")
	publishDir := defaultPublishDir(outDir)
	ffmpegPath := fakeFFmpeg(t, dir)

	result, err := Run(context.Background(), Config{
		RecordingResultPath: recordingResultPath,
		OutputDir:           outDir,
		FFmpegPath:          ffmpegPath,
		DisableCovers:       true,
	})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	if result.CoversEnabled {
		t.Fatal("CoversEnabled = true, want false")
	}
	if result.Shorts[0].CoverPath != "" || len(result.Shorts[0].CoverCommand) != 0 {
		t.Fatalf("cover data should be empty: %#v", result.Shorts[0])
	}
	if _, err := os.Stat(filepath.Join(publishDir, "01_seg-001_martinezsa_de_ancient_2k_ak47.cover.jpg")); !os.IsNotExist(err) {
		t.Fatalf("cover should not exist, stat err = %v", err)
	}
}

func TestRunCoverFailureIsWarningOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	recordingResultPath := writeRecordingResultFixture(t, dir)
	outDir := filepath.Join(dir, "shorts")
	publishDir := defaultPublishDir(outDir)
	ffmpegPath := fakeFFmpegFailingCovers(t, dir)

	result, err := Run(context.Background(), Config{
		RecordingResultPath: recordingResultPath,
		OutputDir:           outDir,
		FFmpegPath:          ffmpegPath,
	})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(publishDir, "01_seg-001_martinezsa_de_ancient_2k_ak47.mp4")); err != nil {
		t.Fatalf("publish output missing: %v", err)
	}
	joined := strings.Join(result.Warnings, "\n")
	if !strings.Contains(joined, "cover seg-001") {
		t.Fatalf("warnings missing cover failure:\n%s", joined)
	}
}

func TestRunShortRenderFailureWritesLog(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	recordingResultPath := writeRecordingResultFixture(t, dir)
	outDir := filepath.Join(dir, "shorts")
	ffmpegPath := fakeFFmpegFailingShorts(t, dir)

	result, err := Run(context.Background(), Config{
		RecordingResultPath: recordingResultPath,
		OutputDir:           outDir,
		FFmpegPath:          ffmpegPath,
	})
	if err == nil {
		t.Fatal("Run error = nil, want short render failure")
	}
	// A real render that failed must not claim it executed, and the persisted
	// artifact must record executed:false.
	if result.Executed {
		t.Fatalf("result.Executed = true after a failed render, want false")
	}
	var persisted Result
	body, readResultErr := os.ReadFile(filepath.Join(outDir, "shorts-result.json"))
	if readResultErr != nil {
		t.Fatalf("read shorts-result.json: %v", readResultErr)
	}
	if err := json.Unmarshal(body, &persisted); err != nil {
		t.Fatalf("decode shorts-result.json: %v", err)
	}
	if persisted.Executed {
		t.Fatalf("persisted result executed = true after a failed render, want false")
	}
	if persisted.Error == "" {
		t.Fatalf("persisted result error = empty, want the render failure recorded")
	}
	if len(result.Shorts) == 0 || result.Shorts[0].RenderLogPath == "" {
		t.Fatalf("render log path missing: %#v", result.Shorts)
	}
	b, readErr := os.ReadFile(result.Shorts[0].RenderLogPath)
	if readErr != nil {
		t.Fatalf("read render log: %v", readErr)
	}
	if got := strings.TrimSpace(string(b)); got == "" {
		t.Fatal("render log is empty, want ffmpeg output or process start error")
	} else if !strings.Contains(got, "short render failed") && !strings.Contains(got, "ffmpeg short edit") {
		t.Fatalf("render log missing failure diagnostic:\n%s", b)
	}
}

func TestRunAutoDiscoversKillPlanFromPipelineResult(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	result := testRecordingResult(dir)
	result.Plan.DemoMap = ""
	result.Plan.DemoPath = filepath.Join(dir, "match.dem")
	recordingResultPath := writeRecordingResult(t, dir, result)
	planPath := writeKillPlanFixture(t, dir, "de_ancient")
	writeJSONFile(t, filepath.Join(dir, "pipeline-result.json"), map[string]string{"killplan": planPath})

	outDir := filepath.Join(dir, "shorts")
	_, err := Run(context.Background(), Config{
		RecordingResultPath: recordingResultPath,
		OutputDir:           outDir,
		FFmpegPath:          filepath.Join(dir, "missing-ffmpeg"),
		DryRun:              true,
	})
	if err != nil {
		t.Fatalf("Run dry-run error = %v", err)
	}

	var manifest Manifest
	b, err := os.ReadFile(filepath.Join(outDir, "edit-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.KillPlan != planPath {
		t.Fatalf("manifest.KillPlan = %q, want %q", manifest.KillPlan, planPath)
	}
	if got := manifest.Shorts[0].Label; got != "MartinezSa | de_ancient | 2K" {
		t.Fatalf("label = %q", got)
	}
	if len(manifest.Warnings) != 0 {
		t.Fatalf("warnings = %v", manifest.Warnings)
	}
}

func writeRecordingResultFixture(t *testing.T, dir string) string {
	t.Helper()
	return writeRecordingResult(t, dir, testRecordingResult(dir))
}

func writeRecordingResult(t *testing.T, dir string, result any) string {
	t.Helper()
	path := filepath.Join(dir, "recording", "recording-result.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeKillPlanFixture(t *testing.T, dir, mapName string) string {
	t.Helper()
	path := filepath.Join(dir, "plan.json")
	plan := killplan.NewPlan()
	plan.Demo.Map = mapName
	plan.Target.NameInDemo = "MartinezSa"
	writeJSONFile(t, path, plan)
	return path
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

var (
	fakeFFmpegOnce       sync.Once
	fakeFFmpegDir        string
	fakeFFmpegPath       string
	fakeFFmpegCoverPath  string
	fakeFFmpegShortPath  string
	fakeFFmpegFixtureErr error
)

func TestMain(m *testing.M) {
	code := m.Run()
	if err := removeFakeFFmpegFixture(); err != nil {
		fmt.Fprintf(os.Stderr, "remove fake ffmpeg fixture: %v\n", err)
		code = 1
	}
	os.Exit(code)
}

func fakeFFmpeg(t *testing.T, _ string) string {
	t.Helper()
	ensureFakeFFmpegFixture(t)
	return fakeFFmpegPath
}

func fakeFFmpegFailingCovers(t *testing.T, _ string) string {
	t.Helper()
	ensureFakeFFmpegFixture(t)
	return fakeFFmpegCoverPath
}

func fakeFFmpegFailingShorts(t *testing.T, _ string) string {
	t.Helper()
	ensureFakeFFmpegFixture(t)
	return fakeFFmpegShortPath
}

func ensureFakeFFmpegFixture(t *testing.T) {
	t.Helper()
	fakeFFmpegOnce.Do(buildFakeFFmpegFixture)
	if fakeFFmpegFixtureErr != nil {
		t.Fatalf("build fake ffmpeg fixture: %v", fakeFFmpegFixtureErr)
	}
}

func buildFakeFFmpegFixture() {
	fakeFFmpegDir, fakeFFmpegFixtureErr = os.MkdirTemp("", "zv-editor-fake-ffmpeg-*")
	if fakeFFmpegFixtureErr != nil {
		return
	}
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	fakeFFmpegPath = filepath.Join(fakeFFmpegDir, "ffmpeg"+ext)
	fakeFFmpegCoverPath = filepath.Join(fakeFFmpegDir, "ffmpeg-fail-cover"+ext)
	fakeFFmpegShortPath = filepath.Join(fakeFFmpegDir, "ffmpeg-fail-short"+ext)

	if runtime.GOOS == "windows" {
		src := filepath.Join(fakeFFmpegDir, "fake-ffmpeg.go")
		body := `package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		return
	}
	mode := strings.ToLower(filepath.Base(os.Args[0]))
	out := os.Args[len(os.Args)-1]
	if strings.Contains(mode, "fail-short") && strings.HasSuffix(out, ".mp4") {
		_, _ = fmt.Fprintln(os.Stderr, "short render failed")
		os.Exit(2)
	}
	if strings.Contains(mode, "fail-cover") && strings.HasSuffix(out, ".jpg") {
		os.Exit(2)
	}
	if out == "-" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(out), 0755)
	_ = os.WriteFile(out, []byte("fake"), 0644)
}
`
		if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
			fakeFFmpegFixtureErr = err
			return
		}
		goExe, err := exec.LookPath("go")
		if err != nil {
			fakeFFmpegFixtureErr = fmt.Errorf("find go toolchain: %w", err)
			return
		}
		if out, err := exec.Command(goExe, "build", "-o", fakeFFmpegPath, src).CombinedOutput(); err != nil {
			fakeFFmpegFixtureErr = fmt.Errorf("go build: %w: %s", err, out)
			return
		}
	} else {
		body := "#!/bin/sh\nlast=\nfor arg in \"$@\"; do last=\"$arg\"; done\nmode=$(basename \"$0\")\ncase \"$mode:$last\" in *fail-short*:*.mp4) echo short render failed >&2; exit 2;; *fail-cover*:*.jpg) exit 2;; *:-) exit 0;; esac\nmkdir -p \"$(dirname \"$last\")\"\nprintf fake > \"$last\"\n"
		if err := os.WriteFile(fakeFFmpegPath, []byte(body), 0o755); err != nil {
			fakeFFmpegFixtureErr = err
			return
		}
	}
	for _, path := range []string{fakeFFmpegCoverPath, fakeFFmpegShortPath} {
		if err := os.Link(fakeFFmpegPath, path); err != nil {
			fakeFFmpegFixtureErr = fmt.Errorf("link %s: %w", path, err)
			return
		}
	}
}

func removeFakeFFmpegFixture() error {
	if fakeFFmpegDir == "" {
		return nil
	}
	var err error
	for attempt := 0; attempt < 40; attempt++ {
		err = os.RemoveAll(fakeFFmpegDir)
		if err == nil {
			return nil
		}
		if runtime.GOOS != "windows" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return err
}
