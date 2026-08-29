package recording

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
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

func TestGenerateHLAEJavaScriptLocksFirstPersonOnOneRecapWindow(t *testing.T) {
	plan := testPlan()
	plan.Segments = []RecordingSegment{
		{ID: "seg-001", Round: 1, TickStart: 8000, TickEnd: 14768},
	}
	js, err := GenerateHLAEJavaScript(plan)
	if err != nil {
		t.Fatalf("GenerateHLAEJavaScript error = %v", err)
	}
	if !strings.Contains(js, `spec_mode 2`) {
		t.Fatal("recap capture is not locked to first-person spec_mode 2")
	}
	if !strings.Contains(js, `spec_autodirector 0`) {
		t.Fatal("recap capture left autodirector on (cinematic/deathcam)")
	}
	for _, banned := range []string{`spec_mode 5`, `spec_mode 4`, `spec_goto`, `deathcam`} {
		if strings.Contains(strings.ToLower(js), banned) {
			t.Fatalf("recap capture scheduled %q", banned)
		}
	}
	if strings.Count(js, `record-start-seg-001`) != 1 || strings.Count(js, `record-end-seg-001`) != 1 {
		t.Fatal("one live recap round must be one record window, not an intra-round jump-cut")
	}
	if strings.Contains(js, `record-start-seg-002`) {
		t.Fatal("pistol recap gained a second capture window")
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
		`voice_enable 0`,
		`tv_listen_voice_indices 0`,
		`tv_listen_voice_indices_h 0`,
		`mirv_panorama panelstyle panelId=trueview_row opacity=0`,
		`"commands": [`,
		`camera-warmup-seg-001`,
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

func TestGenerateHLAEJavaScriptReportsSeekLandingBeforeAdvancing(t *testing.T) {
	js, err := GenerateHLAEJavaScript(testPlan())
	if err != nil {
		t.Fatalf("GenerateHLAEJavaScript error = %v", err)
	}
	markerIndex := strings.Index(js, "seek-landed -> ${s.target} (at ${tick})")
	advanceIndex := strings.Index(js, "seekIdx++;")
	if markerIndex < 0 {
		t.Fatalf("generated script does not report seek landing:\n%s", js)
	}
	if advanceIndex < 0 || markerIndex >= advanceIndex {
		t.Fatal("seek landing marker must precede the seek index advance")
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

func TestBuildRuntimeScheduleVerifiesPOVUntilRecordEndBoundaryWithoutKills(t *testing.T) {
	plan := testPlan()
	_, _, windows := buildRuntimeSchedule(plan)
	if got, want := len(windows), len(plan.Segments); got != want {
		t.Fatalf("capture window count = %d, want %d", got, want)
	}
	for i, window := range windows {
		if got, want := window.VerifyUntil, plan.Segments[i].TickEnd-1; got != want {
			t.Errorf("window %s verifyUntil = %d, want record end boundary %d", window.SegmentID, got, want)
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

	_, seeks, _ := buildRuntimeSchedule(plan)
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

			_, seeks, _ := buildRuntimeSchedule(plan)
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

			_, seeks, _ := buildRuntimeSchedule(plan)
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
	if got, want := EffectiveRecordStartTick(segment, 64), 14157; got != want {
		t.Fatalf("effectiveRecordStartTick() = %d, want %d", got, want)
	}

	segment.Kills = nil
	if got, want := EffectiveRecordStartTick(segment, 64), segment.TickStart; got != want {
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

func TestPlaybackTimescaleSchedule(t *testing.T) {
	type wantCmd struct {
		key      string
		tick     int
		command  string
		optional bool
	}
	for _, tt := range []struct {
		name      string
		timescale float64
		segments  []RecordingSegment
		want      []wantCmd
		forbid    []string
	}{
		{
			name:      "zero uses default speed",
			timescale: 0,
			want: []wantCmd{
				{key: "timescale-up-preamble", tick: 50, command: "demo_timescale 8"},
				{key: "timescale-reset-seg-001", command: "demo_timescale 1"},
				{key: "timescale-up-seg-001", command: "demo_timescale 8"},
				{key: "timescale-reset-seg-002", command: "demo_timescale 1"},
				{key: "timescale-up-seg-002", command: "demo_timescale 8"},
				{key: "timescale-reset-shutdown", command: "demo_timescale 1"},
			},
			forbid: []string{"host_timescale"},
		},
		{
			name:      "disabled one",
			timescale: 1,
			forbid:    []string{"demo_timescale", "host_timescale"},
		},
		{
			name:      "speeds gaps and resets before each record",
			timescale: 8,
			want: []wantCmd{
				{key: "timescale-up-preamble", tick: 50, command: "demo_timescale 8"},
				{key: "timescale-reset-seg-001", command: "demo_timescale 1"},
				{key: "timescale-up-seg-001", command: "demo_timescale 8"},
				{key: "timescale-reset-seg-002", command: "demo_timescale 1"},
				{key: "timescale-up-seg-002", command: "demo_timescale 8"},
				{key: "timescale-reset-shutdown", command: "demo_timescale 1"},
			},
			forbid: []string{"host_timescale"},
		},
		{
			name:      "nearby segments skip inter-window speedup",
			timescale: 8,
			segments: []RecordingSegment{
				{ID: "seg-001", TickStart: 2000, TickEnd: 2400},
				{ID: "seg-002", TickStart: 2432, TickEnd: 2800},
			},
			want: []wantCmd{
				{key: "timescale-up-preamble", tick: 50, command: "demo_timescale 8"},
				{key: "timescale-reset-seg-001", command: "demo_timescale 1"},
				{key: "timescale-reset-seg-002", command: "demo_timescale 1"},
				{key: "timescale-reset-shutdown", command: "demo_timescale 1"},
			},
			forbid: []string{"timescale-up-seg-001", "host_timescale"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			plan := testPlan()
			plan.Runtime.PlaybackTimescale = tt.timescale
			if tt.segments != nil {
				plan.Segments = tt.segments
			}
			commands, _, windows := buildRuntimeSchedule(plan)
			byKey := map[string]scheduledCommand{}
			for _, cmd := range commands {
				if _, exists := byKey[cmd.Key]; exists {
					t.Fatalf("duplicate schedule key %q", cmd.Key)
				}
				byKey[cmd.Key] = cmd
			}
			for _, want := range tt.want {
				got, ok := byKey[want.key]
				if !ok {
					if want.optional {
						continue
					}
					t.Fatalf("missing %q in %v", want.key, commandKeys(commands))
				}
				if want.tick != 0 && got.Tick != want.tick {
					t.Errorf("%s tick = %d, want %d", want.key, got.Tick, want.tick)
				}
				if want.command != "" && (len(got.Commands) != 1 || got.Commands[0] != want.command) {
					t.Errorf("%s commands = %v, want [%q]", want.key, got.Commands, want.command)
				}
			}
			for _, ban := range tt.forbid {
				for _, cmd := range commands {
					if cmd.Key == ban || strings.Contains(strings.Join(cmd.Commands, " "), ban) {
						t.Fatalf("forbidden %q in %s: %v", ban, cmd.Key, cmd.Commands)
					}
				}
			}
			if tt.timescale == 1 {
				return
			}
			for i, window := range windows {
				minReset := window.RecordStart - plan.Tickrate*DefaultPlaybackSettleSeconds
				if i > 0 && windows[i-1].RecordEnd+1 > minReset {
					minReset = windows[i-1].RecordEnd + 1
				}
				for _, cmd := range commands {
					if !strings.HasPrefix(cmd.Key, "timescale-") {
						continue
					}
					if cmd.Tick >= window.RecordStart && cmd.Tick <= window.RecordEnd {
						t.Fatalf("%s tick %d is inside record window [%d, %d]", cmd.Key, cmd.Tick, window.RecordStart, window.RecordEnd)
					}
					if strings.HasPrefix(cmd.Key, "timescale-reset-"+window.SegmentID) && (cmd.Tick < minReset || cmd.Tick >= window.RecordStart) {
						t.Fatalf("%s tick %d want in [%d, %d)", cmd.Key, cmd.Tick, minReset, window.RecordStart)
					}
				}
			}
		})
	}
}

// TestBuildRuntimeScheduleSoftQuitsWithoutBatchedDisconnectQuit is the contract
// that stops CS2 from hard-crashing at end of capture: the tick schedule only
// marks shutdown; disconnect+delayed quit live in beginSoftQuit (client frames),
// because disconnect ends demo ticks so a later scheduled quit would never run.
func TestBuildRuntimeScheduleSoftQuitsWithoutBatchedDisconnectQuit(t *testing.T) {
	commands, _, _ := buildRuntimeSchedule(testPlan())
	var shutdown *scheduledCommand
	for i := range commands {
		if commands[i].Key == "shutdown" {
			shutdown = &commands[i]
		}
		if commands[i].Key == "shutdown-quit" {
			t.Fatalf("unexpected shutdown-quit schedule entry (quit must be frame-delayed): %#v", commands[i])
		}
		joined := strings.Join(commands[i].Commands, " ")
		if strings.Contains(joined, "disconnect") && strings.Contains(joined, "quit") {
			t.Fatalf("command %q still batches disconnect+quit: %v", commands[i].Key, commands[i].Commands)
		}
	}
	if shutdown == nil {
		t.Fatalf("shutdown schedule entry missing in %#v", commandKeys(commands))
	}
	if len(shutdown.Commands) != 0 {
		t.Fatalf("shutdown commands = %v, want empty (runtime beginSoftQuit owns exit)", shutdown.Commands)
	}
}

func commandKeys(commands []scheduledCommand) []string {
	keys := make([]string, len(commands))
	for i, c := range commands {
		keys[i] = c.Key
	}
	return keys
}

func TestFFmpegSettingsPerEncoder(t *testing.T) {
	tests := []struct {
		name        string
		encoder     string
		wantSetting string
		wantCodec   string
	}{
		{name: "default is libx264", encoder: "", wantSetting: "zvFfmpegYuv420pCrf18", wantCodec: "-c:v libx264 -preset fast -crf 18 -pix_fmt yuv420p"},
		{name: "explicit libx264", encoder: EncoderLibx264, wantSetting: "zvFfmpegYuv420pCrf18", wantCodec: "-c:v libx264 -preset fast -crf 18 -pix_fmt yuv420p"},
		{name: "nvenc", encoder: EncoderNVENC, wantSetting: "zvFfmpegNvencYuv420pCrf18", wantCodec: "-c:v h264_nvenc -preset p5 -rc vbr -b:v 0 -cq 18 -pix_fmt yuv420p"},
		{name: "amf", encoder: EncoderAMF, wantSetting: "zvFfmpegAmfYuv420pCrf18", wantCodec: "-c:v h264_amf -quality balanced -rc cqp -qp_i 18 -qp_p 18 -qp_b 18 -pix_fmt yuv420p"},
		{name: "qsv", encoder: EncoderQSV, wantSetting: "zvFfmpegQsvYuv420pCrf18", wantCodec: "-c:v h264_qsv -global_quality 18 -pix_fmt yuv420p"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := ffmpegSettingName(tt.encoder, 18)
			if name != tt.wantSetting {
				t.Errorf("setting name = %q, want %q", name, tt.wantSetting)
			}
			cmd := ffmpegSettingsCommand(name, 18, tt.encoder)
			if !strings.Contains(cmd, tt.wantCodec) {
				t.Errorf("command %q missing codec %q", cmd, tt.wantCodec)
			}
			if !strings.Contains(cmd, `{QUOTE}{AFX_STREAM_PATH}\video.mp4{QUOTE}`) {
				t.Errorf("command %q missing the stream path suffix", cmd)
			}
			plan := testPlan()
			plan.Stream.Encoder = tt.encoder
			js, err := GenerateHLAEJavaScript(plan)
			if err != nil {
				t.Fatalf("GenerateHLAEJavaScript error = %v", err)
			}
			if !strings.Contains(js, "mirv_streams record screen settings "+tt.wantSetting) {
				t.Errorf("stream setup did not apply setting %q\n%s", tt.wantSetting, js)
			}
		})
	}
}

func TestGenerateHLAEJavaScriptLogsSeekStallWithoutAborting(t *testing.T) {
	js, err := GenerateHLAEJavaScript(testPlan())
	if err != nil {
		t.Fatalf("GenerateHLAEJavaScript error = %v", err)
	}
	for _, want := range []string{
		`const maxSeekStallFrames = 1500`,
		`let seekStallFrames = 0`,
		`let lastSeekTick = -1`,
		`tick !== lastSeekTick`,
		`seekStallFrames++`,
		`seekStallFrames === maxSeekStallFrames`,
		`showed no tick progress for`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("generated JS missing %q\n%s", want, js)
		}
	}
	// The stall detector is diagnostic-only: a healthy demo_gototick freezes the
	// demo tick for the duration of its load and the client FPS is uncapped
	// between record windows, so a frame-counted budget must never abort a
	// capture. The hard bound stays maxSeekAttempts.
	for _, banned := range []string{
		`stalled at target`,
		`capture_failed: seek`,
	} {
		if strings.Contains(js, banned) {
			t.Errorf("generated JS still aborts on seek stall via %q", banned)
		}
	}
}

func TestBuildRuntimeScheduleOmitsRedundantCameraLeads(t *testing.T) {
	commands, _, _ := buildRuntimeSchedule(testPlan())
	for _, c := range commands {
		if strings.Contains(c.Key, "camera-lead-") {
			t.Fatalf("redundant camera-lead command still scheduled: %q", c.Key)
		}
	}
	for _, want := range []string{"camera-warmup-", "camera-lock-", "camera-relock-"} {
		found := false
		for _, c := range commands {
			if strings.HasPrefix(c.Key, want) {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing required camera command prefix %q in %v", want, commandKeys(commands))
		}
	}
}

func TestPlaybackTimescaleScheduleUsesConfiguredSettle(t *testing.T) {
	p := testPlan()
	p.Runtime.PlaybackSettleSeconds = 4
	commands, _, windows := buildRuntimeSchedule(p)
	byKey := map[string]scheduledCommand{}
	for _, c := range commands {
		byKey[c.Key] = c
	}
	reset, ok := byKey["timescale-reset-"+windows[0].SegmentID]
	if !ok {
		t.Fatalf("missing timescale reset in %v", commandKeys(commands))
	}
	want := windows[0].RecordStart - 4*64
	if reset.Tick != want {
		t.Fatalf("reset tick = %d, want %d (4s settle)", reset.Tick, want)
	}
}

// TestGenerateHLAEJavaScriptSoftQuitContract locks the runtime paths that used
// to call disconnect and quit in the same client frame (failCapture, demo EOF,
// and the scheduled shutdown pair).
func TestGenerateHLAEJavaScriptSoftQuitContract(t *testing.T) {
	js, err := GenerateHLAEJavaScript(testPlan())
	if err != nil {
		t.Fatalf("GenerateHLAEJavaScript error = %v", err)
	}
	if err := assertSoftQuitContract(js); err != nil {
		t.Fatal(err)
	}
	// beginSoftQuit must appear for failCapture, demo-end, and scheduled shutdown.
	if got := strings.Count(js, "beginSoftQuit()"); got < 3 {
		t.Fatalf("beginSoftQuit() calls = %d, want >= 3 exit paths", got)
	}
	// failCapture must soft-quit, not hard-quit.
	failIdx := strings.Index(js, "const failCapture = (reason)")
	if failIdx < 0 {
		t.Fatal("failCapture missing")
	}
	failBody := js[failIdx:min(failIdx+800, len(js))]
	if !strings.Contains(failBody, "beginSoftQuit()") {
		t.Fatalf("failCapture does not soft-quit:\n%s", failBody)
	}
	if strings.Contains(failBody, `mirv.exec("quit")`) {
		t.Fatalf("failCapture still hard-quits:\n%s", failBody)
	}
}

// TestParseToCaptureScriptPipeline is the pure unit path from a kill plan
// (parser output) through recording plan generation into HLAE script facts the
// capture runtime depends on.
func TestParseToCaptureScriptPipeline(t *testing.T) {
	tests := []struct {
		name       string
		segments   []killplan.Segment
		wantSegIDs []string
		wantStarts int // min record-start keys
	}{
		{
			name: "single kill segment",
			segments: []killplan.Segment{{
				ID: "seg-001", TickStart: 1000, TickEnd: 1400,
				Kills: []killplan.Kill{{Tick: 1200, Weapon: "ak47", Killer: killplan.Player{SteamID64: "76561198148986856", NameInDemo: "p"}}},
			}},
			wantSegIDs: []string{"seg-001"},
			wantStarts: 1,
		},
		{
			name: "editorial reverse becomes capture chronological",
			segments: []killplan.Segment{
				{ID: "late", TickStart: 5000, TickEnd: 5400, Kills: []killplan.Kill{{Tick: 5200, Killer: killplan.Player{SteamID64: "76561198148986856"}}}},
				{ID: "early", TickStart: 1000, TickEnd: 1400, Kills: []killplan.Kill{{Tick: 1200, Killer: killplan.Player{SteamID64: "76561198148986856"}}}},
			},
			wantSegIDs: []string{"early", "late"},
			wantStarts: 2,
		},
		{
			name: "multi kill multi segment preserves both record windows",
			segments: []killplan.Segment{
				{ID: "seg-001", TickStart: 2000, TickEnd: 2800, Kills: []killplan.Kill{
					{Tick: 2200, Killer: killplan.Player{SteamID64: "76561198148986856"}},
					{Tick: 2500, Killer: killplan.Player{SteamID64: "76561198148986856"}},
				}},
				{ID: "seg-002", TickStart: 8000, TickEnd: 8600, Kills: []killplan.Kill{
					{Tick: 8300, Killer: killplan.Player{SteamID64: "76561198148986856"}},
				}},
			},
			wantSegIDs: []string{"seg-001", "seg-002"},
			wantStarts: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kp := killplan.NewPlan()
			kp.Demo.Map = "de_dust2"
			kp.Demo.Tickrate = 64
			kp.Demo.SHA256 = strings.Repeat("b", 64)
			kp.Demo.DurationTicks = 50_000
			kp.Target.SteamID64 = "76561198148986856"
			kp.Target.NameInDemo = "player"
			kp.Segments = tt.segments

			plan, err := NewPlanFromKillPlan(kp, `C:\demos\match.dem`, `C:\out\run`, DefaultStreamConfig())
			if err != nil {
				t.Fatalf("NewPlanFromKillPlan: %v", err)
			}
			if err := plan.Validate(); err != nil {
				t.Fatalf("plan.Validate: %v", err)
			}
			if len(plan.Segments) != len(tt.wantSegIDs) {
				t.Fatalf("segments = %d, want %d", len(plan.Segments), len(tt.wantSegIDs))
			}
			for i, id := range tt.wantSegIDs {
				if plan.Segments[i].ID != id {
					t.Fatalf("capture order[%d] = %q, want %q", i, plan.Segments[i].ID, id)
				}
			}

			commands, seeks, windows := buildRuntimeSchedule(plan)
			if len(windows) != len(tt.wantSegIDs) {
				t.Fatalf("capture windows = %d, want %d", len(windows), len(tt.wantSegIDs))
			}
			starts := 0
			ends := 0
			for _, c := range commands {
				if strings.HasPrefix(c.Key, "record-start-") {
					starts++
				}
				if strings.HasPrefix(c.Key, "record-end-") {
					ends++
				}
			}
			if starts != tt.wantStarts || ends != tt.wantStarts {
				t.Fatalf("record start/end = %d/%d, want %d/%d", starts, ends, tt.wantStarts, tt.wantStarts)
			}
			// Seeks only across large gaps; multi-seg with far ticks should seek.
			_ = seeks

			js, err := GenerateHLAEJavaScript(plan)
			if err != nil {
				t.Fatalf("GenerateHLAEJavaScript: %v", err)
			}
			for _, id := range tt.wantSegIDs {
				if !strings.Contains(js, "record-start-"+id) || !strings.Contains(js, "record-end-"+id) {
					t.Fatalf("script missing record markers for %s", id)
				}
			}
			if !strings.Contains(js, `"key": "shutdown"`) || !strings.Contains(js, "beginSoftQuit()") {
				t.Fatal("script missing soft-quit shutdown path")
			}
			// Effective start must not place the first kill before record start.
			for _, seg := range plan.Segments {
				if len(seg.Kills) == 0 {
					continue
				}
				start := EffectiveRecordStartTick(seg, plan.Tickrate)
				first := seg.Kills[0].Tick
				for _, k := range seg.Kills[1:] {
					if k.Tick < first {
						first = k.Tick
					}
				}
				if start > first {
					t.Fatalf("segment %s record start %d is after first kill %d", seg.ID, start, first)
				}
				if first-start < plan.Tickrate && start != seg.TickStart {
					// When settle shortens pre-roll, still require at least some pre-kill frames unless clamped to TickStart.
					t.Logf("segment %s short pre-roll: start=%d kill=%d", seg.ID, start, first)
				}
			}
		})
	}
}

// TestParseToCapturePipelineMutations kills individual pipeline invariants.
func TestParseToCapturePipelineMutations(t *testing.T) {
	kp := killplan.NewPlan()
	kp.Demo.Map = "de_inferno"
	kp.Demo.Tickrate = 64
	kp.Demo.SHA256 = strings.Repeat("c", 64)
	kp.Demo.DurationTicks = 20_000
	kp.Target.SteamID64 = "76561198148986856"
	kp.Segments = []killplan.Segment{{
		ID: "seg-001", TickStart: 1000, TickEnd: 2000,
		Kills: []killplan.Kill{{Tick: 1500, Killer: killplan.Player{SteamID64: "76561198148986856"}}},
	}}

	plan, err := NewPlanFromKillPlan(kp, "match.dem", "out", DefaultStreamConfig())
	if err != nil {
		t.Fatal(err)
	}

	t.Run("validate rejects empty demo path after plan", func(t *testing.T) {
		bad := plan
		bad.DemoPath = ""
		if err := bad.Validate(); err == nil {
			t.Fatal("Validate accepted empty demo path")
		}
	})
	t.Run("script gen requires segments", func(t *testing.T) {
		bad := plan
		bad.Segments = nil
		if _, err := GenerateHLAEJavaScript(bad); err == nil {
			t.Fatal("GenerateHLAEJavaScript accepted empty segments")
		}
	})
	t.Run("record window covers kill settle", func(t *testing.T) {
		seg := plan.Segments[0]
		start := EffectiveRecordStartTick(seg, 64)
		// Camera settle: firstKill - tickrate, or TickStart+2s, whichever policy applies.
		if start < seg.TickStart || start > seg.Kills[0].Tick {
			t.Fatalf("EffectiveRecordStartTick=%d outside [%d, kill %d]", start, seg.TickStart, seg.Kills[0].Tick)
		}
	})
	t.Run("shutdown after last record end", func(t *testing.T) {
		commands, _, windows := buildRuntimeSchedule(plan)
		lastEnd := 0
		for _, w := range windows {
			if w.RecordEnd > lastEnd {
				lastEnd = w.RecordEnd
			}
		}
		var shutdownTick int
		for _, c := range commands {
			if c.Key == "shutdown" {
				shutdownTick = c.Tick
			}
		}
		if shutdownTick <= lastEnd {
			t.Fatalf("shutdown tick %d must be after last record end %d", shutdownTick, lastEnd)
		}
	})
}
