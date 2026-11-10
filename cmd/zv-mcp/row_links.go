package main

import (
	"strings"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// joinTitle writes the title of a link from its parts, leaving out the empty ones.
func joinTitle(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(cleanName(p)); p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, " · ")
}

// firstNonEmpty is the first list with items: a report's every round, else its examples.
func firstNonEmpty(lists ...[]tuiclient.Evidence) []tuiclient.Evidence {
	for _, l := range lists {
		if len(l) > 0 {
			return l
		}
	}
	return nil
}

// roundEvidence is the rounds a find_rounds answer lists, as link rounds.
func roundEvidence(rounds []roundRefOut) []tuiclient.Evidence {
	out := make([]tuiclient.Evidence, 0, len(rounds))
	for _, r := range rounds {
		out = append(out, tuiclient.Evidence{MatchID: r.MatchID, Round: r.Round})
	}
	return out
}

// roundsLink opens the rounds find_rounds lists, with the player in focus when it reads one.
func (in findRoundsIn) roundsLink(out findRoundsOut) linkOut {
	link := findingLink{title: joinTitle(out.Play, out.team), refs: roundEvidence(out.Rounds)}
	if link.title == "" {
		link.title = "Rondas"
	}
	if in.SteamID != "" {
		link.focus = []string{in.SteamID}
	}
	return link.out()
}
