package moments

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Sides as the kill plan writes them in team_at_kill.
const (
	SideCT = "CT"
	SideT  = "T"
)

// Kinds a filter can ask for; each one is also a reason code of the moment.
const (
	KindClutch    = "clutch"
	KindAce       = "ace"
	KindMultiKill = "multi_kill"
)

// FilterKinds lists every kind a filter accepts, in the order errors name them.
var FilterKinds = []string{KindClutch, ReasonClutchWon, KindAce, KindMultiKill, ReasonFastDouble, ReasonFastTriple, ReasonFastQuad, ReasonFastAce,
	ReasonNoScope, ReasonThroughSmoke, ReasonAttackerBlind}

var weaponPattern = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)

// Filter narrows a list of moments. Zero fields match everything.
type Filter struct {
	// Weapon keeps the moments whose every kill used it, such as "awp".
	Weapon string
	// Kinds keeps the moments with any of these reason codes.
	Kinds []string
	// Side keeps the moments the player made on CT or on T.
	Side string
	// Player keeps one player's moments of a multi-POV job, by SteamID64 or name.
	Player string
}

// ParseFilter checks and normalizes raw filter values from a request.
func ParseFilter(weapon string, kinds []string, side string) (Filter, error) {
	f := Filter{Weapon: normalizeWeapon(weapon), Side: strings.ToUpper(strings.TrimSpace(side))}
	if f.Weapon != "" && !weaponPattern.MatchString(f.Weapon) {
		return Filter{}, fmt.Errorf("weapon %q is not a weapon id such as awp or ak47", weapon)
	}
	if f.Side != "" && f.Side != SideCT && f.Side != SideT {
		return Filter{}, fmt.Errorf("side must be %s or %s", SideCT, SideT)
	}
	for _, kind := range kinds {
		kind = strings.ToLower(strings.TrimSpace(kind))
		if kind == "" {
			continue
		}
		if !slices.Contains(FilterKinds, kind) {
			return Filter{}, fmt.Errorf("kind must be one of %s", strings.Join(FilterKinds, ", "))
		}
		if !slices.Contains(f.Kinds, kind) {
			f.Kinds = append(f.Kinds, kind)
		}
	}
	return f, nil
}

// Empty reports whether the filter keeps every moment.
func (f Filter) Empty() bool {
	return f.Weapon == "" && len(f.Kinds) == 0 && f.Side == "" && f.Player == ""
}

// Match reports whether one moment passes the filter.
func (f Filter) Match(m Moment) bool {
	if f.Side != "" && m.Side != f.Side {
		return false
	}
	if f.Player != "" && m.SteamID64 != f.Player && !strings.EqualFold(m.Player, f.Player) {
		return false
	}
	if f.Weapon != "" {
		weapons := m.Events.Weapons
		if len(weapons) == 0 {
			weapons = m.Weapons
		}
		if len(weapons) == 0 {
			return false
		}
		for _, w := range weapons {
			if normalizeWeapon(w) != f.Weapon {
				return false
			}
		}
	}
	if len(f.Kinds) > 0 && !slices.ContainsFunc(f.Kinds, func(kind string) bool { return slices.Contains(m.ReasonCodes, kind) }) {
		return false
	}
	return true
}

// Apply returns the moments that pass the filter, in their order.
func Apply(list []Moment, f Filter) []Moment {
	if f.Empty() {
		return list
	}
	out := make([]Moment, 0, len(list))
	for _, m := range list {
		if f.Match(m) {
			out = append(out, m)
		}
	}
	return out
}

func normalizeWeapon(weapon string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(weapon)), "weapon_")
}
