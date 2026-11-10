package httpapi

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func skippedFor(skipped []scoutSkipped, reason string) int {
	n := 0
	for _, s := range skipped {
		if s.Reason == reason {
			n++
		}
	}
	return n
}

func TestTeamScopeFiltersByLevelWhoseMatchesAndMatchCount(t *testing.T) {
	f := newScoutFixture(t)
	profile := func(team string, body map[string]any) teamProfileResponse {
		t.Helper()
		var resp teamProfileResponse
		f.env.json(http.MethodPut, "/api/library/teams/"+team+"/profile", body, http.StatusOK, &resp)
		return resp
	}
	// MOUZ is ranked 12th; Spirit has no rank, so a level filter cannot rate the away match.
	if got := profile(f.mouz.String(), map[string]any{"hltv_rank": 12}); got.HLTVRank != 12 || got.Own || got.OwnTeams != 0 {
		t.Fatalf("PUT profile = %+v", got)
	}
	top := f.scout("?opponent_top=15")
	if len(top.Matches) != 1 || top.Matches[0].MatchID != f.home.ID || skippedFor(top.Skipped, scoutSkipUnranked) != 1 {
		t.Fatalf("top 15 = %+v (skipped %+v), want the home match and one unrated", top.Matches, top.Skipped)
	}
	if five := f.scout("?opponent_top=5"); len(five.Matches) != 0 {
		t.Fatalf("top 5 kept %d matches, want none", len(five.Matches))
	}

	// Spirit is the user's team: the away match is a match of theirs, the home one is scouting.
	if got := profile(f.spirit.String(), map[string]any{"own": true}); !got.Own || got.OwnTeams != 1 {
		t.Fatalf("PUT own = %+v", got)
	}
	if own := f.scout("?whose=own"); len(own.Matches) != 1 || own.Matches[0].MatchID != f.away.ID {
		t.Fatalf("own matches = %+v, want the away match", own.Matches)
	}
	if others := f.scout("?whose=others"); len(others.Matches) != 1 || others.Matches[0].MatchID != f.home.ID {
		t.Fatalf("other teams' matches = %+v, want the home match", others.Matches)
	}
	if one := f.scout("?max_matches=1"); len(one.Matches) != 1 || skippedFor(one.Skipped, scoutSkipBeyondLimits) != 0 {
		t.Fatalf("last match = %+v (skipped %+v), want one match and nothing reported over the cap", one.Matches, one.Skipped)
	}

	var got teamProfileResponse
	f.env.json(http.MethodGet, "/api/library/teams/"+f.spirit.String()+"/profile", nil, http.StatusOK, &got)
	if !got.Own || got.HLTVRank != 0 {
		t.Fatalf("GET profile = %+v", got)
	}
	// The teams list says which team is own and each rank, so a view finds them in one read.
	var list struct {
		Teams []library.Team `json:"teams"`
	}
	f.env.json(http.MethodGet, "/api/library/teams", nil, http.StatusOK, &list)
	for _, team := range list.Teams {
		if (team.ID == f.spirit) != team.Own || (team.ID == f.mouz) != (team.HLTVRank == 12) {
			t.Fatalf("teams list = %+v, want Spirit own and MOUZ ranked 12", list.Teams)
		}
	}
	// One own team: marking MOUZ takes the mark from Spirit.
	if got := profile(f.mouz.String(), map[string]any{"own": true}); !got.Own || got.OwnTeams != 1 || got.HLTVRank != 12 {
		t.Fatalf("PUT own on MOUZ = %+v", got)
	}
	var spirit library.Team
	f.env.json(http.MethodGet, "/api/library/teams/"+f.spirit.String(), nil, http.StatusOK, &spirit)
	if spirit.Own {
		t.Fatalf("Spirit = %+v, still own after MOUZ took the mark", spirit)
	}
	for _, bad := range []string{"?opponent_top=10", "?whose=mine", "?max_matches=0", "?advantage=9v9", "?awp=yes"} {
		if rw := f.env.do(http.MethodGet, "/api/library/teams/"+f.vitality.String()+"/scout"+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Errorf("scout%s = %d, want 400", bad, rw.Code)
		}
	}
	if rw := f.env.do(http.MethodPut, "/api/library/teams/"+f.mouz.String()+"/profile", []byte(`{"hltv_rank":-1}`), "application/json"); rw.Code != http.StatusBadRequest {
		t.Fatalf("PUT a negative rank = %d, want 400", rw.Code)
	}
}

func TestTeamScopeRoundCriteriaReachEveryTeamFigure(t *testing.T) {
	f := newScoutFixture(t)
	all := f.scout("")
	pistols := f.scout("?round_from=1&round_to=1")
	if pistols.Tendencies.RoundCount != 2 || all.Tendencies.RoundCount != 4 {
		t.Fatalf("round 1 only = %d rounds of %d, want one per match", pistols.Tendencies.RoundCount, all.Tendencies.RoundCount)
	}
	var heat spatialResponse
	f.env.json(http.MethodGet, "/api/library/teams/"+f.vitality.String()+"/heatmap?round_from=1&round_to=1", nil, http.StatusOK, &heat)
	if heat.Heatmap == nil || heat.Heatmap.Rounds != len(heat.Matches) {
		t.Fatalf("heatmap of round 1 = %+v, want one round per match read", heat.Heatmap)
	}
	for _, m := range heat.Matches {
		if len(m.Rounds) != 1 || m.Rounds[0] != 1 {
			t.Fatalf("heatmap match %+v, want its kept round 1 listed for the playlist", m)
		}
	}
	var rounds teamRoundsResponse
	f.env.json(http.MethodGet, "/api/library/teams/"+f.vitality.String()+"/rounds?round_from=1&round_to=1&max_matches=50", nil, http.StatusOK, &rounds)
	if rounds.Rounds != pistols.Tendencies.RoundCount || len(rounds.Matches) != 2 || rounds.Matches[0].Rounds[0].Round != 1 || rounds.Matches[0].Total != 2 {
		t.Fatalf("rounds = %+v, want round 1 of both matches, as the scout counts", rounds)
	}
}

func TestTeamScopeTacticKeepsThePlayRounds(t *testing.T) {
	f := newScoutFixture(t)
	play := library.Play{Title: "Exec A", TeamID: &f.vitality, Map: "de_mirage", Side: "T", Kind: "execute", Origin: "manual",
		Examples: []tacticalplan.Evidence{{MatchID: f.away.ID, Round: 2}}}
	if err := f.env.repo.CreatePlay(t.Context(), &play); err != nil {
		t.Fatal(err)
	}
	var rounds teamRoundsResponse
	f.env.json(http.MethodGet, "/api/library/teams/"+f.vitality.String()+"/rounds?tactic="+play.ID.String(), nil, http.StatusOK, &rounds)
	if rounds.Rounds != 1 || len(rounds.Matches) != 1 || rounds.Matches[0].MatchID != f.away.ID || rounds.Matches[0].Rounds[0].Round != 2 {
		t.Fatalf("tactic rounds = %+v, want round 2 of the away match", rounds)
	}
	for _, bad := range []string{"?tactic=nope", "?tactic=" + uuid.NewString(), "?weapon=knife", "?equip_to=0", "?end_reason=surrender"} {
		if rw := f.env.do(http.MethodGet, "/api/library/teams/"+f.vitality.String()+"/rounds"+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Errorf("rounds%s = %d, want 400", bad, rw.Code)
		}
	}
}

func TestTeamCandidatesReadEveryMatch(t *testing.T) {
	env := newLibraryTestEnv(t)
	first := env.seedLibraryMatch("m0", "Vitality", "MOUZ")
	team := *teamOfMatch(t, first, "ct-start").TeamID
	for i := 1; i <= library.MaxListLimit; i++ {
		env.seedLibraryMatch(fmt.Sprintf("m%d", i), "Vitality", "MOUZ")
	}
	got, err := env.h.allMatches(t.Context(), library.MatchFilter{TeamID: &team})
	if err != nil || len(got) != library.MaxListLimit+1 {
		t.Fatalf("listed %d matches (%v), want all %d", len(got), err, library.MaxListLimit+1)
	}
}
