package recording

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
)

// Fixed seed so chaos failures are reproducible in CI and locally.
const captureChaosSeed = 0x71c4c47_20260803

// TestCaptureFlowChaos hammers the shipped capture pipeline with seeded random
// plan shapes and destructive mutations. Valid draws must produce a coherent
// schedule + soft-quit script; invalid draws must reject without panicking.
func TestCaptureFlowChaos(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping capture chaos under -short")
	}
	const iterations = 400
	rng := rand.New(rand.NewSource(captureChaosSeed))
	target := killplan.Player{SteamID64: "76561198148986856", NameInDemo: "chaos"}

	var (
		validOK   int
		invalidOK int
	)
	for i := 0; i < iterations; i++ {
		i := i
		draw := chaosDrawPlan(rng, target)
		// ~35% of iterations apply a destructive mutation that should fail Validate.
		mutate := rng.Float64() < 0.35
		var mutName string
		if mutate {
			mutName = chaosMutatePlan(rng, &draw.plan)
		}

		name := fmt.Sprintf("i%03d/tr%d/segs%d/%s", i, draw.plan.Tickrate, len(draw.plan.Segments), draw.layout)
		if mutate {
			name += "/mut=" + mutName
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic under chaos draw seed=%d i=%d mut=%q: %v", captureChaosSeed, i, mutName, r)
				}
			}()

			if mutate {
				if err := draw.plan.Validate(); err == nil {
					// Some mutations may still be valid (e.g. no-op); only count as
					// success if Generate still works when Validate passes.
					if _, err := GenerateHLAEJavaScript(draw.plan); err != nil {
						t.Fatalf("Validate ok but script gen failed after mut %q: %v", mutName, err)
					}
					return
				}
				// Rejected plans must not be forced through script gen without error.
				if _, err := GenerateHLAEJavaScript(draw.plan); err == nil {
					// GenerateHLAEJavaScript calls Validate — must fail too.
					t.Fatalf("GenerateHLAEJavaScript accepted plan rejected by Validate after mut %q", mutName)
				}
				return
			}

			// Valid path: rebuild through NewPlanFromKillPlan when we still have a kill plan.
			plan := draw.plan
			if err := plan.Validate(); err != nil {
				t.Fatalf("valid draw failed Validate: %v", err)
			}
			commands, seeks, windows := buildRuntimeSchedule(plan)
			if len(windows) != len(plan.Segments) {
				t.Fatalf("windows=%d segments=%d", len(windows), len(plan.Segments))
			}
			for i := 1; i < len(plan.Segments); i++ {
				if plan.Segments[i].TickStart < plan.Segments[i-1].TickStart {
					t.Fatalf("capture order not chronological")
				}
			}
			for _, seg := range plan.Segments {
				var startTick, endTick int
				var hasStart, hasEnd bool
				for _, c := range commands {
					if c.Key == "record-start-"+seg.ID {
						hasStart, startTick = true, c.Tick
					}
					if c.Key == "record-end-"+seg.ID {
						hasEnd, endTick = true, c.Tick
					}
				}
				if !hasStart || !hasEnd || endTick <= startTick {
					t.Fatalf("segment %s record markers start=%v end=%v (%d..%d)", seg.ID, hasStart, hasEnd, startTick, endTick)
				}
			}
			for i := 1; i < len(seeks); i++ {
				if seeks[i].Target <= seeks[i-1].Target {
					t.Fatalf("seeks not increasing: %+v", seeks)
				}
			}
			js, err := GenerateHLAEJavaScript(plan)
			if err != nil {
				t.Fatalf("GenerateHLAEJavaScript: %v", err)
			}
			if err := assertSoftQuitContract(js); err != nil {
				t.Fatal(err)
			}
			// POV contract always present for real captures.
			if !strings.Contains(js, "targetSteamId") || !strings.Contains(js, "failCapture") {
				t.Fatal("script missing POV failCapture contract")
			}
		})
		if mutate {
			invalidOK++
		} else {
			validOK++
		}
	}
	if validOK < 100 || invalidOK < 50 {
		t.Fatalf("chaos mix too thin: valid=%d invalid=%d (seed=%d)", validOK, invalidOK, captureChaosSeed)
	}
	t.Logf("chaos completed seed=%d valid=%d mutated=%d", captureChaosSeed, validOK, invalidOK)
}

// TestCaptureResultChaos mutates recording results and asserts ValidateRunResult /
// ValidateUploadResult never panic and reject broken successes.
func TestCaptureResultChaos(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping result chaos under -short")
	}
	rng := rand.New(rand.NewSource(captureChaosSeed ^ 0x5f3759df))
	base := captureFlowFixture(t, 64, 50_000, []killplan.Segment{
		{ID: "seg-001", TickStart: 1000, TickEnd: 2000, Kills: []killplan.Kill{{Tick: 1500, Killer: killplan.Player{SteamID64: "76561198148986856"}}}},
		{ID: "seg-002", TickStart: 10_000, TickEnd: 11_000, Kills: []killplan.Kill{{Tick: 10_500, Killer: killplan.Player{SteamID64: "76561198148986856"}}}},
	}, DefaultStreamConfig())
	fp, err := CaptureInputFingerprint(base)
	if err != nil {
		t.Fatal(err)
	}
	good := RecordingResult{
		Plan:                    base,
		CaptureMode:             CaptureModeReal,
		CaptureVerified:         true,
		CaptureInputFingerprint: fp,
		Artifacts: []RecordingArtifact{
			{SegmentID: "seg-001", Type: "video", Role: "segment", Path: "seg-001.mp4", SizeBytes: 10},
			{SegmentID: "seg-002", Type: "video", Role: "segment", Path: "seg-002.mp4", SizeBytes: 10},
		},
	}
	if err := ValidateRunResult(good); err != nil {
		t.Fatalf("baseline good result: %v", err)
	}
	if err := ValidateUploadResult(good); err != nil {
		t.Fatalf("baseline upload: %v", err)
	}

	const n = 200
	rejected := 0
	for i := 0; i < n; i++ {
		i := i
		result := good
		// Deep-ish copy of mutable slices.
		result.Artifacts = append([]RecordingArtifact(nil), good.Artifacts...)
		result.Plan.Segments = append([]RecordingSegment(nil), good.Plan.Segments...)
		mut := chaosMutateResult(rng, &result)
		t.Run(fmt.Sprintf("res%03d/%s", i, mut), func(t *testing.T) {
			t.Parallel()
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic on result mut %q: %v", mut, r)
				}
			}()
			runErr := ValidateRunResult(result)
			upErr := ValidateUploadResult(result)
			// If we only corrupted upload artifacts but kept run metadata intact,
			// run may pass and upload fail — both are fine. Never accept both when
			// a clearly fatal mutation was applied.
			switch mut {
			case "clear_verified", "bad_fingerprint", "error_string", "pending_pub", "bad_mode", "empty_demo":
				if runErr == nil {
					t.Fatalf("ValidateRunResult accepted fatal mut %q", mut)
				}
			case "drop_artifacts", "empty_artifact_path", "wrong_segment_id":
				if upErr == nil {
					t.Fatalf("ValidateUploadResult accepted fatal mut %q", mut)
				}
			}
			if runErr != nil || upErr != nil {
				// counted outside via channel would race; just log
			}
		})
		// Count rejections for mix health (best-effort, sequential).
		result2 := good
		result2.Artifacts = append([]RecordingArtifact(nil), good.Artifacts...)
		result2.Plan.Segments = append([]RecordingSegment(nil), good.Plan.Segments...)
		// re-mutate with same sequence is hard; approximate with Validate of mutated
		_ = mut
		if ValidateRunResult(result) != nil || ValidateUploadResult(result) != nil {
			rejected++
		}
	}
	// Parallel subtests may complete after this check; re-run a serial rejection sample.
	rejected = 0
	rng2 := rand.New(rand.NewSource(captureChaosSeed ^ 0x5f3759df))
	for i := 0; i < n; i++ {
		result := good
		result.Artifacts = append([]RecordingArtifact(nil), good.Artifacts...)
		result.Plan.Segments = append([]RecordingSegment(nil), good.Plan.Segments...)
		chaosMutateResult(rng2, &result)
		if ValidateRunResult(result) != nil || ValidateUploadResult(result) != nil {
			rejected++
		}
	}
	if rejected < n/3 {
		t.Fatalf("result chaos too often accepted mutations: rejected=%d/%d", rejected, n)
	}
}

type chaosPlanDraw struct {
	layout string
	plan   RecordingPlan
}

func chaosDrawPlan(rng *rand.Rand, target killplan.Player) chaosPlanDraw {
	tickrates := []int{64, 128}
	tr := tickrates[rng.Intn(len(tickrates))]
	streams := []StreamConfig{
		DefaultStreamConfig(),
		{Mode: StreamModeFFmpegDirect, HUDMode: HUDModeClean, FPS: 60, Width: 1920, Height: 1080, CRF: 18},
		{Mode: StreamModeFFmpegDirect, HUDMode: HUDModeDeathnotices, PortraitSafeKillfeed: true, FPS: 60, Width: 1920, Height: 1080, CRF: 18},
		{Mode: StreamModeFFmpegDirect, HUDMode: HUDModeGameplay, FPS: 60, Width: 1280, Height: 720, CRF: 20},
	}
	stream := streams[rng.Intn(len(streams))]

	// Build 1–5 non-overlapping windows with random gaps.
	nSeg := 1 + rng.Intn(5)
	duration := tr * (500 + rng.Intn(2000))
	if duration < 20_000 {
		duration = 20_000
	}
	cursor := tr * (5 + rng.Intn(50))
	segs := make([]killplan.Segment, 0, nSeg)
	layout := fmt.Sprintf("n%d", nSeg)
	for i := 0; i < nSeg; i++ {
		span := tr * (6 + rng.Intn(20))
		start := cursor
		end := start + span
		if end >= duration-tr {
			duration = end + tr*50
		}
		id := fmt.Sprintf("s%02d", i)
		seg := killplan.Segment{ID: id, TickStart: start, TickEnd: end}
		kind := rng.Intn(4)
		switch kind {
		case 0: // single kill
			seg.Kills = []killplan.Kill{{Tick: start + span/2, Weapon: "ak47", Killer: target}}
			layout += "-k"
		case 1: // multi kill
			k1 := start + span/4
			k2 := start + span/2
			k3 := start + (3*span)/4
			seg.Kills = []killplan.Kill{
				{Tick: k1, Killer: target},
				{Tick: k2, Killer: target},
				{Tick: k3, Killer: target},
			}
			layout += "-mk"
		case 2: // utility only
			th := start + span/3
			seg.Utility = []killplan.UtilityThrow{{
				Type: "smokegrenade", ThrowTick: th, PopTick: th + tr, Thrower: target,
			}}
			layout += "-u"
		default: // kill + utility
			k := start + span/3
			th := start + (2*span)/3
			seg.Kills = []killplan.Kill{{Tick: k, Killer: target}}
			seg.Utility = []killplan.UtilityThrow{{Type: "flashbang", ThrowTick: th, Thrower: target}}
			layout += "-ku"
		}
		segs = append(segs, seg)
		// Gap: either nearby or far (seek-worthy).
		if rng.Float64() < 0.45 {
			cursor = end + tr*(2+rng.Intn(10)) // nearby
		} else {
			cursor = end + tr*(40+rng.Intn(200)) // far
		}
	}

	// Random editorial reverse of kill-plan order (capture will re-sort).
	if nSeg > 1 && rng.Float64() < 0.5 {
		for i, j := 0, len(segs)-1; i < j; i, j = i+1, j-1 {
			segs[i], segs[j] = segs[j], segs[i]
		}
		layout += "-rev"
	}

	kp := killplan.NewPlan()
	kp.Demo.Map = "de_chaos"
	kp.Demo.Tickrate = tr
	kp.Demo.SHA256 = strings.Repeat("c", 64)
	kp.Demo.DurationTicks = duration
	kp.Target.SteamID64 = target.SteamID64
	kp.Target.NameInDemo = target.NameInDemo
	kp.Segments = segs
	plan, err := NewPlanFromKillPlan(kp, `C:\demos\chaos.dem`, `C:\out\chaos`, stream)
	if err != nil {
		// Should not happen for constructed valid plans; surface as empty plan for mutate path.
		return chaosPlanDraw{layout: layout + "-buildfail", plan: RecordingPlan{}}
	}
	return chaosPlanDraw{layout: layout, plan: plan}
}

// chaosMutatePlan applies one destructive edit. Returns mutation name.
func chaosMutatePlan(rng *rand.Rand, plan *RecordingPlan) string {
	if len(plan.Segments) == 0 {
		plan.DemoPath = ""
		return "empty_demo_on_empty_segs"
	}
	mutations := []string{
		"empty_demo",
		"empty_segments",
		"bad_hud",
		"portrait_clean",
		"overlap",
		"kill_oob",
		"utility_oob",
		"dup_id",
		"tick_end_le_start",
		"neg_tick",
		"duration_too_small",
		"bad_crf",
		"bad_editorial",
	}
	name := mutations[rng.Intn(len(mutations))]
	switch name {
	case "empty_demo":
		plan.DemoPath = ""
	case "empty_segments":
		plan.Segments = nil
		plan.EditorialSegmentIDs = nil
	case "bad_hud":
		plan.Stream.HUDMode = HUDMode("not-a-hud")
	case "portrait_clean":
		plan.Stream.HUDMode = HUDModeClean
		plan.Stream.PortraitSafeKillfeed = true
	case "overlap":
		if len(plan.Segments) >= 2 {
			plan.Segments[1].TickStart = plan.Segments[0].TickStart
			plan.Segments[1].TickEnd = plan.Segments[0].TickEnd + 10
			if len(plan.Segments[1].Kills) > 0 {
				plan.Segments[1].Kills[0].Tick = plan.Segments[1].TickStart + 1
			}
		} else {
			plan.DemoPath = ""
			return "overlap_fallback_empty_demo"
		}
	case "kill_oob":
		s := &plan.Segments[rng.Intn(len(plan.Segments))]
		s.Kills = append(s.Kills, killplan.Kill{Tick: s.TickEnd + 100})
	case "utility_oob":
		s := &plan.Segments[rng.Intn(len(plan.Segments))]
		s.Utility = append(s.Utility, killplan.UtilityThrow{Type: "smokegrenade", ThrowTick: s.TickEnd + 5})
	case "dup_id":
		if len(plan.Segments) >= 2 {
			plan.Segments[1].ID = plan.Segments[0].ID
		} else {
			plan.Segments = append(plan.Segments, plan.Segments[0])
		}
	case "tick_end_le_start":
		s := &plan.Segments[rng.Intn(len(plan.Segments))]
		s.TickEnd = s.TickStart
	case "neg_tick":
		s := &plan.Segments[rng.Intn(len(plan.Segments))]
		s.TickStart = -1
	case "duration_too_small":
		plan.DemoDurationTicks = plan.Segments[len(plan.Segments)-1].TickEnd - 1
	case "bad_crf":
		plan.Stream.CRF = 99
	case "bad_editorial":
		plan.EditorialSegmentIDs = []string{"nope"}
	}
	return name
}

func chaosMutateResult(rng *rand.Rand, result *RecordingResult) string {
	mutations := []string{
		"clear_verified",
		"bad_fingerprint",
		"error_string",
		"pending_pub",
		"bad_mode",
		"empty_demo",
		"drop_artifacts",
		"empty_artifact_path",
		"wrong_segment_id",
		"noop",
	}
	name := mutations[rng.Intn(len(mutations))]
	switch name {
	case "clear_verified":
		result.CaptureVerified = false
	case "bad_fingerprint":
		result.CaptureInputFingerprint = "deadbeef"
	case "error_string":
		result.Error = "cs2 crashed"
	case "pending_pub":
		result.PublicationPending = true
	case "bad_mode":
		result.CaptureMode = CaptureMode("dry-run")
	case "empty_demo":
		result.Plan.DemoPath = ""
	case "drop_artifacts":
		result.Artifacts = nil
	case "empty_artifact_path":
		if len(result.Artifacts) > 0 {
			result.Artifacts[0].Path = ""
		}
	case "wrong_segment_id":
		if len(result.Artifacts) > 0 {
			result.Artifacts[0].SegmentID = "ghost"
		}
	case "noop":
		// leave intact
	}
	return name
}
