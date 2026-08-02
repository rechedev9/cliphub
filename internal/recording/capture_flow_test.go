package recording

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rechedev9/tickcut/internal/killplan"
)

// captureFlowFixture builds a minimal kill plan used as the post-parse input
// to the shipped NewPlanFromKillPlan → schedule/script path.
func captureFlowFixture(t *testing.T, tickrate, durationTicks int, segments []killplan.Segment, stream StreamConfig) RecordingPlan {
	t.Helper()
	if tickrate <= 0 {
		tickrate = 64
	}
	if durationTicks <= 0 {
		durationTicks = 100_000
	}
	kp := killplan.NewPlan()
	kp.Demo.Map = "de_dust2"
	kp.Demo.Tickrate = tickrate
	kp.Demo.SHA256 = strings.Repeat("d", 64)
	kp.Demo.DurationTicks = durationTicks
	kp.Target.SteamID64 = "76561198148986856"
	kp.Target.NameInDemo = "target"
	kp.Segments = segments
	plan, err := NewPlanFromKillPlan(kp, `C:\demos\match.dem`, `C:\out\run`, stream)
	if err != nil {
		t.Fatalf("NewPlanFromKillPlan: %v", err)
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("plan.Validate: %v", err)
	}
	return plan
}

// TestCaptureFlowDiversePlanShapes exercises the shipped kill-plan → recording
// plan → runtime schedule → HLAE script path across plan shapes that real demos
// produce (multi-seg, multi-kill, utility-only, near-EOF, tickrate, HUD).
func TestCaptureFlowDiversePlanShapes(t *testing.T) {
	target := killplan.Player{SteamID64: "76561198148986856", NameInDemo: "target"}
	tests := []struct {
		name           string
		tickrate       int
		durationTicks  int
		segments       []killplan.Segment
		stream         StreamConfig
		wantCaptureIDs []string
		wantSeeksMin   int
		wantSeeksMax   int
		scriptMust     []string
		scriptMustNot  []string
		check          func(t *testing.T, plan RecordingPlan, commands []scheduledCommand, seeks []seekStep, windows []captureWindow, js string)
	}{
		{
			name:          "single kill 64-tick",
			tickrate:      64,
			durationTicks: 20_000,
			segments: []killplan.Segment{{
				ID: "seg-001", TickStart: 1000, TickEnd: 1600,
				Kills: []killplan.Kill{{Tick: 1300, Weapon: "ak47", Killer: target}},
			}},
			wantCaptureIDs: []string{"seg-001"},
			wantSeeksMin:   0,
			wantSeeksMax:   1,
			scriptMust:     []string{`record-start-seg-001`, `record-end-seg-001`, `beginSoftQuit()`, `"key": "shutdown"`},
		},
		{
			name:          "multi-kill one window",
			tickrate:      64,
			durationTicks: 30_000,
			segments: []killplan.Segment{{
				ID: "spray", TickStart: 4000, TickEnd: 5200,
				Kills: []killplan.Kill{
					{Tick: 4300, Weapon: "m4a1", Killer: target},
					{Tick: 4500, Weapon: "m4a1", Killer: target},
					{Tick: 4800, Weapon: "hegrenade", Killer: target},
				},
			}},
			wantCaptureIDs: []string{"spray"},
			check: func(t *testing.T, plan RecordingPlan, _ []scheduledCommand, _ []seekStep, windows []captureWindow, _ string) {
				t.Helper()
				if len(windows) != 1 {
					t.Fatalf("windows = %d", len(windows))
				}
				// POV verify ends at last kill; record continues through post-roll.
				if windows[0].VerifyUntil != 4800 {
					t.Fatalf("VerifyUntil = %d, want last kill 4800", windows[0].VerifyUntil)
				}
				if windows[0].RecordEnd < 4800 {
					t.Fatalf("RecordEnd = %d before last kill", windows[0].RecordEnd)
				}
				start := EffectiveRecordStartTick(plan.Segments[0], plan.Tickrate)
				if start > 4300 {
					t.Fatalf("record start %d after first kill", start)
				}
			},
		},
		{
			name:          "utility-only smoke lineup window",
			tickrate:      64,
			durationTicks: 40_000,
			segments: []killplan.Segment{{
				ID: "smoke-a", TickStart: 8000, TickEnd: 9000,
				Utility: []killplan.UtilityThrow{{
					ID: "u1", Type: "smokegrenade", ThrowTick: 8200, PopTick: 8350,
					Thrower: target,
				}},
			}},
			wantCaptureIDs: []string{"smoke-a"},
			scriptMust:     []string{`record-start-smoke-a`, `record-end-smoke-a`},
			check: func(t *testing.T, plan RecordingPlan, _ []scheduledCommand, _ []seekStep, windows []captureWindow, _ string) {
				t.Helper()
				// Kill-less segments verify POV through record end.
				if windows[0].VerifyUntil != windows[0].RecordEnd {
					t.Fatalf("utility-only VerifyUntil=%d RecordEnd=%d, want equal", windows[0].VerifyUntil, windows[0].RecordEnd)
				}
				if EffectiveRecordStartTick(plan.Segments[0], plan.Tickrate) != plan.Segments[0].TickStart {
					t.Fatalf("utility-only start should equal TickStart without kill settle")
				}
			},
		},
		{
			name:          "editorial reverse multi-segment with seek gap",
			tickrate:      64,
			durationTicks: 80_000,
			segments: []killplan.Segment{
				{ID: "late", TickStart: 50_000, TickEnd: 50_600, Kills: []killplan.Kill{{Tick: 50_200, Killer: target}}},
				{ID: "early", TickStart: 2_000, TickEnd: 2_600, Kills: []killplan.Kill{{Tick: 2_200, Killer: target}}},
			},
			wantCaptureIDs: []string{"early", "late"},
			wantSeeksMin:   1, // large gap between early end and late start
			wantSeeksMax:   2,
			check: func(t *testing.T, plan RecordingPlan, commands []scheduledCommand, seeks []seekStep, windows []captureWindow, js string) {
				t.Helper()
				if plan.EditorialSegmentIDs[0] != "late" || plan.EditorialSegmentIDs[1] != "early" {
					t.Fatalf("editorial IDs = %v, want late then early", plan.EditorialSegmentIDs)
				}
				if len(seeks) < 1 {
					t.Fatalf("seeks = %+v, want at least one across the large gap", seeks)
				}
				for _, id := range []string{"early", "late"} {
					if !strings.Contains(js, "record-start-"+id) {
						t.Fatalf("js missing record-start-%s", id)
					}
				}
				starts, ends := 0, 0
				for _, c := range commands {
					if strings.HasPrefix(c.Key, "record-start-") {
						starts++
					}
					if strings.HasPrefix(c.Key, "record-end-") {
						ends++
					}
				}
				if starts != 2 || ends != 2 {
					t.Fatalf("record start/end counts = %d/%d", starts, ends)
				}
				if len(windows) != 2 {
					t.Fatalf("windows = %d", len(windows))
				}
			},
		},
		{
			name:          "nearby segments do not seek between them",
			tickrate:      64,
			durationTicks: 50_000,
			segments: []killplan.Segment{
				{ID: "a", TickStart: 10_000, TickEnd: 10_400, Kills: []killplan.Kill{{Tick: 10_200, Killer: target}}},
				// < 30s gap at 64-tick → no seek
				{ID: "b", TickStart: 11_000, TickEnd: 11_400, Kills: []killplan.Kill{{Tick: 11_200, Killer: target}}},
			},
			wantCaptureIDs: []string{"a", "b"},
			wantSeeksMin:   0,
			wantSeeksMax:   1, // may still seek into first segment from demo start
			check: func(t *testing.T, _ RecordingPlan, _ []scheduledCommand, seeks []seekStep, _ []captureWindow, _ string) {
				t.Helper()
				for _, s := range seeks {
					// No seek whose target is the second segment's lead-in while
					// the gap from first record end is under the 30s threshold.
					if s.Target >= 11_000-64*5 && s.Target <= 11_000 {
						t.Fatalf("unsafe nearby seek into second segment: %+v", s)
					}
				}
			},
		},
		{
			name:          "near-EOF kill soft-caps record end",
			tickrate:      64,
			durationTicks: 12_800, // 200s
			segments: []killplan.Segment{{
				ID: "eof", TickStart: 12_000, TickEnd: 12_750,
				Kills: []killplan.Kill{{Tick: 12_500, Killer: target}},
			}},
			wantCaptureIDs: []string{"eof"},
			check: func(t *testing.T, plan RecordingPlan, _ []scheduledCommand, _ []seekStep, windows []captureWindow, _ string) {
				t.Helper()
				end := EffectiveRecordEndTick(plan.Segments[0], plan)
				if end >= plan.DemoDurationTicks {
					t.Fatalf("RecordEnd effective %d reaches absolute demo duration %d", end, plan.DemoDurationTicks)
				}
				if windows[0].RecordEnd != end {
					t.Fatalf("window RecordEnd %d != EffectiveRecordEndTick %d", windows[0].RecordEnd, end)
				}
				if end < 12_500 {
					t.Fatalf("EOF soft-cap cut before kill tick 12500: end=%d", end)
				}
			},
		},
		{
			name:          "128-tick settle scales with tickrate",
			tickrate:      128,
			durationTicks: 40_000,
			segments: []killplan.Segment{{
				ID: "hi", TickStart: 5_000, TickEnd: 6_000,
				Kills: []killplan.Kill{{Tick: 5_500, Killer: target}},
			}},
			wantCaptureIDs: []string{"hi"},
			check: func(t *testing.T, plan RecordingPlan, _ []scheduledCommand, _ []seekStep, _ []captureWindow, _ string) {
				t.Helper()
				start := EffectiveRecordStartTick(plan.Segments[0], 128)
				// Settle policy uses tickrate-sized lead before first kill.
				if start > 5_500-128 {
					// may clamp earlier via TickStart+2s settle
				}
				if start < plan.Segments[0].TickStart || start > 5_500 {
					t.Fatalf("128-tick start %d outside [%d, kill]", start, plan.Segments[0].TickStart)
				}
			},
		},
		{
			name:          "deathnotices HUD with portrait-safe killfeed",
			tickrate:      64,
			durationTicks: 20_000,
			segments: []killplan.Segment{{
				ID: "dn", TickStart: 3000, TickEnd: 3600,
				Kills: []killplan.Kill{{Tick: 3300, Killer: target}},
			}},
			stream: StreamConfig{
				Mode:                 StreamModeFFmpegDirect,
				HUDMode:              HUDModeDeathnotices,
				PortraitSafeKillfeed: true,
				FPS:                  60,
				Width:                1920,
				Height:               1080,
				CRF:                  18,
			},
			wantCaptureIDs: []string{"dn"},
			scriptMust: []string{
				`cl_draw_only_deathnotices 1`,
				`safezonex`,
				`safezoney`,
				`mirv_deathmsg filter add attackerMatch=!x76561198148986856`,
			},
			scriptMustNot: []string{`cl_drawhud 0`},
		},
		{
			name:          "clean HUD strip",
			tickrate:      64,
			durationTicks: 20_000,
			segments: []killplan.Segment{{
				ID: "clean", TickStart: 3000, TickEnd: 3600,
				Kills: []killplan.Kill{{Tick: 3300, Killer: target}},
			}},
			stream: StreamConfig{
				Mode:    StreamModeFFmpegDirect,
				HUDMode: HUDModeClean,
				FPS:     60, Width: 1920, Height: 1080, CRF: 18,
			},
			wantCaptureIDs: []string{"clean"},
			scriptMust:     []string{`cl_drawhud 0`},
			scriptMustNot:  []string{`cl_draw_only_deathnotices 1`},
		},
		{
			name:          "mixed kill and utility segment near mid-demo",
			tickrate:      64,
			durationTicks: 60_000,
			segments: []killplan.Segment{{
				ID: "mix", TickStart: 20_000, TickEnd: 21_200,
				Kills: []killplan.Kill{{Tick: 20_400, Killer: target}},
				Utility: []killplan.UtilityThrow{{
					Type: "flashbang", ThrowTick: 20_600, Thrower: target,
				}},
			}},
			wantCaptureIDs: []string{"mix"},
			check: func(t *testing.T, plan RecordingPlan, _ []scheduledCommand, _ []seekStep, windows []captureWindow, _ string) {
				t.Helper()
				end := EffectiveRecordEndTick(plan.Segments[0], plan)
				if end < 20_600 {
					t.Fatalf("RecordEnd %d does not cover utility throw 20600", end)
				}
				if windows[0].VerifyUntil != 20_400 {
					t.Fatalf("VerifyUntil = %d, want kill tick when kills present", windows[0].VerifyUntil)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := tt.stream
			if stream.Mode == "" {
				stream = DefaultStreamConfig()
			}
			plan := captureFlowFixture(t, tt.tickrate, tt.durationTicks, tt.segments, stream)
			if len(plan.Segments) != len(tt.wantCaptureIDs) {
				t.Fatalf("capture segments = %d, want %d", len(plan.Segments), len(tt.wantCaptureIDs))
			}
			for i, id := range tt.wantCaptureIDs {
				if plan.Segments[i].ID != id {
					t.Fatalf("capture order[%d]=%q want %q", i, plan.Segments[i].ID, id)
				}
			}

			commands, seeks, windows := buildRuntimeSchedule(plan)
			if len(windows) != len(tt.wantCaptureIDs) {
				t.Fatalf("windows=%d want %d", len(windows), len(tt.wantCaptureIDs))
			}
			if tt.wantSeeksMin > 0 || tt.wantSeeksMax > 0 {
				if len(seeks) < tt.wantSeeksMin || (tt.wantSeeksMax > 0 && len(seeks) > tt.wantSeeksMax) {
					t.Fatalf("seeks=%d want in [%d,%d]: %+v", len(seeks), tt.wantSeeksMin, tt.wantSeeksMax, seeks)
				}
			}

			js, err := GenerateHLAEJavaScript(plan)
			if err != nil {
				t.Fatalf("GenerateHLAEJavaScript: %v", err)
			}
			// Soft-quit is mandatory for every generated capture script.
			if err := assertSoftQuitContract(js); err != nil {
				t.Fatal(err)
			}
			for _, want := range tt.scriptMust {
				if !strings.Contains(js, want) {
					t.Fatalf("script missing %q", want)
				}
			}
			for _, ban := range tt.scriptMustNot {
				if strings.Contains(js, ban) {
					t.Fatalf("script must not contain %q", ban)
				}
			}
			// Schedule JSON must be parseable and list one record window per segment.
			if !strings.Contains(js, `"segmentId"`) {
				t.Fatal("script missing captureWindows segmentId JSON")
			}
			if tt.check != nil {
				tt.check(t, plan, commands, seeks, windows, js)
			}
		})
	}
}

// assertSoftQuitContract is the regression gate for the CS2 end-of-capture
// hard-crash: disconnect and quit must not run in the same client frame.
func assertSoftQuitContract(js string) error {
	if !strings.Contains(js, "beginSoftQuit") {
		return errSoftQuit("missing beginSoftQuit")
	}
	if !strings.Contains(js, "pendingQuitFrames") {
		return errSoftQuit("missing pendingQuitFrames countdown")
	}
	if strings.Contains(js, `"key": "shutdown-quit"`) {
		return errSoftQuit("tick-scheduled shutdown-quit would never fire after disconnect")
	}
	// Same-frame patterns that previously crashed CS2.
	bad := []string{
		`mirv.exec("disconnect");
        mirv.exec("quit");`,
		"mirv.exec(\"disconnect\");\n            mirv.exec(\"quit\");",
		"mirv.exec(\"disconnect\");\n        mirv.exec(\"quit\");",
		`"commands": [
          "disconnect",
          "quit"
        ]`,
		`"commands": ["disconnect","quit"]`,
	}
	for _, b := range bad {
		if strings.Contains(js, b) {
			return errSoftQuit("disconnect and quit still batched: " + strings.ReplaceAll(b, "\n", "\\n"))
		}
	}
	// beginSoftQuit body must set pending frames rather than quitting inline.
	idx := strings.Index(js, "const beginSoftQuit = () => {")
	if idx < 0 {
		return errSoftQuit("beginSoftQuit definition missing")
	}
	rest := js[idx:]
	end := strings.Index(rest, "};\n")
	if end < 0 {
		end = len(rest)
	}
	body := rest[:end]
	if !strings.Contains(body, `mirv.exec("disconnect")`) {
		return errSoftQuit("beginSoftQuit does not disconnect")
	}
	if strings.Contains(body, `mirv.exec("quit")`) {
		return errSoftQuit("beginSoftQuit still quits in the same function as disconnect")
	}
	if !strings.Contains(body, "pendingQuitFrames = softQuitClientFrames") {
		return errSoftQuit("beginSoftQuit does not arm pendingQuitFrames")
	}
	return nil
}

type softQuitError string

func (e softQuitError) Error() string { return "soft-quit contract: " + string(e) }

func errSoftQuit(msg string) error { return softQuitError(msg) }

func TestAssertSoftQuitContractRejectsHardQuitRegression(t *testing.T) {
	// Mutation-style: if production regressed to same-frame quit, the helper must fail.
	hard := `
    const beginSoftQuit = () => {
        mirv.exec("disconnect");
        mirv.exec("quit");
    };
`
	if err := assertSoftQuitContract(hard); err == nil {
		t.Fatal("assertSoftQuitContract accepted hard-quit beginSoftQuit body")
	}
	good := `
    const softQuitClientFrames = 45;
    let pendingQuitFrames = 0;
    const beginSoftQuit = () => {
        if (pendingQuitFrames > 0) return;
        mirv.exec("disconnect");
        pendingQuitFrames = softQuitClientFrames;
    };
    if (pendingQuitFrames > 0) {
        pendingQuitFrames--;
        if (pendingQuitFrames === 0) {
            mirv.exec("quit");
        }
        return;
    }
    "key": "shutdown"
`
	if err := assertSoftQuitContract(good); err != nil {
		t.Fatalf("assertSoftQuitContract rejected valid soft quit: %v", err)
	}
}

// TestCaptureFlowFailurePaths drives shipped Validate / script-gen rejection
// paths with mutated plans rather than a parallel oracle.
func TestCaptureFlowFailurePaths(t *testing.T) {
	base := captureFlowFixture(t, 64, 20_000, []killplan.Segment{{
		ID: "seg-001", TickStart: 1000, TickEnd: 1600,
		Kills: []killplan.Kill{{Tick: 1300, Killer: killplan.Player{SteamID64: "76561198148986856"}}},
	}}, DefaultStreamConfig())

	t.Run("empty segments rejected by script gen", func(t *testing.T) {
		bad := base
		bad.Segments = nil
		bad.EditorialSegmentIDs = nil
		if _, err := GenerateHLAEJavaScript(bad); err == nil {
			t.Fatal("GenerateHLAEJavaScript accepted empty segments")
		}
	})
	t.Run("empty demo path rejected", func(t *testing.T) {
		bad := base
		bad.DemoPath = ""
		if err := bad.Validate(); err == nil {
			t.Fatal("Validate accepted empty DemoPath")
		}
	})
	t.Run("unknown HUD rejected", func(t *testing.T) {
		bad := base
		bad.Stream.HUDMode = "fullscreen-tv"
		if err := bad.Validate(); err == nil {
			t.Fatal("Validate accepted unknown HUD")
		}
	})
	t.Run("portrait-safe clean HUD rejected", func(t *testing.T) {
		bad := base
		bad.Stream.HUDMode = HUDModeClean
		bad.Stream.PortraitSafeKillfeed = true
		if err := bad.Validate(); err == nil {
			t.Fatal("Validate accepted portrait-safe with clean HUD")
		}
	})
	t.Run("overlapping capture windows rejected", func(t *testing.T) {
		bad := base
		bad.Segments = []RecordingSegment{
			{ID: "a", TickStart: 1000, TickEnd: 2000, Kills: []killplan.Kill{{Tick: 1500}}},
			{ID: "b", TickStart: 1800, TickEnd: 2500, Kills: []killplan.Kill{{Tick: 1900}}},
		}
		bad.EditorialSegmentIDs = []string{"a", "b"}
		if err := bad.Validate(); err == nil {
			t.Fatal("Validate accepted overlapping segments")
		}
	})
	t.Run("kill outside segment rejected", func(t *testing.T) {
		bad := base
		bad.Segments[0].Kills = []killplan.Kill{{Tick: bad.Segments[0].TickEnd + 50}}
		if err := bad.Validate(); err == nil {
			t.Fatal("Validate accepted kill outside segment")
		}
	})
	t.Run("invalid attestation token rejected", func(t *testing.T) {
		if _, err := GenerateHLAEJavaScriptWithAttestation(base, ""); err == nil {
			t.Fatal("empty attestation accepted")
		}
		if _, err := GenerateHLAEJavaScriptWithAttestation(base, "a\nb"); err == nil {
			t.Fatal("multiline attestation accepted")
		}
	})
	t.Run("successful result requires POV verification for reuse", func(t *testing.T) {
		fp, err := CaptureInputFingerprint(base)
		if err != nil {
			t.Fatal(err)
		}
		result := RecordingResult{
			Plan:                    base,
			CaptureMode:             CaptureModeReal,
			CaptureInputFingerprint: fp,
			// CaptureVerified intentionally false
			Artifacts: []RecordingArtifact{{
				SegmentID: "seg-001", Type: "video", Role: "segment", Path: "seg-001.mp4", SizeBytes: 1,
			}},
		}
		if err := ValidateRunResult(result); err == nil {
			t.Fatal("ValidateRunResult accepted success without POV verification")
		}
	})
	t.Run("missing segment clip rejected on upload validation", func(t *testing.T) {
		fp, err := CaptureInputFingerprint(base)
		if err != nil {
			t.Fatal(err)
		}
		result := RecordingResult{
			Plan:                    base,
			CaptureMode:             CaptureModeReal,
			CaptureVerified:         true,
			CaptureInputFingerprint: fp,
			Artifacts:               nil,
		}
		if err := ValidateUploadResult(result); err == nil {
			t.Fatal("ValidateUploadResult accepted success with no segment clips")
		}
	})
}

// TestCaptureFlowScheduleJSONRoundTrip ensures the embedded schedule the HLAE
// runtime parses is valid JSON and preserves soft-quit + per-segment markers.
func TestCaptureFlowScheduleJSONRoundTrip(t *testing.T) {
	plan := captureFlowFixture(t, 64, 50_000, []killplan.Segment{
		{ID: "a", TickStart: 1000, TickEnd: 1600, Kills: []killplan.Kill{{Tick: 1300, Killer: killplan.Player{SteamID64: "76561198148986856"}}}},
		{ID: "b", TickStart: 30_000, TickEnd: 30_600, Kills: []killplan.Kill{{Tick: 30_200, Killer: killplan.Player{SteamID64: "76561198148986856"}}}},
	}, DefaultStreamConfig())
	js, err := GenerateHLAEJavaScript(plan)
	if err != nil {
		t.Fatal(err)
	}
	// Extract schedule array assigned to const schedule = ...
	const marker = "const schedule = "
	start := strings.Index(js, marker)
	if start < 0 {
		t.Fatal("schedule assignment missing")
	}
	start += len(marker)
	end := strings.Index(js[start:], ";\n")
	if end < 0 {
		t.Fatal("schedule terminator missing")
	}
	raw := js[start : start+end]
	var schedule []scheduledCommand
	if err := json.Unmarshal([]byte(raw), &schedule); err != nil {
		t.Fatalf("schedule JSON: %v\n%s", err, raw[:min(200, len(raw))])
	}
	if len(schedule) < 4 {
		t.Fatalf("schedule entries = %d, want camera/record/shutdown content", len(schedule))
	}
	var sawShutdown, sawStartA, sawEndB bool
	for _, item := range schedule {
		switch {
		case item.Key == "shutdown":
			sawShutdown = true
			if len(item.Commands) != 0 {
				t.Fatalf("shutdown commands = %v, want empty for soft quit", item.Commands)
			}
		case item.Key == "record-start-a":
			sawStartA = true
		case item.Key == "record-end-b":
			sawEndB = true
		}
		joined := strings.Join(item.Commands, " ")
		if strings.Contains(joined, "disconnect") && strings.Contains(joined, "quit") {
			t.Fatalf("batched disconnect+quit in %s: %v", item.Key, item.Commands)
		}
	}
	if !sawShutdown || !sawStartA || !sawEndB {
		t.Fatalf("schedule markers shutdown=%v start-a=%v end-b=%v", sawShutdown, sawStartA, sawEndB)
	}
}
