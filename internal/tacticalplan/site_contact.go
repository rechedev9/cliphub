package tacticalplan

import "strings"

// SiteContactPlayers is how many alive players of a team in one bombsite's
// callouts make a site contact; one player there is a lurk, not a hit.
const SiteContactPlayers = 2

// SiteFromPlace classifies a CS2 place name. Only names that unambiguously
// identify a site count; "Apartments" is near A on Mirage but is not A.
func SiteFromPlace(place string) Site {
	name := strings.ToLower(strings.TrimSpace(place))
	if name == "" {
		return SiteNone
	}
	switch {
	case strings.Contains(name, "bombsite a"), strings.Contains(name, "bombsitea"),
		name == "site a", name == "a site":
		return SiteA
	case strings.Contains(name, "bombsite b"), strings.Contains(name, "bombsiteb"),
		name == "site b", name == "b site":
		return SiteB
	case name == "middle", name == "mid", strings.Contains(name, "midsection"):
		return SiteMid
	default:
		return SiteNone
	}
}

func isBombsite(site Site) bool { return site == SiteA || site == SiteB }

// sitePresence is the first tick at which SiteContactPlayers of the team stood
// together in each bombsite's callouts.
func sitePresence(track []teamFrame) map[Site]int {
	out := map[Site]int{}
	for _, f := range track {
		at := map[Site]int{}
		for _, callout := range f.callouts {
			if site := SiteFromPlace(callout); isBombsite(site) {
				at[site]++
			}
		}
		for site, n := range at {
			if _, seen := out[site]; !seen && n >= SiteContactPlayers {
				out[site] = f.tick
			}
		}
	}
	return out
}

// siteContact is when and where attackers first reached a bombsite: the earliest
// of presence, plant and a duel one of them fought there; else 0 and SiteNone.
func siteContact(doc Document, r Round, team map[uint8]bool, presence map[Site]int) (int, Site) {
	tick, site := 0, SiteNone
	consider := func(t int, s Site) {
		if t > 0 && isBombsite(s) && (tick == 0 || t < tick) {
			tick, site = t, s
		}
	}
	for s, t := range presence {
		consider(t, s)
	}
	if r.Bomb != nil && r.Bomb.PlantTick > 0 {
		consider(r.Bomb.PlantTick, r.Bomb.Site)
	}
	for _, ev := range r.Events {
		if ev.Kind != EventKill {
			continue
		}
		if ev.ActorSlot != nil && team[*ev.ActorSlot] {
			consider(ev.Tick, SiteFromPlace(duelPlace(doc, ev, true)))
		}
		if ev.TargetSlot != nil && team[*ev.TargetSlot] {
			consider(ev.Tick, SiteFromPlace(duelPlace(doc, ev, false)))
		}
	}
	return tick, site
}

// roundTempo is the tick a round's pace is read at: the site contact for the
// attackers, the round's first kill for the defenders. 0 means never.
func roundTempo(doc Document, r Round, side Side, team map[uint8]bool, presence map[Site]int) int {
	if side == SideCT {
		return r.Class.FirstContactTick
	}
	tick, _ := siteContact(doc, r, team, presence)
	return tick
}
