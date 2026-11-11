package main

import (
	"net/url"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/radarmap"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestHeatmapViewReadsAsTheOrchestratorAndTheReplayURL(t *testing.T) {
	in := heatmapViewIn{Kind: "kill", KillAt: "victim", ClockFrom: 15, ClockTo: 45.5}
	q, err := in.query()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := q.Encode(), "kill_at=victim&kind=kill&t_from=15&t_to=45.5"; got != want {
		t.Fatalf("query = %s, want %s", got, want)
	}
	page := url.Values{}
	in.studioQuery(page)
	if got, want := page.Encode(), "baja=victim&cuenta=kill&forma=calor&tramo=15-46"; got != want {
		t.Fatalf("studio query = %s, want %s", got, want)
	}
	plant := url.Values{}
	heatmapViewIn{FromPlant: true}.studioQuery(plant)
	if plant.Get("tramo") != "plant" || plant.Has("cuenta") {
		t.Fatalf("plant studio query = %s, want the plant tramo on positions", plant.Encode())
	}
	for name, bad := range map[string]heatmapViewIn{
		"an unknown kind":             {Kind: "smell"},
		"kill_at on deaths":           {Kind: "death", KillAt: "victim"},
		"a grenade outside utility":   {Kind: "position", Grenade: []string{"smoke"}},
		"a window that ends too soon": {ClockFrom: 30, ClockTo: 10},
		"too many cells":              {Limit: 51},
		"a floor that does not exist": {Floor: "roof"},
	} {
		if _, err := bad.query(); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestTeamHeatmapPathCarriesTheTeamScope(t *testing.T) {
	team := uuid.NewString()
	in := teamHeatmapIn{TeamID: team, analysisFilterIn: analysisFilterIn{Side: "ct", From: "2026-10-01"},
		teamScopeIn: teamScopeIn{teamRoundScopeIn: teamRoundScopeIn{Buy: []string{"semi", "force"}, AWP: "with", MaxMatches: 5}},
		Players: []string{"76561198000000001"}, heatmapViewIn: heatmapViewIn{Kind: "death", ClockTo: 55}}
	heat, err := in.teamValues()
	if err != nil {
		t.Fatal(err)
	}
	if heat.Get("player") != "76561198000000001" || heat.Get("kind") != "death" || heat.Has("buy") {
		t.Fatalf("team query = %s, want the focus and view only: the buys travel in the scope", heat.Encode())
	}
	q, err := teamQuery(in.analysisFilterIn, in.teamScopeIn)
	if err != nil {
		t.Fatal(err)
	}
	want := "/equipos/" + team + "?awp=with&buy=half&cuenta=death&forma=calor&from=2026-10-01&jugador=76561198000000001&limit=5&lineup=all&map=de_nuke&side=CT&tab=replay&tramo=0-55"
	if got, err := teamHeatmapPath(in, "de_nuke", q); err != nil || got != want {
		t.Fatalf("path = %s, %v; want %s", got, err, want)
	}
	in.Buy = []string{"eco", "full"}
	if q, err = teamQuery(in.analysisFilterIn, in.teamScopeIn); err == nil {
		_, err = teamHeatmapPath(in, "de_nuke", q)
	}
	if err == nil {
		t.Fatal("two buys the bar cannot hold: no error")
	}
}

func TestRestrictToAreaKeepsTheCellsInTheZoneAndFloor(t *testing.T) {
	got := tuiclient.SpatialHeatmap{Heatmap: &tacticalplan.Heatmap{CellSize: 100, Cells: []tacticalplan.HeatmapCell{
		{X: 1, Y: 1, Count: 3}, {X: 5, Y: 5, Count: 9}, {X: 1, Y: 1, Level: "lower", Count: 2},
	}}}
	lowerCut := -495.0
	got.Geometry = &tacticalplan.MapGeometry{Calibration: radarmap.Calibration{LowerAltitudeMax: &lowerCut}}
	area := &tacticalplan.PatternArea{MinX: 0, MinY: 0, MaxX: 300, MaxY: 300, Level: "lower"}
	if cells := restrictToArea(got, area).Heatmap.Cells; len(cells) != 1 || cells[0].Count != 2 {
		t.Fatalf("lower area cells = %+v, want the lower cell in the zone", cells)
	}
	area.Level = ""
	if cells := restrictToArea(got, area).Heatmap.Cells; len(cells) != 2 {
		t.Fatalf("both floors cells = %+v, want the two cells in the zone", cells)
	}
	if len(got.Heatmap.Cells) != 3 {
		t.Fatal("the answer was changed in place")
	}
}

func TestRestrictToAreaIgnoresTheFloorOnAOneFloorMapAndKeepsTheEdge(t *testing.T) {
	got := tuiclient.SpatialHeatmap{Heatmap: &tacticalplan.Heatmap{CellSize: 100, Cells: []tacticalplan.HeatmapCell{
		{X: 1, Y: 1, Count: 3}, {X: 2, Y: 1, Count: 4},
	}}}
	// The first cell's centre is (150, 150): exactly on the box edge, which counts as inside.
	area := &tacticalplan.PatternArea{MinX: 0, MinY: 0, MaxX: 150, MaxY: 150, Level: "lower"}
	if cells := restrictToArea(got, area).Heatmap.Cells; len(cells) != 1 || cells[0].Count != 3 {
		t.Fatalf("cells = %+v, want the edge cell kept on a map with one floor", cells)
	}
}

func TestByPlayerNeedsOneToFiveDifferentPlayers(t *testing.T) {
	ids := []string{"76561198000000001", "76561198000000002", "76561198000000003", "76561198000000004", "76561198000000005", "76561198000000006"}
	for name, players := range map[string][]string{"none": nil, "six": ids} {
		if _, err := (teamHeatmapIn{ByPlayer: true, Players: players}).teamValues(); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	in := teamHeatmapIn{ByPlayer: true, Players: append(ids[:5:5], ids[0])}
	if _, err := in.teamValues(); err != nil || len(in.playerIDs()) != 5 {
		t.Fatalf("five different players with a repeat: err = %v, ids = %v", err, in.playerIDs())
	}
}

func TestPlayerHeatValuesReplacesThePlayerAndKeepsTheView(t *testing.T) {
	heat := url.Values{"kind": {"death"}, "t_to": {"55"}, "player": {"1", "2"}}
	one := playerHeatValues(heat, "2")
	if one.Encode() != "kind=death&player=2&t_to=55" {
		t.Fatalf("query = %s", one.Encode())
	}
	if len(heat["player"]) != 2 {
		t.Fatal("the shared query was changed")
	}
}

func TestSummarizeHeatmapNamesAndSumsTheHottestCells(t *testing.T) {
	match := uuid.New()
	got := tuiclient.SpatialHeatmap{
		Map: "de_nuke",
		Geometry: &tacticalplan.MapGeometry{Callouts: []tacticalplan.Callout{
			{Name: "Outside", Level: "default", Center: [2]float64{0, 0}},
			{Name: "Ramp", Level: "default", Center: [2]float64{1000, 0}},
		}},
		Heatmap: &tacticalplan.Heatmap{Kind: tacticalplan.HeatKill, KillAt: tacticalplan.KillAtKiller, CellSize: 64, Rounds: 12,
			Sides: []tacticalplan.HeatmapSide{{Side: tacticalplan.SideCT, Level: "default", Total: 10, Rounds: 6}},
			Cells: []tacticalplan.HeatmapCell{
				{Side: tacticalplan.SideCT, Level: "default", X: 15, Y: 0, Count: 2, Rounds: 2, Seconds: 30},
				{Side: tacticalplan.SideCT, Level: "default", X: 0, Y: 0, Count: 5, Rounds: 4, Seconds: 20,
					Examples: []tacticalplan.Evidence{{MatchID: match, Round: 7}}},
				{Side: tacticalplan.SideCT, Level: "default", X: 1, Y: 0, Count: 3, Rounds: 3, Seconds: 25},
			}},
	}
	out := summarizeHeatmap(got, 2)
	if out.Unit != "kills" || out.KillAt != "killer" || out.Rounds != 12 || !out.Truncated || len(out.Cells) != 2 {
		t.Fatalf("summary = %+v, want two kill cells of three", out)
	}
	first := out.Cells[0]
	if first.Callout != "Outside" || first.Count != 5 || first.Share != 0.5 || first.Center != [3]int{32, 32, 0} || first.Examples[0] != match.String()+"/7" {
		t.Fatalf("hottest cell = %+v", first)
	}
	if len(out.Zones) != 2 || out.Zones[0].Callout != "Outside" || out.Zones[0].Count != 8 || out.Zones[0].Cells != 2 || out.Zones[1].Callout != "Ramp" {
		t.Fatalf("zones = %+v, want Outside with both near cells, then Ramp", out.Zones)
	}
}

func TestFloorNarrowsTheCellsAndTotalsAndOpensTheSameFloorInStudio(t *testing.T) {
	got := tuiclient.SpatialHeatmap{Heatmap: &tacticalplan.Heatmap{
		Cells: []tacticalplan.HeatmapCell{{X: 1, Y: 1, Count: 3}, {X: 1, Y: 1, Level: "lower", Count: 2}},
		Sides: []tacticalplan.HeatmapSide{{Side: tacticalplan.SideCT, Level: "default", Total: 3}, {Side: tacticalplan.SideCT, Level: "lower", Total: 2}},
	}}
	lower := restrictToFloor(got, "Lower")
	if len(lower.Heatmap.Cells) != 1 || lower.Heatmap.Cells[0].Count != 2 || len(lower.Heatmap.Sides) != 1 || lower.Heatmap.Sides[0].Total != 2 {
		t.Fatalf("lower = %+v, want the lower floor's cell and total only", lower.Heatmap)
	}
	if upper := restrictToFloor(got, "upper"); len(upper.Heatmap.Cells) != 1 || upper.Heatmap.Cells[0].Count != 3 || len(upper.Heatmap.Sides) != 1 {
		t.Fatalf("upper = %+v, want the upper floor's cell and total only", upper.Heatmap)
	}
	if both := restrictToFloor(got, "both"); len(both.Heatmap.Cells) != 2 || len(got.Heatmap.Cells) != 2 {
		t.Fatalf("both = %+v, want everything, and the answer read from untouched", both.Heatmap)
	}
	page := url.Values{}
	heatmapViewIn{Floor: "lower"}.studioQuery(page)
	if page.Get("piso") != "inferior" {
		t.Fatalf("studio query = %s, want piso=inferior", page.Encode())
	}
	page = url.Values{}
	heatmapViewIn{Floor: "upper", Kind: "death"}.studioQuery(page)
	if page.Get("piso") != "superior" || page.Get("cuenta") != "death" {
		t.Fatalf("studio query = %s, want piso=superior on deaths", page.Encode())
	}
}

func TestHeatmapZonesNameOnePlacePerLineAndTellFloorsApart(t *testing.T) {
	got := tuiclient.SpatialHeatmap{
		Map: "de_nuke",
		Geometry: &tacticalplan.MapGeometry{Callouts: []tacticalplan.Callout{
			{Name: "Rafters", Level: "default", Center: [2]float64{864, -288}},
			{Name: "BombsiteA", Level: "default", Center: [2]float64{640, -1040}},
			{Name: "Ramp", Level: "default", Center: [2]float64{600, 330}},
			{Name: "Ramp", Level: "lower", Center: [2]float64{650, 160}},
		}},
		Heatmap: &tacticalplan.Heatmap{Kind: tacticalplan.HeatPosition, CellSize: 64,
			Sides: []tacticalplan.HeatmapSide{{Side: tacticalplan.SideCT, Level: "default", Total: 156}, {Side: tacticalplan.SideCT, Level: "lower", Total: 25}},
			Cells: []tacticalplan.HeatmapCell{
				// Three hot cells of the one place CS2 calls Rafters, as the real match had them.
				{Side: tacticalplan.SideCT, Level: "default", X: 13, Y: -5, Z: -128, Count: 49},
				{Side: tacticalplan.SideCT, Level: "default", X: 14, Y: -5, Z: -128, Count: 47},
				{Side: tacticalplan.SideCT, Level: "default", X: 10, Y: -10, Z: -207, Count: 30},
				{Side: tacticalplan.SideCT, Level: "default", X: 9, Y: -17, Count: 20},
				{Side: tacticalplan.SideCT, Level: "default", X: 9, Y: 5, Count: 10},
				{Side: tacticalplan.SideCT, Level: "lower", X: 10, Y: 2, Count: 25},
			}},
	}
	out := summarizeHeatmap(got, 50)
	var lines []string
	seen := map[string]bool{}
	for _, z := range out.Zones {
		lines = append(lines, z.Place)
		if seen[z.Place+"|"+z.Side] {
			t.Errorf("%s appears twice in the zone list: %+v", z.Place, out.Zones)
		}
		seen[z.Place+"|"+z.Side] = true
	}
	if len(out.Zones) != 4 || out.Zones[0].Place != "Rafters" || out.Zones[0].Count != 126 || out.Zones[0].Cells != 3 {
		t.Fatalf("zones = %v, want Rafters once with its three cells summed (126), then three more places", lines)
	}
	if out.Zones[1].Place != "Ramp (piso inferior)" || out.Zones[1].Callout != "Ramp" || out.Zones[1].Level != "lower" || out.Zones[2].Place != "Bombsite A" || out.Zones[3].Place != "Ramp" {
		t.Fatalf("zones = %v, want the lower Ramp told apart from the upper one", lines)
	}
	if out.Cells[0].Place != "Rafters" || out.Cells[4].Place != "Bombsite A" {
		t.Fatalf("cells = %+v, want each cell to carry its place", out.Cells)
	}

	// With one floor in the answer nothing needs telling apart.
	got.Heatmap.Sides = got.Heatmap.Sides[:1]
	got.Heatmap.Cells = got.Heatmap.Cells[:5]
	if only := summarizeHeatmap(got, 50); len(only.Zones) != 3 || only.Zones[1].Place != "Bombsite A" || only.Zones[2].Place != "Ramp" {
		t.Fatalf("zones = %+v, want plain names on one floor", only.Zones)
	}

	// A cell no callout names is a place of its own, as the list names it.
	bare := summarizeHeatmap(tuiclient.SpatialHeatmap{Heatmap: &tacticalplan.Heatmap{CellSize: 64,
		Sides: []tacticalplan.HeatmapSide{{Side: tacticalplan.SideT, Level: "default", Total: 5}},
		Cells: []tacticalplan.HeatmapCell{{Side: tacticalplan.SideT, Level: "default", X: 1, Y: 2, Count: 3}, {Side: tacticalplan.SideT, Level: "default", X: 3, Y: 4, Count: 2}}}}, 50)
	if len(bare.Zones) != 2 || bare.Zones[0].Place != "Celda 1, 2" || bare.Zones[1].Place != "Celda 3, 4" {
		t.Fatalf("zones = %+v, want one place per unnamed cell", bare.Zones)
	}
}
