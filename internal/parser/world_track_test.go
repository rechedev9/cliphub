package parser

import (
	"bytes"
	"context"
	"testing"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"

	"github.com/rechedev9/cliphub/internal/worldoverlay"
)

func TestExtractWorldTrackNeedsATickRateAndWindows(t *testing.T) {
	for name, opts := range map[string]WorldTrackOptions{
		"no tick rate": {Windows: []worldoverlay.Window{{ID: "a", TickEnd: 10}}},
		"no windows":   {TickRate: 64},
	} {
		if _, err := ExtractWorldTrack(context.Background(), bytes.NewReader(nil), opts); err == nil {
			t.Errorf("%s: ExtractWorldTrack accepted the options", name)
		}
	}
}

func TestWorldSamplerKeepsOnlyThePOVHitsAndTheGrenadesOfItsWindow(t *testing.T) {
	s := newWorldSampler(WorldTrackOptions{TickRate: 64, Windows: []worldoverlay.Window{
		{ID: "a", POVSteamID64: "7", TickStart: 1000, TickEnd: 2000},
	}})
	if s.lastTick != 2000 || s.lead != worldGrenadeLeadSeconds*64 {
		t.Fatalf("sampler = last %d lead %d", s.lastTick, s.lead)
	}
	s.hurt(7, 9, 1500)
	s.hurt(8, 10, 1500) // Not the POV.
	s.hurt(7, 11, 2500) // Past the window.
	seg := s.segs[0]
	if !seg.hitAt(9, 1500) || seg.hitAt(9, 1501) || seg.hitAt(10, 1500) || seg.hitAt(11, 2500) {
		t.Errorf("hits = %v, want only the POV's hit inside the window", seg.hits)
	}
	point := func(tick int) worldoverlay.Point { return worldoverlay.Point{Tick: tick} }
	seg.grenades = map[int64]*worldoverlay.Grenade{
		1: {Kind: worldoverlay.KindSmoke, Points: []worldoverlay.Point{point(800), point(900)}},  // Landed before the play.
		2: {Kind: worldoverlay.KindFlash, Points: []worldoverlay.Point{point(950), point(1010)}}, // In flight at its start.
		3: {Kind: worldoverlay.KindHE, Points: []worldoverlay.Point{point(1500)}},                // One point draws nothing.
	}
	seg.order = []int64{1, 2, 3}
	got := s.segments()[0].Grenades
	if len(got) != 1 || got[0].Kind != worldoverlay.KindFlash {
		t.Fatalf("grenades = %+v, want only the flash still flying when the play starts", got)
	}
}

func TestExtractWorldTrackReadsARealDemo(t *testing.T) {
	demo := loadRosterDemo(t)
	p := demoinfocs.NewParser(bytes.NewReader(demo))
	roster, err := RosterScan(p)
	p.Close()
	if err != nil || len(roster.Players) == 0 {
		t.Fatalf("RosterScan = %d players, %v", len(roster.Players), err)
	}
	window := worldoverlay.Window{ID: "seg-001", POVSteamID64: roster.Players[0].SteamID64, TickStart: 20000, TickEnd: 20640}
	track, err := ExtractWorldTrack(context.Background(), bytes.NewReader(demo), WorldTrackOptions{DemoSHA256: "sha", TickRate: 64, Windows: []worldoverlay.Window{window}})
	if err != nil {
		t.Fatalf("ExtractWorldTrack error = %v", err)
	}
	seg := track.Segments[0]
	if len(seg.Frames) < 600 || seg.Frames[0].Tick < window.TickStart || seg.Frames[len(seg.Frames)-1].Tick > window.TickEnd {
		t.Fatalf("frames = %d from %d, want one per tick of the window", len(seg.Frames), seg.Frames[0].Tick)
	}
	players := 0
	for _, f := range seg.Frames {
		players += len(f.Players)
	}
	if players == 0 || len(seg.Names) == 0 {
		t.Errorf("no player was sampled in the window: %d samples, names %v", players, seg.Names)
	}
}
