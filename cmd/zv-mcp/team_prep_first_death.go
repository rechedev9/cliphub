package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// firstDeathDefaultTo is the end of the default window, 1:55 to 1:40 on the round clock.
const firstDeathDefaultTo = 15

// prepFirstDeathIn asks team_prep for one side's positions until the round's first death.
type prepFirstDeathIn struct {
	Side     string `json:"side,omitempty" jsonschema:"T or CT, the scope side of Preparar's bar: the warnings (and prepare_team's scouting) count that side's rounds, and first_death reads where each player stands on it until the round's first death; empty counts both sides and leaves first_death out. The prep itself always reads both sides"`
	TFrom    int    `json:"t_from,omitempty" jsonschema:"start of the window, seconds after freeze end: 0 (1:55, the default), 15 (1:40), 30 (1:25) or 45 (1:10), as the Tramo buttons"`
	TTo      int    `json:"t_to,omitempty" jsonschema:"end of the window, seconds after freeze end; 0 runs to the round end. With t_from 0 and no t_to the window is 1:55 to 1:40 (15)"`
	Movement string `json:"movement,omitempty" jsonschema:"static (under 20 u/s, the default), slow (20 to 140 u/s) or fast (over 140 u/s)"`
}

type firstDeathZoneOut struct {
	Rank    int      `json:"rank"`
	Callout string   `json:"callout"`
	Seconds float64  `json:"seconds" jsonschema:"seconds the player spent there per round with positions, inside the window and before the round's first death"`
	Rounds  int      `json:"rounds" jsonschema:"rounds with time in this zone"`
	Refs    []string `json:"refs" jsonschema:"first round refs (match_id/round)"`
	linkOut
}

type firstDeathPlayerOut struct {
	SteamID64           string              `json:"steamid64"`
	Name                string              `json:"name" jsonschema:"untrusted player name from the demo"`
	Role                string              `json:"role" jsonschema:"library role: awp, entry, support, rifler or unknown"`
	MapRole             *mapRoleOut         `json:"map_role,omitempty" jsonschema:"the role on this side of the map as the Ranking reads it, such as Lurk de B or Todoterreno"`
	RoundsWithPositions int                 `json:"rounds_with_positions" jsonschema:"the player's rounds on the side with positions, the seconds' denominator"`
	Zones               []firstDeathZoneOut `json:"zones" jsonschema:"the 5 zones with most time, most first"`
}

type firstDeathOut struct {
	Side         string                `json:"side"`
	Window       string                `json:"window" jsonschema:"on the round clock"`
	Movement     string                `json:"movement"`
	Rounds       int                   `json:"rounds"`
	FramedRounds int                   `json:"framed_rounds" jsonschema:"rounds with positions, which the players are read from"`
	Note         string                `json:"note"`
	Players      []firstDeathPlayerOut `json:"players"`
}

// dataWarningOut is one reason to distrust the preparation, as Preparar shows it.
type dataWarningOut struct {
	Kind    string `json:"kind" jsonschema:"stale (the team's newest analysed match, whatever the filters, over 14 days old), lineup (the five of N of the scope's matches, before the lineup filter, differ from the lineup or the usual five) or small_sample (under 30 rounds in the scope)"`
	Message string `json:"message"`
}

// firstDeathWindow is a checked prepFirstDeathIn; side is empty when no side was asked.
type firstDeathWindow struct {
	side     string
	from, to int
	movement string
}

// check reads the scope side and the first death window.
func (in prepFirstDeathIn) check() (firstDeathWindow, error) {
	side, err := parseSide(in.Side)
	if err != nil {
		return firstDeathWindow{}, err
	}
	if side == "" {
		if in.TFrom != 0 || in.TTo != 0 || in.Movement != "" {
			return firstDeathWindow{}, fmt.Errorf("t_from, t_to and movement need side (T or CT)")
		}
		return firstDeathWindow{}, nil
	}
	w := firstDeathWindow{side: side, from: in.TFrom, to: in.TTo, movement: strings.ToLower(strings.TrimSpace(in.Movement))}
	if w.from == 0 && w.to == 0 {
		w.to = firstDeathDefaultTo
	}
	if _, err = tacticalplan.ParseMovement(w.movement); err != nil {
		return firstDeathWindow{}, err
	}
	if w.from < 0 || w.to < 0 || w.from > roundClockSeconds || (w.to > 0 && w.to <= w.from) {
		return firstDeathWindow{}, fmt.Errorf("t_from and t_to must be seconds after freeze end with t_to after t_from")
	}
	return w, nil
}

// prepExtrasRequest is a summarized prep with the team, the checked window and the scope it was read with.
type prepExtrasRequest struct {
	teamID string
	window firstDeathWindow
	q      tuiclient.AnalysisQuery
	out    teamPrepOut
}

// addPrepExtras adds the data warnings over the scope (its side included) and,
// with a side, the positions until the first death.
func addPrepExtras(ctx context.Context, c *tuiclient.Client, req prepExtrasRequest) (teamPrepOut, error) {
	out := req.out
	q := req.q
	q.Side = req.window.side
	warnings, err := c.TeamDataWarnings(ctx, tuiclient.TeamWarningsRequest{TeamID: req.teamID, Query: q})
	if err != nil {
		return out, toolError(err)
	}
	out.Warnings = dataWarnings(warnings, out.Team)
	if req.window.side == "" {
		return out, nil
	}
	report, err := c.TeamFirstDeathPositions(ctx, tuiclient.FirstDeathRequest{
		TeamID: req.teamID, Query: q, From: req.window.from, To: req.window.to, Movement: req.window.movement,
	})
	if err != nil {
		return out, toolError(err)
	}
	out.FirstDeath = summarizeFirstDeath(report)
	return out, nil
}

func summarizeFirstDeath(r tuiclient.TeamFirstDeath) *firstDeathOut {
	out := &firstDeathOut{
		Side: cleanName(string(r.Side)), Window: windowName(r.Window.FromSeconds, r.Window.ToSeconds), Movement: cleanName(string(r.Movement)),
		Rounds: r.RoundCount, FramedRounds: r.Framed, Players: []firstDeathPlayerOut{},
		Note: "Seconds per round with positions, counted inside the window and only until the round's first death (anyone's), from the 8 Hz positions.",
	}
	team := cleanName(r.TeamName)
	for _, p := range r.Players {
		po := firstDeathPlayerOut{SteamID64: cleanName(p.SteamID64), Name: cleanName(p.Name), Role: cleanName(p.Role), MapRole: mapRoleOf(r.Side, p.MapRole), RoundsWithPositions: p.Rounds, Zones: []firstDeathZoneOut{}}
		for i, z := range p.Zones {
			refs := make([]tuiclient.Evidence, 0, len(z.Refs))
			for _, e := range z.Refs {
				refs = append(refs, tuiclient.Evidence{MatchID: e.MatchID.String(), Round: e.Round})
			}
			link := findingLink{title: joinTitle(team, po.Name, z.Callout), refs: refs, focus: []string{p.SteamID64}, tramo: windowTramo(r.Window.FromSeconds, r.Window.ToSeconds), zones: []string{z.Callout}}
			po.Zones = append(po.Zones, firstDeathZoneOut{Rank: i + 1, Callout: cleanName(z.Callout), Seconds: z.Seconds, Rounds: z.Rounds, Refs: refStrings(refs),
				linkOut: link.outFor(z.Rounds)})
		}
		out.Players = append(out.Players, po)
	}
	return out
}

func dataWarnings(warnings []tacticalplan.DataWarning, team string) []dataWarningOut {
	out := []dataWarningOut{}
	for _, w := range warnings {
		var msg string
		switch w.Kind {
		case tacticalplan.DataWarningStale:
			msg = fmt.Sprintf("The last match of %s was %d days ago: they may have changed their playbook.", team, w.Days)
		case tacticalplan.DataWarningLineup:
			msg = fmt.Sprintf("Different lineup in %d of %d matches.", w.Changed, w.Matches)
		case tacticalplan.DataWarningSmallSample:
			msg = fmt.Sprintf("Small sample: %d rounds.", w.Rounds)
		default:
			continue
		}
		out = append(out, dataWarningOut{Kind: string(w.Kind), Message: msg})
	}
	return out
}
