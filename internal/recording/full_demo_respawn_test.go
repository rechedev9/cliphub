package recording

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

// Independent boundaries from the reported Mirage demo (SHA256 eb885378...).
// Voice extends both rounds to round_start; the target dies in R3, then
// respawns exactly at R4's old requested start, 25341.
func respawnCaptureFixture(t *testing.T) RecordingPlan {
	t.Helper()
	death3, death4 := 22663, 29226
	f := recapplan.Facts{SchemaVersion: recapplan.DocumentVersion, DemoSHA256: strings.Repeat("a", 64), TargetSteamID64: "76561198386265483", ClockKind: recapplan.ClockIngame, TickRate: 64, EndTick: 33000, Complete: true,
		Rounds: []recapplan.RoundFacts{
			{ID: "round-003", Number: 3, StartTick: 19030, FreezeEndTick: 20310, RoundEndTick: 24893, NextStartTick: 25341, DeathTick: &death3, Evidence: "round-events"},
			{ID: "round-004", Number: 4, StartTick: 25341, FreezeEndTick: 26621, RoundEndTick: 32057, NextStartTick: 32505, DeathTick: &death4, Evidence: "round-events"},
		}}
	o := recapplan.DefaultOptions()
	o.Capture.Crosshair.AllowCaptureDefault = true
	o.Audio.Music.Enabled, o.Sponsor.Enabled = false, false
	voice := recapplan.VoiceEvidence{Availability: "available", IndexHash: strings.Repeat("b", 64), IndexRef: "voice/index.json", ClockKind: recapplan.ClockIngame, Activity: []recapplan.TickRange{{Start: 19030, End: 20310}, {Start: 25341, End: 26621}}}
	d, err := recapplan.Plan(f, o, voice, nil, "facts.json")
	if err != nil {
		t.Fatal(err)
	}
	kp := killplan.NewPlan()
	kp.Demo = killplan.Demo{SHA256: f.DemoSHA256, Tickrate: 64, DurationTicks: 33000}
	kp.Target.SteamID64 = f.TargetSteamID64
	p, err := NewPlanFromKillPlan(d.KillPlan(kp), "demo.dem", "out", DefaultStreamConfig(), &d)
	if err != nil {
		t.Fatal(err)
	}
	p.Runtime.PlaybackTimescale = 8
	return p
}

func TestFullDemoRespawnAcquisition(t *testing.T) {
	for _, tc := range []struct {
		name        string
		extra       map[string]any
		failed      bool
		disableGate bool
	}{
		{name: "delayed lock across death seek and respawn"},
		{name: "slow lock paused before exact approved start", extra: map[string]any{"pov_lock_delay_frames": 400}},
		{name: "same delay without acquisition gate fails", extra: map[string]any{"pov_lock_delay_frames": 400}, failed: true, disableGate: true},
		{name: "refused lock is bounded and restored", extra: map[string]any{"refuse_pov_lock": true}, failed: true},
		{name: "missing target is bounded", extra: map[string]any{"target_present": false}, failed: true},
		{name: "refused pause cannot shift capture", extra: map[string]any{"refuse_pause": true, "pov_lock_delay_frames": 400}, failed: true},
		{name: "wrong player during live play still fails immediately", extra: map[string]any{"observer_overrides": []map[string]any{{"from_tick": 27000, "to_tick": 27000, "observed_steamid": "76561199440218013"}}}, failed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := respawnCaptureFixture(t)
			script, err := GenerateHLAEJavaScriptWithAttestation(p, "respawn-test")
			if err != nil {
				t.Fatal(err)
			}
			if tc.disableGate {
				script = strings.Replace(script, "if (!prepareFullDemoPOV(captureWindow, tick)) return;", "", 1)
			}
			scenario := map[string]any{"schema_version": 1, "name": tc.name, "target_steamid": p.TargetSteamID64, "start_tick": 0, "max_frames": 16000, "frame_stage": "render-before", "simulate_pov_lock": true, "pov_lock_delay_frames": 96, "simulate_timescale": true, "dead_observed_steamid": "76561199440218013", "observer_overrides": []map[string]any{{"from_tick": 22780, "to_tick": 25340, "target_alive": false}}, "expect": map[string]any{"outcome": "verified", "soft_quit": true, "recorded_segments": []string{"round-003", "round-004"}}}
			for key, value := range tc.extra {
				scenario[key] = value
			}
			if tc.failed {
				scenario["expect"] = map[string]any{"outcome": "failed", "soft_quit": true}
			}
			dir := t.TempDir()
			js, sc, out := filepath.Join(dir, "recording.js"), filepath.Join(dir, "scenario.json"), filepath.Join(dir, "result.json")
			if err := os.WriteFile(js, []byte(script), 0600); err != nil {
				t.Fatal(err)
			}
			b, _ := json.Marshal(scenario)
			if err := os.WriteFile(sc, b, 0600); err != nil {
				t.Fatal(err)
			}
			output, err := exec.Command("node", filepath.Join("..", "..", "scripts", "capturelab", "run.mjs"), "--script", js, "--scenario", sc, "--out", out).CombinedOutput()
			if err != nil {
				t.Fatalf("simulation: %v\n%s", err, output)
			}
			b, err = os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			var result struct {
				Paused           bool
				Events           []struct{ Kind, Value string }
				RecordOperations []struct {
					ID        string
					StartTick int `json:"start_tick"`
				} `json:"record_operations"`
			}
			if err = json.Unmarshal(b, &result); err != nil {
				t.Fatal(err)
			}
			if result.Paused {
				t.Fatal("acquisition left the demo paused")
			}
			var log strings.Builder
			for _, e := range result.Events {
				if e.Kind == "message" {
					log.WriteString(e.Value)
					log.WriteByte('\n')
				}
			}
			evidence, err := ReadFullDemoCaptureEvidence(strings.NewReader(log.String()), "respawn-test", p)
			if tc.failed {
				if err == nil {
					t.Fatal("failed acquisition published reusable evidence")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(log.String(), "pov-acquired round-004") {
				t.Fatal("fixture did not exercise asynchronous reacquisition")
			}
			for i, op := range result.RecordOperations {
				if op.StartTick != p.FullDemo.Rounds[i].RequestedStartTick {
					t.Fatalf("capture shifted audio/video coverage: %+v", op)
				}
			}
			if evidence.CertifiedEnds["round-003"] != 22779 {
				t.Fatalf("tail must end at the last confirmed POV tick, before the unrecorded tick 22780: %+v", evidence.CertifiedEnds)
			}
			// The real 64 Hz Mirage capture contains 2,435 frames here.
			// Its first unconfirmed tick would promise an unavailable frame;
			// the corrected evidence fits without relaxing frame acceptance.
			recorded := RecordingResult{Plan: p, FullDemoEvidence: evidence, Artifacts: []RecordingArtifact{{SegmentID: "round-003", Role: "segment", Type: "video", Path: "round-003.mp4", SizeBytes: 1, FrameCount: 2435, FrameRate: "60/1"}}}
			start := p.Segments[0].TickStart
			end := evidence.CertifiedEnds["round-003"]
			if err := recorded.ValidateFullDemoRoundFrames("round-003", start, end); err != nil {
				t.Fatal(err)
			}
			if err := recorded.ValidateFullDemoRoundFrames("round-003", start, 22780); err == nil {
				t.Fatal("the unrecorded tick must still fail exact frame validation")
			}
			recorded.Artifacts[0].FrameCount--
			if err := recorded.ValidateFullDemoRoundFrames("round-003", start, end); err == nil {
				t.Fatal("a genuinely short clip must still fail with corrected POV evidence")
			}
		})
	}
}

func TestFullDemoLegacyPlanRemainsReadableButCannotBeRecorded(t *testing.T) {
	p := respawnCaptureFixture(t)
	p.FullDemo.PlannerVersion = recapplan.LegacyPlannerVersion
	p.FullDemo.PlanHash, _ = p.FullDemo.Hash()
	if err := p.Validate(); err != nil {
		t.Fatalf("legacy evidence unreadable: %v", err)
	}
	if _, err := GenerateHLAEJavaScript(p); err == nil || !strings.Contains(err.Error(), recapplan.ErrPlanStale) {
		t.Fatalf("legacy capture should require reapproval: %v", err)
	}
}
