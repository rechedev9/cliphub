package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestLibraryCallsCrud(t *testing.T) {
	env := newPlaysTestEnv(t)
	var call library.Call
	env.json(http.MethodPost, "/api/library/calls", map[string]any{"name": " Rotar ", "side": "ct", "meaning": "a B por mid", "tags": []string{"Ret"}}, http.StatusCreated, &call)
	if call.Name != "Rotar" || call.Side != "CT" || call.Status != library.StatusActive || len(call.Tags) != 1 {
		t.Fatalf("created call = %+v", call)
	}
	env.json(http.MethodPost, "/api/library/calls", map[string]any{"name": "Bait"}, http.StatusCreated, nil)
	for body, code := range map[string]int{`{"name":""}`: http.StatusBadRequest, `{"name":"x","side":"mid"}`: http.StatusBadRequest, `{"name":"x","id":"1"}`: http.StatusBadRequest} {
		if rw := env.do(http.MethodPost, "/api/library/calls", []byte(body), "application/json"); rw.Code != code {
			t.Fatalf("POST %s = %d, want %d", body, rw.Code, code)
		}
	}
	var listed struct {
		Calls []library.Call `json:"calls"`
	}
	for query, want := range map[string]int{"": 2, "?side=T": 1, "?side=CT": 2, "?tag=ret": 1} {
		env.json(http.MethodGet, "/api/library/calls"+query, nil, http.StatusOK, &listed)
		if len(listed.Calls) != want {
			t.Fatalf("GET calls%s = %d, want %d", query, len(listed.Calls), want)
		}
	}
	env.json(http.MethodPatch, "/api/library/calls/"+call.ID.String(), map[string]any{"status": "archived"}, http.StatusOK, &call)
	if call.Status != library.StatusArchived || call.Meaning != "a B por mid" {
		t.Fatalf("patched call = %+v", call)
	}
	env.json(http.MethodGet, "/api/library/calls", nil, http.StatusOK, &listed)
	if len(listed.Calls) != 1 {
		t.Fatalf("archived call still listed: %+v", listed.Calls)
	}
	env.json(http.MethodDelete, "/api/library/calls/"+call.ID.String(), nil, http.StatusNoContent, nil)
	env.json(http.MethodGet, "/api/library/calls/"+call.ID.String(), nil, http.StatusNotFound, nil)
}

// seedCalloutMatch gives the fixture match's geometry two places, one on two floors.
func seedCalloutMatch(env libraryTestEnv) library.Match {
	env.t.Helper()
	m := env.seedLibraryMatch("callouts", "Vitality", "MOUZ")
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
	doc.Geometry.Callouts = []tacticalplan.Callout{
		{Name: "BombsiteB", Level: "default", Center: [2]float64{1, 2}, Samples: 5},
		{Name: "BombsiteB", Level: "lower", Center: [2]float64{9, 9}, Samples: 1},
		{Name: "Apartments", Level: "default", Center: [2]float64{3, 4}, Samples: 3},
	}
	b, _ := json.Marshal(doc)
	if err := env.storage.Put(key, bytes.NewReader(b)); err != nil {
		env.t.Fatal(err)
	}
	return m
}

func TestLibraryMapCalloutsNameAndReset(t *testing.T) {
	env := newPlaysTestEnv(t)
	m := seedCalloutMatch(env)
	env.json(http.MethodPut, "/api/library/maps/de_mirage/callouts/BombsiteB", map[string]any{"name": " Bé "}, http.StatusOK, nil)
	env.json(http.MethodPut, "/api/library/maps/de_mirage/callouts/Kitchen", map[string]any{"name": "Cocina"}, http.StatusOK, nil)
	var got mapCalloutsResponse
	env.json(http.MethodGet, "/api/library/maps/de_mirage/callouts", nil, http.StatusOK, &got)
	if got.ReferenceMatchID == nil || *got.ReferenceMatchID != m.ID || got.Geometry != nil || len(got.Callouts) != 3 {
		t.Fatalf("callouts = %+v", got)
	}
	b := got.Callouts[1]
	if got.Callouts[0].Token != "Apartments" || b.Token != "BombsiteB" || b.Name != "Bé" || b.Center == nil || b.Center[0] != 1 || got.Callouts[2].Center != nil {
		t.Fatalf("callouts = %+v, want the most sampled floor and the name without a place last", got.Callouts)
	}
	env.json(http.MethodGet, "/api/library/maps/de_mirage/callouts?geometry=true", nil, http.StatusOK, &got)
	if got.Geometry == nil {
		t.Fatal("geometry=true returned no geometry")
	}
	env.json(http.MethodDelete, "/api/library/maps/de_mirage/callouts/BombsiteB", nil, http.StatusNoContent, nil)
	env.json(http.MethodDelete, "/api/library/maps/de_mirage/callouts/BombsiteB", nil, http.StatusNotFound, nil)
	if rw := env.do(http.MethodPut, "/api/library/maps/de_mirage/callouts/BombsiteB", []byte(`{"name":""}`), "application/json"); rw.Code != http.StatusBadRequest {
		t.Fatalf("PUT empty name = %d, want 400", rw.Code)
	}
	if rw := env.do(http.MethodGet, "/api/library/maps/Mirage!/callouts", nil, ""); rw.Code != http.StatusBadRequest {
		t.Fatalf("GET bad map = %d, want 400", rw.Code)
	}
	var empty mapCalloutsResponse
	env.json(http.MethodGet, "/api/library/maps/de_nuke/callouts", nil, http.StatusOK, &empty)
	if empty.ReferenceMatchID != nil || empty.Callouts == nil || len(empty.Callouts) != 0 {
		t.Fatalf("map without matches = %+v", empty)
	}
}

func TestLibretaFileCarriesEverySection(t *testing.T) {
	src := newPlaysTestEnv(t)
	m := src.seedLibraryMatch("file", "Vitality", "MOUZ")
	team := teamOfMatch(t, m, "ct-start").TeamID.String()
	src.json(http.MethodPost, "/api/library/plays", map[string]any{"title": "Exec A", "map": "de_mirage", "side": "T", "kind": "execute", "origin": "manual",
		"roles": []map[string]any{{"name": "Entry"}}}, http.StatusCreated, nil)
	src.json(http.MethodPost, "/api/library/lineups", map[string]any{"title": "Pop", "map": "de_nuke", "side": "T", "grenade": "flash",
		"origin": []float64{1, 2, 3}, "pitch": -10, "yaw": 90}, http.StatusCreated, nil)
	src.json(http.MethodPost, "/api/library/calls", map[string]any{"name": "Bait", "status": "archived"}, http.StatusCreated, nil)
	src.json(http.MethodPut, "/api/library/maps/de_mirage/callouts/BombsiteB", map[string]any{"name": "Bé"}, http.StatusOK, nil)
	src.json(http.MethodPut, "/api/library/teams/"+team+"/roles", map[string]any{"map": "de_mirage", "t": []map[string]any{{"name": "AWP"}}, "ct": []map[string]any{}}, http.StatusOK, nil)

	var file stratbookFile
	src.json(http.MethodGet, "/api/library/libreta/export", nil, http.StatusOK, &file)
	if file.Format != stratbookFormat || len(file.Plays) != 1 || len(file.Lineups) != 1 || len(file.Calls) != 1 || len(file.CalloutNames) != 1 ||
		len(file.TeamRoles) != 1 || file.TeamRoles[0].TeamName != "Vitality" {
		t.Fatalf("libreta file = %+v", file)
	}
	raw, _ := json.Marshal(file)

	dst := newPlaysTestEnv(t)
	var resp stratbookImportResponse
	dst.json(http.MethodPost, "/api/library/plays/import", json.RawMessage(raw), http.StatusOK, &resp)
	want := sectionImport{Imported: 1}
	for _, key := range []string{"lineups", "calls", "callout_names", "team_roles"} {
		if resp.Sections[key] != want {
			t.Fatalf("import %s = %+v, want one imported (all: %+v)", key, resp.Sections[key], resp.Sections)
		}
	}
	if resp.Imported != 1 || len(resp.Plays[0].Roles) != 1 {
		t.Fatalf("plays import = %+v", resp)
	}
	dst.json(http.MethodPost, "/api/library/plays/import", json.RawMessage(raw), http.StatusOK, &resp)
	want = sectionImport{Duplicates: 1}
	for _, key := range []string{"lineups", "calls", "callout_names", "team_roles"} {
		if resp.Sections[key] != want {
			t.Fatalf("reimport %s = %+v, want one duplicate", key, resp.Sections[key])
		}
	}
}
