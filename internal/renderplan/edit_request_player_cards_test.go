package renderplan

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPlayerCardSlotsTravelInTheEditRequest(t *testing.T) {
	req := NormalizeEditRequest(EditRequest{OpeningCard: " Carta ", EndCard: "mvp", CoverCard: "ficha", StatsCard: true})
	if err := req.Validate(); err != nil {
		t.Fatal(err)
	}
	if req.OpeningCard != "carta" || req.EndCard != "mvp" || req.CoverCard != "ficha" {
		t.Fatalf("slots lost: %+v", req)
	}
	if req.StatsCard {
		t.Fatal("the end card should replace the text stats card")
	}
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"opening_card":"carta"`, `"end_card":"mvp"`, `"cover_card":"ficha"`} {
		if !strings.Contains(string(body), field) {
			t.Fatalf("%s missing from %s", field, body)
		}
	}
	if body, _ := json.Marshal(DefaultEditRequest()); strings.Contains(string(body), "opening_card") || strings.Contains(string(body), "end_card") || strings.Contains(string(body), "cover_card") {
		t.Fatalf("an edit without cards should carry no slot: %s", body)
	}
}

func TestPlayerCardSlotsAreOnlyForAOnePlayerShort(t *testing.T) {
	for name, req := range map[string]EditRequest{
		"unknown template":      {OpeningCard: "fifa"},
		"landscape":             {Format: FormatLandscape16x9, EndCard: "carta"},
		"match recap":           {MatchRecap: true, OpeningCard: "carta"},
		"cover without a cover": {CoverCard: "mvp", CoverStrategy: CoverStrategyNone},
	} {
		if err := NormalizeEditRequest(req).Validate(); err == nil {
			t.Errorf("%s: accepted %+v", name, req)
		}
	}
}
