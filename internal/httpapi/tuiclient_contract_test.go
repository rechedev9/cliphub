package httpapi

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func newContractClient(t *testing.T, f scoutFixture) *tuiclient.Client {
	t.Helper()
	t.Setenv("ZV_MUTATION_TOKEN", "")
	srv := httptest.NewServer(Routes(f.env.h))
	t.Cleanup(srv.Close)
	return tuiclient.New(tuiclient.Config{BaseURL: srv.URL})
}

func TestTUIClientReadsTheLibraryRoutesItShares(t *testing.T) {
	f := newScoutFixture(t)
	c := newContractClient(t, f)
	ctx := context.Background()
	vitality := f.vitality.String()

	team, err := c.GetLibraryTeam(ctx, vitality)
	if err != nil {
		t.Fatal(err)
	}
	if team.Name != "Vitality" || len(team.Roster) == 0 || team.Roster[0].SteamID64 == "" || team.Roster[0].Matches == 0 || team.Roster[0].LastPlayedAt.IsZero() {
		t.Fatalf("team = %+v", team)
	}
	player, err := c.GetLibraryPlayer(ctx, team.Roster[0].SteamID64)
	if err != nil {
		t.Fatal(err)
	}
	if len(player.Nicks) == 0 || player.Nicks[0].Nick == "" || player.Nicks[0].FirstSeenAt.IsZero() {
		t.Fatalf("player nicks = %+v", player.Nicks)
	}

	stats, err := c.TeamSideStats(ctx, tuiclient.SideStatsQuery{TeamID: vitality})
	if err != nil {
		t.Fatal(err)
	}
	if stats.TeamName != "Vitality" || stats.Rounds != 4 || stats.Overall.RoundsPlayed.Count != 4 || stats.Overall.RoundsWon.Total != 4 || stats.Overall.MapsWon.Total != 2 || stats.Overall.MapsWon.Value == nil {
		t.Fatalf("side stats = %d rounds, overall %+v", stats.Rounds, stats.Overall)
	}

	scout, err := c.TeamScout(ctx, vitality, tuiclient.AnalysisQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(scout.Maps) == 0 || scout.Maps[0].WinRate.Total != scout.Maps[0].Rounds || scout.Tendencies.RoundCount != 4 {
		t.Fatalf("scout maps = %+v, %d rounds", scout.Maps, scout.Tendencies.RoundCount)
	}
}

func TestTUIClientWritesTheLibraryRoutesItShares(t *testing.T) {
	f := newScoutFixture(t)
	c := newContractClient(t, f)
	ctx := context.Background()

	pins := []tuiclient.PinnedPlayer{{SteamID64: "76561198000000002", Name: "ZywOo"}, {SteamID64: "76561198000000001", Name: "donk"}}
	if _, err := c.SetPinnedPlayers(ctx, pins); err != nil {
		t.Fatal(err)
	}
	gotPins, err := c.GetPinnedPlayers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotPins) != 2 || gotPins[0] != pins[0] || gotPins[1] != pins[1] {
		t.Fatalf("pins = %+v, want %+v", gotPins, pins)
	}

	sheet := tuiclient.TeamRoleSheet{Map: "de_mirage", T: []tuiclient.RoleSlot{{Name: "Entry", Task: "abre B"}}, CT: []tuiclient.RoleSlot{{Name: "Ancla", Callout: "BombsiteB"}}}
	if _, err := c.SetTeamRoles(ctx, f.vitality.String(), sheet); err != nil {
		t.Fatal(err)
	}
	roles, err := c.GetTeamRoles(ctx, f.vitality.String(), "de_mirage")
	if err != nil {
		t.Fatal(err)
	}
	if len(roles.Sheet.T) != 1 || roles.Sheet.T[0] != sheet.T[0] || len(roles.Sheet.CT) != 1 || roles.Sheet.CT[0] != sheet.CT[0] {
		t.Fatalf("role sheet = %+v, want %+v", roles.Sheet, sheet)
	}

	format := "bo3"
	teams := []string{f.vitality.String(), f.mouz.String()}
	vetoes := []tuiclient.VetoStep{{Team: "b", Action: "ban", Map: "de_mirage"}, {Team: "a", Action: "pick", Map: "de_nuke"}}
	created, err := c.CreateLibrarySeries(ctx, tuiclient.SeriesInput{Format: &format, TeamIDs: &teams, Vetoes: &vetoes})
	if err != nil {
		t.Fatal(err)
	}
	series, err := c.GetLibrarySeries(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(series.Vetoes) != 2 || series.Vetoes[0] != vetoes[0] || series.Vetoes[1] != vetoes[1] || len(series.Teams) != 2 {
		t.Fatalf("series = %+v", series)
	}
}
