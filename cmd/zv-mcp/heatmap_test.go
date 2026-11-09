package main

import (
	"net/url"
	"testing"

	"github.com/google/uuid"

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
	} {
		if _, err := bad.query(); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestTeamHeatmapPathCarriesTheTeamScope(t *testing.T) {
	team := uuid.NewString()
	in := teamHeatmapIn{TeamID: team, analysisFilterIn: analysisFilterIn{Side: "ct", From: "2026-10-01"}, Buy: "full",
		Players: []string{"76561198000000001"}, heatmapViewIn: heatmapViewIn{Kind: "death", ClockTo: 55}}
	heat, err := in.teamValues()
	if err != nil {
		t.Fatal(err)
	}
	if heat.Get("player") != "76561198000000001" || heat.Get("buy") != "full" || heat.Get("kind") != "death" {
		t.Fatalf("team query = %s", heat.Encode())
	}
	want := "/equipos/" + team + "?buy=full&cuenta=death&forma=calor&from=2026-10-01&jugador=76561198000000001&map=de_nuke&side=CT&tab=replay&tramo=0-55"
	if got := teamHeatmapPath(in, "de_nuke", heat); got != want {
		t.Fatalf("path = %s, want %s", got, want)
	}
	if _, err := (teamHeatmapIn{Buy: "lots"}).teamValues(); err == nil {
		t.Fatal("an unknown buy: no error")
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
