package parser

import (
	"bytes"
	"context"
	"testing"

	demoinfocs "github.com/rechedev9/demo-parser-rsn/pkg/demoinfocs"

	"github.com/rechedev9/cliphub/internal/radaroverlay"
)

func TestExtractRadarTrackRefusesAnUncalibratedMapAndEmptyOptions(t *testing.T) {
	window := []radaroverlay.Window{{ID: "a", TickEnd: 10}}
	for name, opts := range map[string]RadarTrackOptions{
		"no calibration": {TickRate: 64, Map: "de_unknown", Windows: window},
		"no tick rate":   {Map: "de_mirage", Windows: window},
		"no windows":     {TickRate: 64, Map: "de_mirage"},
	} {
		if _, err := ExtractRadarTrack(context.Background(), bytes.NewReader(nil), opts); err == nil {
			t.Errorf("%s: ExtractRadarTrack accepted the options", name)
		}
	}
}

func TestRadarSamplerKeepsTheRoundDeathsAndTheUtilityOfEachWindow(t *testing.T) {
	s := newRadarSampler(RadarTrackOptions{TickRate: 64, Map: "de_mirage", Windows: []radaroverlay.Window{
		{ID: "a", TickStart: 10000, TickEnd: 11000},
	}})
	if s.step != 4 || s.lastTick != 11000 {
		t.Fatalf("sampler step %d last %d, want 4 and 11000", s.step, s.lastTick)
	}
	s.deaths = []radaroverlay.Death{
		{Slot: 1, Tick: 8000},              // Last round: cleared below.
		{Slot: 2, Tick: 9500},              // This round, before the play: shows from its start.
		{Slot: 3, Tick: 10500},             // During the play.
		{Slot: 4, Tick: 12000},             // After the play.
		{Slot: 5, Tick: 7000, Until: 9000}, // An older round.
	}
	s.utility = []radaroverlay.Utility{
		{Kind: radaroverlay.KindSmoke, Tick: 9000},                 // Still burning when the play starts.
		{Kind: radaroverlay.KindSmoke, Tick: 8000, EndTick: 9800},  // Gone before it.
		{Kind: radaroverlay.KindFlash, Tick: 9000},                 // A flash before the play shows nothing.
		{Kind: radaroverlay.KindHE, Tick: 10200},                   // During it.
		{Kind: radaroverlay.KindFire, Tick: 10800, EndTick: 11200}, // Lit during it.
		{Kind: radaroverlay.KindSmoke, Tick: 11500},                // After it.
	}
	s.openSmokes[7] = 0
	s.roundStart(8500)
	if s.deaths[0].Until != 8500 || s.utility[0].EndTick != 8500 || len(s.openSmokes) != 0 {
		t.Fatalf("round start left %+v / %+v open", s.deaths[0], s.utility[0])
	}
	// The round restarted at 8500: undo it for the rest of the case, which is one round.
	s.deaths[1].Until, s.deaths[2].Until, s.deaths[3].Until = 0, 0, 0
	s.utility[0].EndTick = 0
	seg := s.segments()[0]
	var slots []int
	for _, d := range seg.Deaths {
		slots = append(slots, d.Slot)
	}
	if len(slots) != 2 || slots[0] != 2 || slots[1] != 3 {
		t.Errorf("death slots = %v, want the round's deaths up to the play's end [2 3]", slots)
	}
	var kinds []string
	for _, u := range seg.Utility {
		kinds = append(kinds, u.Kind)
	}
	if len(kinds) != 3 || kinds[0] != radaroverlay.KindSmoke || kinds[1] != radaroverlay.KindHE || kinds[2] != radaroverlay.KindFire {
		t.Errorf("utility = %v, want the burning smoke, the HE and the fire", kinds)
	}
}

func TestExtractRadarTrackReadsARealDemo(t *testing.T) {
	demo := loadRosterDemo(t)
	p := demoinfocs.NewParser(bytes.NewReader(demo))
	roster, err := RosterScan(p)
	p.Close()
	if err != nil || len(roster.Players) == 0 {
		t.Fatalf("RosterScan = %d players, %v", len(roster.Players), err)
	}
	window := radaroverlay.Window{ID: "seg-001", POVSteamID64: roster.Players[0].SteamID64, TickStart: 20000, TickEnd: 20640}
	track, err := ExtractRadarTrack(context.Background(), bytes.NewReader(demo), RadarTrackOptions{
		DemoSHA256: "sha", TickRate: 64, Map: roster.Match.Map, Windows: []radaroverlay.Window{window},
	})
	if err != nil {
		t.Fatalf("ExtractRadarTrack error = %v", err)
	}
	seg := track.Segments[0]
	if len(seg.Frames) < 150 || seg.Frames[0].Tick < window.TickStart || seg.Frames[len(seg.Frames)-1].Tick > window.TickEnd {
		t.Fatalf("frames = %d, want about %d samples a second inside the window", len(seg.Frames), radaroverlay.SamplesPerSecond)
	}
	players := 0
	for _, f := range seg.Frames {
		players += len(f.Players)
		for _, pl := range f.Players {
			if px, py := track.Calibration.WorldToPixel(pl.X, pl.Y); px < -64 || py < -64 || px > 1088 || py > 1088 {
				t.Fatalf("player %d at radar pixel (%.0f, %.0f), outside the map", pl.Slot, px, py)
			}
		}
	}
	if players == 0 {
		t.Error("no player was sampled in the window")
	}
}
