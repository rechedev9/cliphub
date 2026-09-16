package editor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

// FullDemoTimingManifestEntry is one real generated item command, produced by
// the production fullDemoItemCommand builder. It is consumed by the local
// resource harness at .local/render-analysis/timing-item-benchmark.go.
type FullDemoTimingManifestEntry struct {
	Name            string   `json:"name"`
	Role            string   `json:"role"`
	Encoder         string   `json:"encoder"`
	Category        string   `json:"category"`
	MediaSeconds    float64  `json:"media_seconds"`
	ExcerptSeconds  float64  `json:"excerpt_seconds"`
	ExpectedFrames  int64    `json:"expected_frames"`
	ExpectedSamples int64    `json:"expected_samples"`
	SourceSHA256    string   `json:"source_sha256,omitempty"`
	MediaDigest     string   `json:"media_digest,omitempty"`
	CommandDigest   string   `json:"command_digest,omitempty"`
	Command         []string `json:"command"`
}

type fullDemoTimingManifest struct {
	Note              string                        `json:"note"`
	ConfigDigest      string                        `json:"config_digest,omitempty"`
	VoiceDigest       string                        `json:"voice_digest,omitempty"`
	VoiceVerification string                        `json:"voice_verification,omitempty"`
	Entries           []FullDemoTimingManifestEntry `json:"entries"`
}

func fullDemoTimingDigest(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		hash.Write([]byte(part))
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// Opt-in generator for the local item resource harness. It never runs timed
// media trials itself; it only asks the production builder for representative
// long/short round item commands with a Carbon HUD and transitions, for NVENC
// and the software fallback. Set FULL_DEMO_TIMING_ITEM_GEN=1 to run it.
//
// Saved rounds can be used read-only by pointing
// FULL_DEMO_TIMING_ITEM_MEDIA_DIR at a directory containing long.nut and
// short.nut; otherwise synthetic testsrc2 sources are generated.
func TestFullDemoTimingItemCommandManifest(t *testing.T) {
	if os.Getenv("FULL_DEMO_TIMING_ITEM_GEN") != "1" {
		t.Skip("set FULL_DEMO_TIMING_ITEM_GEN=1 to generate the item benchmark manifest")
	}
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	// A durable work dir is required: the generated command embeds the Round HUD
	// ASS path and reads the source media, so both must survive the test for the
	// local harness to replay the command later.
	dir := os.Getenv("FULL_DEMO_TIMING_ITEM_WORKDIR")
	if dir == "" {
		dir = filepath.Join("..", "..", ".local", "render-analysis", "timing-item-work")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	// Absolute paths keep the manifest replayable from any working directory.
	if absolute, err := filepath.Abs(dir); err == nil {
		dir = absolute
	}

	const (
		tickRate   = 60
		longFrames = 1800 // ~30 s representative round
		shortSub   = 60   // ~1 s insert/round
	)
	longPath, shortPath := fullDemoTimingBenchSources(ctx, t, ffmpeg, dir, longFrames, shortSub)
	theme := os.Getenv("FULL_DEMO_TIMING_ITEM_HUD_THEME")
	if theme == "" {
		theme = "carbon"
	}

	hud := customhud.Timeline{
		Version:       customhud.TelemetryVersion,
		DemoSHA256:    strings.Repeat("a", 64),
		TargetSteamID: customhud.ExampleTarget,
		TickRate:      tickRate,
		EndTick:       longFrames + shortSub,
		Snapshots:     []customhud.Snapshot{customhud.Example()},
	}
	hud.Snapshots[0].Tick = 0

	perFrame := int64(recapplan.SamplesPerFrame)
	document := recapplan.Document{
		Clock:   recapplan.Clock{TickRate: tickRate},
		Options: recapplan.DefaultOptions(),
		Timeline: []recapplan.TimelineItem{
			{Role: "round", SourceRef: "long", SourceStartTick: 0, SourceEndTick: longFrames, StartFrame: 0, EndFrame: longFrames, StartSample: 0, EndSample: int64(longFrames) * perFrame},
			{Role: "round", SourceRef: "short", SourceStartTick: 0, SourceEndTick: shortSub, StartFrame: longFrames, EndFrame: longFrames + shortSub, StartSample: int64(longFrames) * perFrame, EndSample: int64(longFrames+shortSub) * perFrame},
		},
	}
	document.Options.Audio.Music.Enabled = false
	document.Options.Overlays.HUDTheme = theme
	document.Options.Capture.HUDProfile = customhud.CaptureProfile
	transitions := recapplan.DefaultTransitions()
	transitions.Enabled, transitions.Whip, transitions.Zoom, transitions.Flash = true, true, false, true
	transitions.Direction, transitions.FlashIntensity = "left", .2
	transitions.Whoosh, transitions.Impact = false, false
	document.Options.Transitions = &transitions

	base := ShortEdit{
		Preset:       PresetGameplayPOV60,
		OutputFormat: OutputFormatLandscape16x9,
		OutputFPS:    60,
		VideoCRF:     18,
		VideoPreset:  "ultrafast",
		Threads:      2,
		Parts:        []ShortPart{{SegmentID: "long", Input: longPath}, {SegmentID: "short", Input: shortPath}},
		FullDemo:     &FullDemoRenderEvidence{Effective: document},
		fullDemo: &fullDemoRenderContext{
			hud:    &hud,
			ffmpeg: ffmpeg,
			recording: recording.RecordingResult{Plan: recording.RecordingPlan{Segments: []recording.RecordingSegment{
				{ID: "long", TickStart: 0},
				{ID: "short", TickStart: 0},
			}}},
		},
	}

	var entries []FullDemoTimingManifestEntry
	for _, encoder := range []struct {
		name  string
		value string
	}{{"nvenc", VideoEncoderNVENC}, {"software", ""}} {
		short := base
		short.VideoEncoder = encoder.value
		for _, item := range document.Timeline {
			output := filepath.Join(dir, fmt.Sprintf("item-%s-%s.nut", item.SourceRef, encoder.name))
			command, err := fullDemoItemCommand(short, item, output)
			if err != nil {
				t.Fatalf("generate %s %s item command: %v", item.SourceRef, encoder.name, err)
			}
			if !fullDemoCommandHas(command, "-filter_complex") {
				t.Fatalf("generated %s %s item command is not a real item graph: %v", item.SourceRef, encoder.name, command)
			}
			entries = append(entries, FullDemoTimingManifestEntry{
				Name:            fmt.Sprintf("%s-%s", item.SourceRef, encoder.name),
				Role:            item.SourceRef,
				Encoder:         encoder.name,
				Category:        "synthetic",
				MediaSeconds:    float64(item.EndFrame-item.StartFrame) / 60,
				ExcerptSeconds:  float64(item.EndFrame-item.StartFrame) / 60,
				ExpectedFrames:  item.EndFrame - item.StartFrame,
				ExpectedSamples: item.EndSample - item.StartSample,
				Command:         command,
			})
		}
	}

	writeFullDemoTimingManifest(t, "timing-item-manifest.json", fullDemoTimingManifest{
		Note:    "Generated by TestFullDemoTimingItemCommandManifest via fullDemoItemCommand. Commands write the item output to their last argument; the harness redirects that to a scratch path.",
		Entries: entries,
	})
}

// Opt-in generator for the local harness using the coordinator's verified saved
// render inputs at .local/render-analysis/phase-a-out/baseline-validation. It
// loads the real Full Demo execution, recording result, edit manifest and HUD
// telemetry, materializes the exact production prepared voice WAVs, then asks
// production fullDemoItemCommand for bounded excerpts of representative rounds
// with the actual Carbon HUD and transitions. The saved NVENC p5/CQ16 settings
// are preserved; the software variant keeps the saved baseline preset/CRF.
//
// Set FULL_DEMO_TIMING_ITEM_BASELINE_DIR to that directory. This generates
// commands and prepared voice fixtures only: it never runs item trials and never
// modifies the saved inputs.
func TestFullDemoTimingSavedItemCommandManifest(t *testing.T) {
	baseline := os.Getenv("FULL_DEMO_TIMING_ITEM_BASELINE_DIR")
	if baseline == "" {
		t.Skip("set FULL_DEMO_TIMING_ITEM_BASELINE_DIR to the verified baseline-validation directory")
	}
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Minute)
	defer cancel()
	read := func(name string, into any) {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(baseline, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if err := json.Unmarshal(body, into); err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
	}
	var manifest Manifest
	read(filepath.Join("out", "edit-manifest.json"), &manifest)
	if len(manifest.Shorts) == 0 {
		t.Fatal("edit manifest has no shorts")
	}
	short := manifest.Shorts[0]
	var result recording.RecordingResult
	read(filepath.Join("inputs", "recording-result.json"), &result)
	var execution FullDemoExecution
	read(filepath.Join("inputs", "full-demo-execution.json"), &execution)
	if execution.HUDTelemetry == nil {
		t.Fatal("saved execution has no HUD telemetry")
	}
	hud, err := customhud.Load(execution.HUDTelemetry.Path)
	if err != nil {
		t.Fatalf("load HUD telemetry: %v", err)
	}
	workDir := os.Getenv("FULL_DEMO_TIMING_ITEM_SAVED_WORKDIR")
	if workDir == "" {
		workDir = filepath.Join("..", "..", ".local", "render-analysis", "timing-item-saved-work")
	}
	if absolute, err := filepath.Abs(workDir); err == nil {
		workDir = absolute
	}
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		t.Fatal(err)
	}
	short.fullDemo = &fullDemoRenderContext{
		execution: execution,
		recording: result,
		hud:       &hud,
		ffmpeg:    ffmpeg,
		workDir:   workDir,
	}
	if err := prepareFullDemoTransitions(ctx, &short); err != nil {
		t.Fatalf("prepare transitions from saved inputs: %v", err)
	}

	// Production voice preparation is mandatory and must reproduce the saved
	// gain/filter evidence: the item pipeline decodes these WAVs, so omitting or
	// substituting them would change decoder cost and the comparison.
	savedLevels := short.FullDemo.TrackLevels
	if len(execution.VoiceTracks) == 0 || len(savedLevels) == 0 {
		t.Fatal("saved execution has no team voice tracks; the production item pipeline requires them")
	}
	for i, voice := range execution.VoiceTracks {
		if _, err := os.Stat(voice.Path); err != nil {
			t.Fatalf("saved voice %d is required for production voice preparation: %v", i, err)
		}
	}
	short.FullDemo.TrackLevels = nil
	if err := prepareFullDemoTracks(ctx, &short, nil); err != nil {
		t.Fatalf("prepare production voice WAVs: %v", err)
	}
	if len(short.fullDemo.voicePaths) != len(execution.VoiceTracks) {
		t.Fatalf("expected %d prepared voice files, got %d; all voice files are required", len(execution.VoiceTracks), len(short.fullDemo.voicePaths))
	}
	if len(short.FullDemo.TrackLevels) != len(savedLevels) {
		t.Fatalf("prepared %d track levels, saved baseline has %d", len(short.FullDemo.TrackLevels), len(savedLevels))
	}
	voiceParts := make([]string, 0, len(short.fullDemo.voicePaths))
	for i, path := range short.fullDemo.voicePaths {
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 {
			t.Fatalf("prepared voice %d is missing or empty: %v", i, err)
		}
		level := short.FullDemo.TrackLevels[i]
		if level.Policy != savedLevels[i].Policy || math.Abs(level.AppliedGainDB-savedLevels[i].AppliedGainDB) > 1e-6 {
			t.Fatalf("voice %d gain evidence differs from the saved baseline: got %s %.6f, want %s %.6f", i, level.Policy, level.AppliedGainDB, savedLevels[i].Policy, savedLevels[i].AppliedGainDB)
		}
		if !fullDemoTimingMeasurementEqual(level.Measurement, savedLevels[i].Measurement) {
			t.Fatalf("voice %d loudness measurement differs from the saved baseline", i)
		}
		// The saved reference logs differ only in FFmpeg memory addresses and
		// elapsed lines; the parsed measurement above is the exact evidence.
		reference := filepath.Join(workDir, fmt.Sprintf("voice-%d-reference.txt", i))
		if info, err := os.Stat(reference); err != nil || info.Size() == 0 {
			t.Fatalf("prepared voice %d loudness reference is missing: %v", i, err)
		}
		voiceParts = append(voiceParts, execution.VoiceTracks[i].SHA256, strconv.FormatFloat(level.AppliedGainDB, 'f', 6, 64))
	}
	voiceDigest := fullDemoTimingDigest(voiceParts...)
	voiceVerification := verifyFullDemoVoiceDeterminism(ctx, t, short, workDir)
	t.Logf("prepared %d production voice WAVs (voice digest %s, determinism %s)", len(short.fullDemo.voicePaths), voiceDigest[:12], voiceVerification)

	rounds := make([]recapplan.TimelineItem, 0, len(short.FullDemo.Effective.Timeline))
	for _, item := range short.FullDemo.Effective.Timeline {
		if item.Role == "round" {
			rounds = append(rounds, item)
		}
	}
	if len(rounds) < 2 {
		t.Fatalf("saved plan has %d round items, need at least 2", len(rounds))
	}
	sort.Slice(rounds, func(i, j int) bool {
		return rounds[i].EndFrame-rounds[i].StartFrame < rounds[j].EndFrame-rounds[j].StartFrame
	})
	excerptSeconds := fullDemoTimingEnvFloat("FULL_DEMO_TIMING_ITEM_EXCERPT_SECONDS", 8)
	longSeconds := fullDemoTimingEnvFloat("FULL_DEMO_TIMING_ITEM_LONG_SECONDS", 30)
	const picks = 6
	chosen := make([]recapplan.TimelineItem, 0, picks)
	for k := 0; k < picks; k++ {
		index := 0
		if picks > 1 {
			index = k * (len(rounds) - 1) / (picks - 1)
		}
		candidate := rounds[index]
		if len(chosen) == 0 || chosen[len(chosen)-1].StartFrame != candidate.StartFrame {
			chosen = append(chosen, candidate)
		}
	}

	// The saved encoder is nvenc-h264 (p5/CQ16); the software variant uses the
	// saved baseline preset/CRF through the same builder.
	var entries []FullDemoTimingManifestEntry
	for _, encoder := range []struct {
		name  string
		value string
	}{{"nvenc", VideoEncoderNVENC}, {"software", ""}} {
		variant := short
		variant.VideoEncoder = encoder.value
		for _, source := range chosen {
			item := fullDemoTimingExcerpt(source, excerptSeconds)
			entries = append(entries, fullDemoTimingSavedEntry(t, variant, item, encoder.name, "sweep", excerptSeconds))
		}
	}
	// One bounded, longer NVENC-only sample keeps a heavier encode in view
	// without running every profile over multi-minute software rounds.
	longest := rounds[len(rounds)-1]
	longItem := fullDemoTimingExcerpt(longest, longSeconds)
	longVariant := short
	longVariant.VideoEncoder = VideoEncoderNVENC
	entries = append(entries, fullDemoTimingSavedEntry(t, longVariant, longItem, "nvenc-long", "long", longSeconds))

	configDigest := fullDemoTimingDigest(func() []string {
		var parts []string
		for _, entry := range entries {
			parts = append(parts, entry.Name, entry.Encoder, entry.MediaDigest, entry.CommandDigest)
		}
		parts = append(parts, voiceDigest)
		return parts
	}()...)
	writeFullDemoTimingManifest(t, "timing-item-manifest-saved.json", fullDemoTimingManifest{
		Note:              "Generated from the verified saved baseline-validation inputs via fullDemoItemCommand. Encoder, HUD, transitions and prepared voice WAVs match that render; entries are bounded excerpts labelled by excerpt_seconds.",
		ConfigDigest:      configDigest,
		VoiceDigest:       voiceDigest,
		VoiceVerification: voiceVerification,
		Entries:           entries,
	})
	t.Logf("wrote %d saved item commands (%d sweep rounds x 2 encoders + 1 long) to %s", len(entries), len(chosen), fullDemoTimingManifestPath("timing-item-manifest-saved.json"))
}

func fullDemoTimingSavedEntry(t *testing.T, short ShortEdit, item recapplan.TimelineItem, encoder, category string, excerptSeconds float64) FullDemoTimingManifestEntry {
	t.Helper()
	output := filepath.Join(short.fullDemo.workDir, fmt.Sprintf("saved-%s-%d-%s.nut", category, item.StartFrame, encoder))
	command, err := fullDemoItemCommand(short, item, output)
	if err != nil {
		t.Fatalf("generate saved %s %s item command: %v", item.SourceRef, encoder, err)
	}
	if !fullDemoCommandHas(command, "-filter_complex") {
		t.Fatalf("generated saved %s %s item command is not a real item graph", item.SourceRef, encoder)
	}
	sourceSHA := ""
	for _, part := range short.Parts {
		if part.SegmentID == item.SourceRef {
			sourceSHA = part.SourceArtifact.ContentSHA256
			break
		}
	}
	hudSHA := ""
	if short.fullDemo.execution.HUDTelemetry != nil {
		hudSHA = short.fullDemo.execution.HUDTelemetry.SHA256
	}
	entry := FullDemoTimingManifestEntry{
		Name:            fmt.Sprintf("round-%d-%s", item.StartFrame, encoder),
		Role:            item.SourceRef,
		Encoder:         encoder,
		Category:        category,
		MediaSeconds:    float64(item.EndFrame-item.StartFrame) / recapplan.OutputFPS,
		ExcerptSeconds:  excerptSeconds,
		ExpectedFrames:  item.EndFrame - item.StartFrame,
		ExpectedSamples: item.EndSample - item.StartSample,
		SourceSHA256:    sourceSHA,
		Command:         command,
	}
	entry.MediaDigest = fullDemoTimingDigest(entry.SourceSHA256, hudSHA, fmt.Sprintf("%.3f", excerptSeconds), strconv.FormatInt(entry.ExpectedFrames, 10), strconv.FormatInt(entry.ExpectedSamples, 10), encoder)
	entry.CommandDigest = fullDemoTimingDigest(command[:len(command)-1]...)
	return entry
}

// fullDemoTimingExcerpt bounds an item to a coherent prefix: the global start
// frame/sample and the source start tick stay exact, so the HUD window, source
// trim and incoming transition remain the production ones; only the end is
// shortened and clearly labelled.
func fullDemoTimingExcerpt(item recapplan.TimelineItem, seconds float64) recapplan.TimelineItem {
	fps := float64(recapplan.OutputFPS)
	frames := int64(math.Round(seconds * fps))
	sourceFrames := item.EndFrame - item.StartFrame
	if frames <= 0 || frames > sourceFrames {
		frames = sourceFrames
	}
	excerpt := item
	excerpt.EndFrame = item.StartFrame + frames
	excerpt.EndSample = item.StartSample + frames*int64(recapplan.SamplesPerFrame)
	if sourceFrames > 0 {
		tickSpan := item.SourceEndTick - item.SourceStartTick
		excerpt.SourceEndTick = item.SourceStartTick + int(math.Round(float64(tickSpan)*float64(frames)/float64(sourceFrames)))
	}
	return excerpt
}

func fullDemoTimingEnvFloat(name string, fallback float64) float64 {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func fullDemoTimingMeasurementEqual(a, b LoudnessMeasurement) bool {
	if a.Status != b.Status {
		return false
	}
	for _, pair := range [][2]*float64{{a.IntegratedLUFS, b.IntegratedLUFS}, {a.TruePeakDBTP, b.TruePeakDBTP}, {a.LRA, b.LRA}, {a.Threshold, b.Threshold}, {a.Offset, b.Offset}} {
		if (pair[0] == nil) != (pair[1] == nil) {
			return false
		}
		if pair[0] != nil && math.Abs(*pair[0]-*pair[1]) > 1e-6 {
			return false
		}
	}
	return true
}

// verifyFullDemoVoiceDeterminism re-materializes the production voice WAVs and
// compares decoded PCM digests. FULL_DEMO_TIMING_ITEM_VERIFY_VOICE controls it:
// "0" skips, "quick" relies on the gain/reference comparison already performed,
// anything else (default) compares full WAV digests.
func verifyFullDemoVoiceDeterminism(ctx context.Context, t *testing.T, short ShortEdit, workDir string) string {
	t.Helper()
	mode := os.Getenv("FULL_DEMO_TIMING_ITEM_VERIFY_VOICE")
	if mode == "0" {
		return "skipped"
	}
	first := make([]string, len(short.fullDemo.voicePaths))
	for i, path := range short.fullDemo.voicePaths {
		digest, err := mediaassets.FileDigest(ctx, path, 8<<30)
		if err != nil {
			t.Fatalf("digest prepared voice %d: %v", i, err)
		}
		first[i] = digest
	}
	if mode == "quick" {
		return "quick (gain and loudnorm reference matched)"
	}
	verify := short
	verifyDir := workDir + "-verify"
	if err := os.MkdirAll(verifyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(verifyDir)
	verify.fullDemo = &fullDemoRenderContext{
		execution: short.fullDemo.execution,
		recording: short.fullDemo.recording,
		hud:       short.fullDemo.hud,
		ffmpeg:    short.fullDemo.ffmpeg,
		workDir:   verifyDir,
	}
	evidence := *short.FullDemo
	evidence.TrackLevels = nil
	verify.FullDemo = &evidence
	if err := prepareFullDemoTracks(ctx, &verify, nil); err != nil {
		t.Fatalf("re-prepare production voice WAVs: %v", err)
	}
	if len(verify.fullDemo.voicePaths) != len(first) {
		t.Fatalf("determinism pass produced %d voices, want %d", len(verify.fullDemo.voicePaths), len(first))
	}
	for i, path := range verify.fullDemo.voicePaths {
		digest, err := mediaassets.FileDigest(ctx, path, 8<<30)
		if err != nil {
			t.Fatalf("digest verification voice %d: %v", i, err)
		}
		if digest != first[i] {
			t.Fatalf("voice %d PCM is not deterministic across production preparation passes", i)
		}
	}
	return "full (WAV digest matched across two production passes)"
}

func writeFullDemoTimingManifest(t *testing.T, defaultName string, manifest fullDemoTimingManifest) {
	t.Helper()
	path := fullDemoTimingManifestPath(defaultName)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(body, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %d real item commands to %s", len(manifest.Entries), path)
}

func fullDemoTimingManifestPath(defaultName string) string {
	path := os.Getenv("FULL_DEMO_TIMING_ITEM_MANIFEST")
	if path == "" {
		path = filepath.Join("..", "..", ".local", "render-analysis", defaultName)
	}
	if absolute, err := filepath.Abs(path); err == nil {
		return absolute
	}
	return path
}

func fullDemoTimingBenchSources(ctx context.Context, t *testing.T, ffmpeg, dir string, longFrames, shortFrames int) (string, string) {
	t.Helper()
	if mediaDir := os.Getenv("FULL_DEMO_TIMING_ITEM_MEDIA_DIR"); mediaDir != "" {
		long, short := filepath.Join(mediaDir, "long.nut"), filepath.Join(mediaDir, "short.nut")
		for _, path := range []string{long, short} {
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("FULL_DEMO_TIMING_ITEM_MEDIA_DIR must contain long.nut and short.nut: %v", err)
			}
		}
		return long, short
	}
	make := func(name string, frames int) string {
		t.Helper()
		path := filepath.Join(dir, name+".nut")
		seconds := float64(frames) / 60
		command := []string{ffmpeg, "-y", "-v", "error",
			"-f", "lavfi", "-i", "testsrc2=s=1920x1080:r=60:d=" + decimal(seconds),
			"-f", "lavfi", "-i", "sine=f=440:r=48000:d=" + decimal(seconds),
			"-map", "0:v", "-map", "1:a", "-c:v", "libx264", "-preset", "ultrafast", "-bf", "0", "-c:a", "pcm_f32le", "-ac", "2", path}
		if _, err := runFFmpegOutput(ctx, command, "synthetic benchmark source"); err != nil {
			t.Fatal(err)
		}
		return path
	}
	return make("long", longFrames), make("short", shortFrames)
}
