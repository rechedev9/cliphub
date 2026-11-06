package tacticalplan

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/rechedev9/cliphub/internal/placename"
)

// Tactic kinds, in the order the playbook lists them. A tactic is one fact a
// round shows; a key reads "kind:value", such as "zone:Middle".
const (
	TacticCategory      = "category"
	TacticSite          = "site"
	TacticBuy           = "buy"
	TacticOpponentBuy   = "opponent_buy"
	TacticOpening       = "opening"        // where the team's player stood in the opening duel
	TacticOpeningResult = "opening_result" // won or lost
	TacticAWP           = "awp"            // where the team's AWP stood at AWPStartSeconds
	TacticZone          = "zone"
	TacticUtility       = "utility" // value "smoke@Callout" or "molotov@Callout"
)

// TacticKinds lists the tactic kinds in display order.
var TacticKinds = []string{
	TacticCategory, TacticSite, TacticBuy, TacticOpponentBuy, TacticOpening, TacticOpeningResult, TacticAWP, TacticZone, TacticUtility,
}

// AWPStartSeconds is the second after freeze end the AWP's spot is read at.
const AWPStartSeconds = 12

// MaxTacticFilter caps the keys of one with or without list.
const MaxTacticFilter = 20

var tacticKeyPattern = regexp.MustCompile(`^[a-z_]+:[A-Za-z0-9_@.\-]{1,80}$`)

// CheckTacticKeys validates a with or without list.
func CheckTacticKeys(name string, keys []string) error {
	if len(keys) > MaxTacticFilter {
		return fmt.Errorf("%s takes at most %d tactics", name, MaxTacticFilter)
	}
	for _, key := range keys {
		kind, _, _ := strings.Cut(key, ":")
		if !tacticKeyPattern.MatchString(key) || !slices.Contains(TacticKinds, kind) {
			return fmt.Errorf("%s: %q is not a tactic key such as zone:Middle", name, key)
		}
	}
	return nil
}

func tacticKey(kind, value string) string { return kind + ":" + value }

// PlaybookTactic is the rounds that show one tactic. Share is over the rounds
// the combination kept.
type PlaybookTactic struct {
	Key  string `json:"key"`
	Kind string `json:"kind"`
	// Value is the raw value: a category, site, buy, callout, won or lost.
	Value string `json:"value"`
	// Grenade is smoke or molotov on a utility tactic.
	Grenade string `json:"grenade,omitempty"`
	// Label is the callout's name on tactics read at a place.
	Label string `json:"label,omitempty"`
	PlaybookGroup
}

// PlaybookCombo is the combination a playbook is narrowed to: a round stays
// when it shows every With tactic and no Without tactic.
type PlaybookCombo struct {
	With    []string `json:"with"`
	Without []string `json:"without"`
}

func (c PlaybookCombo) keeps(tactics []string) bool {
	for _, key := range c.With {
		if !slices.Contains(tactics, key) {
			return false
		}
	}
	for _, key := range c.Without {
		if slices.Contains(tactics, key) {
			return false
		}
	}
	return true
}

// roundTactics lists every tactic of a played round, without repeats.
func (p playbookRead) roundTactics(row PlaybookRound, frames []Frame) []string {
	out := []string{
		tacticKey(TacticCategory, row.Category),
		tacticKey(TacticSite, string(row.Site)),
	}
	if row.Buy != "" {
		out = append(out, tacticKey(TacticBuy, string(row.Buy)))
	}
	if row.OpponentBuy != "" {
		out = append(out, tacticKey(TacticOpponentBuy, string(row.OpponentBuy)))
	}
	if ev, ok := openingEvent(p.round); ok {
		won := ev.ActorSlot != nil && p.team[*ev.ActorSlot]
		if place := duelPlace(p.doc, ev, won); place != "" {
			out = append(out, tacticKey(TacticOpening, place))
		}
		result := "lost"
		if won {
			result = "won"
		}
		out = append(out, tacticKey(TacticOpeningResult, result))
	}
	for _, callout := range p.awpSpots(frames) {
		out = append(out, tacticKey(TacticAWP, callout))
	}
	for _, hold := range p.zones {
		out = append(out, tacticKey(TacticZone, hold.Zone))
	}
	out = append(out, p.keyUtility()...)
	return uniqueKeys(out)
}

// awpSpots names where each alive player of the team carrying an AWP stood on
// the first frame at or after AWPStartSeconds. Blobs without gear read none.
func (p playbookRead) awpSpots(frames []Frame) []string {
	awp := EquipmentByName(awpWeapon)
	for _, f := range frames {
		secs, ok := secondsSince(p.round, f.Tick, p.doc.Demo.Tickrate)
		if !ok || secs < AWPStartSeconds {
			continue
		}
		out := []string{}
		for _, s := range f.Samples {
			if !p.team[s.Slot] || !s.Flags.Has(FlagAlive) || !carries(s.Gear, awp) {
				continue
			}
			if callout := NearestCallout(p.doc.Geometry, [3]float64{s.X, s.Y, s.Z}); callout != "" {
				out = append(out, callout)
			}
		}
		return out
	}
	return nil
}

func carries(g *Gear, item Equipment) bool {
	if g == nil {
		return false
	}
	if g.ActiveWeapon == item {
		return true
	}
	return slices.ContainsFunc(g.Inventory, func(it InventoryItem) bool { return it.Item == item })
}

// keyUtility is the smokes and molotovs the team landed before the plant or
// the round end, one tactic per grenade kind and landing callout.
func (p playbookRead) keyUtility() []string {
	r := p.round
	cutoff := PlayCutoffTick(r)
	out := []string{}
	for _, ev := range r.Events {
		if (ev.Kind != EventSmoke && ev.Kind != EventMolotov) || ev.ActorSlot == nil || !p.team[*ev.ActorSlot] {
			continue
		}
		if ev.Tick < r.TickFreezeEnd || (cutoff > 0 && ev.Tick > cutoff) {
			continue
		}
		if callout := NearestCallout(p.doc.Geometry, ev.Pos); callout != "" {
			out = append(out, tacticKey(TacticUtility, string(ev.Kind)+"@"+callout))
		}
	}
	return out
}

func uniqueKeys(keys []string) []string {
	seen := make(map[string]bool, len(keys))
	out := keys[:0]
	for _, key := range keys {
		if !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	return out
}

// tacticOf describes a key: its kind, value, grenade and place label.
func tacticOf(key string) PlaybookTactic {
	kind, value, _ := strings.Cut(key, ":")
	t := PlaybookTactic{Key: key, Kind: kind, Value: value}
	switch kind {
	case TacticUtility:
		grenade, callout, _ := strings.Cut(value, "@")
		t.Grenade, t.Value, t.Label = grenade, callout, placename.Label(callout)
	case TacticOpening, TacticAWP, TacticZone:
		t.Label = placename.Label(value)
	}
	return t
}

// sortedTactics lists the groups by kind order, then most rounds first.
func sortedTactics(groups map[string]*refGroup, total int) []PlaybookTactic {
	out := make([]PlaybookTactic, 0, len(groups))
	for key, g := range groups {
		t := tacticOf(key)
		t.PlaybookGroup = g.result(total)
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		ki, kj := slices.Index(TacticKinds, out[i].Kind), slices.Index(TacticKinds, out[j].Kind)
		if ki != kj {
			return ki < kj
		}
		if out[i].Rounds != out[j].Rounds {
			return out[i].Rounds > out[j].Rounds
		}
		return out[i].Key < out[j].Key
	})
	return out
}
