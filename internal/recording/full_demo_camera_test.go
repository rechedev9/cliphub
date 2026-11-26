package recording

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

const (
	cameraTarget = "76561198377256168"
	cameraEnemyA = "76561198000000101"
	cameraEnemyB = "76561198000000102"
)

func cameraKill(tick int, enemy string) killplan.Kill {
	return killplan.Kill{Tick: tick, Weapon: "ak47",
		Killer: killplan.Player{SteamID64: cameraTarget, NameInDemo: "pov", TeamAtKill: "CT"},
		Victim: killplan.Player{SteamID64: enemy, NameInDemo: "enemy", TeamAtKill: "T"}}
}

func secondCameraPlan(t *testing.T, enabled bool) RecordingPlan {
	t.Helper()
	f := recapplan.Facts{SchemaVersion: recapplan.DocumentVersion, DemoSHA256: strings.Repeat("a", 64), TargetSteamID64: cameraTarget, ClockKind: recapplan.ClockIngame, TickRate: 64, EndTick: 3200, Complete: true,
		Holds: &recapplan.HoldEvidence{Version: recapplan.HoldEvidenceVersion, Kills: []recapplan.KillHold{
			{Tick: 900, VictimSteamID64: cameraEnemyA, MoveUnits: 8, Covered: true},
			{Tick: 2100, VictimSteamID64: cameraEnemyB, MoveUnits: 12, Covered: true},
		}},
		Rounds: []recapplan.RoundFacts{
			{ID: "round-001", Number: 1, StartTick: 100, FreezeEndTick: 400, RoundEndTick: 1100, NextStartTick: 1300, Evidence: "round-events", Kills: []killplan.Kill{cameraKill(900, cameraEnemyA)}, Utility: []killplan.UtilityThrow{}},
			{ID: "round-002", Number: 2, StartTick: 1300, FreezeEndTick: 1600, RoundEndTick: 2400, Evidence: "round-events", Kills: []killplan.Kill{cameraKill(2100, cameraEnemyB)}, Utility: []killplan.UtilityThrow{}},
		}}
	o := recapplan.DefaultOptions()
	o.Audio.Voice.Enabled, o.Audio.Music.Enabled, o.Editorial.KeepFreezeVoice = false, false, false
	o.Capture.Crosshair.AllowCaptureDefault = true
	if enabled {
		o.SecondCamera = &recapplan.SecondCameraOptions{Enabled: true, MaxWindows: 5}
	}
	d, err := recapplan.Plan(f, o, recapplan.VoiceEvidence{Availability: "not_requested"}, nil, "facts.json")
	if err != nil {
		t.Fatal(err)
	}
	if enabled && (d.SecondCamera == nil || len(d.SecondCamera.Windows) != 2) {
		t.Fatalf("fixture plans windows %+v, want one per round", d.SecondCamera)
	}
	kp := killplan.NewPlan()
	kp.Demo = killplan.Demo{SHA256: f.DemoSHA256, Tickrate: 64, DurationTicks: 3200}
	kp.Target.SteamID64 = f.TargetSteamID64
	p, err := NewPlanFromKillPlan(d.KillPlan(kp), "demo.dem", "out", DefaultStreamConfig(), &d)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func captureIDs(p RecordingPlan) []string {
	var ids []string
	for _, s := range p.CaptureSegments() {
		ids = append(ids, s.ID)
	}
	return ids
}

func TestFullDemoSecondCameraRecordsEachWindowBeforeTheRounds(t *testing.T) {
	off := secondCameraPlan(t, false)
	if got := captureIDs(off); !slices.Equal(got, []string{"round-001", "round-002"}) {
		t.Fatalf("plan without the option captures %v", got)
	}
	p := secondCameraPlan(t, true)
	windows := p.FullDemo.SecondCamera.Windows
	want := []string{windows[0].ID, windows[1].ID, "round-001", "round-002"}
	if got := captureIDs(p); !slices.Equal(got, want) {
		t.Fatalf("capture order = %v, want %v", got, want)
	}
	for i, s := range p.Segments {
		if s.SecondCamera == nil || *s.SecondCamera != windows[i] {
			t.Fatalf("%s carries window %+v, want %+v", s.ID, s.SecondCamera, windows[i])
		}
	}
	camera := p.CaptureSegments()[0]
	if camera.POVSteamID64 != cameraEnemyA || camera.TickStart != windows[0].StartTick || camera.TickEnd != windows[0].EndTick || len(camera.Kills) != 1 {
		t.Fatalf("camera capture = %+v, want the enemy's window around its death", camera)
	}
	_, seeks, captureWindows := buildRuntimeSchedule(p)
	rewinds := slices.IndexFunc(seeks, func(s seekStep) bool { return s.Rewind })
	if rewinds < 0 {
		t.Fatal("the rounds need a rewind after the camera pass")
	}
	for _, w := range captureWindows {
		camera := strings.Contains(w.SegmentID, "-camera-")
		if w.Camera != camera || (w.Pass == 0) != camera {
			t.Fatalf("window %+v: cameras run in pass 0 marked as camera, rounds in pass 1, so the match never ends before a rewind", w)
		}
		if camera && w.VerifyUntil >= windows[0].KillTick && w.SegmentID == windows[0].ID {
			t.Fatalf("camera verification must stop before the enemy dies: %+v", w)
		}
	}
	script, err := GenerateHLAEJavaScript(p)
	if err != nil || !strings.Contains(script, `"camera": true`) {
		t.Fatalf("script lacks the camera windows: %v", err)
	}

	if err := p.MarkReplayOnly([]string{"round-001", "round-002"}); err != nil {
		t.Fatalf("camera-only capture refused: %v", err)
	}
	if got := captureIDs(p); !slices.Equal(got, want[:2]) {
		t.Fatalf("camera-only capture order = %v", got)
	}
	_, seeks, captureWindows = buildRuntimeSchedule(p)
	if slices.ContainsFunc(seeks, func(s seekStep) bool { return s.Rewind }) || slices.ContainsFunc(captureWindows, func(w captureWindow) bool { return w.Pass != 0 }) {
		t.Fatal("a camera-only capture must run in one pass without a rewind")
	}
}

func TestFullDemoSecondCameraSeeksForwardToARoundAfterTheLastWindow(t *testing.T) {
	p := secondCameraPlan(t, true)
	if err := p.MarkReplayOnly([]string{"round-001"}); err != nil {
		t.Fatal(err)
	}
	p.Segments[1].SecondCamera = nil
	camera := p.Segments[0].SecondCamera.ID
	if got := captureIDs(p); !slices.Equal(got, []string{camera, "round-002"}) {
		t.Fatalf("capture order = %v", got)
	}
	_, seeks, windows := buildRuntimeSchedule(p)
	if slices.ContainsFunc(seeks, func(s seekStep) bool { return s.Rewind }) || slices.ContainsFunc(windows, func(w captureWindow) bool { return w.Pass != 0 }) {
		t.Fatalf("a round that starts after the last window is reached in the same pass, never by a rewind that points forward: %+v", seeks)
	}
}

func TestFullDemoSecondCameraValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*RecordingPlan)
	}{
		{"observe stays refused", func(p *RecordingPlan) { p.Segments[0].Observe = &killplan.Target{SteamID64: cameraEnemyA} }},
		{"pov override stays refused", func(p *RecordingPlan) { p.Segments[0].POVSteamID64 = cameraEnemyA }},
		{"window not in the documents", func(p *RecordingPlan) {
			w := *p.Segments[0].SecondCamera
			w.EndTick--
			p.Segments[0].SecondCamera = &w
		}},
		{"enemy is the target", func(p *RecordingPlan) {
			w := *p.Segments[0].SecondCamera
			w.EnemySteamID64 = cameraTarget
			p.Segments[0].SecondCamera = &w
		}},
		{"window of another round", func(p *RecordingPlan) { p.Segments[0].SecondCamera = p.Segments[1].SecondCamera }},
		{"replay only without a camera", func(p *RecordingPlan) {
			p.Segments[0].SecondCamera = nil
			p.Segments[0].ReplayOnly = true
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := secondCameraPlan(t, true)
			tc.mutate(&p)
			if err := p.Validate(); err == nil {
				t.Fatal("plan accepted")
			}
		})
	}
	p := secondCameraPlan(t, true)
	short := RecordingPlan{CaptureContract: CaptureContractVersion, KillPlanSchemaVersion: killplan.SchemaVersion, DemoPath: "x.dem", DemoSHA256: p.DemoSHA256, DemoDurationTicks: p.DemoDurationTicks, OutputDir: "out",
		TargetSteamID64: p.TargetSteamID64, TargetAccountID: p.TargetAccountID, Tickrate: 64, Stream: DefaultStreamConfig(), Segments: []RecordingSegment{p.Segments[0]}}
	short.Segments[0].ExactWindow = false
	if err := short.Validate(); err == nil || !strings.Contains(err.Error(), "Full Demo") {
		t.Fatalf("a Short plan accepted a second camera: %v", err)
	}
}

func TestFullDemoEvidenceAcceptsAnUnverifiedCameraEnd(t *testing.T) {
	p := secondCameraPlan(t, true)
	evidence := expectedCaptureEvidence(t, p)
	evidence.CertifiedEnds = map[string]int{}
	for _, s := range p.Segments {
		evidence.CertifiedEnds[s.ID] = s.TickEnd
	}
	cameras := p.CaptureSegments()[:2]
	evidence.CertifiedEnds[cameras[0].ID] = cameras[0].TickStart
	evidence.CertifiedEnds[cameras[1].ID] = cameras[1].TickEnd
	if err := evidence.Validate(p); err != nil {
		t.Fatalf("an unverified camera failed the capture: %v", err)
	}
	evidence.CertifiedEnds[cameras[0].ID] = cameras[0].TickEnd + 1
	if err := evidence.Validate(p); err == nil {
		t.Fatal("a camera end outside its window was accepted")
	}
	delete(evidence.CertifiedEnds, cameras[0].ID)
	if err := evidence.Validate(p); err == nil {
		t.Fatal("a camera without a certified end was accepted")
	}
}

func TestFullDemoSecondCameraRuntimeInMIRVSimulator(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("Node is required for Full Demo simulator verification")
	}
	const token = "camera-test-nonce"
	for _, tc := range []struct {
		name    string
		players []string
		camera  string
		forward bool
	}{
		{name: "enemy POV acquired", players: []string{cameraEnemyA, cameraEnemyB}, camera: "complete"},
		{name: "enemy POV never settles", camera: "unverified"},
		{name: "round after the last window", players: []string{cameraEnemyA, cameraEnemyB}, camera: "complete", forward: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := secondCameraPlan(t, true)
			if tc.forward {
				if err := p.MarkReplayOnly([]string{"round-001"}); err != nil {
					t.Fatal(err)
				}
				p.Segments[1].SecondCamera = nil
			}
			script, err := GenerateHLAEJavaScriptWithAttestation(p, token)
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			scriptPath, scenarioPath, outputPath := filepath.Join(dir, "recording.js"), filepath.Join(dir, "scenario.json"), filepath.Join(dir, "result.json")
			if err := os.WriteFile(scriptPath, []byte(script), 0600); err != nil {
				t.Fatal(err)
			}
			scenario := map[string]any{"schema_version": 1, "name": tc.name, "target_steamid": p.TargetSteamID64, "start_tick": 0, "max_frames": 12000, "frame_stage": "render-before", "expect": map[string]any{"outcome": "verified", "soft_quit": true}}
			if tc.players != nil {
				scenario["players"] = tc.players
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
				Operations []struct {
					ID string `json:"id"`
				} `json:"record_operations"`
			}
			if err := json.Unmarshal(b, &result); err != nil {
				t.Fatal(err)
			}
			if !result.OK {
				t.Fatalf("simulator rejected the camera script: %s", b)
			}
			var recorded []string
			for _, op := range result.Operations {
				recorded = append(recorded, op.ID)
			}
			if want := captureIDs(p); !slices.Equal(recorded, want) {
				t.Fatalf("recorded %v, want %v", recorded, want)
			}
			var log strings.Builder
			for _, event := range result.Events {
				if event.Kind == "message" {
					log.WriteString(event.Value + "\n")
				}
			}
			evidence, err := ReadFullDemoCaptureEvidence(strings.NewReader(log.String()), token, p)
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range p.CaptureSegments() {
				end := evidence.CertifiedEnds[s.ID]
				switch {
				case !isCameraCapture(s) && end != s.TickEnd:
					t.Fatalf("round %s certified %d, want %d", s.ID, end, s.TickEnd)
				case isCameraCapture(s) && tc.camera == "complete" && end != s.TickEnd:
					t.Fatalf("camera %s certified %d, want its whole window", s.ID, end)
				case isCameraCapture(s) && tc.camera == "unverified" && end >= s.TickEnd:
					t.Fatalf("an unverified camera %s certified its whole window", s.ID)
				}
			}
			segments := p.CaptureSegments()
			firstRound := slices.IndexFunc(segments, func(s RecordingSegment) bool { return !isCameraCapture(s) })
			cleanHUD := strings.Index(log.String(), "[zackvideo] window-hud camera "+segments[0].ID)
			roundHUD := strings.Index(log.String(), "[zackvideo] window-hud round "+segments[firstRound].ID)
			firstRoundStart := strings.Index(log.String(), "[zackvideo] record-start-"+segments[firstRound].ID+":")
			if firstRound < 1 || cleanHUD < 0 || roundHUD < cleanHUD || firstRoundStart < roundHUD || strings.Count(log.String(), "[zackvideo] window-hud ") != 2 {
				t.Fatalf("the cameras record first without radar or killfeed and the rounds get their HUD back before they record; log:\n%s", log.String())
			}
			if tc.camera == "complete" {
				firstCameraStart := strings.Index(log.String(), "[zackvideo] record-start-"+segments[0].ID+":")
				if firstCameraStart < cleanHUD || firstCameraStart > roundHUD {
					t.Fatalf("camera %s did not record under its clean HUD", segments[0].ID)
				}
			}
			if tc.camera == "unverified" && !strings.Contains(log.String(), `"reason":"camera_unverified"`) {
				t.Fatal("the unverified camera was not reported")
			}
		})
	}
}
