package demooverlay

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/killplan"
)

func halftimeRounds(firstSide string, firstHalfWinners string, secondHalf int) []killplan.RoundResult {
	sides := map[byte]string{'C': "CT", 'T': "T"}
	var rounds []killplan.RoundResult
	for i := range len(firstHalfWinners) {
		rounds = append(rounds, killplan.RoundResult{Number: i + 1, Winner: sides[firstHalfWinners[i]], TargetSide: firstSide})
	}
	for range secondHalf {
		rounds = append(rounds, killplan.RoundResult{Number: len(rounds) + 1, Winner: "CT", TargetSide: oppositeSide(firstSide)})
	}
	return rounds
}

func halftimeDocument() Document {
	return Document{TargetSteamID64: "pov", Outro: Scoreboard{Teams: []TeamBoard{
		{Name: "Vitality", Side: "T", Players: []PlayerCard{{SteamID64: "pov"}}},
		{Name: "Spirit", Side: "CT", Players: []PlayerCard{{SteamID64: "rival"}}},
	}}}
}

func TestHalftimeScoreComesFromTheRoundsBeforeTheSideSwitch(t *testing.T) {
	rounds := halftimeRounds("CT", "CCCCCCCTTTTT", 6)
	got := BuildHalftime(halftimeDocument(), rounds, customhud.Teams{})
	if got == nil || got.Round != 13 {
		t.Fatalf("an MR12 match switches sides at round 13: %+v", got)
	}
	if got.Teams[0] != (HalftimeTeam{Name: "Vitality", Side: "T", Score: 7}) || got.Teams[1] != (HalftimeTeam{Name: "Spirit", Side: "CT", Score: 5}) {
		t.Fatalf("the POV team goes first with its first half score: %+v", got.Teams)
	}
	short := BuildHalftime(halftimeDocument(), halftimeRounds("T", "TTCTCTCT", 3), customhud.Teams{})
	if short == nil || short.Round != 9 || short.Teams[0].Score != 5 || short.Teams[1].Score != 3 {
		t.Fatalf("the switch round comes from the data, not from MR12: %+v", short)
	}
	overtime := halftimeRounds("CT", "CCCCCCTTTTTT", 12)
	for i := range 3 {
		overtime = append(overtime, killplan.RoundResult{Number: 25 + i, Winner: "T", TargetSide: "CT"})
	}
	if got := BuildHalftime(halftimeDocument(), overtime, customhud.Teams{}); got == nil || got.Round != 13 {
		t.Fatalf("an overtime switch must not move the halftime: %+v", got)
	}
}

func TestHalftimeTeamNamesFollowThePlanThenTheDemo(t *testing.T) {
	rounds := halftimeRounds("CT", "CCT", 2)
	named := BuildHalftime(halftimeDocument(), rounds, customhud.Teams{CTStart: " Mi equipo ", TStart: "Rival"})
	if named.Teams[0].Name != "Mi equipo" || named.Teams[1].Name != "Rival" {
		t.Fatalf("the plan names go by the side each team started on: %+v", named.Teams)
	}
	doc := halftimeDocument()
	doc.Outro.Teams[0].Name, doc.Outro.Teams[1].Name = "Terrorists", "Counter-Terrorists"
	sides := BuildHalftime(doc, rounds, customhud.Teams{})
	if sides.Teams[0].Name != "T" || sides.Teams[1].Name != "CT" {
		t.Fatalf("a demo without team names shows the sides of the second half: %+v", sides.Teams)
	}
}

func TestHalftimeIsUnknownWithoutCompleteRoundResults(t *testing.T) {
	full := halftimeRounds("CT", "CCCCCCCTTTTT", 4)
	cases := map[string][]killplan.RoundResult{
		"no rounds":      nil,
		"no side switch": halftimeRounds("CT", "CCCCCCCTTTTT", 0),
	}
	gap := append([]killplan.RoundResult{}, full[:5]...)
	cases["a round without a winner"] = append(gap, full[6:]...)
	unknown := append([]killplan.RoundResult{}, full...)
	unknown[11].TargetSide = ""
	cases["an unknown side before the switch"] = unknown
	draw := append([]killplan.RoundResult{}, full...)
	draw[3].Winner = ""
	cases["a draw"] = draw
	for name, rounds := range cases {
		if got := BuildHalftime(halftimeDocument(), rounds, customhud.Teams{}); got != nil {
			t.Errorf("%s produced a halftime score: %+v", name, got)
		}
	}
	late := append([]killplan.RoundResult{}, full...)
	late[0].TargetSide, late[1].TargetSide = "", ""
	if got := BuildHalftime(halftimeDocument(), late, customhud.Teams{}); got == nil || got.Round != 13 || got.Teams[0].Score != 7 {
		t.Fatalf("a POV who joined late keeps the team score: %+v", got)
	}
}

func TestHalftimeAndSubscribePagesFollowTheDesign(t *testing.T) {
	halftime := Halftime{Round: 13, Teams: [2]HalftimeTeam{{Name: `<b>Vitality</b>`, Side: "T", Score: 7}, {Name: "Spirit", Side: "CT", Score: 5}}}
	palette := &Palette{CT: "112233", T: "AABBCC", Background: "000000", Text: "FFFFFF", Muted: "999999", Radius: 4}
	for name, design := range map[string]*Palette{"neon": nil, "hud": palette} {
		card, err := HalftimeHTML(halftime, design)
		if err != nil {
			t.Fatalf("%s halftime: %v", name, err)
		}
		page := string(card)
		if !strings.Contains(page, ">DESCANSO<") || !strings.Contains(page, "&lt;b&gt;Vitality&lt;/b&gt;") || strings.Contains(page, "<b>Vitality") {
			t.Fatalf("%s halftime lacks its label or escaped names", name)
		}
		if strings.Index(page, "Vitality") > strings.Index(page, "Spirit") || !strings.Contains(page, `class="lead">7<`) || strings.Contains(page, `class="lead">5<`) {
			t.Fatalf("%s halftime must put the POV team first and mark the leader", name)
		}
		button, err := SubscribeHTML(design)
		if err != nil {
			t.Fatalf("%s subscribe: %v", name, err)
		}
		for _, html := range []string{page, string(button)} {
			if strings.Contains(html, "http://") || strings.Contains(html, "https://") || strings.Contains(html, "<script") || strings.Contains(html, "<img") {
				t.Fatalf("%s page loads something from outside", name)
			}
			if strings.Contains(html, "#112233") != (design != nil) || strings.Contains(html, "#d99aff") != (design == nil) {
				t.Fatalf("%s page does not use its own colours", name)
			}
		}
		if !strings.Contains(string(button), ">SUSCRÍBETE<") || !strings.Contains(string(button), "<svg") {
			t.Fatalf("%s subscribe lacks its label or bell", name)
		}
	}
}

func TestOverlayDocumentWithoutHalftimeKeepsItsWireShape(t *testing.T) {
	raw, err := json.Marshal(Document{SchemaVersion: SchemaVersion})
	if err != nil || strings.Contains(string(raw), "halftime") {
		t.Fatalf("halftime reached a document without it: %s %v", raw, err)
	}
	doc := Document{SchemaVersion: SchemaVersion, Halftime: &Halftime{Round: 13, Teams: [2]HalftimeTeam{{Name: "A", Side: "T", Score: 7}, {Name: "B", Side: "CT", Score: 5}}}}
	raw, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var restored Document
	if err := json.Unmarshal(raw, &restored); err != nil || restored.Halftime == nil || *restored.Halftime != *doc.Halftime {
		t.Fatalf("halftime round trip: %+v %v", restored.Halftime, err)
	}
}
