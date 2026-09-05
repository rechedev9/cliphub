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

func fullDemoCaptureFixture(t *testing.T) RecordingPlan {
	t.Helper()
	death := 700
	f := recapplan.Facts{SchemaVersion: recapplan.DocumentVersion, DemoSHA256: strings.Repeat("a", 64), TargetSteamID64: "76561198377256168", ClockKind: recapplan.ClockIngame, TickRate: 64, EndTick: 2000, Complete: true,
		Rounds: []recapplan.RoundFacts{{ID: "round-001", Number: 1, StartTick: 200, FreezeEndTick: 400, RoundEndTick: 800, DeathTick: &death, Evidence: "round-events", Kills: []killplan.Kill{}, Utility: []killplan.UtilityThrow{}}}}
	o := recapplan.DefaultOptions()
	o.Audio.Voice.Enabled, o.Audio.Music.Enabled, o.Sponsor.Enabled, o.Editorial.KeepFreezeVoice = false, false, false, false
	o.Capture.Crosshair.AllowCaptureDefault = true
	d, err := recapplan.Plan(f, o, recapplan.VoiceEvidence{Availability: "not_requested"}, nil, "facts.json")
	if err != nil {
		t.Fatal(err)
	}
	kp := killplan.NewPlan()
	kp.Demo = killplan.Demo{SHA256: f.DemoSHA256, Tickrate: 64, DurationTicks: 2000}
	kp.Target.SteamID64 = f.TargetSteamID64
	kp = d.KillPlan(kp)
	p, err := NewPlanFromKillPlan(kp, "demo.dem", "out", DefaultStreamConfig(), &d)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFullDemoExactRuntimeInExistingMIRVSimulator(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("Node is required for Full Demo simulator verification")
	}
	const token = "full-demo-test-nonce"
	for _, tc := range []struct {
		name, outcome string
		override      map[string]any
		missing       []string
		refuse        []string
		trim          bool
		wantEnd       int
		providedCode  bool
	}{
		{name: "complete", outcome: "verified", trim: true, wantEnd: 892},
		{name: "unknown single live frame", outcome: "failed", trim: true, override: map[string]any{"from_tick": 500, "to_tick": 500, "observed_steamid": nil}},
		{name: "wrong player single frame", outcome: "failed", trim: true, override: map[string]any{"from_tick": 500, "to_tick": 500, "observed_steamid": "76561198000000001"}},
		{name: "third person", outcome: "failed", trim: true, override: map[string]any{"from_tick": 500, "to_tick": 500, "observer_mode": 3}},
		{name: "roaming", outcome: "failed", trim: true, override: map[string]any{"from_tick": 500, "to_tick": 500, "observer_mode": 4}},
		{name: "approved death tail trim", outcome: "verified", trim: true, wantEnd: 704, override: map[string]any{"from_tick": 704, "to_tick": 800, "observed_steamid": nil}},
		{name: "unapproved death tail trim", outcome: "failed", trim: false, override: map[string]any{"from_tick": 704, "to_tick": 800, "observed_steamid": nil}},
		{name: "missing voice mute cvar", outcome: "failed", trim: true, missing: []string{"voice_modenable"}},
		{name: "voice mute readback mismatch", outcome: "failed", trim: true, refuse: []string{"snd_voipvolume"}},
		{name: "provided crosshair values", outcome: "verified", trim: true, wantEnd: 892, providedCode: true},
		{name: "crosshair readback mismatch", outcome: "failed", trim: true, refuse: []string{"cl_crosshairgap"}, providedCode: true},
		{name: "missing crosshair cvar", outcome: "failed", trim: true, missing: []string{"cl_fixedcrosshairgap"}, providedCode: true},
		{name: "missing clean HUD cvar", outcome: "failed", trim: true, missing: []string{"hud_showtargetid"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := fullDemoCaptureFixture(t)
			if tc.providedCode {
				p.FullDemo.Options.Capture.Crosshair = recapplan.CrosshairOptions{Mode: "provided-code", Code: "CSGO-WsnnD-eHaMw-QNDf9-oxuDh-ydOUD"}
				p.Stream.FullDemoCapture = p.FullDemo.Options.Capture
			}
			p.FullDemo.Options.Editorial.AllowSafeTailTrim = tc.trim
			p.FullDemo.PlanHash, _ = p.FullDemo.Hash()
			script, err := GenerateHLAEJavaScriptWithAttestation(p, token)
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			scriptPath, scenarioPath, outputPath := filepath.Join(dir, "recording.js"), filepath.Join(dir, "scenario.json"), filepath.Join(dir, "result.json")
			if err := os.WriteFile(scriptPath, []byte(script), 0600); err != nil {
				t.Fatal(err)
			}
			scenario := map[string]any{"schema_version": 1, "name": tc.name, "target_steamid": p.TargetSteamID64, "start_tick": 0, "tick_step": 1, "max_frames": 1400, "frame_stage": "render-before", "missing_cvars": tc.missing, "refuse_cvar_writes": tc.refuse, "expect": map[string]any{"outcome": tc.outcome, "soft_quit": true}}
			if tc.override != nil {
				scenario["observer_overrides"] = []any{tc.override}
			}
			b, _ := json.Marshal(scenario)
			if err := os.WriteFile(scenarioPath, b, 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(node, filepath.Join("..", "..", "scripts", "capturelab", "run.mjs"), "--script", scriptPath, "--scenario", scenarioPath, "--out", outputPath)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("simulator: %v\n%s", err, output)
			}
			b, err = os.ReadFile(outputPath)
			if err != nil {
				t.Fatal(err)
			}
			var result struct {
				OK         bool `json:"ok"`
				Events     []struct{ Kind, Value string }
				FinalCvars map[string]any `json:"final_cvars"`
			}
			if err := json.Unmarshal(b, &result); err != nil {
				t.Fatal(err)
			}
			if !result.OK {
				t.Fatalf("simulator rejected exact script: %s", b)
			}
			var log strings.Builder
			for _, event := range result.Events {
				if event.Kind == "message" {
					log.WriteString(event.Value)
					log.WriteByte('\n')
				}
			}
			evidence, err := ReadFullDemoCaptureEvidence(strings.NewReader(log.String()), token, p)
			if tc.outcome == "verified" {
				if err != nil {
					t.Fatal(err)
				}
				if evidence.CertifiedEnds["round-001"] != tc.wantEnd {
					t.Fatalf("certified end=%v", evidence.CertifiedEnds)
				}
				if EffectiveRecordStartTick(p.Segments[0], 64) != 200 {
					t.Fatal("camera warmup changed editorial start")
				}
				if result.FinalCvars["snd_voipvolume"] != 0.63 || result.FinalCvars["tv_listen_voice_indices"] != float64(7) || result.FinalCvars["cl_show_observer_crosshair"] != float64(1) {
					t.Fatal("restoration guessed defaults instead of restoring actual values")
				}
			} else if err == nil {
				t.Fatal("failed capture produced reusable evidence")
			}
		})
	}
}
