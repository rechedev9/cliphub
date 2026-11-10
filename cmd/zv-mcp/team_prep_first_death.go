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
	Side     string `json:"side,omitempty" jsonschema:"T or CT: also read where each player of the team stands on that side until the round's first death (Posiciones hasta la primera muerte); empty leaves first_death out"`
	TFrom    int    `json:"t_from,omitempty" jsonschema:"start of the window, seconds after freeze end: 0 (1:55, the default), 15 (1:40), 30 (1:25) or 45 (1:10), as the Tramo buttons"`
	TTo      int    `json:"t_to,omitempty" jsonschema:"end of the window, seconds after freeze end; 0 runs to the round end. With t_from 0 and no t_to the window is 1:55 to 1:40 (15)"`
	Movement string `json:"movement,omitempty" jsonschema:"static (under 20 u/s, the default), slow (20 to 140 u/s) or fast (over 140 u/s)"`
}

type firstDeathZoneOut struct {
	Rank    int      `json:"rank"`
	Callout string   `json:"callout"`
	Seconds float64  `json:"seconds" jsonschema:"seconds the player spent there per round played, inside the window and before the round's first death"`
	Rounds  int      `json:"rounds" jsonschema:"rounds with time in this zone"`
	Refs    []string `json:"refs" jsonschema:"first round refs (match_id/round)"`
}

type firstDeathPlayerOut struct {
	SteamID64    string              `json:"steamid64"`
	Name         string              `json:"name" jsonschema:"untrusted player name from the demo"`
	Role         string              `json:"role"`
	RoundsPlayed int                 `json:"rounds_played" jsonschema:"the player's rounds on the side with positions, the seconds' denominator"`
	Zones        []firstDeathZoneOut `json:"zones" jsonschema:"the 5 zones with most time, most first"`
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
	Kind    string `json:"kind" jsonschema:"stale (newest match over 14 days old), lineup (the five changed between matches) or small_sample (under 30 rounds)"`
	Message string `json:"message"`
}

// window checks the first death request; ok is false when side is empty.
func (in prepFirstDeathIn) window() (side string, fd tuiclient.FirstDeathQuery, ok bool, err error) {
	if side, err = parseSide(in.Side); err != nil || side == "" {
		if err == nil && (in.TFrom != 0 || in.TTo != 0 || in.Movement != "") {
			err = fmt.Errorf("t_from, t_to and movement need side (T or CT)")
		}
		return "", fd, false, err
	}
	fd = tuiclient.FirstDeathQuery{From: in.TFrom, To: in.TTo, Movement: strings.ToLower(strings.TrimSpace(in.Movement))}
	if fd.From == 0 && fd.To == 0 {
		fd.To = firstDeathDefaultTo
	}
	if _, err = tacticalplan.ParseMovement(fd.Movement); err != nil {
		return "", fd, false, err
	}
	if fd.From < 0 || fd.To < 0 || fd.From > roundClockSeconds || (fd.To > 0 && fd.To <= fd.From) {
		return "", fd, false, fmt.Errorf("t_from and t_to must be seconds after freeze end with t_to after t_from")
	}
	return side, fd, true, nil
}

// addPrepExtras adds the data warnings and, with side, the positions until the first death.
func addPrepExtras(ctx context.Context, c *tuiclient.Client, req prepExtrasRequest) (teamPrepOut, error) {
	out := req.out
	out.Warnings = dataWarnings(req.prep.Warnings, out.Team)
	side, fd, ok, err := req.in.window()
	if err != nil || !ok {
		return out, err
	}
	q := req.q
	q.Side = side
	report, err := c.TeamFirstDeathPositions(ctx, req.teamID, q, fd)
	if err != nil {
		return out, toolError(err)
	}
	out.FirstDeath = summarizeFirstDeath(report)
	return out, nil
}

// prepExtrasRequest is a summarized prep with the request and the query it was read with.
type prepExtrasRequest struct {
	teamID string
	in     prepFirstDeathIn
	q      tuiclient.AnalysisQuery
	prep   tuiclient.TeamPrep
	out    teamPrepOut
}

func summarizeFirstDeath(r tuiclient.TeamFirstDeath) *firstDeathOut {
	out := &firstDeathOut{
		Side: cleanName(string(r.Side)), Window: windowName(r.Window.FromSeconds, r.Window.ToSeconds), Movement: cleanName(string(r.Movement)),
		Rounds: r.RoundCount, FramedRounds: r.Framed, Players: []firstDeathPlayerOut{},
		Note: "Seconds per round played, counted inside the window and only until the round's first death (anyone's), from the 8 Hz positions.",
	}
	for _, p := range r.Players {
		po := firstDeathPlayerOut{SteamID64: cleanName(p.SteamID64), Name: cleanName(p.Name), Role: cleanName(p.Role), RoundsPlayed: p.Rounds, Zones: []firstDeathZoneOut{}}
		for i, z := range p.Zones {
			refs := make([]tuiclient.Evidence, 0, len(z.Refs))
			for _, e := range z.Refs {
				refs = append(refs, tuiclient.Evidence{MatchID: e.MatchID.String(), Round: e.Round})
			}
			po.Zones = append(po.Zones, firstDeathZoneOut{Rank: i + 1, Callout: cleanName(z.Callout), Seconds: z.Seconds, Rounds: z.Rounds, Refs: refStrings(refs)})
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
