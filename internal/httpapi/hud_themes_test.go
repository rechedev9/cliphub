package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/hudthemes"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/store"
)

func TestHUDThemesDuplicateEditDeleteAndPreview(t *testing.T) {
	env := newLibraryTestEnv(t)
	env.h.hudThemes = store.NewMemoryHUDThemeRepository()
	var list hudThemesBody
	env.json(http.MethodGet, "/api/full-demo/hud-themes", nil, http.StatusOK, &list)
	if list.Themes == nil || len(list.Themes) != 0 || list.Max != hudthemes.MaxThemes {
		t.Fatalf("empty list = %+v", list)
	}
	var created hudthemes.Theme
	env.json(http.MethodPost, "/api/full-demo/hud-themes", map[string]any{"base": "arena", "name": "  Mío ", "ct": "#42c9e8", "t": "FFB32C", "background": "101927", "radius": 10}, http.StatusCreated, &created)
	if created.Name != "Mío" || created.CT != "42C9E8" || created.Base != "arena" || created.Radius == nil || *created.Radius != 10 {
		t.Fatalf("created = %+v; want a cleaned copy of arena", created)
	}
	for _, bad := range []string{
		`{"base":"nope","name":"a","ct":"000000","t":"000000","background":"000000"}`,
		`{"base":"arena","name":"","ct":"000000","t":"000000","background":"000000"}`,
		`{"base":"arena","name":"a","ct":"00000","t":"000000","background":"000000"}`,
		`{"base":"arena","name":"a","ct":"000000","t":"000000","background":"000000","radius":16}`,
	} {
		if rw := env.do(http.MethodPost, "/api/full-demo/hud-themes", []byte(bad), "application/json"); rw.Code != http.StatusBadRequest {
			t.Fatalf("POST %s = %d, want 400", bad, rw.Code)
		}
	}
	var updated hudthemes.Theme
	env.json(http.MethodPut, "/api/full-demo/hud-themes/"+created.ID.String(), map[string]any{"name": "Otro", "ct": "111111", "t": "222222", "background": "333333"}, http.StatusOK, &updated)
	if updated.Name != "Otro" || updated.Radius != nil || updated.Base != "arena" {
		t.Fatalf("updated = %+v", updated)
	}
	for range hudthemes.MaxThemes - 1 {
		env.json(http.MethodPost, "/api/full-demo/hud-themes", map[string]any{"base": "apex", "name": "x", "ct": "111111", "t": "222222", "background": "333333"}, http.StatusCreated, nil)
	}
	if rw := env.do(http.MethodPost, "/api/full-demo/hud-themes", []byte(`{"base":"apex","name":"x","ct":"111111","t":"222222","background":"333333"}`), "application/json"); rw.Code != http.StatusConflict || !strings.Contains(rw.Body.String(), "hud_theme_limit") {
		t.Fatalf("eleventh theme = %d %s, want 409 hud_theme_limit", rw.Code, rw.Body.String())
	}
	env.json(http.MethodDelete, "/api/full-demo/hud-themes/"+created.ID.String(), nil, http.StatusNoContent, nil)
	env.json(http.MethodDelete, "/api/full-demo/hud-themes/"+created.ID.String(), nil, http.StatusNotFound, nil)

	rw := httptest.NewRecorder()
	env.router.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/full-demo/hud-themes/preview?base=arena&ct=112233&t=445566&background=000000&radius=4&compact_header=1&round_win_banner=1&t_name=Spirit", nil))
	body := rw.Body.String()
	if rw.Code != http.StatusOK || rw.Header().Get("Content-Type") != "image/svg+xml" || !strings.Contains(body, "#112233") || !strings.Contains(body, "SPIRIT") || !strings.Contains(body, "WINS THE ROUND") {
		t.Fatalf("preview = %d %s, %.200s", rw.Code, rw.Header().Get("Content-Type"), body)
	}
	if rw := env.do(http.MethodGet, "/api/full-demo/hud-themes/preview?base=arena&ct=zz&t=445566&background=000000", nil, ""); rw.Code != http.StatusBadRequest {
		t.Fatalf("bad preview colour = %d, want 400", rw.Code)
	}
}

func TestTeamNamesComeFromTheLibraryMatchBySide(t *testing.T) {
	got := teamNamesFromMatch(library.Match{Teams: []library.MatchTeam{
		{StartSide: "CT", ClanName: "Team\x00Vitality", TeamName: ""},
		{StartSide: "T", ClanName: "spirit clan", TeamName: "Team Spirit"},
	}})
	if got.CTStart != "TeamVitality" || got.TStart != "Team Spirit" {
		t.Fatalf("team names = %+v; want the assigned team, then the cleaned clan name", got)
	}
}
