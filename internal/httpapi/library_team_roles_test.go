package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

func TestLibraryTeamRolesSuggestThenSave(t *testing.T) {
	env := newLibraryTestEnv(t)
	m := env.seedLibraryMatch("roles", "Vitality", "MOUZ")
	base := "/api/library/teams/" + teamOfMatch(t, m, "ct-start").TeamID.String() + "/roles"

	var got teamRolesResponse
	env.json(http.MethodGet, base+"?map=de_mirage", nil, http.StatusOK, &got)
	if got.TeamName != "Vitality" || got.Matches != 1 || len(got.Sheet.T) != 0 || got.Sheet.Map != "de_mirage" || len(got.Suggested.T) == 0 {
		t.Fatalf("roles = %+v", got)
	}
	if s := got.Suggested.T[0]; s.SteamID64 == "" || s.Player == "" || s.Role == "" {
		t.Fatalf("T suggestion = %+v", s)
	}
	// The fixture map has no callouts, so no CT player has a setup spot.
	if got.Suggested.CT == nil || len(got.Suggested.CT) != 0 {
		t.Fatalf("CT suggestions = %+v, want an empty list", got.Suggested.CT)
	}

	sheet := map[string]any{"map": "de_mirage", "t": []map[string]any{{"name": " Entry ", "steamid64": got.Suggested.T[0].SteamID64, "task": "abre B"}},
		"ct": []map[string]any{{"name": "Ancla", "callout": "BombsiteB"}}}
	var saved library.TeamRoleSheet
	env.json(http.MethodPut, base, sheet, http.StatusOK, &saved)
	if len(saved.T) != 1 || saved.T[0].Name != "Entry" || saved.CT[0].Callout != "BombsiteB" || saved.UpdatedAt.IsZero() {
		t.Fatalf("saved = %+v", saved)
	}
	env.json(http.MethodGet, base+"?map=de_mirage", nil, http.StatusOK, &got)
	if len(got.Sheet.T) != 1 || got.Sheet.T[0].Task != "abre B" {
		t.Fatalf("sheet after save = %+v", got.Sheet)
	}
	six := strings.Repeat(`{"name":"x"},`, 5) + `{"name":"x"}`
	for _, bad := range []string{`{"map":"Mirage!","t":[],"ct":[]}`, `{"map":"de_mirage","t":[` + six + `],"ct":[]}`,
		`{"map":"de_mirage","t":[{"name":"x","steamid64":"abc"}],"ct":[]}`, `{"map":"de_mirage","t":[{"name":""}],"ct":[]}`} {
		if rw := env.do(http.MethodPut, base, []byte(bad), "application/json"); rw.Code != http.StatusBadRequest {
			t.Fatalf("PUT %s = %d, want 400", bad, rw.Code)
		}
	}
	missing := "/api/library/teams/" + uuid.NewString() + "/roles"
	if rw := env.do(http.MethodPut, missing, []byte(`{"map":"de_mirage","t":[],"ct":[]}`), "application/json"); rw.Code != http.StatusNotFound {
		t.Fatalf("PUT missing team = %d, want 404", rw.Code)
	}
	if rw := env.do(http.MethodGet, base+"?map=de_mirage&side=T", nil, ""); rw.Code != http.StatusBadRequest {
		t.Fatalf("GET with side = %d, want 400", rw.Code)
	}
}

func TestLibraryPlayRolesPatch(t *testing.T) {
	env := newPlaysTestEnv(t)
	var p library.Play
	env.json(http.MethodPost, "/api/library/plays", map[string]any{"title": "Exec A", "map": "de_mirage", "side": "T", "kind": "execute", "origin": "manual"}, http.StatusCreated, &p)
	if p.Roles == nil || len(p.Roles) != 0 {
		t.Fatalf("new play roles = %+v, want an empty list", p.Roles)
	}
	env.json(http.MethodPatch, "/api/library/plays/"+p.ID.String(), map[string]any{"roles": []map[string]any{{"name": "Entry", "callout": "Palace", "task": "primero"}}}, http.StatusOK, &p)
	if len(p.Roles) != 1 || p.Roles[0].Callout != "Palace" || p.Roles[0].Task != "primero" {
		t.Fatalf("patched roles = %+v", p.Roles)
	}
	if rw := env.do(http.MethodPatch, "/api/library/plays/"+p.ID.String(), []byte(`{"roles":[{"name":""}]}`), "application/json"); rw.Code != http.StatusBadRequest {
		t.Fatalf("PATCH empty role name = %d, want 400", rw.Code)
	}
}
