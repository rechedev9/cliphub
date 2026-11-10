package tacticalplan

import "github.com/rechedev9/cliphub/internal/placename"

// MapRoleAllroundLabel names a player with no main role or callout on a side.
const MapRoleAllroundLabel = "Todoterreno"

// siteRoleLabels name the roles tied to one bombsite; they read before the callout.
var siteRoleLabels = map[string]string{
	PositionAnchorA: "Ancla A",
	PositionAnchorB: "Ancla B",
	PositionLurkA:   "Lurk de A",
	PositionLurkB:   "Lurk de B",
}

// SideMapRole is a player's role on one side of one map, read with the
// Ranking's rule: the role, the callout held (or allround) and its label.
type SideMapRole struct {
	Role    string `json:"role"`
	MapRole string `json:"map_role"`
	Label   string `json:"label"`
}

// PlayerMapRoles is a player's SideMapRole per side; nil on a side with too few sampled rounds.
type PlayerMapRoles struct {
	CT *SideMapRole `json:"ct,omitempty"`
	T  *SideMapRole `json:"t,omitempty"`
}

// On is the role on one side, or nil.
func (r PlayerMapRoles) On(side Side) *SideMapRole {
	if side == SideCT {
		return r.CT
	}
	return r.T
}

// sideMapRole labels a side: a site role first ("Lurk de B"), else the callout
// holding MapRoleShare of the rounds, else Todoterreno; nil on a small sample.
func (t RoleCounts) sideMapRole(side Side) *SideMapRole {
	role, mapRole := t.role(side), t.mapRole()
	if role == "" {
		return nil
	}
	label, ok := siteRoleLabels[role]
	switch {
	case ok:
	case mapRole == MapRoleAllround:
		label = MapRoleAllroundLabel
	default:
		label = placename.Label(mapRole)
	}
	return &SideMapRole{Role: role, MapRole: mapRole, Label: label}
}

// MapRoles is each player's role per side over the folded matches, by SteamID64;
// players without a read on either side are left out.
func (s *LeaderboardSample) MapRoles() map[string]PlayerMapRoles {
	out := map[string]PlayerMapRoles{}
	for steam, p := range s.players {
		r := PlayerMapRoles{CT: p.countsOn(SideCT).sideMapRole(SideCT), T: p.countsOn(SideT).sideMapRole(SideT)}
		if r.CT != nil || r.T != nil {
			out[steam] = r
		}
	}
	return out
}
