package renderplan

import (
	"fmt"
	"strings"

	"github.com/rechedev9/cliphub/internal/playercard"
)

func normalizePlayerCards(req EditRequest) EditRequest {
	for _, slot := range []*string{&req.OpeningCard, &req.EndCard, &req.CoverCard} {
		*slot = strings.ToLower(strings.TrimSpace(*slot))
	}
	if req.EndCard != "" {
		req.StatsCard = false
	}
	return req
}

// validatePlayerCards keeps each slot on a catalog template and the cards on a 9:16 Short of plays.
func (r EditRequest) validatePlayerCards() error {
	chosen := false
	for _, slot := range []struct{ field, id string }{{"opening_card", r.OpeningCard}, {"end_card", r.EndCard}, {"cover_card", r.CoverCard}} {
		if slot.id == "" {
			continue
		}
		if _, ok := playercard.Lookup(slot.id); !ok {
			return fmt.Errorf("unknown %s %q; valid ids: %s", slot.field, slot.id, strings.Join(playercard.IDs(), ", "))
		}
		chosen = true
	}
	switch {
	case !chosen:
		return nil
	case r.Format != FormatShort9x16:
		return fmt.Errorf("player cards need the %s format", FormatShort9x16)
	case r.MatchRecap || r.FullDemo != nil:
		return fmt.Errorf("player cards are for a Short of plays")
	case r.CoverCard != "" && r.CoverStrategy == CoverStrategyNone:
		return fmt.Errorf("cover_card needs a cover; drop %s", CoverStrategyNone)
	}
	return nil
}
