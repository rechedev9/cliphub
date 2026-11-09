package tacticalplan

import (
	"cmp"
	"slices"

	"github.com/google/uuid"
)

// What a team does after a grenade is read in the GrenadeAfterSeconds after it
// went off; a player is in a callout once they stay there GrenadeStaySeconds.
const (
	GrenadeAfterSeconds = 20
	GrenadeStaySeconds  = 3
)

// GrenadeAfter is what the throwers' team did in the GrenadeAfterSeconds after
// a grenade went off.
type GrenadeAfter struct {
	// Zones is each callout a player of the team stayed in, by name; empty without positions.
	Zones []string `json:"zones"`
	// Fight is where the team's player stood in the team's first duel of the window.
	Fight string `json:"fight,omitempty"`
}

// afterRead is the round a grenade was thrown in, with the throwers' track.
type afterRead struct {
	doc   Document
	round Round
	team  map[uint8]bool
	track []teamFrame
}

func grenadeAfter(a afterRead, tick int) GrenadeAfter {
	out := GrenadeAfter{Zones: []string{}}
	from, ok := secondsSince(a.round, tick, a.doc.Demo.Tickrate)
	if !ok {
		return out
	}
	last := tick + int(GrenadeAfterSeconds*a.doc.Demo.Tickrate)
	first := -1
	for _, ev := range a.round.Events {
		if ev.Kind != EventKill || ev.Tick < tick || ev.Tick > last || (first >= 0 && ev.Tick >= first) {
			continue
		}
		won := ev.ActorSlot != nil && a.team[*ev.ActorSlot]
		if won || (ev.TargetSlot != nil && a.team[*ev.TargetSlot]) {
			first, out.Fight = ev.Tick, duelPlace(a.doc, ev, won)
		}
	}
	out.Zones = stayedZones(a.track, from, from+GrenadeAfterSeconds)
	return out
}

// stayedZones lists the callouts, out of spawn, a player of the track stayed in
// for GrenadeStaySeconds between from and to.
func stayedZones(track []teamFrame, from, to float64) []string {
	type stay struct {
		callout     string
		start, last float64
	}
	current := map[uint8]stay{}
	seen := map[string]bool{}
	for _, f := range track {
		if f.seconds < from {
			continue
		}
		if f.seconds > to {
			break
		}
		for slot, callout := range f.callouts {
			s, ok := current[slot]
			if !ok || s.callout != callout {
				s = stay{callout: callout, start: f.seconds}
			}
			s.last = f.seconds
			current[slot] = s
			if s.last-s.start >= GrenadeStaySeconds && !isSpawnZone(callout) {
				seen[callout] = true
			}
		}
		for slot := range current {
			if _, alive := f.callouts[slot]; !alive {
				delete(current, slot)
			}
		}
	}
	out := make([]string, 0, len(seen))
	for callout := range seen {
		out = append(out, callout)
	}
	slices.Sort(out)
	return out
}

// Kinds of a follow-up fact.
const (
	FollowUpPlay  = "play"  // the round's category and site
	FollowUpFight = "fight" // where the first duel after the grenade was
	FollowUpZone  = "zone"  // a callout a player stayed in after the grenade
)

var followUpOrder = []string{FollowUpPlay, FollowUpFight, FollowUpZone}

// FollowUpFact is one thing the team did after a grenade, in how many of its
// rounds and how many of those it won.
type FollowUpFact struct {
	Kind string `json:"kind"`
	// Value is a callout, or for a play GrenadeRound.Play.
	Value  string `json:"value"`
	Rounds int    `json:"rounds"`
	Won    int    `json:"won"`
}

// FollowUp is what a team did after one grenade: every fact counts over the
// same Rounds, the rounds the grenade was thrown in.
type FollowUp struct {
	Rounds int            `json:"rounds"`
	Won    int            `json:"won"`
	Facts  []FollowUpFact `json:"facts"`
}

// Play is the round's category and site as "execute@A", the category alone
// without a site, or empty for a round the playbook could not read.
func (r GrenadeRound) Play() string {
	if r.Category == "" || r.Site == "" || r.Site == SiteNone {
		return r.Category
	}
	return r.Category + "@" + string(r.Site)
}

type roundKey struct {
	match uuid.UUID
	round int
}

// FollowUpOf counts what the team did after the grenades, once per round: a
// round thrown twice reads its first throw.
func FollowUpOf(grenades []TeamGrenade, rounds []GrenadeRound) FollowUp {
	byRound := make(map[roundKey]GrenadeRound, len(rounds))
	for _, r := range rounds {
		byRound[roundKey{r.MatchID, r.Round}] = r
	}
	seen := map[roundKey]bool{}
	facts := map[[2]string]*FollowUpFact{}
	out := FollowUp{Facts: []FollowUpFact{}}
	add := func(kind, value string, won bool) {
		if value == "" {
			return
		}
		f, ok := facts[[2]string{kind, value}]
		if !ok {
			f = &FollowUpFact{Kind: kind, Value: value}
			facts[[2]string{kind, value}] = f
		}
		f.Rounds++
		if won {
			f.Won++
		}
	}
	for _, g := range grenades {
		key := roundKey{g.MatchID, g.Round}
		if seen[key] {
			continue
		}
		seen[key] = true
		r, ok := byRound[key]
		if !ok {
			r = GrenadeRound{Won: g.RoundWon}
		}
		out.Rounds++
		if r.Won {
			out.Won++
		}
		add(FollowUpPlay, r.Play(), r.Won)
		add(FollowUpFight, g.After.Fight, r.Won)
		for _, zone := range g.After.Zones {
			add(FollowUpZone, zone, r.Won)
		}
	}
	for _, f := range facts {
		out.Facts = append(out.Facts, *f)
	}
	slices.SortFunc(out.Facts, func(a, b FollowUpFact) int {
		return cmp.Or(cmp.Compare(b.Rounds, a.Rounds), cmp.Compare(slices.Index(followUpOrder, a.Kind), slices.Index(followUpOrder, b.Kind)), cmp.Compare(a.Value, b.Value))
	})
	return out
}
