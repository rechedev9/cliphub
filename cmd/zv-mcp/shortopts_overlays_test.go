package main

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestResolveEditShortOverlays(t *testing.T) {
	on := true
	quarter, half, odd := 0.25, 0.5, 0.75
	edit, err := resolveEdit(shortChoices{PlayBadges: &on, StatsCard: &on, VictimReplay: &on, VictimReplaySpeed: &quarter})
	if err != nil {
		t.Fatal(err)
	}
	req := decodeEdit(t, edit)
	if !req.PlayBadges || !req.StatsCard || req.VictimReplaySpeed != 0.25 {
		t.Fatalf("overlays not sent: %+v", req)
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("renderplan rejected the overlays: %v", err)
	}
	if edit, err := resolveEdit(shortChoices{VictimReplay: &on, VictimReplaySpeed: &half}); err != nil || edit.VictimReplaySpeed != 0 {
		t.Fatalf("default speed = %v, %v; want unset like the UI", edit.VictimReplaySpeed, err)
	}
	for name, c := range map[string]shortChoices{
		"speed without replay": {VictimReplaySpeed: &quarter},
		"unknown speed":        {VictimReplay: &on, VictimReplaySpeed: &odd},
		"landscape card":       {Format: "landscape-16x9", StatsCard: &on},
	} {
		if _, err := resolveEdit(c); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	summary := describeShortRequest(tuiclient.GenerateRequest{Edit: edit})
	if !strings.Contains(summary, "play_badges") || !strings.Contains(summary, "stats_card") || !strings.Contains(summary, "repetición a 0.25x") {
		t.Errorf("summary misses the overlays: %s", summary)
	}
}
