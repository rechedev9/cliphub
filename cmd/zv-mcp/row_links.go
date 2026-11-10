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

// scoutRound is a round of the scope the scouting report counted.
type scoutRound struct {
	ref   tuiclient.Evidence
	round tuiclient.TeamRound
}

func scoutRoundsOf(got tuiclient.TeamRounds) []scoutRound {
	var out []scoutRound
	for _, m := range got.Matches {
		for _, r := range m.Rounds {
			out = append(out, scoutRound{ref: tuiclient.Evidence{MatchID: m.MatchID, Round: r.Round}, round: r})
		}
	}
	return out
}

// addScoutLinks gives each scouting row the studio_path of its rounds, picked from the scope's
// rounds the way the report buckets them: a buy, a site, or a round shape on the shape's side.
func addScoutLinks(out *scoutTeamOut, got tuiclient.TeamRounds) {
	rounds := scoutRoundsOf(got)
	link := func(row *linkOut, total int, label string, keep func(tuiclient.TeamRound) bool) {
		var refs []tuiclient.Evidence
		for _, r := range rounds {
			if keep(r.round) {
				refs = append(refs, r.ref)
			}
		}
		*row = findingLink{title: joinTitle(out.Team, label), refs: refs}.outFor(total)
	}
	for i := range out.Buys {
		name := out.Buys[i].Name
		link(&out.Buys[i].linkOut, out.Buys[i].Rounds, "compra "+name, func(r tuiclient.TeamRound) bool { return r.Buy == name })
	}
	for i := range out.Sites {
		name := out.Sites[i].Name
		link(&out.Sites[i].linkOut, out.Sites[i].Rounds, "sitio "+name, func(r tuiclient.TeamRound) bool { return r.Site == name })
	}
	for _, side := range []struct {
		name string
		rows []shareOut
	}{{"T", out.TPatterns}, {"CT", out.CTPatterns}} {
		for i := range side.rows {
			name := side.rows[i].Name
			link(&side.rows[i].linkOut, side.rows[i].Rounds, side.name+" "+name, func(r tuiclient.TeamRound) bool { return r.Side == side.name && r.Pattern == name })
		}
	}
	for i := range out.Executes {
		e := &out.Executes[i]
		link(&e.linkOut, e.Rounds, "ejecuciones al sitio "+e.Site, func(r tuiclient.TeamRound) bool {
			return r.Side == "T" && (r.Pattern == "execute" || r.Pattern == "split") && r.Site == e.Site
		})
	}
}
