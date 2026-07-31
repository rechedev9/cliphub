package recording

import (
	"strings"
	"testing"

	"github.com/rechedev9/tickcut/internal/killplan"
)

func TestStreamSetupCommandsQuotesRecordName(t *testing.T) {
	plan := testPlan()
	// A double quote in the output path must be escaped, not left to terminate
	// the quoted console argument and inject further MIRV commands.
	plan.OutputDir = `C:\out\evil" ; quit ;`

	cmds := streamSetupCommands(plan)
	nameCmd := ""
	for _, c := range cmds {
		if strings.HasPrefix(c, "mirv_streams record name ") {
			nameCmd = c
			break
		}
	}
	if nameCmd == "" {
		t.Fatal("no 'mirv_streams record name' command generated")
	}
	if !strings.Contains(nameCmd, `\"`) {
		t.Fatalf("record name command did not escape embedded quote: %q", nameCmd)
	}
}

func testPlan() RecordingPlan {
	return RecordingPlan{
		CaptureContract:       CaptureContractVersion,
		KillPlanSchemaVersion: killplan.SchemaVersion,
		DemoPath:              `C:\demos\x.dem`,
		DemoSHA256:            strings.Repeat("a", 64),
		DemoDurationTicks:     40000,
		OutputDir:             `C:\out`,
		TargetSteamID64:       "76561198148986856",
		TargetNameInDemo:      "maaryy",
		TargetAccountID:       188721128,
		Tickrate:              64,
		Segments: []RecordingSegment{
			{ID: "seg-001", TickStart: 22086, TickEnd: 22406},
			{ID: "seg-002", TickStart: 31746, TickEnd: 32258},
		},
		Stream:  DefaultStreamConfig(),
		Runtime: RuntimeConfig{QuitTickPad: 200},
	}
}

func TestGenerateHLAEJavaScriptUsesOneShotTickSchedule(t *testing.T) {
	js, err := GenerateHLAEJavaScript(testPlan())
	if err != nil {
		t.Fatalf("GenerateHLAEJavaScript error = %v", err)
	}
	for _, want := range []string{
		`mirv.events.clientFrameStageNotify.on`,
		`if (!mirv.isPlayingDemo()) {`,
		`mirv.getDemoTick()`,
		`if (tick === undefined || tick < 0) return;`,
		`tick < item.tick`,
		`fired[item.key] = true`,
		`cl_demo_predict 0`,
		`cl_trueview_show_status 0`,
		`mirv_panorama panelstyle panelId=trueview_row opacity=0`,
		`"commands": [`,
		`camera-warmup-seg-001`,
		`camera-lead-3s-seg-001`,
		`camera-lead-2s-seg-001`,
		`camera-lead-1s-seg-001`,
		`camera-lock-seg-001`,
		`camera-relock-seg-001`,
		// Seeks are driven by the runtime (re-issued until they land), declared as
		// targets in the seeks array rather than one-shot demo_gototick commands.
		`const seeks = `,
		"\"target\": 21766",
		"\"target\": 31426",
		`const maxSeekAttempts = 6000`,
		`seekAttempts % 60 === 0`,
		`did not reach tick`,
		"mirv.exec(`demo_gototick ${s.target}`)",
		`demoui`,
		`mirv_streams record fps 60`,
		`mirv_streams record screen enabled 1`,
		`mirv_streams settings add ffmpeg zvFfmpegYuv420pCrf18`,
		`-crf 18`,
		`mirv_streams record screen settings zvFfmpegYuv420pCrf18`,
		`"disconnect"`,
		`"quit"`,
		`spec_show_xray 0`,
		`cl_spec_show_bindings 0`,
		`cl_drawhud 1`,
		`cl_draw_only_deathnotices 0`,
		`cl_show_observer_crosshair 2`,
		`crosshair 1`,
		`mirv_streams record start`,
		`mirv_streams record end`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("generated JS missing %q\n%s", want, js)
		}
	}
}

func TestGenerateHLAEJavaScriptBindsRecordEndToActiveSegment(t *testing.T) {
	js, err := GenerateHLAEJavaScript(testPlan())
	if err != nil {
		t.Fatalf("GenerateHLAEJavaScript error = %v", err)
	}
	for _, want := range []string{
		"activeSegment !== null",
		"activeSegment !== window.segmentId",
		"capture end ${item.key} does not match active segment",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("generated script missing %q", want)
		}
	}
	if strings.Contains(js, `if (item.key.startsWith("record-end-")) activeSegment = null;`) {
		t.Fatal("generated script contains unbound record-end state reset")
	}
}

func TestGenerateHLAEJavaScriptLocksAndVerifiesTheObservedSteamID(t *testing.T) {
	token := strings.Repeat("unit-test-", 4)
	js, err := GenerateHLAEJavaScriptWithAttestation(testPlan(), token)
	if err != nil {
		t.Fatalf("GenerateHLAEJavaScript error = %v", err)
	}
	for _, want := range []string{
		`const targetSteamId = "76561198148986856"`,
		`mirv.getHighestEntityIndex()`,
		`entity.isPlayerController()`,
		`entity.getSteamId().toString() === targetSteamId`,
		`mirv.getEntityFromSplitScreenPlayer(0)`,
		`getObserverTargetHandle()`,
		`spec_player ${targetIndex}`,
		`[zackvideo] capture_failed:`,
		CaptureFailedMarker,
		CaptureVerifiedMarker,
		CaptureFailedAttestation(token),
		CaptureVerifiedAttestation(token),
		`observer target`,
		`const maxUnknownObserverFrames = 3`,
		`if (observed === null)`,
		`unknownObserverFrames++`,
		`unknownObserverFrames >= maxUnknownObserverFrames`,
		`unknownObserverFrames = 0`,
		`} else if (observed !== targetSteamId) {`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("generated JS missing verified POV contract %q\n%s", want, js)
		}
	}
	if strings.Contains(js, `spec_player_by_accountid`) || strings.Contains(js, `spec_player "maaryy"`) {
		t.Fatalf("generated JS still depends on an unverified account/name selector:\n%s", js)
	}
}

func TestGenerateHLAEJavaScriptRejectsInvalidAttestationToken(t *testing.T) {
	for _, token := range []string{"", "line1\nline2"} {
		if _, err := GenerateHLAEJavaScriptWithAttestation(testPlan(), token); err == nil {
			t.Fatalf("token %q was accepted", token)
		}
	}
}

func TestGenerateHLAEJavaScriptAttestsWhenDemoPlaybackEndsEarly(t *testing.T) {
	token := strings.Repeat("unit-test-", 4)
	js, err := GenerateHLAEJavaScriptWithAttestation(testPlan(), token)
	if err != nil {
		t.Fatalf("GenerateHLAEJavaScript error = %v", err)
	}
	for _, want := range []string{
		`const demoEndedGraceFrames = 30`,
		`if (!armed || fired["shutdown"]) return;`,
		`demoEndedFrames++`,
		`demoEndedFrames < demoEndedGraceFrames`,
		`demo playback ended before every protected segment completed`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("generated JS missing demo-end attestation contract %q\n%s", want, js)
		}
	}
	// The verified attestation must be emitted by both completion paths (message
	// plus echo each): the scheduled shutdown item and the demo-end fallback.
	if got, want := strings.Count(js, CaptureVerifiedAttestation(token)), 4; got != want {
		t.Errorf("verified attestation occurrences = %d, want %d\n%s", got, want, js)
	}
}

func TestBuildRuntimeScheduleAllowsPOVDriftDuringKillPostRoll(t *testing.T) {
	plan := testPlan()
	// Keep the segment well before DemoDurationTicks so EOF soft-cap does not
	// rewrite the editorial post-roll under test.
	plan.DemoDurationTicks = 200_000
	plan.Segments = []RecordingSegment{{
		ID:        "seg-021",
		TickStart: 178844,
		TickEnd:   179484,
		Kills: []killplan.Kill{
			{Tick: 179164},
			{Tick: 179100},
		},
	}}

	_, _, windows := buildRuntimeSchedule(plan)
	if got, want := len(windows), 1; got != want {
		t.Fatalf("capture window count = %d, want %d", got, want)
	}
	window := windows[0]
	if got, want := window.VerifyUntil, 179164; got != want {
		t.Errorf("window %s verifyUntil = %d, want last kill %d", window.SegmentID, got, want)
	}
	if got, want := window.RecordEnd, 179484; got != want {
		t.Errorf("window %s recordEnd = %d, want post-roll end %d", window.SegmentID, got, want)
	}
	if preKillTick := 179150; preKillTick < window.LockFrom || preKillTick > window.VerifyUntil {
		t.Fatalf("pre-kill tick %d should remain inside the protected POV window: %+v", preKillTick, window)
	}
	if deathTick := 179242; deathTick <= window.VerifyUntil || deathTick > window.RecordEnd {
		t.Fatalf("death tick %d should be recorded after POV verification ends: %+v", deathTick, window)
	}
}

func TestEffectiveRecordEndTickPullsBackNearDemoEOF(t *testing.T) {
	plan := testPlan()
	plan.DemoDurationTicks = 10_500
	plan.Tickrate = 64
	segment := RecordingSegment{
		ID:        "seg-eof",
		TickStart: 10_000,
		TickEnd:   10_500, // would record into absolute EOF
		Kills:     []killplan.Kill{{Tick: 10_200}},
	}
	got := EffectiveRecordEndTick(segment, plan)
	want := 10_500 - 2*64 // soft cap (kill is still before softCap)
	if got != want {
		t.Fatalf("EffectiveRecordEndTick = %d, want soft-cap %d", got, want)
	}
	if got >= plan.DemoDurationTicks {
		t.Fatalf("EffectiveRecordEndTick = %d must stay before demo duration %d", got, plan.DemoDurationTicks)
	}
}

func TestEffectiveRecordEndTickLeavesMiddleOfDemoPostRollIntact(t *testing.T) {
	plan := testPlan()
	plan.DemoDurationTicks = 200_000
	plan.Tickrate = 64
	segment := RecordingSegment{
		ID:        "seg-mid",
		TickStart: 50_000,
		TickEnd:   50_640,
		Kills:     []killplan.Kill{{Tick: 50_320}},
	}
	got := EffectiveRecordEndTick(segment, plan)
	if got != segment.TickEnd {
		t.Fatalf("EffectiveRecordEndTick = %d, want unchanged post-roll %d", got, segment.TickEnd)
	}
}

func TestEffectiveRecordEndTickShortTailWhenKillInsideEOFMargin(t *testing.T) {
	// Parser short-tail leaves TickEnd covering a near-EOF kill (e.g. 9999).
	// Soft-capping alone would stop before the kill (9872); short-tail must cover it.
	plan := testPlan()
	plan.DemoDurationTicks = 10_000
	plan.Tickrate = 64
	const killTick = 9_950
	segment := RecordingSegment{
		ID:        "seg-kill-margin",
		TickStart: 9_500,
		TickEnd:   9_999, // plan already short-tailed by parser
		Kills:     []killplan.Kill{{Tick: killTick}},
	}
	got := EffectiveRecordEndTick(segment, plan)
	softCap := 10_000 - 2*64 // 9872 — must NOT win
	if got == softCap {
		t.Fatalf("EffectiveRecordEndTick soft-capped to %d before kill %d", got, killTick)
	}
	if got < killTick {
		t.Fatalf("EffectiveRecordEndTick = %d ends before kill %d", got, killTick)
	}
	if got >= plan.DemoDurationTicks {
		t.Fatalf("EffectiveRecordEndTick = %d lands on absolute duration %d", got, plan.DemoDurationTicks)
	}
	// Short tail wants kill+64=10014, hard headroom is duration-1=9999.
	if want := plan.DemoDurationTicks - 1; got != want {
		t.Fatalf("EffectiveRecordEndTick = %d, want EOF headroom %d", got, want)
	}
}

func TestEffectiveRecordEndTickShortTailWhenUtilityThrowInsideEOFMargin(t *testing.T) {
	// Regression: last event must include utility ThrowTick/PopTick, not only kills.
	// duration=10000 tickrate=64 throw=9950 TickEnd=9999 → must not RecordEnd=9872.
	plan := testPlan()
	plan.DemoDurationTicks = 10_000
	plan.Tickrate = 64
	const throwTick = 9_950
	segment := RecordingSegment{
		ID:        "seg-util-margin",
		TickStart: 9_500,
		TickEnd:   9_999,
		Utility: []killplan.UtilityThrow{{
			Type:      "smokegrenade",
			ThrowTick: throwTick,
			PopTick:   throwTick + 20,
		}},
	}
	got := EffectiveRecordEndTick(segment, plan)
	softCap := 10_000 - 2*64
	if got <= softCap {
		t.Fatalf("EffectiveRecordEndTick = %d soft-capped at/before %d; must cover utility throw %d", got, softCap, throwTick)
	}
	if got < throwTick {
		t.Fatalf("EffectiveRecordEndTick = %d ends before utility throw %d", got, throwTick)
	}
	if got < throwTick+20 {
		t.Fatalf("EffectiveRecordEndTick = %d ends before utility pop %d", got, throwTick+20)
	}
	if got >= plan.DemoDurationTicks {
		t.Fatalf("EffectiveRecordEndTick = %d lands on absolute duration %d", got, plan.DemoDurationTicks)
	}
	if want := plan.DemoDurationTicks - 1; got != want {
		t.Fatalf("EffectiveRecordEndTick = %d, want EOF headroom %d", got, want)
	}
}

func TestBuildRuntimeScheduleRecordEndUsesEOFSoftCap(t *testing.T) {
	plan := testPlan()
	plan.DemoDurationTicks = 10_500
	plan.Tickrate = 64
	plan.Segments = []RecordingSegment{{
		ID:        "seg-eof",
		TickStart: 10_000,
		TickEnd:   10_500,
		Kills:     []killplan.Kill{{Tick: 10_200}},
	}}
	_, _, windows := buildRuntimeSchedule(plan)
	if len(windows) != 1 {
		t.Fatalf("windows = %d, want 1", len(windows))
	}
	want := 10_500 - 2*64
	if windows[0].RecordEnd != want {
		t.Fatalf("RecordEnd = %d, want soft-cap %d (not absolute duration)", windows[0].RecordEnd, want)
	}
	if windows[0].RecordEnd >= plan.DemoDurationTicks {
		t.Fatalf("RecordEnd = %d lands on demo duration %d", windows[0].RecordEnd, plan.DemoDurationTicks)
	}
}

func TestBuildRuntimeScheduleRecordEndCoversUtilityInsideEOFMargin(t *testing.T) {
	plan := testPlan()
	plan.DemoDurationTicks = 10_000
	plan.Tickrate = 64
	const throwTick = 9_950
	plan.Segments = []RecordingSegment{{
		ID:        "seg-util",
		TickStart: 9_500,
		TickEnd:   9_999,
		Utility:   []killplan.UtilityThrow{{Type: "smokegrenade", ThrowTick: throwTick, PopTick: throwTick + 20}},
	}}
	_, _, windows := buildRuntimeSchedule(plan)
	if len(windows) != 1 {
		t.Fatalf("windows = %d, want 1", len(windows))
	}
	if windows[0].RecordEnd < throwTick+20 {
		t.Fatalf("RecordEnd = %d does not cover utility through pop %d", windows[0].RecordEnd, throwTick+20)
	}
	if windows[0].RecordEnd >= plan.DemoDurationTicks {
		t.Fatalf("RecordEnd = %d lands on duration %d", windows[0].RecordEnd, plan.DemoDurationTicks)
	}
}

func TestBuildRuntimeScheduleVerifiesPOVThroughRecordEndWithoutKills(t *testing.T) {
	plan := testPlan()
	_, _, windows := buildRuntimeSchedule(plan)
	if got, want := len(windows), len(plan.Segments); got != want {
		t.Fatalf("capture window count = %d, want %d", got, want)
	}
	for i, window := range windows {
		if got, want := window.VerifyUntil, plan.Segments[i].TickEnd; got != want {
			t.Errorf("window %s verifyUntil = %d, want record end %d", window.SegmentID, got, want)
		}
	}
}

func TestBuildScheduleDoesNotSeekAcrossNearbySegments(t *testing.T) {
	plan := testPlan()
	plan.Segments = []RecordingSegment{
		{ID: "seg-001", TickStart: 114075, TickEnd: 114587},
		{ID: "seg-002", TickStart: 115421, TickEnd: 115933},
		{ID: "seg-003", TickStart: 117663, TickEnd: 118175},
		{ID: "seg-004", TickStart: 141619, TickEnd: 142530},
	}

	_, seeks := buildSchedule(plan)
	if got, want := len(seeks), 2; got != want {
		t.Fatalf("seek count = %d, want %d: %+v", got, want, seeks)
	}
	if got, want := seeks[0].Target, 113755; got != want {
		t.Errorf("first seek target = %d, want %d", got, want)
	}
	if got, want := seeks[1].Target, 141299; got != want {
		t.Errorf("second seek target = %d, want %d", got, want)
	}
	for _, seek := range seeks {
		if seek.Target == 117343 {
			t.Fatalf("nearby segment generated unsafe seek: %+v", seek)
		}
	}
}

func TestBuildScheduleSeekGapThreshold(t *testing.T) {
	for _, tt := range []struct {
		name      string
		gapTicks  int
		wantSeeks int
	}{
		{name: "one tick below threshold", gapTicks: 30*64 - 1, wantSeeks: 0},
		{name: "at threshold", gapTicks: 30 * 64, wantSeeks: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			plan := testPlan()
			seekAfter := 50
			leadTicks := 5 * plan.Tickrate
			plan.Segments = []RecordingSegment{{
				ID:        "seg-001",
				TickStart: seekAfter + leadTicks + tt.gapTicks,
				TickEnd:   seekAfter + leadTicks + tt.gapTicks + plan.Tickrate,
			}}

			_, seeks := buildSchedule(plan)
			if got := len(seeks); got != tt.wantSeeks {
				t.Fatalf("seek count = %d, want %d: %+v", got, tt.wantSeeks, seeks)
			}
		})
	}
}

func TestBuildScheduleLaterSegmentSeekGapThreshold(t *testing.T) {
	for _, tt := range []struct {
		name      string
		gapTicks  int
		wantSeeks int
	}{
		{name: "one tick below threshold", gapTicks: 30*64 - 1, wantSeeks: 0},
		{name: "at threshold", gapTicks: 30 * 64, wantSeeks: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			plan := testPlan()
			seekAfter := 1000 + 32
			leadTicks := 5 * plan.Tickrate
			plan.Segments = []RecordingSegment{
				{ID: "seg-001", TickStart: 370, TickEnd: 1000},
				{
					ID:        "seg-002",
					TickStart: seekAfter + leadTicks + tt.gapTicks,
					TickEnd:   seekAfter + leadTicks + tt.gapTicks + plan.Tickrate,
				},
			}

			_, seeks := buildSchedule(plan)
			if got := len(seeks); got != tt.wantSeeks {
				t.Fatalf("seek count = %d, want %d: %+v", got, tt.wantSeeks, seeks)
			}
		})
	}
}

func TestGenerateHLAEJavaScriptUsesConfiguredCRF(t *testing.T) {
	p := testPlan()
	p.Stream.CRF = 16
	js, err := GenerateHLAEJavaScript(p)
	if err != nil {
		t.Fatalf("GenerateHLAEJavaScript error = %v", err)
	}
	for _, want := range []string{
		`mirv_streams settings add ffmpeg zvFfmpegYuv420pCrf16`,
		`-crf 16`,
		`mirv_streams record screen settings zvFfmpegYuv420pCrf16`,
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("generated JS missing %q\n%s", want, js)
		}
	}
	if strings.Contains(js, `afxFfmpegYuv420p`) {
		t.Fatalf("generated JS should use the CRF-specific preset:\n%s", js)
	}
}

func TestEffectiveRecordStartTickAllowsCameraToSettleBeforeFirstKill(t *testing.T) {
	segment := RecordingSegment{
		ID:        "seg-001",
		TickStart: 14029,
		TickEnd:   14770,
		Kills: []killplan.Kill{
			{Tick: 14221},
			{Tick: 14450},
		},
	}
	if got, want := effectiveRecordStartTick(segment, 64), 14157; got != want {
		t.Fatalf("effectiveRecordStartTick() = %d, want %d", got, want)
	}

	segment.Kills = nil
	if got, want := effectiveRecordStartTick(segment, 64), segment.TickStart; got != want {
		t.Fatalf("effectiveRecordStartTick() without kills = %d, want %d", got, want)
	}
}

func TestGenerateHLAEJavaScriptDoesNotSplitDemoControlledNames(t *testing.T) {
	plan := testPlan()
	plan.TargetNameInDemo = "victim;quit;rest"

	js, err := GenerateHLAEJavaScript(plan)
	if err != nil {
		t.Fatalf("GenerateHLAEJavaScript error = %v", err)
	}
	if strings.Contains(js, "item.cmd.split") || strings.Contains(js, "victim;quit;rest") {
		t.Fatalf("generated JS permits injected command:\n%s", js)
	}
	if !strings.Contains(js, "for (const cmd of item.commands)") {
		t.Fatalf("generated JS does not use structured commands:\n%s", js)
	}
}

func TestGenerateHLAEJavaScriptGameplayHUDIsDefault(t *testing.T) {
	p := testPlan()
	p.Stream.HUDMode = ""
	js, err := GenerateHLAEJavaScript(p)
	if err != nil {
		t.Fatalf("GenerateHLAEJavaScript error = %v", err)
	}
	if !strings.Contains(js, `cl_drawhud 1`) {
		t.Fatalf("generated JS missing gameplay HUD:\n%s", js)
	}
	if strings.Contains(js, `cl_drawhud 0`) {
		t.Fatalf("generated JS hides HUD in default mode:\n%s", js)
	}
	if strings.Contains(js, `mirv_deathmsg`) || strings.Contains(js, `safezonex`) || strings.Contains(js, `safezoney`) {
		t.Fatalf("plain gameplay unexpectedly configures portrait killfeed:\n%s", js)
	}
}

func TestGenerateHLAEJavaScriptPortraitSafeGameplayHUD(t *testing.T) {
	p := testPlan()
	p.Stream.HUDMode = HUDModeGameplay
	p.Stream.PortraitSafeKillfeed = true

	js, err := GenerateHLAEJavaScript(p)
	if err != nil {
		t.Fatalf("GenerateHLAEJavaScript error = %v", err)
	}
	for _, want := range []string{
		`cl_drawhud 1`,
		`cl_draw_only_deathnotices 0`,
		`mirv_deathmsg clear`,
		`mirv_deathmsg filter clear`,
		`mirv_deathmsg filter add attackerMatch=!x76561198148986856 block=1 lastRule=1`,
		`mirv_deathmsg localPlayer -1`,
		`mirv_deathmsg lifetime 1.6`,
		`mirv_deathmsg localPlayer default`,
		`mirv_deathmsg lifetime default`,
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("generated JS missing %q\n%s", want, js)
		}
	}
	if strings.Contains(js, `cl_draw_only_deathnotices 1`) {
		t.Fatalf("portrait-safe gameplay hid the full HUD:\n%s", js)
	}
	if strings.Contains(js, `safezonex`) || strings.Contains(js, `safezoney`) {
		t.Fatalf("portrait-safe gameplay moved the full HUD into the center crop:\n%s", js)
	}
}

func TestGenerateHLAEJavaScriptCleanHUDMode(t *testing.T) {
	p := testPlan()
	p.Stream.HUDMode = HUDModeClean
	js, err := GenerateHLAEJavaScript(p)
	if err != nil {
		t.Fatalf("GenerateHLAEJavaScript error = %v", err)
	}
	for _, want := range []string{`"spec_show_xray 0"`, `"cl_drawhud 0"`} {
		if !strings.Contains(js, want) {
			t.Fatalf("generated JS missing clean HUD command %q:\n%s", want, js)
		}
	}
	if strings.Contains(js, `cl_draw_only_deathnotices 0`) {
		t.Fatalf("clean mode should not enable gameplay HUD commands:\n%s", js)
	}
}

func TestGenerateHLAEJavaScriptDeathnoticesHUDMode(t *testing.T) {
	p := testPlan()
	p.Stream.HUDMode = HUDModeDeathnotices
	p.Stream.PortraitSafeKillfeed = true
	js, err := GenerateHLAEJavaScript(p)
	if err != nil {
		t.Fatalf("GenerateHLAEJavaScript error = %v", err)
	}
	for _, want := range []string{
		`cl_drawhud 1`,
		`cl_draw_only_deathnotices 1`,
		`cl_show_observer_crosshair 2`,
		`crosshair 1`,
		`mirv_deathmsg clear`,
		`mirv_deathmsg filter clear`,
		`mirv_deathmsg filter add attackerMatch=!x76561198148986856 block=1 lastRule=1`,
		`mirv_deathmsg localPlayer -1`,
		`mirv_deathmsg lifetime 1.6`,
		`safezonex 0.28`,
		`safezoney 0.82`,
		`mirv_deathmsg localPlayer default`,
		`mirv_deathmsg lifetime default`,
		`safezonex 1`,
		`safezoney 1`,
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("generated JS missing %q\n%s", want, js)
		}
	}
	if strings.Contains(js, `cl_drawhud 0`) || strings.Contains(js, `cl_draw_only_deathnotices 0`) {
		t.Fatalf("deathnotices mode should keep death notices visible:\n%s", js)
	}
}

func TestGenerateHLAEJavaScriptLandscapeDeathnoticesKeepNativeSafeZone(t *testing.T) {
	p := testPlan()
	p.Stream.HUDMode = HUDModeDeathnotices

	js, err := GenerateHLAEJavaScript(p)
	if err != nil {
		t.Fatalf("GenerateHLAEJavaScript error = %v", err)
	}
	if strings.Contains(js, `safezonex 0.28`) {
		t.Fatalf("landscape deathnotices moved into portrait safe zone:\n%s", js)
	}
	if strings.Contains(js, `safezoney 0.82`) {
		t.Fatalf("landscape deathnotices moved into portrait safe zone:\n%s", js)
	}
	if !strings.Contains(js, `mirv_deathmsg filter add attackerMatch=!x76561198148986856 block=1 lastRule=1`) {
		t.Fatalf("landscape deathnotices missing target filter:\n%s", js)
	}
}

func TestGenerateHLAEJavaScriptEscapesCommandsViaJSON(t *testing.T) {
	p := testPlan()
	p.OutputDir = `C:\Users\name with spaces\out`
	js, err := GenerateHLAEJavaScript(p)
	if err != nil {
		t.Fatalf("GenerateHLAEJavaScript error = %v", err)
	}
	if !strings.Contains(js, `C:/Users/name with spaces/out`) {
		t.Errorf("generated JS should use slash-normalized output path:\n%s", js)
	}
	if strings.Contains(js, `\Users`) {
		t.Errorf("generated JS contains unescaped Windows backslashes:\n%s", js)
	}
}

func TestGenerateHLAEJavaScriptTimescale(t *testing.T) {
	p := testPlan()
	p.Runtime.HostTimescale = 2
	js, err := GenerateHLAEJavaScript(p)
	if err != nil {
		t.Fatalf("GenerateHLAEJavaScript error = %v", err)
	}
	if !strings.Contains(js, `host_timescale 2`) || !strings.Contains(js, `host_timescale 1`) {
		t.Errorf("generated JS missing host_timescale wrapper:\n%s", js)
	}
}
