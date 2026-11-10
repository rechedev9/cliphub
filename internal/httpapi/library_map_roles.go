package httpapi

import (
	"log"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// teamMapRoles reads each player's role per side on one map's matches with the
// Ranking's rule and its cached reads, so a card and a Ranking row agree.
func (h *Handlers) teamMapRoles(kept []keptMatch) map[string]tacticalplan.PlayerMapRoles {
	var sample tacticalplan.LeaderboardSample
	for _, k := range kept {
		doc := k.scout.Document
		e, err := h.cachedMatchRead(matchReadInput{match: k.match, doc: &doc, withRoles: true})
		if err != nil {
			log.Printf("httpapi: map roles skip match %s: %v", k.match.ID, err)
			continue
		}
		sample.Add(e.read, "")
	}
	return sample.MapRoles()
}
