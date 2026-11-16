package customhud

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/hudthemes"
)

func styledNodes(t *testing.T, r *Renderer, s Snapshot) map[string]Node {
	t.Helper()
	nodes := map[string]Node{}
	for _, n := range r.Scene(s, ExampleTarget) {
		nodes[n.ID] = n
	}
	return nodes
}

func TestUserStyleRecoloursAndRoundsTheBaseDesign(t *testing.T) {
	r, err := NewRenderer("apex")
	if err != nil {
		t.Fatal(err)
	}
	radius := 12
	r.Theme = r.Theme.WithStyle(hudthemes.Style{Name: "Mío", CT: "112233", T: "445566", Background: "000000", Radius: &radius})
	nodes := styledNodes(t, r, Example())
	if n := nodes["score/ct-plate"]; n.Color != "112233" || !strings.Contains(n.Path, "C ") {
		t.Fatalf("ct plate = %+v; want the user colour with rounded corners", n)
	}
	if n := nodes["score/plate"]; n.Color != "000000" {
		t.Fatalf("score plate colour = %q; want the user background", n.Color)
	}
	square := 0
	r.Theme = r.Theme.WithStyle(hudthemes.Style{Name: "Recto", CT: "112233", T: "445566", Background: "000000", Radius: &square})
	if n := styledNodes(t, r, Example())["score/ct-plate"]; strings.Contains(n.Path, "C ") || r.Theme.Shape != "square" {
		t.Fatalf("radius 0 = %+v (shape %q); want square plates", n, r.Theme.Shape)
	}
	base, _ := Lookup("apex")
	kept := base.WithStyle(hudthemes.Style{Name: "Igual", CT: "112233", T: "445566", Background: "000000"})
	if kept.Shape != base.Shape || kept.Layout != base.Layout || kept.ScoreWidth != base.ScoreWidth {
		t.Fatalf("a style without radius changed the base design: %+v", kept)
	}
}

func TestCompositionSwitchesDropTheirPanels(t *testing.T) {
	r, err := NewRenderer("arena")
	if err != nil {
		t.Fatal(err)
	}
	full := styledNodes(t, r, Example())
	if _, ok := full["loadout/plate"]; !ok {
		t.Fatal("the default HUD lost its loadout")
	}
	r.Options = Options{CompactHeader: true, HideLoadout: true}
	got := styledNodes(t, r, Example())
	for id := range got {
		if strings.HasPrefix(id, "player/") || strings.HasPrefix(id, "loadout/") || id == "focus/ammo" {
			t.Fatalf("node %q drawn with the compact header and no loadout", id)
		}
	}
	for _, id := range []string{"score/time", "score/ct-alive", "focus/name"} {
		if _, ok := got[id]; !ok {
			t.Fatalf("node %q missing; the switches must keep the scoreboard and the focus card", id)
		}
	}
}

func TestTeamNamesFollowTheStartingSideAndWinBannerShowsTheWinner(t *testing.T) {
	r, err := NewRenderer("arena")
	if err != nil {
		t.Fatal(err)
	}
	r.Teams, r.Options = Teams{CTStart: "Vitality", TStart: "Spirit"}, Options{RoundWinBanner: true}
	tracker := &roundTracker{}
	first := tracker.next(roundState(1, 0, 0, false))
	if nodes := styledNodes(t, r, first); nodes["score/ct-label"].Text != "VITALITY" || nodes["score/t-label"].Text != "SPIRIT" {
		t.Fatalf("labels before halftime = %q / %q", nodes["score/ct-label"].Text, nodes["score/t-label"].Text)
	}
	after := tracker.next(roundState(13, 0, 12, true))
	if nodes := styledNodes(t, r, after); nodes["score/ct-label"].Text != "SPIRIT" || nodes["score/t-label"].Text != "VITALITY" {
		t.Fatalf("labels after halftime = %q / %q; names follow the teams", nodes["score/ct-label"].Text, nodes["score/t-label"].Text)
	}
	ended := roundState(13, 1, 12, true)
	ended.Phase = "ended"
	won := tracker.next(ended)
	nodes := styledNodes(t, r, won)
	if won.Winner != "CT" || nodes["round-win/team"].Text != "SPIRIT" || !nodes["round-win/team"].FadeIn {
		t.Fatalf("winner %q, banner %+v; want the CT team now on CT", won.Winner, nodes["round-win/team"])
	}
	freeze := roundState(14, 1, 12, true)
	freeze.Phase = "freeze"
	tracker.next(freeze)
	again := roundState(14, 1, 12, true)
	again.Phase = "ended"
	if got := tracker.next(again); got.Winner != "" {
		t.Fatalf("a new round kept the last winner %q without a score change", got.Winner)
	}
	plain, _ := NewRenderer("arena")
	if nodes := styledNodes(t, plain, won); nodes["score/ct-label"].Text != "CT" {
		t.Fatalf("without names or clan names the label = %q; want the side", nodes["score/ct-label"].Text)
	}
	if _, ok := styledNodes(t, plain, won)["round-win/team"]; ok {
		t.Fatal("the win banner drew without its switch")
	}
	clan := won
	clan.CTName = "NaVi"
	if got := plain.teamName(clan, "CT"); got != "NAVI" {
		t.Fatalf("clan fallback = %q", got)
	}
}
