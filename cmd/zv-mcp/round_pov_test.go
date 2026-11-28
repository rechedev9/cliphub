package main

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestRoundPOVPlanIsOneRoundWithoutEffects(t *testing.T) {
	got, err := applyFullDemoChoices(recapplan.DefaultOptions(), roundPOVPlanIn("job-1", 7))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Rounds) != 1 || got.Rounds[0] != 7 {
		t.Fatalf("rounds = %v, want [7]", got.Rounds)
	}
	if got.Transitions != nil && got.Transitions.Enabled {
		t.Fatal("a one round POV must not plan effects between rounds")
	}
	if got.Overlays.Scoreboard {
		t.Fatal("a one round POV must end with the round, without the final scoreboard over it")
	}
}

func TestCheckRoundPOV(t *testing.T) {
	const steam = "76561198000000001"
	m := tuiclient.LibraryMatch{RoundCount: 24, Players: []tuiclient.LibraryMatchPlayer{{SteamID64: steam}}}
	for name, tc := range map[string]struct {
		in   recordRoundPOVIn
		want string
	}{
		"ok":           {recordRoundPOVIn{Round: 24, SteamID: steam}, ""},
		"first round":  {recordRoundPOVIn{Round: 1, SteamID: steam}, ""},
		"round zero":   {recordRoundPOVIn{Round: 0, SteamID: steam}, "between 1 and 24"},
		"past the end": {recordRoundPOVIn{Round: 25, SteamID: steam}, "between 1 and 24"},
		"not in match": {recordRoundPOVIn{Round: 3, SteamID: "76561198000000002"}, "did not play"},
	} {
		err := checkRoundPOV(m, tc.in)
		switch {
		case tc.want == "" && err != nil:
			t.Errorf("%s: %v", name, err)
		case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
			t.Errorf("%s: err = %v, want %q", name, err, tc.want)
		}
	}
}

func TestIsRoundPOVPlan(t *testing.T) {
	options, err := applyFullDemoChoices(recapplan.DefaultOptions(), roundPOVPlanIn("job-1", 7))
	if err != nil {
		t.Fatal(err)
	}
	doc := recapplan.Document{Options: options}
	if !isRoundPOVPlan(doc, 7) {
		t.Fatal("a saved one round plan without effects should be kept")
	}
	if isRoundPOVPlan(doc, 8) {
		t.Fatal("another round needs a new plan")
	}
	withScoreboard := doc
	withScoreboard.Options.Overlays.Scoreboard = true
	if isRoundPOVPlan(withScoreboard, 7) {
		t.Fatal("a one round plan saved with the final scoreboard needs a new plan")
	}
	effects := *doc.Options.Transitions
	effects.Enabled = true
	doc.Options.Transitions = &effects
	if doc.Options.ValidateCurrentFullDemoPolicy() != nil {
		t.Fatal("the transitions case must pass the policy gate so it tests the effects condition alone")
	}
	if isRoundPOVPlan(doc, 7) {
		t.Fatal("a plan with effects is not the plain one")
	}
}
