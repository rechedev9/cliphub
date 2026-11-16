package worldoverlay

import (
	"strings"
	"testing"
)

// segmentLookingAlongX is a CT POV standing still and looking along +X for ticks 1000 to 1000+n.
func segmentLookingAlongX(n int, players func(tick int) []Player) Segment {
	seg := Segment{ID: "seg-1", Names: []string{"mate", "foe"}}
	for tick := 1000; tick <= 1000+n; tick++ {
		seg.Frames = append(seg.Frames, Frame{Tick: tick, Camera: &Camera{Eye: Vec{0, 0, 64}, Side: "CT"}, Players: players(tick)})
	}
	return seg
}

func testLayout(seconds float64) Layout {
	return Layout{Width: 1920, Height: 1080, FPS: 60, TickRate: 64, StartTick: 1000, DurationSeconds: seconds, Labels: true, Trails: true}
}

func dialogues(script, contains string) []string {
	var out []string
	for line := range strings.SplitSeq(script, "\n") {
		if strings.HasPrefix(line, "Dialogue:") && strings.Contains(line, contains) {
			out = append(out, line)
		}
	}
	return out
}

func TestScriptLabelsTeammatesAndOnlyEnemiesThePOVSaw(t *testing.T) {
	seg := segmentLookingAlongX(70, func(tick int) []Player {
		return []Player{
			{Slot: 0, Side: "CT", Health: 100, Eye: Vec{600, 0, 64}},
			{Slot: 1, Side: "T", Health: 40, Eye: Vec{600, 100, 64}, Spotted: tick == 1010},
		}
	})
	script, drawn, err := Script(seg, testLayout(1))
	if err != nil || !drawn {
		t.Fatalf("Script = drawn %v, err %v", drawn, err)
	}
	if got := len(dialogues(script, "}mate")); got != 60 {
		t.Errorf("teammate labelled on %d frames, want every one of 60", got)
	}
	// Frames 10 to 24 sit at ticks 1010.7 to 1025.6, within spottedHoldTicks of the one spotted tick.
	if got := len(dialogues(script, "}foe")); got != 15 {
		t.Errorf("enemy labelled on %d frames, want the 15 after CS2 spotted it", got)
	}
	if !strings.Contains(script, "PlayResX: 1920\nPlayResY: 1080") {
		t.Error("the script is not on the capture frame")
	}
}

func TestScriptHoldsAnEnemyLabelAfterAHit(t *testing.T) {
	seg := segmentLookingAlongX(70, func(tick int) []Player {
		return []Player{{Slot: 1, Side: "T", Health: 60, Eye: Vec{600, 100, 64}, Hit: tick == 1010}}
	})
	script, _, err := Script(seg, testLayout(1))
	if err != nil {
		t.Fatal(err)
	}
	// Frames 10 to 39 sit at ticks 1010.7 to 1041.6, within hitHoldTicks of the hit.
	if got := len(dialogues(script, "}foe")); got != 30 {
		t.Errorf("enemy labelled on %d frames after the hit, want 30", got)
	}
}

func TestScriptFramesAreContiguous(t *testing.T) {
	seg := segmentLookingAlongX(70, func(int) []Player {
		return []Player{{Slot: 0, Side: "CT", Health: 100, Eye: Vec{600, 0, 64}}}
	})
	script, _, err := Script(seg, testLayout(1))
	if err != nil {
		t.Fatal(err)
	}
	lines := dialogues(script, "}mate")
	if !strings.HasPrefix(lines[0], "Dialogue: 2,0:00:00.00,0:00:00.01,") {
		t.Fatalf("first frame = %q, want it to start at 0", lines[0])
	}
	for i := 1; i < len(lines); i++ {
		prevEnd := strings.Split(lines[i-1], ",")[2]
		if start := strings.Split(lines[i], ",")[1]; start != prevEnd {
			t.Fatalf("frame %d starts at %s, the previous ends at %s", i, start, prevEnd)
		}
	}
}

func TestScriptHidesWhileScopedOrDead(t *testing.T) {
	seg := segmentLookingAlongX(70, func(int) []Player {
		return []Player{{Slot: 0, Side: "CT", Health: 100, Eye: Vec{600, 0, 64}}}
	})
	for i := range seg.Frames {
		if i%2 == 0 {
			seg.Frames[i].Camera = nil
		} else {
			seg.Frames[i].Camera.Scoped = true
		}
	}
	script, drawn, err := Script(seg, testLayout(1))
	if err != nil || drawn || len(dialogues(script, "")) != 0 {
		t.Fatalf("Script drew %v with err %v while the POV was dead or scoped", drawn, err)
	}
}

func TestScriptDrawsOwnTeamTrailsAndFadesThem(t *testing.T) {
	seg := segmentLookingAlongX(140, func(int) []Player { return nil })
	smoke := Grenade{Kind: KindSmoke, Side: "CT"}
	for tick := 1000; tick <= 1030; tick++ {
		y := 100 - 10*float64(min(tick-1000, 20))
		smoke.Points = append(smoke.Points, Point{Tick: tick, Pos: Vec{300, y, 64}})
	}
	enemy := smoke
	enemy.Side = "T"
	seg.Grenades = []Grenade{smoke}
	layout := testLayout(2)
	script, drawn, err := Script(seg, layout)
	if err != nil || !drawn {
		t.Fatalf("Script = drawn %v, err %v", drawn, err)
	}
	trails := dialogues(script, `\p2\bord1\shad0\1c&HE6E6E6&`)
	// The smoke rests at tick 1020 and the trail lasts trailHoldSeconds more: frames 1 to 90.
	if len(trails) != 90 {
		t.Fatalf("trail drawn on %d frames, want 90", len(trails))
	}
	if !strings.Contains(trails[0], `\alpha&H00&`) || strings.Contains(trails[89], `\alpha&H00&`) {
		t.Errorf("the trail does not start opaque and fade out:\n%s\n%s", trails[0], trails[89])
	}
	seg.Grenades = []Grenade{enemy}
	if _, drawn, _ := Script(seg, layout); drawn {
		t.Error("an enemy grenade's trail was drawn")
	}
	layout.Trails = false
	seg.Grenades = []Grenade{smoke}
	if _, drawn, _ := Script(seg, layout); drawn {
		t.Error("a trail was drawn with trails off")
	}
}

func TestScriptRefusesABadLayout(t *testing.T) {
	if _, _, err := Script(Segment{}, Layout{Width: 1920, Height: 1080, TickRate: 64, DurationSeconds: 1}); err == nil {
		t.Error("a layout without fps was accepted")
	}
}

func TestAssTextKeepsNamesPlain(t *testing.T) {
	if got := assText(" {\\b1}x\ny "); got != `(/b1)x y` {
		t.Errorf("assText = %q", got)
	}
	if got := assText("abcdefghijklmnopqrstuvwxyz"); got != "abcdefghijklmnopq…" {
		t.Errorf("assText = %q, want 17 runes and an ellipsis", got)
	}
}

func TestValidateRefusesTracksTheOverlayCouldMisread(t *testing.T) {
	good := Track{Version: Version, TickRate: 64, Segments: []Segment{{
		ID: "a", Names: []string{"x"},
		Frames:   []Frame{{Tick: 1, Players: []Player{{Slot: 0}}}, {Tick: 2}},
		Grenades: []Grenade{{Kind: KindHE, Points: []Point{{Tick: 1}, {Tick: 2}}}},
	}}}
	if err := good.Validate(); err != nil {
		t.Fatalf("a good track was refused: %v", err)
	}
	cases := map[string]func(*Track){
		"version":       func(t *Track) { t.Version = "world-overlay-v0" },
		"tick rate":     func(t *Track) { t.TickRate = 0 },
		"repeated id":   func(t *Track) { t.Segments = append(t.Segments, t.Segments[0]) },
		"frame order":   func(t *Track) { t.Segments[0].Frames[1].Tick = 1 },
		"unnamed slot":  func(t *Track) { t.Segments[0].Frames[0].Players[0].Slot = 1 },
		"grenade kind":  func(t *Track) { t.Segments[0].Grenades[0].Kind = "c4" },
		"grenade order": func(t *Track) { t.Segments[0].Grenades[0].Points[1].Tick = 0 },
	}
	for name, mutate := range cases {
		bad := good
		bad.Segments = []Segment{good.Segments[0]}
		bad.Segments[0].Frames = []Frame{{Tick: 1, Players: []Player{{Slot: 0}}}, {Tick: 2}}
		bad.Segments[0].Grenades = []Grenade{{Kind: KindHE, Points: []Point{{Tick: 1}, {Tick: 2}}}}
		mutate(&bad)
		if err := bad.Validate(); err == nil {
			t.Errorf("%s: the track was accepted", name)
		}
	}
}
