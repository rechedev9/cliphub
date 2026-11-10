package community

import (
	"regexp"
	"strconv"
	"strings"
)

// Veto actions of a pro series, the same words the library series uses.
const (
	ProVetoBan     = "ban"
	ProVetoPick    = "pick"
	ProVetoDecider = "decider"
)

// ProSidecarVeto is one veto line of the HLTV match page as the harvester reads it:
// the team's HLTV name (empty for the map left over), the action and the HLTV map name.
type ProSidecarVeto struct {
	Team   string `json:"team,omitempty"`
	Action string `json:"action"`
	Map    string `json:"map"`
}

// ProVetoStep is one veto step of the catalog. TeamID is 0 for the decider and
// for a team name that matches neither team of the match.
type ProVetoStep struct {
	TeamID int    `json:"team_id,omitempty"`
	Team   string `json:"team,omitempty"`
	Action string `json:"action"`
	Map    string `json:"map"`
}

// ProSeriesMap is one map of the series in play order: picked by a team, the decider, or neither when the veto is unknown.
type ProSeriesMap struct {
	Number   int    `json:"number"`
	Map      string `json:"map"`
	PickedBy int    `json:"picked_by,omitempty"`
	Decider  bool   `json:"decider,omitempty"`
}

// ProSeries is the series of an HLTV match. Format is bo1, bo2, bo3 or bo5, or
// empty when neither the page nor the map count tells it.
type ProSeries struct {
	Format string         `json:"format,omitempty"`
	Maps   []ProSeriesMap `json:"maps"`
	Veto   []ProVetoStep  `json:"veto"`
}

var proFormatPattern = regexp.MustCompile(`^bo[1-9]$`)

// ProMapName is the demo name of an HLTV map name, such as de_dust2 for "Dust2".
func ProMapName(hltv string) string {
	name := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			return r
		case r >= 'A' && r <= 'Z':
			return r + 'a' - 'A'
		}
		return -1
	}, hltv)
	if name == "" || strings.HasPrefix(name, "de_") || strings.HasPrefix(name, "cs_") {
		return name
	}
	return "de_" + name
}

// ProSeriesOf reads the series of a sidecar. Without the page's format, the map
// count gives it, since HLTV lists every map of a series, the unplayed decider too.
func ProSeriesOf(s ProSidecar) ProSeries {
	out := ProSeries{Format: strings.ToLower(s.Format), Maps: []ProSeriesMap{}, Veto: []ProVetoStep{}}
	maps := []string{}
	for _, m := range s.Maps {
		if name := ProMapName(m); name != "" {
			maps = append(maps, name)
		}
	}
	if !proFormatPattern.MatchString(out.Format) {
		out.Format = ""
		if n := len(maps); n == 1 || n == 2 || n == 3 || n == 5 {
			out.Format = "bo" + strconv.Itoa(n)
		}
	}
	out.Veto = proVeto(s)
	for i, m := range maps {
		sm := ProSeriesMap{Number: i + 1, Map: m}
		for _, v := range out.Veto {
			if v.Map == m && v.Action == ProVetoPick {
				sm.PickedBy = v.TeamID
			}
			if v.Map == m && v.Action == ProVetoDecider {
				sm.Decider = true
			}
		}
		// A bo1, bo3 or bo5 always ends on the map left over, the one map of a bo1 included.
		if len(out.Veto) == 0 && i == len(maps)-1 && out.Format == "bo"+strconv.Itoa(len(maps)) && len(maps)%2 == 1 {
			sm.Decider = true
		}
		out.Maps = append(out.Maps, sm)
	}
	return out
}

// proVeto reads the veto lines, each map once. When the names match only one of the two
// teams, a ban or pick whose name matches neither belongs to the other team.
func proVeto(s ProSidecar) []ProVetoStep {
	out := []ProVetoStep{}
	seen := map[string]bool{}
	matched := map[int]bool{}
	for _, v := range s.Veto {
		step := ProVetoStep{Action: strings.ToLower(v.Action), Map: ProMapName(v.Map)}
		switch {
		case step.Map == "" || seen[step.Map]:
			continue
		case step.Action == ProVetoDecider:
		case step.Action == ProVetoBan || step.Action == ProVetoPick:
			step.Team = strings.TrimSpace(v.Team)
			if i := proSidecarTeam(step.Team, s.Teams); i >= 0 {
				step.TeamID = s.Teams[i].ID
				matched[i] = true
			}
		default:
			continue
		}
		seen[step.Map] = true
		out = append(out, step)
	}
	if len(s.Teams) != 2 || len(matched) != 1 {
		return out
	}
	other := s.Teams[0]
	if matched[0] {
		other = s.Teams[1]
	}
	for i, step := range out {
		if step.Action != ProVetoDecider && step.TeamID == 0 {
			out[i].TeamID = other.ID
		}
	}
	return out
}

// proSidecarTeam is the index of the sidecar team a veto line names, or -1.
func proSidecarTeam(name string, teams []ProSidecarTeam) int {
	best, bestScore := -1, 0
	for i, t := range teams {
		if s := proNameScore(name, t.Name); s > bestScore {
			best, bestScore = i, s
		}
	}
	return best
}
