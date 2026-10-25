// Package pipeline holds the one deterministic end-to-end check of the Short
// data flow: parsed demo events to kill plan, moments, capture contract,
// simulated capture and the dry-run editor plan, each step pinned by a golden
// file. Run `go test ./internal/pipeline -update` to rewrite the goldens, then
// review their diff.
package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/moments"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata/")

const (
	topMoments  = 3
	demoSHA256  = "0000000000000000000000000000000000000000000000000000000000000001"
	workToken   = "$WORK"
	goldenDir   = "testdata"
	fixtureFile = "mirage-parsed.json"
)

var fixedClock = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

type parsedFixture struct {
	Map           string `json:"map"`
	Tickrate      int    `json:"tickrate"`
	DurationTicks int    `json:"duration_ticks"`
	Target        struct {
		SteamID64   string `json:"steamid64"`
		NameInDemo  string `json:"name_in_demo"`
		TeamAtStart string `json:"team_at_start"`
	} `json:"target"`
	RoundStarts []parser.RoundStart `json:"round_starts"`
	RoundEnds   []parser.RoundEnd   `json:"round_ends"`
	Kills       []struct {
		Tick       int             `json:"tick"`
		Round      int             `json:"round"`
		Weapon     string          `json:"weapon"`
		Headshot   bool            `json:"headshot"`
		Wallbang   bool            `json:"wallbang"`
		KillerTeam string          `json:"killer_team"`
		Victim     killplan.Player `json:"victim"`
		KillerPos  [3]float64      `json:"killer_pos"`
		VictimPos  [3]float64      `json:"victim_pos"`
	} `json:"kills"`
}

// TestShortPipelineGolden walks one Short through every stage that runs
// without CS2, HLAE, network or a video encode.
func TestShortPipelineGolden(t *testing.T) {
	work := t.TempDir()
	fixture := readFixture(t)

	plan := buildKillPlan(t, fixture)
	assertGolden(t, "01-killplan.json", normalizedJSON(t, work, plan))

	ranked := moments.Rank(plan)
	assertGolden(t, "02-moments.json", normalizedJSON(t, work, ranked))
	ids := make([]string, 0, topMoments)
	for _, moment := range ranked[:min(topMoments, len(ranked))] {
		ids = append(ids, moment.SegmentID)
	}
	selected, err := killplan.Select(plan, ids)
	if err != nil {
		t.Fatalf("select top moments: %v", err)
	}
	selectedPath := filepath.Join(work, "selected-plan.json")
	writeJSON(t, selectedPath, selected)

	demoPath := filepath.Join(work, "match.dem")
	recordingDir := filepath.Join(work, "recording")
	recPlan, err := recording.NewPlanFromKillPlan(selected, demoPath, recordingDir, recording.DefaultStreamConfig())
	if err != nil {
		t.Fatalf("recording plan: %v", err)
	}
	script, err := recording.GenerateHLAEJavaScript(recPlan)
	if err != nil {
		t.Fatalf("HLAE script: %v", err)
	}
	assertGolden(t, "03-recording-plan.json", normalizedJSON(t, work, recPlan))
	assertGolden(t, "04-recording.js", normalizePaths(script, work))

	capture := simulateCapture(t, work, script, fixture.Target.SteamID64, recording.SegmentIDs(recording.RecordingResult{Plan: recPlan}))
	assertGolden(t, "05-capture.json", normalizedJSON(t, work, capture))

	resultPath := writeRecordingResult(t, recPlan)
	studioEdit := renderplan.DefaultEditRequest()
	result, err := editor.Run(context.Background(), editor.Config{
		RecordingResultPath: resultPath,
		KillPlanPath:        selectedPath,
		OutputDir:           filepath.Join(work, "render"),
		PublishDir:          filepath.Join(work, "publish"),
		FFmpegPath:          "ffmpeg",
		FFprobePath:         "ffprobe",
		OutputFormat:        studioEdit.Format,
		KillEffect:          studioEdit.KillEffect,
		Transition:          studioEdit.Transition,
		DryRun:              true,
	})
	if err != nil {
		t.Fatalf("editor dry run: %v", err)
	}
	if len(result.Shorts) != len(ids) {
		t.Fatalf("editor planned %d shorts, want %d", len(result.Shorts), len(ids))
	}
	var manifest editor.Manifest
	readJSON(t, filepath.Join(work, "render", "edit-manifest.json"), &manifest)
	manifestGolden := normalizedJSON(t, work, manifest)
	assertGolden(t, "06-edit-manifest.json", manifestGolden)
	var normalized editor.Manifest
	if err := json.Unmarshal([]byte(manifestGolden), &normalized); err != nil {
		t.Fatalf("decode normalized manifest: %v", err)
	}
	assertGolden(t, "07-ffmpeg-commands.txt", formatCommands(normalized))
}

func readFixture(t *testing.T) parsedFixture {
	t.Helper()
	var fixture parsedFixture
	readJSON(t, filepath.Join(goldenDir, fixtureFile), &fixture)
	return fixture
}

// buildKillPlan replays the parsed events through the same Collector the demo
// parser feeds, so segmentation and filtering run as in production.
func buildKillPlan(t *testing.T, fixture parsedFixture) killplan.Plan {
	t.Helper()
	collector := parser.NewCollector(fixture.Target.SteamID64, rules.Default())
	collector.RecordTargetIdentity(fixture.Target.NameInDemo, fixture.Target.TeamAtStart)
	for _, start := range fixture.RoundStarts {
		collector.RecordRoundStart(start)
	}
	for _, kill := range fixture.Kills {
		collector.RecordKill(parser.RawKill{
			Tick:     kill.Tick,
			Round:    kill.Round,
			Weapon:   kill.Weapon,
			Headshot: kill.Headshot,
			Wallbang: kill.Wallbang,
			Killer: killplan.Player{
				SteamID64:  fixture.Target.SteamID64,
				NameInDemo: fixture.Target.NameInDemo,
				TeamAtKill: kill.KillerTeam,
			},
			Victim:    kill.Victim,
			KillerPos: kill.KillerPos,
			VictimPos: kill.VictimPos,
		})
	}
	for _, end := range fixture.RoundEnds {
		collector.RecordRoundEnd(end)
	}
	plan, err := collector.BuildPlan(parser.PlanMeta{
		DemoPath:      "match.dem",
		SHA256:        demoSHA256,
		Map:           fixture.Map,
		Tickrate:      fixture.Tickrate,
		DurationTicks: fixture.DurationTicks,
	}, parser.SegmentModeKills)
	if err != nil {
		t.Fatalf("build kill plan: %v", err)
	}
	plan.GeneratedAt = fixedClock
	return plan
}

type captureView struct {
	Outcome          string   `json:"outcome"`
	RecordedSegments []string `json:"recorded_segments"`
	RecordOperations any      `json:"record_operations"`
	ExecutedCommands []string `json:"executed_commands"`
	FailureMessages  []string `json:"failure_messages"`
}

// simulateCapture runs the generated HLAE script in the Capture Lab MIRV
// simulator (scripts/capturelab), the same harness the recording tests use.
func simulateCapture(t *testing.T, work, script, target string, want []string) captureView {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("node is required for the Capture Lab simulator step")
	}
	scriptPath := filepath.Join(work, "recording.js")
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	scenarioPath := filepath.Join(work, "scenario.json")
	writeJSON(t, scenarioPath, map[string]any{
		"schema_version": 1,
		"name":           "short-pipeline",
		"target_steamid": target,
		"tick_step":      1,
		"max_frames":     200000,
		"expect": map[string]any{
			"outcome":           "verified",
			"recorded_segments": want,
			"soft_quit":         true,
		},
	})
	runner := filepath.Join(repoRoot(t), "scripts", "capturelab", "run.mjs")
	command := exec.Command(node, runner, "--script", scriptPath, "--scenario", scenarioPath)
	command.Dir = repoRoot(t)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("capture simulator: %v\n%s", err, output)
	}
	var view captureView
	if err := json.Unmarshal(output, &view); err != nil {
		t.Fatalf("decode capture summary: %v\n%s", err, output)
	}
	return view
}

// writeRecordingResult stands in for the recorder's capture: one artifact per
// planned segment, sized like a real HLAE take, with no media on disk.
func writeRecordingResult(t *testing.T, plan recording.RecordingPlan) string {
	t.Helper()
	artifacts := make([]recording.RecordingArtifact, 0, len(plan.Segments))
	for _, segment := range plan.Segments {
		start := recording.EffectiveRecordStartTick(segment, plan.Tickrate)
		end := recording.EffectiveRecordEndTick(segment, plan)
		artifacts = append(artifacts, recording.RecordingArtifact{
			SegmentID:       segment.ID,
			Role:            "segment",
			Type:            "video",
			Path:            filepath.Join(plan.OutputDir, "segments", segment.ID+".mp4"),
			SizeBytes:       1,
			DurationSeconds: float64(end-start) / float64(plan.Tickrate),
			FrameRate:       fmt.Sprintf("%d/1", plan.Stream.FPS),
			Codec:           "h264",
			Width:           plan.Stream.Width,
			Height:          plan.Stream.Height,
			SampleRate:      48000,
			Channels:        2,
		})
	}
	path := filepath.Join(plan.OutputDir, "recording-result.json")
	writeJSON(t, path, recording.RecordingResult{
		Plan:        plan,
		Script:      filepath.Join(plan.OutputDir, "recording.js"),
		Artifacts:   artifacts,
		CaptureMode: recording.CaptureModeFake,
	})
	return path
}

// formatCommands prints each FFmpeg command one argument per line, with the
// filter graph split per chain, so a golden diff points at the changed filter.
func formatCommands(manifest editor.Manifest) string {
	var b strings.Builder
	for _, short := range manifest.Shorts {
		fmt.Fprintf(&b, "# %s\n", short.SegmentID)
		for i, arg := range short.FFmpegCommand {
			if i > 0 && slices.Contains([]string{"-filter_complex", "-vf", "-af"}, short.FFmpegCommand[i-1]) {
				for _, chain := range strings.Split(arg, ";") {
					for _, filter := range splitFilters(chain) {
						fmt.Fprintf(&b, "    %s\n", filter)
					}
				}
				continue
			}
			fmt.Fprintf(&b, "  %s\n", arg)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// splitFilters splits a filter chain at the commas FFmpeg treats as filter
// separators: not escaped and not inside single quotes.
func splitFilters(chain string) []string {
	var filters []string
	quoted, start := false, 0
	for i := 0; i < len(chain); i++ {
		switch chain[i] {
		case '\\':
			i++
		case '\'':
			quoted = !quoted
		case ',':
			if !quoted {
				filters = append(filters, chain[start:i])
				start = i + 1
			}
		}
	}
	return append(filters, chain[start:])
}

// normalizedJSON encodes value with the temp dir and path separators
// normalized. JSON escapes `\` like JavaScript does, so one normalizer serves both.
func normalizedJSON(t *testing.T, work string, value any) string {
	t.Helper()
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("encode golden value: %v", err)
	}
	return normalizePaths(string(encoded), work) + "\n"
}

// assertGolden compares got with testdata/<name>, or rewrites it under -update.
func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join(goldenDir, name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create it)", path, err)
	}
	want := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if got == want {
		return
	}
	t.Errorf("%s differs from the pipeline output (review the diff, then rerun with -update if intended):\n%s", path, lineDiff(want, got))
}

// normalizePaths rewrites the temp dir to $WORK in escaped text (JSON or JS
// source) and folds the separators of the path after it to "/", so Windows and
// Linux produce the same golden. There `\\` is one separator and `\"` ends the
// string.
func normalizePaths(text, work string) string {
	slash := filepath.ToSlash(work)
	text = strings.NewReplacer(
		strings.ReplaceAll(work, `\`, `\\`), workToken,
		strings.Replace(slash, ":", `\\\\:`, 1), workToken,
		strings.Replace(slash, ":", `\\:`, 1), workToken,
		slash, workToken,
	).Replace(text)
	var out strings.Builder
	for {
		i := strings.Index(text, workToken)
		if i < 0 {
			out.WriteString(text)
			return out.String()
		}
		out.WriteString(text[:i+len(workToken)])
		text = text[i+len(workToken):]
		end := 0
	tail:
		for end < len(text) {
			c := text[end]
			escapedQuote := c == '\\' && end+1 < len(text) && (text[end+1] == '"' || text[end+1] == '\'')
			switch {
			case strings.IndexByte("\"' \n,;:|", c) >= 0 || escapedQuote:
				break tail
			case c == '\\' && end+1 < len(text) && text[end+1] == '\\':
				out.WriteByte('/')
				end += 2
			case c == '\\':
				out.WriteByte('/')
				end++
			default:
				out.WriteByte(c)
				end++
			}
		}
		text = text[end:]
	}
}

// lineDiff lists the first differing lines with context, enough to read in a
// terminal without an external diff tool.
func lineDiff(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	var b bytes.Buffer
	shown := 0
	for i := 0; i < max(len(wantLines), len(gotLines)) && shown < 20; i++ {
		var w, g string
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w == g {
			continue
		}
		fmt.Fprintf(&b, "line %d:\n  - %s\n  + %s\n", i+1, w, g)
		shown++
	}
	if len(wantLines) != len(gotLines) {
		fmt.Fprintf(&b, "line count: golden %d, got %d\n", len(wantLines), len(gotLines))
	}
	return b.String()
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readJSON(t *testing.T, path string, into any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

// repoRoot relies on go test running in the package directory, which holds
// under -trimpath too, unlike runtime.Caller paths.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	return root
}
