package tacticalplan

import (
	"reflect"
	"testing"
)

// spatialMatch is scoutDocument with positions for round 1: slot 0 walks +X at
// one cell per second from freeze end and dies 2.5 s in; slot 2 stands still.
func spatialMatch(t *testing.T, filter Filter) SpatialMatch {
	t.Helper()
	doc := scoutDocument()
	var frames []Frame
	for tick := 640; tick <= 640+3*64; tick += 8 {
		walker := Sample{Slot: 0, X: float64(tick - 640), Y: 10, Flags: FlagAlive}
		if tick >= 640+160 {
			walker.Flags = 0
		}
		frames = append(frames, Frame{Tick: tick, Samples: []Sample{walker, {Slot: 2, X: 500, Y: 500, Flags: FlagAlive | FlagSideT}}})
	}
	blob, err := EncodePositions([]RoundFrames{{Round: 1, Frames: frames}}, 8, 64)
	if err != nil {
		t.Fatalf("EncodePositions: %v", err)
	}
	doc.Positions = blob.Descriptor
	return SpatialMatch{MatchID: scoutMatchOne, Document: doc, Filter: filter, Blob: func() ([]byte, error) { return blob.Data, nil }}
}

func findCell(h Heatmap, side Side, x, y int) (HeatmapCell, bool) {
	for _, c := range h.Cells {
		if c.Side == side && c.X == x && c.Y == y {
			return c, true
		}
	}
	return HeatmapCell{}, false
}

func TestHeatmapDeathsFollowTheTeamAndCarryEvidence(t *testing.T) {
	m := spatialMatch(t, Filter{TeamKey: "t-start"})
	h, err := BuildHeatmap([]SpatialMatch{m}, HeatmapOptions{Kind: HeatDeath})
	if err != nil {
		t.Fatal(err)
	}
	if h.Rounds != 4 || len(h.Cells) != 1 {
		t.Fatalf("heatmap = %+v, want 4 rounds and the one death of t-start", h)
	}
	cell, ok := findCell(h, SideCT, 14, 14)
	if !ok || cell.Count != 1 || cell.Seconds != 21.3 || cell.Level != "default" {
		t.Fatalf("death cell = %+v, want (14,14) on CT at 21.3 s", h.Cells)
	}
	if want := []Evidence{{MatchID: scoutMatchOne, Round: 3, Tick: 12000}}; !reflect.DeepEqual(cell.Examples, want) {
		t.Fatalf("examples = %+v, want %+v", cell.Examples, want)
	}
	if len(h.Sides) != 1 || h.Sides[0].Total != 1 || h.Sides[0].MaxCount != 1 {
		t.Fatalf("sides = %+v", h.Sides)
	}
	other, _ := BuildHeatmap([]SpatialMatch{spatialMatch(t, Filter{TeamKey: "ct-start"})}, HeatmapOptions{Kind: HeatDeath})
	if len(other.Cells) != 0 {
		t.Fatalf("ct-start deaths = %+v, want none", other.Cells)
	}
}

func TestHeatmapUtilityByGrenadeAndWindow(t *testing.T) {
	m := spatialMatch(t, Filter{TeamKey: "ct-start"})
	all, err := BuildHeatmap([]SpatialMatch{m}, HeatmapOptions{Kind: HeatUtility})
	if err != nil {
		t.Fatal(err)
	}
	if c, ok := findCell(all, SideT, 15, 15); !ok || c.Count != 2 || c.Rounds != 1 || len(c.Examples) != 1 {
		t.Fatalf("T cell = %+v, want the smoke and the flash of round 3 in one round", all.Cells)
	}
	if c, ok := findCell(all, SideCT, -16, -16); !ok || c.Count != 1 {
		t.Fatalf("CT cell = %+v, want the round 2 molotov", all.Cells)
	}
	smokes, _ := BuildHeatmap([]SpatialMatch{m}, HeatmapOptions{Kind: HeatUtility, Grenades: []EventKind{EventSmoke}})
	if c, ok := findCell(smokes, SideT, 15, 15); !ok || c.Count != 1 || len(smokes.Cells) != 1 {
		t.Fatalf("smokes = %+v, want only the T smoke", smokes.Cells)
	}
	early, _ := BuildHeatmap([]SpatialMatch{m}, HeatmapOptions{Kind: HeatUtility, Window: SpatialWindow{To: 10}})
	if len(early.Cells) != 1 || early.Cells[0].Side != SideCT {
		t.Fatalf("first 10 s = %+v, want only the molotov at 0.9 s", early.Cells)
	}
}

func TestHeatmapPositionsAndTimingsFromTheBlob(t *testing.T) {
	m := spatialMatch(t, Filter{TeamKey: "ct-start", RoundTo: 1})
	pos, err := BuildHeatmap([]SpatialMatch{m}, HeatmapOptions{Kind: HeatPosition})
	if err != nil {
		t.Fatal(err)
	}
	if len(pos.Cells) != 3 || pos.MissingPositions != 0 {
		t.Fatalf("positions = %+v, want one sample a second in cells 0, 1 and 2 before the death", pos.Cells)
	}
	for x := range 3 {
		c, ok := findCell(pos, SideCT, x, 0)
		if !ok || c.Count != 1 || c.Examples[0].Tick != 640+64*x {
			t.Fatalf("cell %d = %+v", x, c)
		}
	}
	timing, err := BuildHeatmap([]SpatialMatch{m}, HeatmapOptions{Kind: HeatTiming})
	if err != nil {
		t.Fatal(err)
	}
	if c, ok := findCell(timing, SideCT, 2, 0); !ok || c.Count != 1 || c.Seconds != 2 {
		t.Fatalf("timing of cell 2 = %+v, want the first arrival at 2 s", timing.Cells)
	}
	m.Blob = nil
	none, _ := BuildHeatmap([]SpatialMatch{m}, HeatmapOptions{Kind: HeatPosition})
	if none.MissingPositions != 1 || len(none.Cells) != 0 {
		t.Fatalf("no blob = %+v, want the match reported as missing positions", none)
	}
}

func TestKeepEarliestKeepsTheFastestArrivals(t *testing.T) {
	var list []timedEvidence
	for i, s := range []float64{9, 4, 7, 4, 1} {
		list = keepEarliest(list, timedEvidence{s, Evidence{Round: i + 1}})
	}
	got := []int{list[0].ev.Round, list[1].ev.Round, list[2].ev.Round}
	if want := []int{5, 2, 4}; !reflect.DeepEqual(got, want) {
		t.Fatalf("rounds = %v, want %v (1 s, then the first 4 s, then the second)", got, want)
	}
}

func TestOverlaySyncsRoundsFromFreezeEnd(t *testing.T) {
	m := spatialMatch(t, Filter{TeamKey: "ct-start"})
	o, err := BuildOverlay([]SpatialMatch{m}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if o.Total != 4 || len(o.Rounds) != 2 || o.Limit != 2 {
		t.Fatalf("overlay = total %d, %d rounds; want 4 matched and 2 drawn", o.Total, len(o.Rounds))
	}
	first := o.Rounds[0]
	if first.Round != 1 || first.Side != SideCT || first.DurationSeconds != 60 || len(first.Tracks) != 1 {
		t.Fatalf("round 1 = %+v", first)
	}
	track := first.Tracks[0]
	if track.Slot != 0 || track.Name != "a1" || track.Start != 0 || !track.Died || len(track.Points) != 5 {
		t.Fatalf("track = %+v, want 5 points (0 to 2 s) and a death", track)
	}
	if track.Points[4] != [3]int{128, 10, 0} {
		t.Fatalf("point at 2 s = %v, want x 128", track.Points[4])
	}
	second := o.Rounds[1]
	if len(second.Tracks) != 0 || len(second.Events) != 1 || !second.Events[0].Subject || second.Events[0].Seconds != 0.94 {
		t.Fatalf("round 2 = %+v, want no positions and the team's molotov at 0.94 s", second)
	}
	again, _ := BuildOverlay([]SpatialMatch{spatialMatch(t, Filter{TeamKey: "ct-start"})}, 2)
	if !reflect.DeepEqual(o, again) {
		t.Fatal("the same input built a different overlay")
	}
}
