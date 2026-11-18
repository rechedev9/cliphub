package main

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestShortOptionsCarryThePlayerCardSlots(t *testing.T) {
	edit, err := resolveEdit(shortChoices{playerCardChoices: playerCardChoices{OpeningCard: " Carta ", EndCard: "mvp", CoverCard: "ficha"}})
	if err != nil {
		t.Fatal(err)
	}
	if edit.OpeningCard != "carta" || edit.EndCard != "mvp" || edit.CoverCard != "ficha" || edit.StatsCard {
		t.Fatalf("slots: %+v", edit)
	}
	summary := describeShortRequest(tuiclient.GenerateRequest{Edit: edit})
	if !strings.Contains(summary, "cartas de jugador: apertura Carta, tarjeta final MVP, portada Ficha") {
		t.Fatalf("the approval summary hides the cards: %s", summary)
	}
	if plain, _ := resolveEdit(shortChoices{}); plain.OpeningCard != "" || plain.EndCard != "" || plain.CoverCard != "" {
		t.Fatalf("a Short has no card by default: %+v", plain)
	}
}

func TestShortOptionsRefuseAPlayerCardOutOfPlace(t *testing.T) {
	on, off := true, false
	for name, c := range map[string]shortChoices{
		"unknown template":    {playerCardChoices: playerCardChoices{OpeningCard: "fifa"}},
		"landscape":           {Format: "landscape-16x9", playerCardChoices: playerCardChoices{EndCard: "carta"}},
		"two closing cards":   {StatsCard: &on, playerCardChoices: playerCardChoices{EndCard: "mvp"}},
		"cover without cover": {Cover: &off, playerCardChoices: playerCardChoices{CoverCard: "ficha"}},
	} {
		if _, err := resolveEdit(c); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
