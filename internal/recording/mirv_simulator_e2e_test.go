package recording

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
)

type captureLabScenario struct {
	SchemaVersion     int                      `json:"schema_version"`
	Name              string                   `json:"name"`
	TargetSteamID     string                   `json:"target_steamid"`
	StartTick         int                      `json:"start_tick"`
	TickStep          int                      `json:"tick_step"`
	TickOverrides     []captureLabTickOverride `json:"tick_overrides,omitempty"`
	MaxFrames         int                      `json:"max_frames"`
	SeekDelayFrames   int                      `json:"seek_delay_frames,omitempty"`
	DemoEndTick       *int                     `json:"demo_end_tick,omitempty"`
	ObserverOverrides []captureLabObserverSpan `json:"observer_overrides,omitempty"`
	Expected          captureLabExpected       `json:"expect"`
}

type captureLabTickOverride struct {
	Frame int `json:"frame"`
	Tick  int `json:"tick"`
}

type captureLabObserverSpan struct {
	FromTick        int     `json:"from_tick"`
	ToTick          int     `json:"to_tick"`
	ObservedSteamID *string `json:"observed_steamid"`
	TargetPresent   *bool   `json:"target_present,omitempty"`
}

type captureLabExpected struct {
	Outcome          string   `json:"outcome"`
	FailureContains  string   `json:"failure_contains,omitempty"`
	RecordedSegments []string `json:"recorded_segments,omitempty"`
	MustExec         []string `json:"must_exec,omitempty"`
	SoftQuit         bool     `json:"soft_quit,omitempty"`
}

type captureLabSummary struct {
	OK                  bool     `json:"ok"`
	Outcome             string   `json:"outcome"`
	ExpectationFailures []string `json:"expectation_failures"`
	DisconnectFrame     *int     `json:"disconnect_frame"`
	QuitFrame           *int     `json:"quit_frame"`
}

func ptr[T any](value T) *T { return &value }

func captureLabPlan(segments ...RecordingSegment) RecordingPlan {
	return RecordingPlan{
		CaptureContract:       CaptureContractVersion,
		KillPlanSchemaVersion: killplan.SchemaVersion,
		DemoPath:              filepath.Join("fixtures", "capturelab.dem"),
		DemoSHA256:            strings.Repeat("a", 64),
		DemoMap:               "de_nuke",
		DemoDurationTicks:     6000,
		OutputDir:             filepath.Join("out", "capturelab"),
		TargetSteamID64:       "76561198377256168",
		TargetNameInDemo:      "target",
		TargetAccountID:       416990440,
		Tickrate:              64,
		Segments:              segments,
		Stream:                DefaultStreamConfig(),
		Runtime: RuntimeConfig{
			PlaybackTimescale: DefaultPlaybackTimescale,
			QuitTickPad:       200,
		},
	}
}

func TestGeneratedHLAEScriptRunsInMIRVSimulator(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for the exact-script Capture Lab integration")
	}
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	runner := filepath.Join(repoRoot, "scripts", "capturelab", "run.mjs")

	target := "76561198377256168"
	other := "76561198000000001"
	one := captureLabPlan(RecordingSegment{
		ID: "seg-001", TickStart: 64, TickEnd: 192,
		Kills: []killplan.Kill{{Tick: 128, Killer: killplan.Player{SteamID64: target}}},
	})
	two := captureLabPlan(
		RecordingSegment{ID: "seg-001", TickStart: 64, TickEnd: 192, Kills: []killplan.Kill{{Tick: 128}}},
		RecordingSegment{ID: "seg-002", TickStart: 3200, TickEnd: 3328, Kills: []killplan.Kill{{Tick: 3264}}},
	)
	tests := []struct {
		name     string
		plan     RecordingPlan
		scenario captureLabScenario
	}{
		{
			name: "healthy capture and delayed soft quit",
			plan: one,
			scenario: captureLabScenario{
				Name: "healthy", MaxFrames: 600, DemoEndTick: ptr(6000),
				Expected: captureLabExpected{
					Outcome: "verified", RecordedSegments: []string{"seg-001"},
					MustExec: []string{"mirv_streams record start", "mirv_streams record end", "disconnect", "quit"}, SoftQuit: true,
				},
			},
		},
		{
			name: "delayed seek lands before second segment",
			plan: two,
			scenario: captureLabScenario{
				Name: "delayed-seek", MaxFrames: 1200, SeekDelayFrames: 20, DemoEndTick: ptr(6000),
				Expected: captureLabExpected{
					Outcome: "verified", RecordedSegments: []string{"seg-001", "seg-002"},
					MustExec: []string{"demo_gototick", "disconnect", "quit"}, SoftQuit: true,
				},
			},
		},
		{
			name: "brief unknown observer stays within tolerance",
			plan: one,
			scenario: captureLabScenario{
				Name: "observer-briefly-unknown", MaxFrames: 600, DemoEndTick: ptr(6000),
				ObserverOverrides: []captureLabObserverSpan{{FromTick: 65, ToTick: 66, ObservedSteamID: nil}},
				Expected:          captureLabExpected{Outcome: "verified", RecordedSegments: []string{"seg-001"}, SoftQuit: true},
			},
		},
		{
			name: "unknown observer fails protected window",
			plan: one,
			scenario: captureLabScenario{
				Name: "observer-unknown", MaxFrames: 300, DemoEndTick: ptr(6000),
				ObserverOverrides: []captureLabObserverSpan{{FromTick: 65, ToTick: 70, ObservedSteamID: nil}},
				Expected:          captureLabExpected{Outcome: "failed", FailureContains: "observer target remained unknown", SoftQuit: true},
			},
		},
		{
			name: "target controller can appear after warmup starts",
			plan: one,
			scenario: captureLabScenario{
				Name: "target-temporarily-missing", MaxFrames: 600, DemoEndTick: ptr(6000),
				ObserverOverrides: []captureLabObserverSpan{
					{FromTick: 0, ToTick: 40, ObservedSteamID: &other, TargetPresent: ptr(false)},
					{FromTick: 41, ToTick: 55, ObservedSteamID: &other, TargetPresent: ptr(true)},
				},
				Expected: captureLabExpected{Outcome: "verified", RecordedSegments: []string{"seg-001"}, MustExec: []string{"spec_player"}, SoftQuit: true},
			},
		},
		{
			name: "brief observer drift re-specs and continues",
			plan: one,
			scenario: captureLabScenario{
				Name: "observer-brief-drift", MaxFrames: 600, DemoEndTick: ptr(6000),
				ObserverOverrides: []captureLabObserverSpan{{FromTick: 65, ToTick: 66, ObservedSteamID: &other}},
				Expected: captureLabExpected{
					Outcome: "verified", RecordedSegments: []string{"seg-001"},
					MustExec: []string{"spec_player"}, SoftQuit: true,
				},
			},
		},
		{
			name: "persistent wrong observer fails after respec budget",
			plan: one,
			scenario: captureLabScenario{
				Name: "observer-drift", MaxFrames: 300, DemoEndTick: ptr(6000),
				ObserverOverrides: []captureLabObserverSpan{{FromTick: 65, ToTick: 90, ObservedSteamID: &other}},
				Expected: captureLabExpected{
					Outcome: "failed", FailureContains: "drifted from",
					MustExec: []string{"spec_player"}, SoftQuit: true,
				},
			},
		},
		{
			name: "rewind and stalled ticks do not repeat consumed commands",
			plan: one,
			scenario: captureLabScenario{
				Name: "rewind-stall", MaxFrames: 700, DemoEndTick: ptr(6000),
				TickOverrides: []captureLabTickOverride{{Frame: 90, Tick: 40}, {Frame: 91, Tick: 40}, {Frame: 92, Tick: 40}},
				Expected:      captureLabExpected{Outcome: "verified", RecordedSegments: []string{"seg-001"}, SoftQuit: true},
			},
		},
		{
			name: "coarse frames execute all due commands in order",
			plan: two,
			scenario: captureLabScenario{
				Name: "coarse-frames", TickStep: 16, MaxFrames: 700, DemoEndTick: ptr(6000),
				Expected: captureLabExpected{Outcome: "verified", RecordedSegments: []string{"seg-001", "seg-002"}, SoftQuit: true},
			},
		},
		{
			name: "full round windows without kills preserve boundaries",
			plan: captureLabPlan(
				RecordingSegment{ID: "round-01", TickStart: 64, TickEnd: 1500},
				RecordingSegment{ID: "round-02", TickStart: 1800, TickEnd: 3500},
			),
			scenario: captureLabScenario{
				Name: "full-rounds", MaxFrames: 4000, DemoEndTick: ptr(6000),
				Expected: captureLabExpected{Outcome: "verified", RecordedSegments: []string{"round-01", "round-02"}, SoftQuit: true},
			},
		},
		{
			name: "demo ends before segment completion",
			plan: one,
			scenario: captureLabScenario{
				Name: "early-eof", MaxFrames: 300, DemoEndTick: ptr(100),
				Expected: captureLabExpected{Outcome: "failed", FailureContains: "demo playback ended before every protected segment completed", SoftQuit: true},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := test.plan
			plan.EditorialSegmentIDs = nil
			for _, segment := range plan.Segments {
				plan.EditorialSegmentIDs = append(plan.EditorialSegmentIDs, segment.ID)
			}
			if err := plan.Validate(); err != nil {
				t.Fatalf("plan.Validate: %v", err)
			}
			script, err := GenerateHLAEJavaScriptWithAttestation(plan, "capturelab-token")
			if err != nil {
				t.Fatalf("GenerateHLAEJavaScriptWithAttestation: %v", err)
			}
			test.scenario.SchemaVersion = 1
			test.scenario.TargetSteamID = target
			if test.scenario.TickStep == 0 {
				test.scenario.TickStep = 1
			}
			dir := t.TempDir()
			scriptPath := filepath.Join(dir, "recording.js")
			scenarioPath := filepath.Join(dir, "scenario.json")
			if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
				t.Fatal(err)
			}
			scenarioJSON, err := json.Marshal(test.scenario)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(scenarioPath, scenarioJSON, 0o600); err != nil {
				t.Fatal(err)
			}
			command := exec.Command(node, runner, "--script", scriptPath, "--scenario", scenarioPath)
			command.Dir = repoRoot
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("MIRV simulator: %v\n%s", err, output)
			}
			// Compare the entire transcript, including command ordering, failure
			// diagnostics, record boundaries and soft-quit frames, to the former
			// scan-every-frame dispatcher. Only the dispatch loop is substituted.
			referencePath := filepath.Join(dir, "reference.js")
			if err := os.WriteFile(referencePath, []byte(legacyScheduleDispatcher(t, script)), 0o600); err != nil {
				t.Fatal(err)
			}
			referenceCommand := exec.Command(node, runner, "--script", referencePath, "--scenario", scenarioPath)
			referenceCommand.Dir = repoRoot
			referenceOutput, err := referenceCommand.CombinedOutput()
			if err != nil {
				t.Fatalf("reference MIRV simulator: %v\n%s", err, referenceOutput)
			}
			if !bytes.Equal(output, referenceOutput) {
				t.Fatalf("dispatch changed the simulator transcript\ncurrent: %s\nreference: %s", output, referenceOutput)
			}
			var summary captureLabSummary
			if err := json.Unmarshal(output, &summary); err != nil {
				t.Fatalf("decode simulator summary: %v\n%s", err, output)
			}
			if !summary.OK {
				t.Fatalf("simulator expectations failed: %v", summary.ExpectationFailures)
			}
			if summary.DisconnectFrame == nil || summary.QuitFrame == nil || *summary.QuitFrame <= *summary.DisconnectFrame {
				t.Fatalf("soft quit frames disconnect=%v quit=%v", summary.DisconnectFrame, summary.QuitFrame)
			}
		})
	}
}

func legacyScheduleDispatcher(t *testing.T, script string) string {
	t.Helper()
	const current = `        while (scheduleIndex < schedule.length) {
            const item = schedule[scheduleIndex];
            if (tick < item.tick) break;
            scheduleIndex++;
            if (fired[item.key]) continue;`
	const reference = `        for (const item of schedule) {
            if (fired[item.key] || tick < item.tick) continue;`
	if strings.Count(script, current) != 1 {
		t.Fatal("expected exactly one dispatcher to replace with the pre-optimization reference")
	}
	return strings.Replace(script, current, reference, 1)
}
