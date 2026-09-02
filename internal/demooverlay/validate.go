package demooverlay

import (
	"errors"
	"fmt"
	"strings"
)

// ValidateFACEITEnrichment enforces the Full Demo FACEIT contract before
// capture or render. Optional profile decoration such as avatar and ranking
// may be absent, but every roster member must resolve to a real FACEIT profile.
func ValidateFACEITEnrichment(roster Roster, enrichment map[string]Enrichment) error {
	if len(roster.Players) == 0 {
		return errors.New("FACEIT overlay requires a parsed roster")
	}
	var failures []error
	for _, player := range roster.Players {
		steamID := strings.TrimSpace(player.SteamID64)
		label := firstNonEmpty(player.Name, steamID, "unknown player")
		if steamID == "" {
			failures = append(failures, fmt.Errorf("%s has no SteamID64", label))
			continue
		}
		profile, ok := enrichment[steamID]
		if !ok {
			failures = append(failures, fmt.Errorf("%s (%s) has no FACEIT profile", label, steamID))
			continue
		}
		if strings.TrimSpace(profile.Nickname) == "" {
			failures = append(failures, fmt.Errorf("%s (%s) has no FACEIT nickname", label, steamID))
		}
		if profile.ELO <= 0 {
			failures = append(failures, fmt.Errorf("%s (%s) has no FACEIT ELO", label, steamID))
		}
		if profile.SkillLevel <= 0 {
			failures = append(failures, fmt.Errorf("%s (%s) has no FACEIT skill level", label, steamID))
		}
	}
	return errors.Join(failures...)
}
