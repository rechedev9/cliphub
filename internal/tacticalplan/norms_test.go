package tacticalplan

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// The library norms must equal the team views summed over every team, so a
// team's number and its reference follow one definition.
func TestNormsPoolEveryTeamWithTheTeamDefinitions(t *testing.T) {
	m := pbMatch(t)
	teams := []string{"t-start", "ct-start"}
	asTeam := func(key string) FramedMatch {
		c := m
		c.TeamKey = key
		return c
	}
	b := NewNormsBuilder(true)
	b.Add(m)
	norms := b.Norms()
	if norms.Matches != 1 || len(norms.Playbook) != 2 {
		t.Fatalf("norms = %d matches, %d playbook sides", norms.Matches, len(norms.Playbook))
	}

	for _, p := range norms.Playbook {
		wantRounds, wantWins := 0, 0
		perCategory := map[string][2]int{}
		for _, key := range teams {
			pb := BuildPlaybook([]FramedMatch{asTeam(key)}, ScoutFilter{Side: p.Side})
			wantRounds += pb.RoundCount
			wantWins += pb.WinRate.Count
			for _, c := range pb.Categories {
				sum := perCategory[c.Category]
				perCategory[c.Category] = [2]int{sum[0] + c.Rounds, sum[1] + c.WinRate.Count}
			}
		}
		wantRate(t, string(p.Side)+" win", p.WinRate, wantWins, wantRounds)
		for _, c := range p.Categories {
			want := perCategory[c.Category]
			wantRate(t, string(p.Side)+" "+c.Category+" share", c.Share, want[0], wantRounds)
			wantRate(t, string(p.Side)+" "+c.Category+" won", c.WinRate, want[1], want[0])
		}
	}

	matchups := map[NormMatchup]int{}
	for _, key := range teams {
		for _, row := range BuildPrep([]FramedMatch{asTeam(key)}, PrepOptions{}).Matchups {
			k := NormMatchup{Side: row.Side, Buy: row.Buy, OpponentBuy: row.OpponentBuy}
			matchups[k] += row.WinRate.Count
		}
	}
	if len(norms.Matchups) != len(matchups) {
		t.Fatalf("matchups = %+v, want the keys of %v", norms.Matchups, matchups)
	}
	for _, row := range norms.Matchups {
		k := NormMatchup{Side: row.Side, Buy: row.Buy, OpponentBuy: row.OpponentBuy}
		if won, ok := matchups[k]; !ok || won != row.WinRate.Count {
			t.Fatalf("matchup %+v won %d, want %d", k, row.WinRate.Count, won)
		}
	}
}

func TestNormsBombsitesSumBothTeams(t *testing.T) {
	doc := bombsiteDocument()
	id := uuid.MustParse("00000000-0000-0000-0000-0000000000b2")
	at := time.Date(2026, 10, 1, 18, 0, 0, 0, time.UTC)
	b := NewNormsBuilder(false)
	b.Add(FramedMatch{ScoutMatch: ScoutMatch{MatchID: id, PlayedAt: at, Document: doc}})
	norms := b.Norms()
	if len(norms.Playbook) != 0 || len(norms.Bombsites) != 1 {
		t.Fatalf("norms without positions = %d playbook sides, %d maps", len(norms.Playbook), len(norms.Bombsites))
	}
	got := norms.Bombsites[0]
	home := Bombsites([]ScoutMatch{{MatchID: id, TeamKey: "home", PlayedAt: at, Document: doc}}, ScoutFilter{}).Maps[0]
	away := Bombsites([]ScoutMatch{{MatchID: id, TeamKey: "away", PlayedAt: at, Document: doc}}, ScoutFilter{}).Maps[0]
	if got.Matches != 1 || got.CTRounds != home.CTRounds+away.CTRounds {
		t.Fatalf("map = %+v", got)
	}
	for i, site := range got.Sites {
		h, a := home.Sites[i], away.Sites[i]
		wantRate(t, "CT held", site.CT.Held, h.CT.Held.Count+a.CT.Held.Count, h.CT.Held.Total+a.CT.Held.Total)
		wantRate(t, "T post-plant", site.T.PostPlantWon, h.T.PostPlantWon.Count+a.T.PostPlantWon.Count, h.T.PostPlantWon.Total+a.T.PostPlantWon.Total)
		if len(site.CT.Examples) != 0 || len(site.T.Examples) != 0 {
			t.Fatalf("site %s keeps examples; a norm names no round", site.Site)
		}
	}
	// Every hit site is attacked by the other team, so the pooled CT and T hits match.
	a := got.Sites[0]
	if a.CT.Hit.Count != a.T.Attacks.Count {
		t.Fatalf("A hit %d times as CT, attacked %d times as T", a.CT.Hit.Count, a.T.Attacks.Count)
	}
}
