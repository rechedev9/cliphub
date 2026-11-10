package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func partWith(first int, games ...int) proPart {
	doc := tacticalplan.NewDocument()
	for _, g := range games {
		doc.Rounds = append(doc.Rounds, tacticalplan.Round{Number: g, ScoreCTBefore: g - 1})
	}
	return proPart{header: analysisHeader{Document: doc}, first: first}
}

func TestOwnedRoundsKeepsEachGameRoundInOnePart(t *testing.T) {
	// The server went down in game round 14; the restart part replays it.
	first, restart := partWith(1, 1, 2, 13, 14), partWith(14, 14, 15)
	owned := ownedRounds([]proPart{first, restart})
	if !owned[0][13] || owned[0][14] || !owned[1][14] || !owned[1][15] {
		t.Fatalf("owned = %v, want round 14 in the later part only", owned)
	}
	// A demo uploaded twice owns every round once, in the first copy.
	dup := ownedRounds([]proPart{partWith(1, 1, 2), partWith(1, 1, 2)})
	if len(dup[0]) != 2 || len(dup[1]) != 0 {
		t.Fatalf("duplicate = %v, want the first copy to keep the rounds", dup)
	}
}

func TestProPlayersWithoutAnIndexIsAnEmptyBoard(t *testing.T) {
	_, h := testAPI(t)
	for _, q := range []string{"", "?map=de_anubis&role=b_lurk"} {
		rec := do(t, h, http.MethodGet, "/v1/pro-players"+q, nil)
		var board community.ProPlayersBoard
		if err := json.Unmarshal(rec.Body.Bytes(), &board); rec.Code != http.StatusOK || err != nil || len(board.Players) != 0 {
			t.Fatalf("%q = %d %s, want an empty board", q, rec.Code, rec.Body.String())
		}
	}
}
