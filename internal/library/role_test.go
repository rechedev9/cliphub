package library

import (
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// roleRound is one T round where slot 0 (the player) and slot 1 (a teammate)
// both play; opening marks the player in the opening duel.
type roleRound struct {
	kills      []string
	grenades   int
	mateNades  int
	opening    bool
	playerSide tacticalplan.Side
}

func roleDoc(rounds []roleRound) tacticalplan.Document {
	doc := tacticalplan.Document{Players: []tacticalplan.Player{
		{Slot: 0, SteamID64: "765", TeamKey: "t-start"},
		{Slot: 1, SteamID64: "766", TeamKey: "t-start"},
		{Slot: 5, SteamID64: "999", TeamKey: "ct-start"},
	}}
	player, mate, rival := uint8(0), uint8(1), uint8(5)
	for i, spec := range rounds {
		side := spec.playerSide
		if side == "" {
			side = tacticalplan.SideT
		}
		round := tacticalplan.Round{
			Number: i + 1,
			Class:  tacticalplan.Class{OpeningTick: 1000 + i},
			Players: []tacticalplan.PlayerRound{
				{Slot: player, Side: side, OpeningKill: spec.opening},
				{Slot: mate, Side: side},
				{Slot: rival, Side: tacticalplan.SideCT},
			},
		}
		for k, weapon := range spec.kills {
			round.Events = append(round.Events, tacticalplan.Event{Tick: 2000 + k, Kind: tacticalplan.EventKill, ActorSlot: &player, Weapon: weapon})
		}
		for g := 0; g < spec.grenades; g++ {
			round.Events = append(round.Events, tacticalplan.Event{Tick: 3000 + g, Kind: tacticalplan.EventFlash, ActorSlot: &player})
		}
		for g := 0; g < spec.mateNades; g++ {
			round.Events = append(round.Events, tacticalplan.Event{Tick: 4000 + g, Kind: tacticalplan.EventSmoke, ActorSlot: &mate})
		}
		// A rival grenade never counts for the player's team.
		round.Events = append(round.Events, tacticalplan.Event{Tick: 5000, Kind: tacticalplan.EventHE, ActorSlot: &rival})
		doc.Rounds = append(doc.Rounds, round)
	}
	return doc
}

func repeatRounds(n int, spec roleRound) []roleRound {
	out := make([]roleRound, n)
	for i := range out {
		out[i] = spec
	}
	return out
}

func TestInferPlayerRole(t *testing.T) {
	match := uuid.MustParse("00000000-0000-0000-0000-0000000000aa")
	cases := []struct {
		name   string
		rounds []roleRound
		want   string
	}{
		{"too few rounds", repeatRounds(RoleMinRounds-1, roleRound{kills: []string{"awp"}}), RoleUnknown},
		{"awp share below the threshold", append(repeatRounds(5, roleRound{kills: []string{"awp", "ak47"}}), repeatRounds(15, roleRound{kills: []string{"ak47"}})...), RoleRifler},
		{"awp share at the threshold", append(repeatRounds(6, roleRound{kills: []string{"awp"}}), repeatRounds(14, roleRound{kills: []string{"ak47"}})...), RoleAWP},
		{"entry on T", append(repeatRounds(6, roleRound{opening: true, kills: []string{"ak47"}}), repeatRounds(14, roleRound{kills: []string{"ak47"}})...), RoleEntry},
		{"openings on CT do not make an entry", repeatRounds(20, roleRound{opening: true, playerSide: tacticalplan.SideCT}), RoleRifler},
		{"support utility", repeatRounds(20, roleRound{grenades: 3, mateNades: 2}), RoleSupport},
		{"rifler", repeatRounds(20, roleRound{kills: []string{"m4a1"}, grenades: 1, mateNades: 1}), RoleRifler},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := InferPlayerRole("765", []RoleDocument{{MatchID: match, Document: roleDoc(tc.rounds)}})
			if got.Role != tc.want {
				t.Fatalf("role = %q, want %q (%+v)", got.Role, tc.want, got)
			}
		})
	}
}

func TestInferPlayerRoleKeepsEvidence(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	docs := []RoleDocument{
		{MatchID: first, Document: roleDoc(repeatRounds(4, roleRound{kills: []string{"awp"}, opening: true, grenades: 1}))},
		{MatchID: uuid.New(), Document: tacticalplan.Document{}},
		{MatchID: second, Document: roleDoc(repeatRounds(4, roleRound{kills: []string{"awp"}, mateNades: 2}))},
	}
	got := InferPlayerRole("765", docs)
	if got.MatchCount != 2 || got.Rounds != 8 {
		t.Fatalf("matches, rounds = %d, %d; want 2, 8", got.MatchCount, got.Rounds)
	}
	if got.AWPKills.Count != 8 || got.AWPKills.Total != 8 || len(got.AWPKills.Examples) != roleExampleLimit {
		t.Fatalf("awp kills = %+v, want 8 of 8 with %d examples", got.AWPKills, roleExampleLimit)
	}
	if e := got.AWPKills.Examples[4]; e.MatchID != second || e.Round != 1 || e.Tick != 2000 {
		t.Fatalf("fifth awp example = %+v, want match %s round 1 tick 2000", e, second)
	}
	if got.TOpenings.Count != 4 || got.TOpenings.Total != 8 || got.TOpenings.Examples[0].Tick != 1000 {
		t.Fatalf("t openings = %+v, want 4 of 8 from tick 1000", got.TOpenings)
	}
	if got.Utility.Count != 4 || got.Utility.Total != 8 || got.TeamUtility.Count != 8 || got.TeamUtility.Total != 8 {
		t.Fatalf("utility = %+v, team = %+v", got.Utility, got.TeamUtility)
	}
	if got.Role != RoleUnknown || got.Thresholds.MinRounds != RoleMinRounds {
		t.Fatalf("role = %q thresholds = %+v, want unknown under %d rounds", got.Role, got.Thresholds, RoleMinRounds)
	}
}
