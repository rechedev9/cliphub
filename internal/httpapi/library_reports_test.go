package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

type scoutFixture struct {
	env          libraryTestEnv
	home, away   library.Match
	other        library.Match
	vitality     uuid.UUID
	mouz, spirit uuid.UUID
}

// newScoutFixture seeds Vitality on the CT-start side of one match and on the
// T-start side of another, plus a match Vitality did not play.
func newScoutFixture(t *testing.T) scoutFixture {
	env := newLibraryTestEnv(t)
	f := scoutFixture{env: env}
	f.home = env.seedLibraryMatch("home", "Vitality", "MOUZ")
	f.away = env.seedLibraryMatch("away", "Spirit", "Vitality")
	f.other = env.seedLibraryMatch("other", "NAVI", "G2")
	f.vitality = *teamOfMatch(t, f.home, "ct-start").TeamID
	if got := teamOfMatch(t, f.away, "t-start").TeamID; got == nil || *got != f.vitality {
		t.Fatalf("away match T-start team = %v, want Vitality %s through its alias", got, f.vitality)
	}
	f.mouz = *teamOfMatch(t, f.home, "t-start").TeamID
	f.spirit = *teamOfMatch(t, f.away, "ct-start").TeamID
	return f
}

func (f scoutFixture) scout(query string) scoutResponse {
	f.env.t.Helper()
	var resp scoutResponse
	f.env.json(http.MethodGet, "/api/library/teams/"+f.vitality.String()+"/scout"+query, nil, http.StatusOK, &resp)
	return resp
}

func TestLibraryTeamScoutAggregatesTheTeamsMatches(t *testing.T) {
	f := newScoutFixture(t)

	all := f.scout("")
	if all.TeamName != "Vitality" || len(all.Matches) != 2 || all.Tendencies.MatchCount != 2 {
		t.Fatalf("scout = %s with %d matches, want Vitality over its 2 matches", all.TeamName, len(all.Matches))
	}
	if all.Tendencies.RoundCount != 4 {
		t.Fatalf("rounds = %d, want the 2 rounds of each match", all.Tendencies.RoundCount)
	}
	keys := map[uuid.UUID]string{}
	for _, m := range all.Matches {
		keys[m.MatchID] = m.TeamKey
	}
	if keys[f.home.ID] != "ct-start" || keys[f.away.ID] != "t-start" {
		t.Fatalf("team keys = %v, want Vitality read from its own side of each match", keys)
	}
	for _, m := range all.Matches {
		if m.MatchID == f.away.ID && (m.Opponent != "Spirit" || m.OpponentID != f.spirit.String()) {
			t.Fatalf("away rival = %+v, want Spirit", m)
		}
	}
	// The fixture's first round is a pistol: every example names its match.
	for _, row := range all.Pistols.Rows {
		for _, e := range row.Examples {
			if e.MatchID != f.home.ID && e.MatchID != f.away.ID {
				t.Fatalf("pistol evidence %+v names a match outside the scout", e)
			}
		}
	}

	if ct := f.scout("?side=ct"); ct.Tendencies.RoundCount != 2 || ct.Filter.Side != tacticalplan.SideCT {
		t.Fatalf("CT scout = %d rounds (side %q), want the 2 first-half rounds of the home match", ct.Tendencies.RoundCount, ct.Filter.Side)
	}
	if rival := f.scout("?opponent_id=" + f.mouz.String()); len(rival.Matches) != 1 || rival.Matches[0].MatchID != f.home.ID {
		t.Fatalf("scout against MOUZ = %+v, want the home match", rival.Matches)
	}
	if none := f.scout("?map=de_nuke"); len(none.Matches) != 0 || none.Tendencies.RoundCount != 0 {
		t.Fatalf("nuke scout = %+v, want no matches", none.Matches)
	}

	missing := uuid.New()
	picked := f.scout("?match_id=" + f.home.ID.String() + "&match_id=" + f.other.ID.String() + "&match_id=" + missing.String())
	if len(picked.Matches) != 1 || len(picked.Skipped) != 2 {
		t.Fatalf("picked scout = %d matches, skipped %+v; want 1 and 2 skipped", len(picked.Matches), picked.Skipped)
	}
	reasons := map[uuid.UUID]string{}
	for _, s := range picked.Skipped {
		reasons[s.MatchID] = s.Reason
	}
	if reasons[f.other.ID] != scoutSkipTeamMissing || reasons[missing] != scoutSkipNotFound {
		t.Fatalf("skip reasons = %v", reasons)
	}

	for _, bad := range []string{"?side=mid", "?map=Mirage!", "?match_id=nope", "?from=yesterday"} {
		if rw := f.env.do(http.MethodGet, "/api/library/teams/"+f.vitality.String()+"/scout"+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Fatalf("scout%s = %d, want 400", bad, rw.Code)
		}
	}
	if rw := f.env.do(http.MethodGet, "/api/library/teams/"+uuid.NewString()+"/scout", nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("scout of an unknown team = %d, want 404", rw.Code)
	}
}

func TestLibraryReportsLifecycle(t *testing.T) {
	f := newScoutFixture(t)
	env := f.env

	var report library.Report
	env.json(http.MethodPost, "/api/library/reports", map[string]any{
		"title": "  Vitality   antes de la final ", "team_id": f.vitality,
		"match_ids": []uuid.UUID{f.home.ID, f.away.ID}, "side": "t",
	}, http.StatusCreated, &report)
	if report.Title != "Vitality antes de la final" || report.Side != "T" || report.TeamName != "Vitality" || len(report.MatchIDs) != 2 {
		t.Fatalf("created report = %+v", report)
	}

	rw := env.do(http.MethodPost, "/api/library/reports", mustJSON(t, map[string]any{
		"title": "x", "team_id": f.vitality, "match_ids": []uuid.UUID{f.other.ID},
	}), "application/json")
	if rw.Code != http.StatusBadRequest || errorCode(t, rw.Body.Bytes()) != scoutSkipTeamMissing {
		t.Fatalf("report with a match the team did not play = %d %s, want 400 %s", rw.Code, rw.Body.String(), scoutSkipTeamMissing)
	}
	rw = env.do(http.MethodPost, "/api/library/reports", mustJSON(t, map[string]any{"title": "x", "team_id": uuid.New()}), "application/json")
	if rw.Code != http.StatusNotFound {
		t.Fatalf("report on an unknown team = %d, want 404", rw.Code)
	}
	rw = env.do(http.MethodPost, "/api/library/reports", mustJSON(t, map[string]any{"title": " ", "team_id": f.vitality}), "application/json")
	if rw.Code != http.StatusBadRequest {
		t.Fatalf("report without a title = %d, want 400", rw.Code)
	}

	var list struct {
		Reports []library.Report `json:"reports"`
	}
	env.json(http.MethodGet, "/api/library/reports?team_id="+f.vitality.String(), nil, http.StatusOK, &list)
	if len(list.Reports) != 1 || list.Reports[0].ID != report.ID {
		t.Fatalf("reports = %+v, want the one created", list.Reports)
	}

	var scout scoutResponse
	env.json(http.MethodGet, "/api/library/reports/"+report.ID.String()+"/scout", nil, http.StatusOK, &scout)
	// Vitality plays T only in the away match's first half.
	if len(scout.Matches) != 2 || scout.Tendencies.RoundCount != 2 || scout.Filter.Side != tacticalplan.SideT {
		t.Fatalf("report scout = %d matches, %d rounds, side %q; want 2, 2, T", len(scout.Matches), scout.Tendencies.RoundCount, scout.Filter.Side)
	}

	var patched library.Report
	env.json(http.MethodPatch, "/api/library/reports/"+report.ID.String(), map[string]any{
		"notes": "Fuerzan tras perder el pistol.", "side": "", "match_ids": []uuid.UUID{f.home.ID},
	}, http.StatusOK, &patched)
	if patched.Notes != "Fuerzan tras perder el pistol." || patched.Side != "" || len(patched.MatchIDs) != 1 {
		t.Fatalf("patched report = %+v", patched)
	}
	rw = env.do(http.MethodPatch, "/api/library/reports/"+report.ID.String(), mustJSON(t, map[string]any{"team_id": f.mouz}), "application/json")
	if rw.Code != http.StatusBadRequest {
		t.Fatalf("changing the report's team = %d, want 400", rw.Code)
	}

	if rw := env.do(http.MethodDelete, "/api/library/reports/"+report.ID.String(), nil, ""); rw.Code != http.StatusNoContent {
		t.Fatalf("delete report = %d, want 204", rw.Code)
	}
	if rw := env.do(http.MethodGet, "/api/library/reports/"+report.ID.String(), nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("deleted report = %d, want 404", rw.Code)
	}
	if rw := env.do(http.MethodDelete, "/api/library/reports/"+report.ID.String(), nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("second delete = %d, want 404", rw.Code)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func errorCode(t *testing.T, body []byte) string {
	t.Helper()
	var e struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		t.Fatalf("decode error body %s: %v", body, err)
	}
	return e.Code
}
