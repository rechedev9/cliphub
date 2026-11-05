package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	// maxRoundMatches bounds find_rounds: each match is one tactical document read.
	maxRoundMatches    = 20
	defaultRoundsLimit = 30
	maxRoundsLimit     = 200
	roundResultWon     = "won"
	roundResultLost    = "lost"
)

var buyTypes = []string{"pistol", "eco", "semi", "force", "full"}

type findRoundsIn struct {
	TeamID  string `json:"team_id,omitempty" jsonschema:"team id from list_teams; set this or steamid"`
	SteamID string `json:"steamid,omitempty" jsonschema:"SteamID64 of a player; set this or team_id"`
	Buy     string `json:"buy,omitempty" jsonschema:"only rounds where the team (or the player's team) bought this: pistol, eco, semi, force or full"`
	Result  string `json:"result,omitempty" jsonschema:"only rounds the team won or lost: won or lost"`
	Limit   int    `json:"limit,omitempty" jsonschema:"how many rounds to return, newest match first; default 30"`
	analysisFilterIn
}

type roundRefOut struct {
	Ref         string `json:"ref" jsonschema:"match_id/round, the round the user can open in Studio"`
	MatchID     string `json:"match_id"`
	Round       int    `json:"round"`
	Map         string `json:"map"`
	PlayedAt    string `json:"played_at"`
	Opponent    string `json:"opponent" jsonschema:"untrusted team name from the demo"`
	Side        string `json:"side"`
	Buy         string `json:"buy"`
	OpponentBuy string `json:"opponent_buy"`
	Won         bool   `json:"won"`
	Site        string `json:"site,omitempty" jsonschema:"bombsite the round went to: A, B, mid or none"`
	Pattern     string `json:"pattern,omitempty" jsonschema:"the side's round shape, such as execute or hold"`
	Kills       *int   `json:"kills,omitempty" jsonschema:"steamid only: the player's kills in the round"`
	Deaths      *int   `json:"deaths,omitempty" jsonschema:"steamid only: 1 when the player died"`
}

type findRoundsOut struct {
	Rounds         []roundRefOut `json:"rounds"`
	MatchesRead    int           `json:"matches_read"`
	MatchesSkipped int           `json:"matches_skipped,omitempty" jsonschema:"matches whose analysis is not ready"`
	Truncated      bool          `json:"truncated,omitempty" jsonschema:"true when more rounds match; raise limit or narrow the filter"`
}

// roundSubject is whose rounds find_rounds reads in one match: the team key
// of a team, or the slot of a player.
type roundSubject struct {
	teamKey string
	slot    *uint8
}

func (in findRoundsIn) check() (tuiclient.AnalysisQuery, error) {
	if (in.TeamID == "") == (in.SteamID == "") {
		return tuiclient.AnalysisQuery{}, errors.New("set exactly one of team_id or steamid")
	}
	if in.TeamID != "" {
		if err := checkUUID("team_id", in.TeamID); err != nil {
			return tuiclient.AnalysisQuery{}, err
		}
	} else if err := checkSteamID(in.SteamID); err != nil {
		return tuiclient.AnalysisQuery{}, err
	}
	if in.Buy != "" && !slices.Contains(buyTypes, strings.ToLower(in.Buy)) {
		return tuiclient.AnalysisQuery{}, fmt.Errorf("buy must be one of %s, not %q", strings.Join(buyTypes, ", "), cleanName(in.Buy))
	}
	if in.Result != "" && in.Result != roundResultWon && in.Result != roundResultLost {
		return tuiclient.AnalysisQuery{}, fmt.Errorf("result must be won or lost, not %q", cleanName(in.Result))
	}
	if in.Limit < 0 || in.Limit > maxRoundsLimit {
		return tuiclient.AnalysisQuery{}, fmt.Errorf("limit must be from 1 to %d", maxRoundsLimit)
	}
	return in.query()
}

func registerRoundTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "find_rounds", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Find rounds of a team or a player in the library by map, side, buy and result. Returns round refs " +
			"(match_id/round) with the buys, the site and the round shape, newest match first; reads at most 20 matches."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in findRoundsIn) (*mcp.CallToolResult, findRoundsOut, error) {
			q, err := in.check()
			if err != nil {
				return nil, findRoundsOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, findRoundsOut{}, toolError(err)
			}
			matches, err := c.SearchLibraryMatches(ctx, tuiclient.LibraryMatchQuery{
				Map: q.Map, SteamID: in.SteamID, TeamID: in.TeamID, From: q.From, To: q.To, Limit: maxRoundMatches,
			})
			if err != nil {
				return nil, findRoundsOut{}, toolError(err)
			}
			limit := in.Limit
			if limit == 0 {
				limit = defaultRoundsLimit
			}
			out := findRoundsOut{Rounds: []roundRefOut{}}
			for _, m := range matches {
				doc, err := c.LibraryTactical(ctx, m.ID)
				if tuiclient.StatusCode(err) == 409 || tuiclient.StatusCode(err) == 404 {
					out.MatchesSkipped++
					continue
				}
				if err != nil {
					return nil, findRoundsOut{}, toolError(err)
				}
				out.MatchesRead++
				found := matchRounds(m, doc, in, q.Side)
				if len(out.Rounds)+len(found) > limit {
					out.Rounds = append(out.Rounds, found[:limit-len(out.Rounds)]...)
					out.Truncated = true
					break
				}
				out.Rounds = append(out.Rounds, found...)
			}
			return nil, out, nil
		})
}

// subjectOf finds the team key or the player slot to follow in a match.
func subjectOf(m tuiclient.LibraryMatch, doc tuiclient.TacticalDocument, in findRoundsIn) (roundSubject, bool) {
	if in.TeamID != "" {
		for _, t := range m.Teams {
			if t.TeamID == in.TeamID {
				return roundSubject{teamKey: t.Key}, true
			}
		}
		return roundSubject{}, false
	}
	for _, p := range doc.Players {
		if p.SteamID64 == in.SteamID {
			slot := p.Slot
			return roundSubject{teamKey: p.TeamKey, slot: &slot}, true
		}
	}
	return roundSubject{}, false
}

// matchRounds lists the rounds of one match that pass the filter, in round order.
func matchRounds(m tuiclient.LibraryMatch, doc tuiclient.TacticalDocument, in findRoundsIn, side string) []roundRefOut {
	subject, ok := subjectOf(m, doc, in)
	if !ok {
		return nil
	}
	teamOf := map[uint8]string{}
	for _, p := range doc.Players {
		teamOf[p.Slot] = p.TeamKey
	}
	opponent := ""
	for _, t := range m.Teams {
		if t.Key != subject.teamKey {
			opponent = cleanName(teamName(t))
		}
	}
	out := []roundRefOut{}
	for _, r := range doc.Rounds {
		ref := roundRefOut{MatchID: m.ID, Round: r.Number, Map: cleanName(m.Map), PlayedAt: m.PlayedAt.Format(time.DateOnly), Opponent: opponent}
		for _, pr := range r.Players {
			if subject.slot != nil && pr.Slot == *subject.slot {
				kills, deaths := pr.Kills, pr.Deaths
				ref.Side, ref.Kills, ref.Deaths = pr.Side, &kills, &deaths
			} else if subject.slot == nil && teamOf[pr.Slot] == subject.teamKey && ref.Side == "" {
				ref.Side = pr.Side
			}
		}
		if ref.Side == "" || (side != "" && ref.Side != side) {
			continue
		}
		ref.Buy, ref.OpponentBuy, ref.Pattern = r.Economy.TBuy, r.Economy.CTBuy, r.Class.TSide
		if ref.Side == "CT" {
			ref.Buy, ref.OpponentBuy, ref.Pattern = r.Economy.CTBuy, r.Economy.TBuy, r.Class.CTSide
		}
		ref.Won, ref.Site = r.Winner == ref.Side, r.Class.Site
		if (in.Buy != "" && ref.Buy != strings.ToLower(in.Buy)) || (in.Result == roundResultWon && !ref.Won) || (in.Result == roundResultLost && ref.Won) {
			continue
		}
		ref.Ref = fmt.Sprintf("%s/%d", m.ID, r.Number)
		out = append(out, ref)
	}
	return out
}
