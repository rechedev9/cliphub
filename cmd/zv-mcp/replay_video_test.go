package main

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/replaygif"
)

func TestHiddenLayers(t *testing.T) {
	hide, drawings, err := hiddenLayers(nil)
	if err != nil || hide != (replaygif.Layers{}) || !drawings {
		t.Fatalf("no layers = %+v drawings=%v err=%v, want every layer", hide, drawings, err)
	}
	hide, drawings, err = hiddenLayers([]string{"players", "clock"})
	if err != nil || drawings || hide != (replaygif.Layers{Utility: true, Bomb: true}) {
		t.Fatalf("players and clock = %+v drawings=%v err=%v", hide, drawings, err)
	}
	if _, _, err := hiddenLayers([]string{"callouts"}); err == nil {
		t.Fatal("an unknown layer was accepted")
	}
}

func TestVideoRounds(t *testing.T) {
	got, err := videoRounds(replayVideoIn{Round: 4, Rounds: []int{7, 9}})
	if err != nil || len(got) != 3 || got[0] != 4 || got[2] != 9 {
		t.Fatalf("videoRounds = %v err=%v, want [4 7 9]", got, err)
	}
	if _, err := videoRounds(replayVideoIn{}); err == nil {
		t.Fatal("a video without rounds was accepted")
	}
}
