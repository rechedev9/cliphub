package main

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/prolineups"
)

func clipExample(match string, round, tick int, daysAgo int) prolineups.Example {
	return prolineups.Example{Match: match, MapKey: match + "/a-vs-b-m1-mirage.dem", Round: round, Tick: tick, SteamID64: "76561198000000001",
		AirTime: 1.5, PlayedAt: time.Now().AddDate(0, 0, -daysAgo)}
}

func TestPlanClipsTakesTheNewestRecordableThrowOfEachLineup(t *testing.T) {
	newest, older := clipExample("10", 3, 20000, 1), clipExample("11", 5, 30000, 3)
	recorded := clipExample("12", 2, 9000, 2)
	near := clipExample("11", 5, 30100, 3)
	lineups := []prolineups.Lineup{
		// The newest throw's demo is gone, so the next one is planned.
		{ID: "a", Examples: []prolineups.Example{newest, older}},
		// A lineup with a recorded clip needs nothing.
		{ID: "b", Examples: []prolineups.Example{recorded, clipExample("10", 4, 40000, 1)}},
		// Its window overlaps a's in the same demo, so it waits for a later run.
		{ID: "c", Examples: []prolineups.Example{near}},
	}
	sources := map[string]throwSource{}
	for _, e := range []prolineups.Example{newest, older, recorded, near} {
		sources[prolineups.ClipName(e)] = throwSource{file: e.Match + "__a-vs-b-m1-mirage", tickRate: 64, mapName: "de_mirage", duration: 100000}
	}
	plan := planClips(clipPlanInput{lineups: lineups, sources: sources, max: 10,
		demo:     func(file string) (string, bool) { return "/demos/" + file + ".dem", file == "11__a-vs-b-m1-mirage" },
		recorded: func(name string) bool { return name == prolineups.ClipName(recorded) }})
	if len(plan.Demos) != 1 || len(plan.Demos[0].Clips) != 1 {
		t.Fatalf("plan = %+v", plan)
	}
	d := plan.Demos[0]
	c := d.Clips[0]
	if d.Demo != "/demos/11__a-vs-b-m1-mirage.dem" || d.Map != "de_mirage" || d.TickRate != 64 {
		t.Fatalf("demo = %+v", d)
	}
	if c.Name != prolineups.ClipName(older) || c.StartTick != 30000-192 || c.EndTick != 30000+224 || c.SteamID64 != older.SteamID64 || c.Round != 5 {
		t.Fatalf("clip = %+v", c)
	}
}

func TestPlanClipsWithATeamTakesOnlyThatTeamsThrows(t *testing.T) {
	theirs, ours := clipExample("10", 3, 20000, 1), clipExample("10", 5, 40000, 2)
	theirs.Team, ours.Team = "Vitality", "FURIA"
	other := clipExample("10", 7, 60000, 1)
	other.Team = "Vitality"
	lineups := []prolineups.Lineup{{ID: "a", Examples: []prolineups.Example{theirs, ours}}, {ID: "b", Examples: []prolineups.Example{other}}}
	sources := map[string]throwSource{}
	for _, e := range []prolineups.Example{theirs, ours, other} {
		sources[prolineups.ClipName(e)] = throwSource{file: "10__a-vs-b-m1-mirage", tickRate: 64, mapName: "de_mirage", duration: 100000}
	}
	plan := planClips(clipPlanInput{lineups: lineups, sources: sources, max: 10, team: "furia",
		demo:     func(file string) (string, bool) { return "/demos/" + file + ".dem", true },
		recorded: func(string) bool { return false }})
	if len(plan.Demos) != 1 || len(plan.Demos[0].Clips) != 1 || plan.Demos[0].Clips[0].Name != prolineups.ClipName(ours) {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestPlanClipsSkipsAThrowTheRecorderCouldNotFilm(t *testing.T) {
	bad, good := clipExample("10", 3, 20000, 1), clipExample("10", 5, 40000, 2)
	sources := map[string]throwSource{}
	for _, e := range []prolineups.Example{bad, good} {
		sources[prolineups.ClipName(e)] = throwSource{file: "10__a-vs-b-m1-mirage", tickRate: 64, mapName: "de_mirage", duration: 100000}
	}
	plan := planClips(clipPlanInput{lineups: []prolineups.Lineup{{ID: "a", Examples: []prolineups.Example{bad, good}}}, sources: sources, max: 10,
		demo:     func(file string) (string, bool) { return "/demos/" + file + ".dem", true },
		recorded: func(string) bool { return false },
		failed:   func(name string) bool { return name == prolineups.ClipName(bad) }})
	if len(plan.Demos) != 1 || plan.Demos[0].Clips[0].Name != prolineups.ClipName(good) {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestProClipServesRecordedFilesOnly(t *testing.T) {
	a, h := testAPI(t)
	a.clipsDir = t.TempDir()
	name := "10/a-vs-b-m1-mirage-r3-t20000-76561198000000001.mp4"
	if err := os.MkdirAll(filepath.Join(a.clipsDir, "10"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.clipsDir, filepath.FromSlash(name)), []byte("mp4"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rec := do(t, h, http.MethodGet, "/v1/pro-clips/"+name, nil); rec.Code != http.StatusOK || rec.Body.String() != "mp4" || rec.Header().Get("Content-Type") != "video/mp4" {
		t.Fatalf("clip status %d type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	for _, bad := range []string{"10/missing-r1-t1-76561198000000001.mp4", "10/..%2Fsecret.mp4", "x/a-r1-t1-76561198000000001.mp4", "10/a.txt"} {
		if rec := do(t, h, http.MethodGet, "/v1/pro-clips/"+bad, nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s answered %d", bad, rec.Code)
		}
	}
}

func TestPlanClipsRecordsEachComboThrowAndListsTheCombo(t *testing.T) {
	first, second := clipExample("20", 4, 50000, 1), clipExample("20", 4, 50600, 1)
	second.SteamID64 = "76561198000000002"
	combo := prolineups.Example{Match: "20", MapKey: first.MapKey, Round: 4, Tick: first.Tick, SteamID64: first.SteamID64, PlayedAt: first.PlayedAt,
		Throws: []prolineups.ThrowRef{{Lineup: "a", SteamID64: first.SteamID64, Tick: first.Tick, AirTime: 1.5}, {Lineup: "b", SteamID64: second.SteamID64, Tick: second.Tick, AirTime: 1.5}}}
	sources := map[string]throwSource{}
	for _, e := range []prolineups.Example{first, second} {
		sources[prolineups.ClipName(e)] = throwSource{file: "20__a-vs-b-m1-mirage", tickRate: 64, mapName: "de_mirage", duration: 100000}
	}
	recordedFirst := prolineups.ClipName(first)
	plan := planClips(clipPlanInput{combos: []prolineups.Combo{{ID: "a+b", Examples: []prolineups.Example{combo}}}, sources: sources, max: 10,
		demo:     func(file string) (string, bool) { return "/demos/" + file + ".dem", true },
		recorded: func(name string) bool { return name == recordedFirst }})
	if len(plan.Demos) != 1 || len(plan.Demos[0].Clips) != 1 || plan.Demos[0].Clips[0].Name != prolineups.ClipName(second) {
		t.Fatalf("plan should record only the missing throw: %+v", plan.Demos)
	}
	if len(plan.Combos) != 1 || plan.Combos[0].Name != prolineups.ComboClipName(combo) ||
		!slices.Equal(plan.Combos[0].Parts, []string{recordedFirst, prolineups.ClipName(second)}) {
		t.Fatalf("combos = %+v", plan.Combos)
	}
}

func TestReadClipPicksMapsEachMapToItsTeam(t *testing.T) {
	path := filepath.Join(t.TempDir(), "best-maps.json")
	body := `[{"map":"de_mirage","team_id":9565,"team":"Vitality","maps":12,"win_rate":0.833},{"map":"","team":"x"}]`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	picks, err := readClipPicks(path)
	if err != nil || len(picks) != 1 || picks["de_mirage"] != "Vitality" {
		t.Fatalf("picks = %v, %v", picks, err)
	}
	if err := os.WriteFile(path, []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readClipPicks(path); err == nil {
		t.Fatal("an empty pick list must fail")
	}
}
