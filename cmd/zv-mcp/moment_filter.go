package main

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/moments"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	dateLayout = "2006-01-02"
	// maxSearchMatches bounds find_player_moments: each match can start a parse.
	maxSearchMatches   = 10
	defaultSearchLimit = 20
	orderDate          = "date"
)

// momentFilterIn is the filter list_moments, list_series_moments and
// find_player_moments share; Studio applies it (GET /api/jobs/{id}/moments).
type momentFilterIn struct {
	Weapon string   `json:"weapon,omitempty" jsonschema:"keep only plays where every kill used this weapon id, such as awp, ak47, deagle or m4a1_silencer"`
	Kinds  []string `json:"kinds,omitempty" jsonschema:"keep only plays of any of these kinds: clutch (last one alive on the team), ace (5 kills), multi_kill (2 or more kills)"`
	Side   string   `json:"side,omitempty" jsonschema:"keep only plays made on this side: CT or T"`
	Player string   `json:"player,omitempty" jsonschema:"multi-POV jobs only: keep the plays of this player, by SteamID64 or name"`
}

func (in momentFilterIn) query(jobID string) (tuiclient.MomentsQuery, error) {
	f, err := moments.ParseFilter(in.Weapon, in.Kinds, in.Side)
	if err != nil {
		return tuiclient.MomentsQuery{}, err
	}
	return tuiclient.MomentsQuery{JobID: jobID, Weapon: f.Weapon, Kinds: f.Kinds, Side: f.Side, Player: strings.TrimSpace(in.Player)}, nil
}

type findMomentsIn struct {
	SteamID string `json:"steamid" jsonschema:"SteamID64 of the player from get_match or list_matches"`
	From    string `json:"from,omitempty" jsonschema:"first day the matches were played, YYYY-MM-DD; empty means no lower bound"`
	To      string `json:"to,omitempty" jsonschema:"last day the matches were played, YYYY-MM-DD, included; empty means no upper bound"`
	Map     string `json:"map,omitempty" jsonschema:"only matches on this map, such as de_mirage"`
	Order   string `json:"order,omitempty" jsonschema:"best (default): highest score first; date: oldest match first, each in round order"`
	Limit   int    `json:"limit,omitempty" jsonschema:"how many plays to return; default 20"`
	momentFilterIn
}

type foundMatchOut struct {
	MatchID  string `json:"match_id"`
	Map      string `json:"map"`
	PlayedOn string `json:"played_on,omitempty"`
	JobID    string `json:"job_id,omitempty"`
	Status   string `json:"status,omitempty"`
	Pending  bool   `json:"pending,omitempty" jsonschema:"the player's plays in this match are not ready; wait_for_job on job_id and call again"`
	Skipped  string `json:"skipped,omitempty" jsonschema:"why this match was left out, for example its demo file is no longer in the library"`
	Moments  int    `json:"moments"`
}

type foundMomentOut struct {
	Map      string `json:"map"`
	PlayedOn string `json:"played_on,omitempty"`
	JobID    string `json:"job_id" jsonschema:"the job of this match: generate_short takes this job_id with segment ids of this match only"`
	momentOut
}

type findMomentsOut struct {
	Matches []foundMatchOut  `json:"matches"`
	Moments []foundMomentOut `json:"moments"`
	Total   int              `json:"total" jsonschema:"plays that pass the filter in every ready match"`
}

// foundPlay is one filtered play with the match it came from, before sorting.
type foundPlay struct {
	match tuiclient.LibraryMatch
	jobID string
	m     tuiclient.Moment
}

func registerFindMomentsTool(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "find_player_moments",
		Description: "Find one player's plays across the analysed matches in the Studio library, optionally between two dates and on one map, with the same filters as list_moments " +
			"(for example the player's clutches this month, or only AWP plays on CT). Creates or reuses one job per match, so a match the player was never parsed in starts parsing and comes back pending. " +
			fmt.Sprintf("At most %d matches per call: narrow the dates or the map when there are more.", maxSearchMatches)},
		func(ctx context.Context, _ *mcp.CallToolRequest, in findMomentsIn) (*mcp.CallToolResult, findMomentsOut, error) {
			if err := checkSteamID(in.SteamID); err != nil {
				return nil, findMomentsOut{}, err
			}
			if err := checkOptionalFilter("map", in.Map, mapNamePattern); err != nil {
				return nil, findMomentsOut{}, err
			}
			from, to, err := parseDateRange(in.From, in.To)
			if err != nil {
				return nil, findMomentsOut{}, err
			}
			order := cmp.Or(in.Order, orderBest)
			if order != orderBest && order != orderDate {
				return nil, findMomentsOut{}, fmt.Errorf("order must be %q or %q", orderBest, orderDate)
			}
			if _, err := in.query(""); err != nil {
				return nil, findMomentsOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, findMomentsOut{}, toolError(err)
			}
			// The server applies the range before its page limit; one extra row tells "too many" apart.
			q := tuiclient.LibraryMatchQuery{Map: in.Map, SteamID: in.SteamID, From: from, Limit: maxSearchMatches + 1}
			if !to.IsZero() {
				q.To = to.Add(-time.Millisecond)
			}
			all, err := c.SearchLibraryMatches(ctx, q)
			if err != nil {
				return nil, findMomentsOut{}, toolError(err)
			}
			matches := matchesBetween(all, from, to)
			if len(all) > maxSearchMatches {
				return nil, findMomentsOut{}, fmt.Errorf("more than %d matches fit; narrow from/to or the map", maxSearchMatches)
			}
			out := findMomentsOut{Matches: make([]foundMatchOut, 0, len(matches)), Moments: []foundMomentOut{}}
			var plays []foundPlay
			search := matchSearch{c: c, in: in}
			for _, m := range matches {
				found, matchPlays, err := search.find(ctx, m)
				if err != nil {
					return nil, findMomentsOut{}, err
				}
				out.Matches = append(out.Matches, found)
				plays = append(plays, matchPlays...)
			}
			sortFoundPlays(plays, order)
			out.Total = len(plays)
			limit := in.Limit
			if limit <= 0 {
				limit = defaultSearchLimit
			}
			for _, p := range plays[:min(limit, len(plays))] {
				out.Moments = append(out.Moments, foundMomentOut{Map: cleanName(p.match.Map), PlayedOn: playedOn(p.match), JobID: p.jobID, momentOut: describeMoment(p.m)})
			}
			return nil, out, nil
		})
}

// matchSearch reads one player's filtered plays match by match.
type matchSearch struct {
	c  *tuiclient.Client
	in findMomentsIn
}

// find creates or reuses the player's job in one match and reads its
// filtered plays; a job still parsing comes back pending.
func (s matchSearch) find(ctx context.Context, m tuiclient.LibraryMatch) (foundMatchOut, []foundPlay, error) {
	found := foundMatchOut{MatchID: m.ID, Map: cleanName(m.Map), PlayedOn: playedOn(m)}
	res, err := s.c.CreatePlayerJob(ctx, m.DemoID, s.in.SteamID)
	if code := tuiclient.StatusCode(err); code == http.StatusConflict || code == http.StatusNotFound {
		found.Skipped = "the demo file of this match is no longer in the library"
		return found, nil, nil
	}
	if err != nil {
		return found, nil, fmt.Errorf("match %s: %w", m.ID, toolError(err))
	}
	found.JobID, found.Status = res.JobID, cleanName(res.Status)
	if workerOwnsJob(res.Status) {
		found.Pending = true
		return found, nil, nil
	}
	q, err := s.in.query(res.JobID)
	if err != nil {
		return found, nil, err
	}
	doc, err := s.c.FilterMoments(ctx, q)
	if tuiclient.StatusCode(err) == http.StatusConflict {
		found.Pending = true
		return found, nil, nil
	}
	if err != nil {
		return found, nil, fmt.Errorf("match %s: %w", m.ID, toolError(err))
	}
	found.Moments = len(doc.Moments)
	plays := make([]foundPlay, 0, len(doc.Moments))
	for _, mo := range doc.Moments {
		plays = append(plays, foundPlay{match: m, jobID: res.JobID, m: mo})
	}
	return found, plays, nil
}

// parseDateRange reads from and to as whole days; to is included.
func parseDateRange(fromText, toText string) (time.Time, time.Time, error) {
	var from, to time.Time
	var err error
	if fromText != "" {
		if from, err = time.Parse(dateLayout, fromText); err != nil {
			return from, to, fmt.Errorf("from must be a date like 2026-10-31, not %q", cleanName(fromText))
		}
	}
	if toText != "" {
		if to, err = time.Parse(dateLayout, toText); err != nil {
			return from, to, fmt.Errorf("to must be a date like 2026-10-31, not %q", cleanName(toText))
		}
		to = to.AddDate(0, 0, 1)
	}
	if !from.IsZero() && !to.IsZero() && !from.Before(to) {
		return from, to, fmt.Errorf("from %s is after to %s", fromText, toText)
	}
	return from, to, nil
}

// matchesBetween keeps the matches played in [from, to), oldest first. A
// match with no known date only passes when no range is set.
func matchesBetween(all []tuiclient.LibraryMatch, from, to time.Time) []tuiclient.LibraryMatch {
	out := make([]tuiclient.LibraryMatch, 0, len(all))
	for _, m := range all {
		if (!from.IsZero() || !to.IsZero()) && m.PlayedAt.IsZero() {
			continue
		}
		if !from.IsZero() && m.PlayedAt.Before(from) {
			continue
		}
		if !to.IsZero() && !m.PlayedAt.Before(to) {
			continue
		}
		out = append(out, m)
	}
	slices.SortStableFunc(out, func(a, b tuiclient.LibraryMatch) int { return a.PlayedAt.Compare(b.PlayedAt) })
	return out
}

// sortFoundPlays orders by score (best) or by date then round (date); play
// order always breaks ties so the list is stable.
func sortFoundPlays(plays []foundPlay, order string) {
	slices.SortStableFunc(plays, func(a, b foundPlay) int {
		if order == orderBest && a.m.Score != b.m.Score {
			return cmp.Compare(b.m.Score, a.m.Score)
		}
		return cmp.Or(a.match.PlayedAt.Compare(b.match.PlayedAt), cmp.Compare(a.match.ID, b.match.ID),
			cmp.Compare(a.m.Round, b.m.Round), cmp.Compare(a.m.TickStart, b.m.TickStart))
	})
}

func playedOn(m tuiclient.LibraryMatch) string {
	if m.PlayedAt.IsZero() {
		return ""
	}
	return m.PlayedAt.UTC().Format(dateLayout)
}
