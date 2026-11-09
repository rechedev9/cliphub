package main

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestGifTicksCountFromLiveOrFromThePlant(t *testing.T) {
	round := tuiclient.TacticalRoundFrames{Round: tacticalplan.Round{Number: 3, TickFreezeEnd: 1000, TickEnd: 9000}}
	from, to, err := gifTicks(replayGIFIn{Round: 3, FromSecond: 10, ToSecond: 20}, round, 64)
	if err != nil || from != 1640 || to != 2280 {
		t.Fatalf("live tramo = %d..%d, %v", from, to, err)
	}
	from, to, err = gifTicks(replayGIFIn{Round: 3, FromSecond: 100}, round, 64)
	if err != nil || from != 7400 || to != 9000 {
		t.Fatalf("open end runs to the round end: %d..%d, %v", from, to, err)
	}
	if _, _, err := gifTicks(replayGIFIn{Round: 3, FromPlant: true}, round, 64); err == nil || !strings.Contains(err.Error(), "no plant") {
		t.Fatalf("a round without a plant: %v", err)
	}
	round.Round.Bomb = &tacticalplan.Bomb{PlantTick: 6000}
	from, to, err = gifTicks(replayGIFIn{Round: 3, FromPlant: true, ToSecond: 200}, round, 64)
	if err != nil || from != 6000 || to != 9000 {
		t.Fatalf("plant tramo = %d..%d, %v", from, to, err)
	}
	if _, _, err := gifTicks(replayGIFIn{Round: 3, FromSecond: 200}, round, 64); err == nil {
		t.Fatal("a tramo past the round end should fail")
	}
}
