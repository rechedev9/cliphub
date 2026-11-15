package radaroverlay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/radarmap"
)

func mirage(t *testing.T) radarmap.Calibration {
	t.Helper()
	cal, ok := radarmap.Lookup("de_mirage")
	if !ok {
		t.Fatal("de_mirage has no calibration")
	}
	return cal
}

func TestLoadRefusesATrackTheOverlayCouldMisplace(t *testing.T) {
	good := Track{Version: Version, TickRate: 64, Map: "de_mirage", Calibration: mirage(t), Segments: []Segment{{ID: "a", Frames: []Frame{{Tick: 1}, {Tick: 5}}}}}
	write := func(track Track) string {
		path := filepath.Join(t.TempDir(), "radar.json")
		raw, err := json.Marshal(track)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	if _, err := Load(write(good)); err != nil {
		t.Fatalf("Load(good) = %v", err)
	}
	bad := map[string]func(*Track){
		"version":        func(tr *Track) { tr.Version = "radar-overlay-v0" },
		"no calibration": func(tr *Track) { tr.Calibration = radarmap.Calibration{} },
		"tick order":     func(tr *Track) { tr.Segments[0].Frames[1].Tick = 1 },
		"repeated id":    func(tr *Track) { tr.Segments = append(tr.Segments, Segment{ID: "a"}) },
		"bomb state":     func(tr *Track) { tr.Segments[0].Frames[0].Bomb = &Bomb{State: "thrown"} },
		"utility kind":   func(tr *Track) { tr.Segments[0].Utility = []Utility{{Kind: "decoy"}} },
		"utility end":    func(tr *Track) { tr.Segments[0].Utility = []Utility{{Kind: KindSmoke, Tick: 9, EndTick: 3}} },
	}
	for name, mutate := range bad {
		track := good
		track.Segments = []Segment{{ID: "a", Frames: []Frame{{Tick: 1}, {Tick: 5}}}}
		mutate(&track)
		if _, err := Load(write(track)); err == nil {
			t.Errorf("%s: Load accepted the track", name)
		}
	}
}

func TestPlaceKeepsTheRadarInsideItsSafeArea(t *testing.T) {
	cases := []struct {
		name string
		in   Placement
		want Rect
	}{
		{"portrait default is bottom left above the caption band", Placement{Width: 1080, Height: 1920}, Rect{X: 43, Y: 1652 - 344, W: 344, H: 344}},
		{"portrait top right sits under the HUD strip", Placement{Position: PositionTopRight, Size: SizeSmall, Width: 1080, Height: 1920}, Rect{X: 1080 - 280 - 43, Y: 300, W: 280, H: 280}},
		{"landscape default is top left", Placement{Size: SizeLarge, Width: 1920, Height: 1080}, Rect{X: 43, Y: 43, W: 432, H: 432}},
		{"landscape bottom right", Placement{Position: PositionBottomRight, Width: 1920, Height: 1080}, Rect{X: 1920 - 344 - 43, Y: 1080 - 43 - 344, W: 344, H: 344}},
	}
	for _, tc := range cases {
		if got := Place(tc.in); got != tc.want {
			t.Errorf("%s: Place = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

func TestPlaceStacksClearOfTheWatermarkAndTheBanner(t *testing.T) {
	watermark := Rect{X: 43, Y: 1550, W: 170, H: 100}
	banner := Rect{X: 108, Y: 1300, W: 864, H: 120}
	got := Place(Placement{Width: 1080, Height: 1920, Avoid: []Rect{watermark, banner}})
	if got.overlaps(watermark) || got.overlaps(banner) {
		t.Fatalf("radar %+v overlaps the watermark %+v or the banner %+v", got, watermark, banner)
	}
	if got.Y+got.H > banner.Y {
		t.Errorf("radar %+v should sit above the banner, the nearer edge from a bottom corner", got)
	}
	top := Place(Placement{Position: PositionTopLeft, Width: 1920, Height: 1080, Avoid: []Rect{{X: 43, Y: 43, W: 170, H: 100}}})
	if top.Y != 43+100+21 {
		t.Errorf("top radar y = %d, want under the watermark with half a margin of gap", top.Y)
	}
}

func TestValidateOptions(t *testing.T) {
	for _, ok := range []error{ValidatePosition(""), ValidatePosition(PositionBottomRight), ValidateSize(""), ValidateSize(SizeLarge), ValidateOpacity(0), ValidateOpacity(0.4), ValidateOpacity(1)} {
		if ok != nil {
			t.Errorf("valid option refused: %v", ok)
		}
	}
	for _, bad := range []error{ValidatePosition("center"), ValidateSize("huge"), ValidateOpacity(0.2), ValidateOpacity(1.2)} {
		if bad == nil {
			t.Error("invalid option accepted")
		}
	}
	if EffectiveOpacity(0) != DefaultOpacity || EffectiveOpacity(0.5) != 0.5 {
		t.Error("EffectiveOpacity does not keep a chosen opacity or default an unset one")
	}
}

var moveRE = regexp.MustCompile(`\\move\(([-\d.]+),([-\d.]+),([-\d.]+),([-\d.]+)\)`)

func testLayout(t *testing.T) Layout {
	return Layout{Width: 1080, Height: 1920, TickRate: 64, Calibration: mirage(t), Box: Rect{X: 40, Y: 1300, W: 344, H: 344}, Opacity: 1}
}

func TestScriptMovesPlayersBetweenSamplesOnTheEditedTimeline(t *testing.T) {
	layout := testLayout(t)
	seg := Segment{ID: "a", TickStart: 1000, TickEnd: 1128, POV: 0, Frames: []Frame{
		{Tick: 1000, Players: []Player{{Slot: 0, Side: "CT", X: 0, Y: 0}, {Slot: 1, Side: "T", X: 500, Y: 500}}},
		{Tick: 1064, Players: []Player{{Slot: 0, Side: "CT", X: 200, Y: 0}, {Slot: 1, Side: "T", X: -1500, Y: 500}}},
	}}
	script, err := Script(layout, []Span{{Segment: seg, StartTick: 1000, Offset: 10, From: 10, Until: 12}})
	if err != nil {
		t.Fatal(err)
	}
	var povLine, enemyFirst string
	for _, line := range strings.Split(script, "\n") {
		if strings.HasPrefix(line, "Dialogue: 6,0:00:10.00,0:00:11.00") {
			povLine = line
		}
		if strings.HasPrefix(line, "Dialogue: 4,0:00:10.00,0:00:11.00") {
			enemyFirst = line
		}
	}
	m := moveRE.FindStringSubmatch(povLine)
	if m == nil {
		t.Fatalf("the POV does not move over the first second:\n%s", script)
	}
	x0, _ := strconv.ParseFloat(m[1], 64)
	x1, _ := strconv.ParseFloat(m[3], 64)
	wantShift := 200 / layout.Calibration.Scale * 344 / 1024
	if d := x1 - x0; d < wantShift-0.2 || d > wantShift+0.2 {
		t.Errorf("POV moved %.2f px, want %.2f (200 world units on a 344 px radar)", d, wantShift)
	}
	if !strings.Contains(povLine, `\3c&HFFFFFF&`) || !strings.Contains(povLine, `\fad(150,0)`) {
		t.Errorf("POV line = %s, want a white rim and a fade in at the play's start", povLine)
	}
	if !strings.Contains(enemyFirst, `\pos(`) || strings.Contains(enemyFirst, `\move(`) {
		t.Errorf("enemy line = %s, want it held still across a 2000 unit jump", enemyFirst)
	}
	if !strings.Contains(script, `\clip(40,1300,384,1644)`) {
		t.Error("events are not clipped to the radar box")
	}
}

func TestScriptDrawsUtilityDeathsAndTheBombOnlyInsideTheSpan(t *testing.T) {
	seg := Segment{ID: "a", TickStart: 0, TickEnd: 640, POV: -1,
		Frames: []Frame{
			{Tick: 0, Players: []Player{{Slot: 0, Side: "T"}}, Bomb: &Bomb{State: BombCarried, Carrier: 0}},
			{Tick: 320, Bomb: &Bomb{State: BombPlanted, Carrier: -1, X: 10, Y: 10}},
			{Tick: 384, Bomb: &Bomb{State: BombPlanted, Carrier: -1, X: 10, Y: 10}},
		},
		Deaths: []Death{{Slot: 3, Side: "CT", Tick: 100}},
		Utility: []Utility{
			{Kind: KindSmoke, Tick: -640, EndTick: 320}, // Burning before the play.
			{Kind: KindFire, Tick: 64, EndTick: 512},
			{Kind: KindFlash, Tick: 128},
			{Kind: KindHE, Tick: 900}, // After the span.
		},
	}
	script, err := Script(testLayout(t), []Span{{Segment: seg, StartTick: 0, Offset: 0, From: 0, Until: 10}})
	if err != nil {
		t.Fatal(err)
	}
	count := func(prefix string) int { return strings.Count(script, prefix) }
	if count(`\1c`+inkSmoke) != 1 || count(`\1c`+inkFire) != 1 {
		t.Errorf("want one smoke and one fire disc:\n%s", script)
	}
	if !strings.Contains(script, "Dialogue: 0,0:00:00.00,0:00:05.00") {
		t.Error("the smoke burning before the play should show from its start until it goes out at 5 s")
	}
	if count(`\1c`+inkFlash) != burstSteps || count(`\1c`+inkHE) != 0 {
		t.Errorf("want %d flash rings and no HE after the span", burstSteps)
	}
	if !strings.Contains(script, "Dialogue: 2,0:00:01.56,0:00:10.00") {
		t.Error("the death should stay marked from 1.56 s to the end of the play")
	}
	if count("Dialogue: 3,0:00:05.00,0:00:10.00") != 1 {
		t.Errorf("the planted bomb should be one event from the plant to the end:\n%s", script)
	}
	if count("Dialogue: 5,") != 1 {
		t.Error("the carried bomb should mark its carrier while carried")
	}
}

func TestScriptDimsTheOtherLevelAndAppliesOpacity(t *testing.T) {
	nuke, _ := radarmap.Lookup("de_nuke")
	layout := testLayout(t)
	layout.Calibration, layout.Opacity = nuke, 0.5
	seg := Segment{ID: "a", TickEnd: 64, POV: 0, Frames: []Frame{{Tick: 0, Players: []Player{{Slot: 0, Side: "CT", Z: -300}, {Slot: 1, Side: "T", Z: -700}}}}}
	script, err := Script(layout, []Span{{Segment: seg, Until: 1, Level: radarmap.LevelDefault}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, `\1a&H80&`) || !strings.Contains(script, `\1a&HCC&`) {
		t.Errorf("want the upper player at half opacity (&H80) and the lower one dimmed further (&HCC):\n%s", script)
	}
	if got := seg.POVLevel(nuke); got != radarmap.LevelDefault {
		t.Errorf("POVLevel = %q, want the POV's own level", got)
	}
}

func TestScriptIsEmptyWithNothingToDraw(t *testing.T) {
	script, err := Script(testLayout(t), []Span{{Segment: Segment{ID: "a"}, From: 0, Until: 0.01}})
	if err != nil || script != "" {
		t.Errorf("Script = %q, %v; want empty", script, err)
	}
	if _, err := Script(Layout{}, nil); err == nil {
		t.Error("Script accepted an empty layout")
	}
}

func TestPlateRoundsTheCornersAndScalesAlpha(t *testing.T) {
	plate := Plate(nil, 200, 0.5)
	if got := plate.NRGBAAt(0, 0).A; got != 0 {
		t.Errorf("corner alpha = %d, want a rounded, see-through corner", got)
	}
	centre := plate.NRGBAAt(100, 100)
	want := uint8(float64(plateInk.A)*0.5 + 0.5)
	if centre.A != want || centre.R+1 < plateInk.R || centre.R > plateInk.R+1 {
		t.Errorf("centre = %+v, want the plate ink at half its alpha (%d)", centre, want)
	}
	if edge := plate.NRGBAAt(100, 0); edge.R <= centre.R {
		t.Errorf("edge %+v is not lighter than the centre: the rim is missing", edge)
	}
}
