package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	// maxRoundMatches is how many matches one find_rounds call reads, newest first:
	// the team scope's cap, so a team's rounds are the ones its Replay keeps.
	maxRoundMatches    = library.MaxReportMatches
	defaultRoundsLimit = 30
	maxRoundsLimit     = 200
	roundResultWon     = "won"
	roundResultLost    = "lost"
	// overMatchLimit is the skip reason of a team match past the scope's cap.
	overMatchLimit = "over_match_limit"
	// overReadLimit is the skip reason of a match past the server's 200 analyses read.
	overReadLimit = "over_read_limit"
)

var buyTypes = []string{"pistol", "eco", "semi", "force", "full"}

type findRoundsIn struct {
	TeamID  string `json:"team_id,omitempty" jsonschema:"team id from list_teams; set this or steamid (a Libreta play's own team when only play_id is set)"`
	SteamID string `json:"steamid,omitempty" jsonschema:"SteamID64 of a player; set this or team_id"`
	PlayID  string `json:"play_id,omitempty" jsonschema:"only the rounds of one play, the ones it opens in the Replay: a Libreta play id from list_plays (its example rounds), or a detected play's ref from detect_team_plays called with this same team_id and filter (every round of the group)"`
	Result  string `json:"result,omitempty" jsonschema:"only rounds the team won or lost: won or lost"`
	Limit   int    `json:"limit,omitempty" jsonschema:"how many rounds to return, newest match first; default 30, at most 200"`
	analysisFilterIn
	teamScopeIn
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
	Play           string        `json:"play,omitempty" jsonschema:"untrusted title of the play whose rounds these are"`
	PlayRounds     int           `json:"play_rounds,omitempty" jsonschema:"how many rounds the detected play groups"`
	PlayCapped     bool          `json:"play_rounds_capped,omitempty" jsonschema:"true when the detected play groups more rounds than its examples: only the examples are read"`
	MatchesRead    int           `json:"matches_read"`
	MatchCap       int           `json:"match_cap" jsonschema:"the most matches one call reads, newest first"`
	MoreMatches    bool          `json:"more_matches,omitempty" jsonschema:"true when older matches past match_cap were not read; narrow the period, map, rivals or match_id to reach them"`
	MatchesSkipped int           `json:"matches_skipped,omitempty" jsonschema:"matches whose analysis is not ready or that the scope could not rate"`
	Truncated      bool          `json:"truncated,omitempty" jsonschema:"true when more rounds match; raise limit or narrow the filter"`
	linkOut
	// team names the link of a team's rounds.
	team string
}

// roundSubject is whose rounds find_rounds reads in one match: the team key
// of a team, or the slot of a player.
type roundSubject struct {
	teamKey string
	slot    *uint8
}

// playRounds are the rounds one play stands for, keyed match_id/round, with its matches in order.
type playRounds struct {
	title   string
	teamID  string
	keys    map[string]bool
	matches []string
	// rounds is how many rounds a detected play groups; its examples may be fewer.
	rounds int
}

// roundsCall is one find_rounds read: where to read and what the play keeps.
type roundsCall struct {
	ctx    context.Context
	studio *studio
	client *tuiclient.Client
	query  tuiclient.AnalysisQuery
	play   *playRounds
}

// playRoundKey is the key find_rounds compares play rounds by.
func playRoundKey(matchID string, round int) string {
	return fmt.Sprintf("%s/%d", strings.ToLower(matchID), round)
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
	for _, list := range [][]string{in.Buy, in.OpponentBuy} {
		for _, buy := range list {
			if !slices.Contains(buyTypes, strings.ToLower(strings.TrimSpace(buy))) {
				return tuiclient.AnalysisQuery{}, fmt.Errorf("buy must be one of %s, not %q", strings.Join(buyTypes, ", "), cleanName(buy))
			}
		}
	}
	if in.Result != "" && in.Result != roundResultWon && in.Result != roundResultLost {
		return tuiclient.AnalysisQuery{}, fmt.Errorf("result must be won or lost, not %q", cleanName(in.Result))
	}
	if in.Limit < 0 || in.Limit > maxRoundsLimit {
		return tuiclient.AnalysisQuery{}, fmt.Errorf("limit must be from 1 to %d", maxRoundsLimit)
	}
	if in.SteamID != "" {
		s := in.teamRoundScopeIn
		if len(in.Lineup) > 0 || s.teamOnly() {
			return tuiclient.AnalysisQuery{}, errors.New("lineup, max_matches, opponent_top, whose and the Replay filters (awp, advantage, site, outcome, post_plant, phase, second_round, end_reason, weapon, equip ranges, tactic) are team scope keys: pass team_id")
		}
	}
	return teamQuery(in.analysisFilterIn, in.teamScopeIn)
}

func registerRoundTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "find_rounds", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Find rounds of a team or a player in the library by map, side, buys of both sides, result and a play. Returns round refs " +
			"(match_id/round) with the buys, the site and the round shape, newest match first. With team_id it takes every team scope key " +
			"(lineup, rivals, period, last N matches, opponent level, whose, AWP, advantage, round range), so it lists the rounds the team tools count. " +
			"play_id keeps the rounds of a Libreta play or a detected play. Reads at most 50 matches; match_cap and more_matches say when older ones were left out. " +
			"studio_path opens the rounds listed (at most 200) in the 2D Replay as one playlist, with the player in focus for a steamid."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in findRoundsIn) (*mcp.CallToolResult, findRoundsOut, error) {
			if in.PlayID != "" && in.TeamID == "" && in.SteamID == "" {
				if err := checkUUID("play_id", in.PlayID); err != nil {
					return nil, findRoundsOut{}, errors.New("a detected play's ref needs the team_id detect_team_plays was called with")
				}
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, findRoundsOut{}, toolError(err)
			}
			call := roundsCall{ctx: ctx, studio: s, client: c}
			var play *playRounds
			if in.PlayID != "" {
				if play, err = in.playRounds(call); err != nil {
					return nil, findRoundsOut{}, err
				}
				if in.TeamID == "" && in.SteamID == "" {
					in.TeamID = play.teamID
				}
			}
			q, err := in.check()
			if err != nil {
				return nil, findRoundsOut{}, err
			}
			call.query, call.play = q, play
			var out findRoundsOut
			if in.TeamID != "" {
				out, err = in.teamRounds(call)
			} else {
				out, err = in.playerRounds(call)
			}
			if play != nil {
				out.Play = cleanName(play.title)
				out.PlayRounds, out.PlayCapped = play.rounds, play.rounds > len(play.keys)
			}
			if err == nil {
				out.linkOut = in.roundsLink(out)
			}
			return nil, out, err
		})
}

// playRounds reads the rounds of a Libreta play (a UUID) or of a detected play (its seed ref).
func (in findRoundsIn) playRounds(call roundsCall) (*playRounds, error) {
	ctx, c := call.ctx, call.client
	var examples []tuiclient.Evidence
	out := &playRounds{keys: map[string]bool{}}
	if checkUUID("play_id", in.PlayID) == nil {
		p, err := c.GetPlay(ctx, in.PlayID)
		if err != nil {
			return nil, toolError(err)
		}
		examples, out.title = p.Examples, p.Title
		if p.TeamID != nil {
			out.teamID = *p.TeamID
		}
	} else {
		if _, _, err := parseRoundRef(in.PlayID); err != nil {
			return nil, errors.New("play_id is a play id from list_plays or a detected play's ref (match_id/round) from detect_team_plays")
		}
		if in.TeamID == "" {
			return nil, errors.New("a detected play's ref needs the team_id detect_team_plays was called with")
		}
		det, err := detectPlaysIn{TeamID: in.TeamID, analysisFilterIn: in.analysisFilterIn, teamScopeIn: in.teamScopeIn}.detect(ctx, call.studio)
		if err != nil {
			return nil, err
		}
		i := slices.IndexFunc(det.Plays, func(p tuiclient.DetectedPlay) bool { return strings.EqualFold(refString(p.Seed), in.PlayID) })
		if i < 0 {
			return nil, errors.New("no detected play has that ref with this filter; call detect_team_plays again with the same filter")
		}
		found := det.Plays[i]
		examples, out.title, out.teamID, out.rounds = found.Refs, fmt.Sprintf("%s %s %s", found.Side, found.Kind, found.Site), in.TeamID, found.Rounds
	}
	for _, e := range examples {
		out.keys[playRoundKey(e.MatchID, e.Round)] = true
		if id := strings.ToLower(e.MatchID); !slices.Contains(out.matches, id) {
			out.matches = append(out.matches, id)
		}
	}
	return out, nil
}

// playMatches narrows the scope's matches to the play's, keeping an explicit match_id list as a bound.
func playMatches(q tuiclient.AnalysisQuery, play *playRounds) []string {
	if len(q.Matches) == 0 {
		return play.matches
	}
	var out []string
	for _, id := range play.matches {
		if slices.ContainsFunc(q.Matches, func(m string) bool { return strings.EqualFold(m, id) }) {
			out = append(out, id)
		}
	}
	return out
}

// teamRounds lists the rounds the team scope keeps, the same ones the team tools count.
func (in findRoundsIn) teamRounds(call roundsCall) (findRoundsOut, error) {
	q, play := call.query, call.play
	out := findRoundsOut{Rounds: []roundRefOut{}, MatchCap: maxRoundMatches}
	if play != nil {
		if q.Matches = playMatches(q, play); len(q.Matches) == 0 {
			return out, nil
		}
		if len(q.Matches) > maxRoundMatches {
			q.Matches, out.MoreMatches = q.Matches[:maxRoundMatches], true
		}
	}
	got, err := call.client.TeamRounds(call.ctx, in.TeamID, q)
	if err != nil {
		return out, toolError(err)
	}
	out.team = cleanName(got.TeamName)
	for _, skipped := range got.Skipped {
		if skipped.Reason == overMatchLimit || skipped.Reason == overReadLimit {
			out.MoreMatches = true
		} else {
			out.MatchesSkipped++
		}
	}
	limit := cmp.Or(in.Limit, defaultRoundsLimit)
	for _, m := range got.Matches {
		out.MatchesRead++
		for _, r := range m.Rounds {
			won := r.Winner == r.Side
			if (play != nil && !play.keys[playRoundKey(m.MatchID, r.Round)]) || !in.resultKeeps(won) {
				continue
			}
			if len(out.Rounds) == limit {
				out.Truncated = true
				return out, nil
			}
			out.Rounds = append(out.Rounds, roundRefOut{Ref: playRoundKey(m.MatchID, r.Round), MatchID: m.MatchID, Round: r.Round, Map: cleanName(m.Map),
				PlayedAt: m.PlayedAt.Format(time.DateOnly), Opponent: cleanName(m.Opponent), Side: r.Side, Buy: r.Buy, OpponentBuy: r.OpponentBuy,
				Won: won, Site: r.Site, Pattern: r.Pattern})
		}
	}
	return out, nil
}

func (in findRoundsIn) resultKeeps(won bool) bool {
	return in.Result == "" || (in.Result == roundResultWon) == won
}
