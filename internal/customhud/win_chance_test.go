package customhud

import (
	"strings"
	"testing"
)

// plantedState is a 4v2 at 30 s on the bomb clock, 10 s after the plant at 64 ticks.
func plantedState() Snapshot {
	s := Example()
	s.Tick, s.Phase, s.TimeRemaining, s.PlantTick = 2000, "planted", 30, 2000-640
	s.Players[7].Alive, s.Players[7].Health = true, 100
	for i := range s.Players {
		s.Players[i].Equip = 4000
	}
	return s
}

func TestPostPlantWinChanceNeedsEveryRecordedFact(t *testing.T) {
	ct := postPlantWinCT(plantedState(), "de_mirage", 64)
	if ct < 1 || ct > 99 {
		t.Fatalf("4v2 after the plant = %d, want a shown share", ct)
	}
	later := plantedState()
	later.Tick += 64 * 15
	later.TimeRemaining = 15
	if got := postPlantWinCT(later, "de_mirage", 64); got >= ct {
		t.Fatalf("CT chance should fall as the bomb runs: %d then %d", ct, got)
	}
	for name, change := range map[string]func(*Snapshot){
		"live round":        func(s *Snapshot) { s.Phase, s.PlantTick = "live", 0 },
		"no plant tick":     func(s *Snapshot) { s.PlantTick = 0 },
		"unknown clock":     func(s *Snapshot) { s.TimeRemaining = -1 },
		"under 5 s left":    func(s *Snapshot) { s.TimeRemaining = 4 },
		"one T alive":       func(s *Snapshot) { s.Players[7].Alive = false },
		"unknown slot":      func(s *Snapshot) { s.Players[0].Known = false },
		"missing equipment": func(s *Snapshot) { s.Players[3].Equip = 0 },
	} {
		s := plantedState()
		change(&s)
		if got := postPlantWinCT(s, "de_mirage", 64); got != 0 {
			t.Errorf("%s: win chance %d, want hidden", name, got)
		}
	}
	defusing := plantedState()
	defusing.Phase = "defusing"
	if postPlantWinCT(defusing, "de_mirage", 64) == 0 {
		t.Error("the overlay must stay through a defuse")
	}
}

func TestWinChancePanelFollowsTheOption(t *testing.T) {
	s := plantedState()
	s.WinCT = 38
	if _, ok := sceneNodes(t, s)["win-chance/ct"]; ok {
		t.Fatal("win chance drawn without the option")
	}
	r, err := NewRenderer("arena")
	if err != nil {
		t.Fatal(err)
	}
	r.Options.WinProbability = true
	nodes := map[string]Node{}
	for _, n := range r.Scene(s, ExampleTarget) {
		nodes[n.ID] = n
	}
	if nodes["win-chance/ct"].Text != "38%" || nodes["win-chance/t"].Text != "62%" || nodes["win-chance/label"].Text != "WIN PROBABILITY" {
		t.Fatalf("panel = %+v / %+v", nodes["win-chance/ct"], nodes["win-chance/t"])
	}
	if ct, tr := nodes["win-chance/ct-bar"], nodes["win-chance/t-bar"]; ct.W+tr.W != winChanceWidth-28 || ct.W >= tr.W {
		t.Fatalf("bar split %d/%d, want the CT share left", ct.W, tr.W)
	}
	s.WinCT = 0
	for _, n := range r.Scene(s, ExampleTarget) {
		if strings.HasPrefix(n.ID, "win-chance") {
			t.Fatal("panel drawn with no win chance")
		}
	}
}

func TestASSDrawsTheWinChanceFromTelemetry(t *testing.T) {
	r, err := NewRenderer("arena")
	if err != nil {
		t.Fatal(err)
	}
	r.Options.WinProbability = true
	live := plantedState()
	live.Tick, live.Phase, live.PlantTick, live.TimeRemaining = 1000, "live", 0, 70
	planted := plantedState()
	d := Timeline{Version: TelemetryVersion, DemoSHA256: strings.Repeat("a", 64), TargetSteamID: ExampleTarget, Map: "de_mirage", TickRate: 64, EndTick: 4000, Snapshots: []Snapshot{live, planted}}
	ass, err := r.ASS(d, Window{StartTick: 1000, Frames: 60 * 30})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ass, "WIN PROBABILITY") {
		t.Fatal("ASS has no win chance after the plant")
	}
}
