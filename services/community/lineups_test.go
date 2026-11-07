package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/prolineups"
)

func TestJoinPartsKeepsTheReplayedRoundFromTheLaterPart(t *testing.T) {
	throw := func(round, tick int) prolineups.Throw {
		return prolineups.Throw{Map: "de_mirage", Round: round, SteamID64: "1", ThrowTick: tick}
	}
	// The server went down in round 14; the restart part replays it with ticks from zero.
	first := newMapPart("1/m1-mirage", []prolineups.Throw{throw(13, 200000), throw(14, 210000)})
	restart := newMapPart("1/m1-mirage", []prolineups.Throw{throw(14, 3000), throw(15, 9000)})
	other := newMapPart("2/m1-mirage", []prolineups.Throw{throw(14, 210000)})
	got := joinParts([]mapPart{restart, first, other})
	want := map[[2]int]bool{{13, 200000}: true, {14, 3000}: true, {15, 9000}: true, {14, 210000}: true}
	if len(got) != len(want) {
		t.Fatalf("kept %d throws, want %d: %+v", len(got), len(want), got)
	}
	for _, th := range got {
		if !want[[2]int{th.Round, th.ThrowTick}] {
			t.Errorf("kept round %d tick %d", th.Round, th.ThrowTick)
		}
	}
	if n := len(joinParts([]mapPart{first, restart})); n != 3 {
		t.Errorf("part order changed the result: %d throws", n)
	}
}

func TestBuildLineupsKeepsTheStoredIndexWhenNothingIsRead(t *testing.T) {
	s, err := openStore(filepath.Join(t.TempDir(), "community.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	stored := map[string][]byte{"lineups:de_mirage": []byte(`{"map":"de_mirage"}`), lineupsIndexKey: []byte(`{"maps":[]}`)}
	if err := s.replacePrefixed(ctx, "lineups:", stored); err != nil {
		t.Fatal(err)
	}
	if _, err := buildLineups(ctx, s, filepath.Join(t.TempDir(), "missing"), time.Now()); err == nil {
		t.Fatal("an empty analyses folder must not rebuild the lineups")
	}
	if _, _, err := s.referenceBody(ctx, "lineups:de_mirage"); err != nil {
		t.Fatalf("stored lineups were dropped: %v", err)
	}
}
