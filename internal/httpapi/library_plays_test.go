package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"testing/fstest"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/lineups"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// newPlaysTestEnv is the library test env with a one-smoke lineup catalog.
func newPlaysTestEnv(t *testing.T) libraryTestEnv {
	t.Helper()
	catalog, err := lineups.LoadFS(fstest.MapFS{"de_mirage.smokes.json": {Data: []byte(
		`{"map":"de_mirage","smokes":[{"id":"mirage-window","destination":"Window","landing":[500,500,0],"landing_radius":40}]}`,
	)}})
	if err != nil {
		t.Fatal(err)
	}
	repo := store.NewMemoryLibraryRepository()
	files := newFakeStorage()
	queue := &fakeQueue{}
	h := NewHandlers(newFakeRepo(), files, queue, WithLibrary(repo), WithLineupCatalog(catalog))
	return libraryTestEnv{t: t, repo: repo, storage: files, queue: queue, router: Routes(h)}
}

// seedPlayMatch seeds a match whose first round has a CT smoke on the
// catalog's Window lineup and a CT flash.
func seedPlayMatch(env libraryTestEnv) library.Match {
	env.t.Helper()
	m := env.seedLibraryMatch("plays", "Vitality", "MOUZ")
	key := artifacts.LibraryTacticalIndexKey(m.DemoID)
	rc, err := env.storage.Open(key)
	if err != nil {
		env.t.Fatal(err)
	}
	var doc tacticalplan.Document
	if err := json.NewDecoder(rc).Decode(&doc); err != nil {
		env.t.Fatal(err)
	}
	_ = rc.Close()
	slot := uint8(0)
	doc.Rounds[0].Events = []tacticalplan.Event{
		{Tick: 132, Kind: tacticalplan.EventSmoke, ActorSlot: &slot, Side: tacticalplan.SideCT, Pos: [3]float64{510, 500, 0}},
		{Tick: 150, Kind: tacticalplan.EventFlash, ActorSlot: &slot, Side: tacticalplan.SideCT, Pos: [3]float64{0, 0, 0}},
		{Tick: 150, Kind: tacticalplan.EventFlash, ActorSlot: &slot, Side: tacticalplan.SideCT, Pos: [3]float64{0, 0, 0}},
	}
	b, _ := json.Marshal(doc)
	if err := env.storage.Put(key, bytes.NewReader(b)); err != nil {
		env.t.Fatal(err)
	}
	return m
}

func TestLibraryRoundPlayDraftAndLineups(t *testing.T) {
	env := newPlaysTestEnv(t)
	m := seedPlayMatch(env)
	base := "/api/library/matches/" + m.ID.String() + "/tactical/rounds/1"

	var draft struct {
		Draft library.Play `json:"draft"`
	}
	env.json(http.MethodGet, base+"/play?team_key=ct-start&tick=116", nil, http.StatusOK, &draft)
	d := draft.Draft
	if d.Side != "CT" || d.Kind != "hold" || d.Map != "de_mirage" || d.TeamName != "Vitality" || d.Origin != library.PlayOriginRound {
		t.Fatalf("draft = %+v", d)
	}
	if len(d.Grenades) != 2 || d.Grenades[0].LineupID != "mirage-window" || d.Grenades[0].Player != "one" || d.Grenades[1].LineupID != "" {
		t.Fatalf("draft grenades = %+v, want the smoke on the Window lineup", d.Grenades)
	}
	if len(d.Positions) != 1 || d.Positions[0].Pos[0] != 40 || d.PositionsSeconds != 0.3 {
		t.Fatalf("draft positions = %+v at %v s, want slot 0 at the tick 116 frame", d.Positions, d.PositionsSeconds)
	}
	if len(d.Examples) != 1 || d.Examples[0] != (tacticalplan.Evidence{MatchID: m.ID, Round: 1, Tick: 116}) {
		t.Fatalf("draft examples = %+v", d.Examples)
	}
	for _, bad := range []string{"/play?team_key=spectators", "/play?team_key=ct-start&tick=99999"} {
		if rw := env.do(http.MethodGet, base+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Fatalf("GET %s = %d, want 400", bad, rw.Code)
		}
	}
	if rw := env.do(http.MethodGet, "/api/library/matches/"+m.ID.String()+"/tactical/rounds/9/play?team_key=ct-start", nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("draft of a missing round = %d, want 404", rw.Code)
	}

	var cmp roundLineupsResponse
	env.json(http.MethodGet, base+"/lineups", nil, http.StatusOK, &cmp)
	if cmp.CatalogSize != 1 || cmp.Smokes != 1 || cmp.Matched != 1 || cmp.ThrowChecked || len(cmp.Grenades) != 2 {
		t.Fatalf("lineups = %+v", cmp)
	}
	if cmp.Grenades[0].Status != lineupMatched || cmp.Grenades[0].Lineup.Destination != "Window" || cmp.Grenades[1].Status != lineupNotCovered {
		t.Fatalf("lineup statuses = %+v", cmp.Grenades)
	}
}

func TestLibraryPlaysLifecycleExportAndImport(t *testing.T) {
	env := newPlaysTestEnv(t)
	m := seedPlayMatch(env)
	vitality := *teamOfMatch(t, m, "ct-start").TeamID
	example := []tacticalplan.Evidence{{MatchID: m.ID, Round: 1, Tick: 116}}

	var play library.Play
	env.json(http.MethodPost, "/api/library/plays", map[string]any{
		"title": " Humo a ventana ", "team_id": vitality, "map": "de_mirage", "side": "ct", "kind": "hold", "site": "mid",
		"grenades": []tacticalplan.PlayGrenade{{Kind: tacticalplan.EventSmoke, Seconds: 0.5, Pos: [3]float64{510, 500, 0}}},
		"examples": example,
	}, http.StatusCreated, &play)
	if play.Title != "Humo a ventana" || play.Side != "CT" || play.TeamName != "Vitality" || play.Origin != library.PlayOriginRound {
		t.Fatalf("created play = %+v", play)
	}
	for name, body := range map[string]map[string]any{
		"map_mismatch":  {"title": "x", "map": "de_nuke", "side": "CT", "kind": "hold", "examples": example},
		"unknown_match": {"title": "x", "map": "de_mirage", "side": "CT", "kind": "hold", "examples": []tacticalplan.Evidence{{MatchID: uuid.New(), Round: 1}}},
	} {
		raw, _ := json.Marshal(body)
		rw := env.do(http.MethodPost, "/api/library/plays", raw, "application/json")
		var e struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(rw.Body.Bytes(), &e)
		if rw.Code != http.StatusBadRequest || e.Code != name {
			t.Fatalf("POST with %s = %d %s, want 400 with that code", name, rw.Code, rw.Body.String())
		}
	}
	if rw := env.do(http.MethodPost, "/api/library/plays", []byte(`{"title":"x","map":"de_mirage","side":"T","kind":"hold"}`), "application/json"); rw.Code != http.StatusBadRequest {
		t.Fatalf("CT kind on a T play = %d, want 400", rw.Code)
	}

	var list struct {
		Plays []library.Play `json:"plays"`
	}
	env.json(http.MethodGet, "/api/library/plays?map=de_mirage&side=CT&team_id="+vitality.String(), nil, http.StatusOK, &list)
	if len(list.Plays) != 1 || list.Plays[0].ID != play.ID {
		t.Fatalf("list = %+v", list.Plays)
	}
	env.json(http.MethodGet, "/api/library/plays?kind=execute", nil, http.StatusOK, &list)
	if len(list.Plays) != 0 {
		t.Fatalf("execute list = %+v, want none", list.Plays)
	}

	var patched library.Play
	env.json(http.MethodPatch, "/api/library/plays/"+play.ID.String(), map[string]any{"notes": "antes del default", "team_id": nil}, http.StatusOK, &patched)
	if patched.Notes != "antes del default" || patched.TeamID != nil || len(patched.Grenades) != 1 {
		t.Fatalf("patched = %+v", patched)
	}
	if rw := env.do(http.MethodPatch, "/api/library/plays/"+play.ID.String(), []byte(`{"map":"de_nuke"}`), "application/json"); rw.Code != http.StatusBadRequest {
		t.Fatalf("PATCH map = %d, want 400", rw.Code)
	}
	env.json(http.MethodPatch, "/api/library/plays/"+play.ID.String(), map[string]any{"team_id": vitality}, http.StatusOK, &patched)

	var file stratbookFile
	env.json(http.MethodGet, "/api/library/plays/export", nil, http.StatusOK, &file)
	if file.Format != stratbookFormat || file.Version != 1 || len(file.Plays) != 1 || file.Plays[0].TeamName != "Vitality" {
		t.Fatalf("export = %+v", file)
	}

	var imported stratbookImportResponse
	env.json(http.MethodPost, "/api/library/plays/import", file, http.StatusOK, &imported)
	if imported.Imported != 0 || imported.Duplicates != 1 {
		t.Fatalf("reimport = %+v, want one duplicate", imported)
	}
	env.json(http.MethodDelete, "/api/library/plays/"+play.ID.String(), nil, http.StatusNoContent, nil)
	env.json(http.MethodDelete, "/api/library/plays/"+play.ID.String(), nil, http.StatusNotFound, nil)

	// The file on another PC: a new team name and an example match it does not have.
	other := file.Plays[0]
	other.ID = uuid.New()
	other.TeamName = "Team Falcons"
	other.Examples = []tacticalplan.Evidence{{MatchID: uuid.New(), Round: 4, Tick: 900}}
	bad := file.Plays[0]
	bad.ID, bad.Kind = uuid.New(), "execute"
	file.Plays = append(file.Plays, other, bad)
	env.json(http.MethodPost, "/api/library/plays/import", file, http.StatusOK, &imported)
	if imported.Imported != 2 || imported.Duplicates != 0 || len(imported.Rejected) != 1 || imported.Rejected[0].Index != 2 {
		t.Fatalf("import = %+v, want two plays in and the CT execute rejected", imported)
	}
	if imported.Plays[1].TeamName != "Team Falcons" || imported.Plays[1].Origin != library.PlayOriginImport {
		t.Fatalf("imported play = %+v, want its team created by name", imported.Plays[1])
	}
	var got playResponse
	env.json(http.MethodGet, "/api/library/plays/"+other.ID.String(), nil, http.StatusOK, &got)
	if len(got.MissingMatchIDs) != 1 || got.MissingMatchIDs[0] != other.Examples[0].MatchID {
		t.Fatalf("missing matches = %+v", got.MissingMatchIDs)
	}
	if rw := env.do(http.MethodPost, "/api/library/plays/import", []byte(`{"format":"other","version":1,"plays":[]}`), "application/json"); rw.Code != http.StatusBadRequest {
		t.Fatalf("import of another format = %d, want 400", rw.Code)
	}
}

func TestLibraryTeamPlayDetectionReadsTheTeamsMatches(t *testing.T) {
	env := newPlaysTestEnv(t)
	m := seedPlayMatch(env)
	vitality := *teamOfMatch(t, m, "ct-start").TeamID
	var det playDetectionResponse
	env.json(http.MethodGet, "/api/library/teams/"+vitality.String()+"/plays", nil, http.StatusOK, &det)
	if det.TeamName != "Vitality" || det.Matches != 1 || det.Rounds != 2 || det.Compared != 1 || len(det.Plays) != 0 {
		t.Fatalf("detection = %+v, want 2 rounds read, 1 with enough utility and no repeat", det)
	}
	if det.Thresholds.MinRounds != tacticalplan.PlayMinRounds || det.Thresholds.MinUtilitySimilarity != tacticalplan.PlayMinUtilitySimilarity {
		t.Fatalf("thresholds = %+v", det.Thresholds)
	}
	if rw := env.do(http.MethodGet, "/api/library/teams/"+vitality.String()+"/plays?side=mid", nil, ""); rw.Code != http.StatusBadRequest {
		t.Fatalf("detection with side=mid = %d, want 400", rw.Code)
	}
}
