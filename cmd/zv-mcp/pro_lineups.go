package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/prolineups"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	defaultProRows = 20
	maxProRows     = 100
	// proRowTeams caps the teams a compact row names.
	proRowTeams = 3
)

var proLineupKinds = []string{"smoke", "flash", "he", "molotov"}

type proMapIn struct {
	Map string `json:"map" jsonschema:"such as de_mirage, from list_pro_lineup_maps"`
}

type findProLineupsIn struct {
	proMapIn
	Kind    string `json:"kind,omitempty" jsonschema:"smoke, flash, he or molotov"`
	Side    string `json:"side,omitempty" jsonschema:"T or CT"`
	Landing string `json:"landing,omitempty" jsonschema:"part of the callout it lands at, such as window or jungle; any case"`
	From    string `json:"from,omitempty" jsonschema:"part of the callout it is thrown from, such as tspawn; any case"`
	Insta   bool   `json:"insta,omitempty" jsonschema:"true keeps only lineups thrown in the first seconds of the round"`
	Video   bool   `json:"video,omitempty" jsonschema:"true keeps only lineups with a recorded pro clip (clip_url)"`
	Limit   int    `json:"limit,omitempty" jsonschema:"default 20, at most 100"`
}

type proLineupIn struct {
	proMapIn
	ID string `json:"id" jsonschema:"lineup id or slug from find_pro_lineups"`
}

type proCombosIn struct {
	proMapIn
	Side  string `json:"side,omitempty" jsonschema:"T or CT"`
	Limit int    `json:"limit,omitempty" jsonschema:"default 20, at most 100"`
}

type proMapOut struct {
	Map        string `json:"map"`
	Lineups    int    `json:"lineups"`
	Combos     int    `json:"combos"`
	Maps       int    `json:"maps" jsonschema:"pro maps read, all time"`
	RecentMaps int    `json:"recent_maps" jsonschema:"pro maps read in the recent window"`
}

type proMapsOut struct {
	BuiltAt string      `json:"built_at,omitempty"`
	Fresh   bool        `json:"fresh" jsonschema:"false when the community server did not answer and this is Studio's last copy"`
	Maps    []proMapOut `json:"maps"`
}

type proCountsOut struct {
	Recent int `json:"recent"`
	All    int `json:"all"`
}

type proLineupRowOut struct {
	ID          string       `json:"id"`
	Slug        string       `json:"slug"`
	Title       string       `json:"title"`
	Kind        string       `json:"kind"`
	Side        string       `json:"side"`
	LandPlace   string       `json:"land_place"`
	ThrowPlace  string       `json:"throw_place"`
	Movement    string       `json:"movement"`
	Jump        bool         `json:"jump,omitempty"`
	Insta       bool         `json:"insta,omitempty"`
	Throws      proCountsOut `json:"throws" jsonschema:"throws in the recent window and all time"`
	Matches     proCountsOut `json:"matches" jsonschema:"pro maps with the throw"`
	RoundShare  float64      `json:"round_share" jsonschema:"share of the side's recent rounds on the map with this throw, 0 to 1"`
	RoundWin    float64      `json:"round_win" jsonschema:"share of those rounds the side won, 0 to 1"`
	Landed      int          `json:"landed" jsonschema:"throws from the spot that landed on the lineup's point"`
	Attempts    int          `json:"attempts" jsonschema:"throws of the kind from the spot with the same aim"`
	ClockMedian float64      `json:"clock_median" jsonschema:"median second of the round it is thrown, after freeze end"`
	Teams       []string     `json:"teams,omitempty" jsonschema:"untrusted team names, most rounds first, at most 3"`
}

type findProLineupsOut struct {
	Map        string            `json:"map"`
	Fresh      bool              `json:"fresh"`
	RecentDays int               `json:"recent_days"`
	Total      int               `json:"total" jsonschema:"lineups the filter matched"`
	Lineups    []proLineupRowOut `json:"lineups"`
	Truncated  bool              `json:"truncated,omitempty"`
}

type proUsageOut struct {
	Name   string  `json:"name" jsonschema:"untrusted name from pro demos"`
	Key    string  `json:"key,omitempty" jsonschema:"for with: the partner lineup's id"`
	Count  int     `json:"count"`
	Share  float64 `json:"share"`
	Detail string  `json:"detail,omitempty" jsonschema:"for teams: percent of that team's side rounds with the throw"`
}

type proExampleOut struct {
	Match    string  `json:"match" jsonschema:"untrusted match name"`
	Event    string  `json:"event,omitempty"`
	PlayedAt string  `json:"played_at,omitempty" jsonschema:"YYYY-MM-DD"`
	Team     string  `json:"team,omitempty"`
	Player   string  `json:"player,omitempty"`
	Round    int     `json:"round"`
	Clock    float64 `json:"clock"`
	Tick     int     `json:"tick"`
	// SteamID64 is the thrower, the POV of the clip and of the replay.
	SteamID64 string `json:"steamid64,omitempty"`
	ClipURL   string `json:"clip_url,omitempty" jsonschema:"the recorded throw as a looping MP4 with the thrower's keys drawn on it; open it in a browser"`
}

type proLineupOut struct {
	proLineupRowOut
	Setpos     string          `json:"setpos" jsonschema:"the console command that puts the player on the spot"`
	Origin     [3]float64      `json:"origin"`
	Pitch      float64         `json:"pitch"`
	Yaw        float64         `json:"yaw"`
	Landing    [3]float64      `json:"landing"`
	AirTime    float64         `json:"air_time" jsonschema:"seconds in the air"`
	Trend      *[2]float64     `json:"trend,omitempty" jsonschema:"throws per 100 side rounds in the last 30 days, then in the 60 before; missing until both windows have rounds"`
	Clock      []int           `json:"clock_buckets" jsonschema:"recent throws per 10 seconds of round time, the last after the clock"`
	TeamCount  int             `json:"team_count"`
	TeamUsage  []proUsageOut   `json:"team_usage"`
	Players    []proUsageOut   `json:"players"`
	Events     []proUsageOut   `json:"events"`
	PlantSites []proUsageOut   `json:"plant_sites"`
	With       []proUsageOut   `json:"with" jsonschema:"lineups the same team throws within 15 seconds in the same round"`
	Examples   []proExampleOut `json:"examples" jsonschema:"recent throws to watch"`
	Rank       string          `json:"rank,omitempty" jsonschema:"place among the lineups of the kind landing at the same callout, such as 1 of 3"`
	LastSeen   string          `json:"last_seen,omitempty" jsonschema:"YYYY-MM-DD"`
	ClipURL    string          `json:"clip_url,omitempty" jsonschema:"the newest recorded throw of the lineup, as a looping MP4 with the thrower's keys; missing until one is recorded"`
}

type proComboLineupOut struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type proComboOut struct {
	ID        string              `json:"id"`
	Title     string              `json:"title"`
	Side      string              `json:"side"`
	Lineups   []proComboLineupOut `json:"lineups"`
	Rounds    int                 `json:"rounds" jsonschema:"recent side rounds with the combo"`
	TeamCount int                 `json:"team_count"`
	RoundWin  float64             `json:"round_win"`
	Clock     float64             `json:"clock" jsonschema:"median second of the first throw"`
	Teams     []string            `json:"teams,omitempty" jsonschema:"untrusted team names, at most 3"`
	ClipURL   string              `json:"clip_url,omitempty" jsonschema:"a recorded round of the combo: each throw from its thrower, one after another, with the keys drawn on it"`
}

type proCombosOut struct {
	Map       string        `json:"map"`
	Fresh     bool          `json:"fresh"`
	Total     int           `json:"total"`
	Combos    []proComboOut `json:"combos"`
	Truncated bool          `json:"truncated,omitempty"`
}

func registerProLineupTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: "list_pro_lineup_maps", Annotations: readOnly,
		Description: "List the maps of Lineups pro (grenades pros repeat in pro matches, from the ClipHub community server) with how many lineups and combos each has."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, proMapsOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, proMapsOut{}, toolError(err)
			}
			m, err := c.ListProLineupMaps(ctx)
			if err != nil {
				return nil, proMapsOut{}, proLineupsError(err)
			}
			out := proMapsOut{Fresh: m.Fresh, Maps: make([]proMapOut, 0, len(m.Maps))}
			if !m.BuiltAt.IsZero() {
				out.BuiltAt = m.BuiltAt.Format(time.DateOnly)
			}
			for _, r := range m.Maps {
				out.Maps = append(out.Maps, proMapOut{Map: cleanName(r.Map), Lineups: r.Lineups, Combos: r.Combos, Maps: r.Maps, RecentMaps: r.RecentMaps})
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "find_pro_lineups", Annotations: readOnly,
		Description: "Find a map's pro lineups, most thrown recently first: filter by kind, side, landing callout, throw callout, insta and video (only lineups with a recorded clip). " +
			"Each row has the technique, throws, round share and win rate, accuracy, clock and top teams; get_pro_lineup has the full detail."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in findProLineupsIn) (*mcp.CallToolResult, findProLineupsOut, error) {
			f, err := in.filter()
			if err != nil {
				return nil, findProLineupsOut{}, err
			}
			idx, err := proIndex(ctx, s, f.mapName)
			if err != nil {
				return nil, findProLineupsOut{}, err
			}
			return nil, findProLineups(idx, f), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_pro_lineup", Annotations: readOnly,
		Description: "Read one pro lineup: its setpos, stats, the teams, players and events that throw it, plant sites, the lineups thrown with it, recent examples and its rank among lineups to the same spot."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in proLineupIn) (*mcp.CallToolResult, proLineupOut, error) {
			mapName, err := proMapName(in.Map)
			if err != nil {
				return nil, proLineupOut{}, err
			}
			idx, err := proIndex(ctx, s, mapName)
			if err != nil {
				return nil, proLineupOut{}, err
			}
			l, err := findProLineup(idx, in.ID)
			if err != nil {
				return nil, proLineupOut{}, err
			}
			return nil, proLineupOf(l), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_pro_combos", Annotations: readOnly,
		Description: "List a map's pro combos: two or three pro lineups one team throws within 15 seconds in a round, with rounds, teams, win rate and clock."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in proCombosIn) (*mcp.CallToolResult, proCombosOut, error) {
			mapName, err := proMapName(in.Map)
			if err != nil {
				return nil, proCombosOut{}, err
			}
			side, err := proSide(in.Side)
			if err != nil {
				return nil, proCombosOut{}, err
			}
			limit, err := proLimit(in.Limit)
			if err != nil {
				return nil, proCombosOut{}, err
			}
			idx, err := proIndex(ctx, s, mapName)
			if err != nil {
				return nil, proCombosOut{}, err
			}
			return nil, proCombos(idx, side, limit), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "save_pro_lineup", Annotations: &mcp.ToolAnnotations{},
		Description: "Save a pro lineup into the Libreta, tagged pro with its technique estimated, as Guardar en la Libreta does on Lineups pro. Saving it again returns the copy already there. delete_lineup undoes it."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in proLineupIn) (*mcp.CallToolResult, lineupOut, error) {
			mapName, err := proMapName(in.Map)
			if err != nil {
				return nil, lineupOut{}, err
			}
			id, err := proLineupID(ctx, s, mapName, in.ID)
			if err != nil {
				return nil, lineupOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, lineupOut{}, toolError(err)
			}
			l, err := c.SaveProLineup(ctx, mapName, id)
			if err != nil {
				return nil, lineupOut{}, proLineupsError(err)
			}
			return nil, lineupOf(l), nil
		})
}

// proLineupsError explains the community failures the pro lineup routes answer with.
func proLineupsError(err error) error {
	switch tuiclient.ErrorCode(err) {
	case "community_disabled":
		return errors.New("the ClipHub community server is turned off in this Studio, and Lineups pro come from it")
	case "community_unavailable":
		return errors.New("the ClipHub community server did not answer and Studio has no copy of Lineups pro yet; try again later")
	case "no_pro_lineups":
		return errors.New("there are no pro lineups for this map or id; list_pro_lineup_maps lists the maps and find_pro_lineups the ids")
	}
	return toolError(err)
}

func proIndex(ctx context.Context, s *studio, mapName string) (tuiclient.ProLineupIndex, error) {
	c, err := s.client(ctx)
	if err != nil {
		return tuiclient.ProLineupIndex{}, toolError(err)
	}
	idx, err := c.GetProLineups(ctx, mapName)
	if err != nil {
		return tuiclient.ProLineupIndex{}, proLineupsError(err)
	}
	return idx, nil
}

// proLineupID passes an id through and turns a slug into its id.
func proLineupID(ctx context.Context, s *studio, mapName, raw string) (string, error) {
	ref := strings.TrimSpace(raw)
	if ref == "" || len(ref) > 200 {
		return "", errors.New("id must be a lineup id or slug from find_pro_lineups")
	}
	if !strings.Contains(ref, "/") {
		return ref, nil
	}
	idx, err := proIndex(ctx, s, mapName)
	if err != nil {
		return "", err
	}
	l, err := findProLineup(idx, ref)
	return l.ID, err
}

// proMapPattern is community.MapPattern without "all".
var proMapPattern = regexp.MustCompile(`^[a-z]{2,4}_[a-z0-9_]{1,32}$`)

func proMapName(raw string) (string, error) {
	m := strings.ToLower(strings.TrimSpace(raw))
	if !proMapPattern.MatchString(m) {
		return "", errors.New("map must be a map name such as de_mirage")
	}
	return m, nil
}

func proSide(raw string) (string, error) {
	side := strings.ToUpper(strings.TrimSpace(raw))
	if side != "" && side != "T" && side != "CT" {
		return "", errors.New("side must be T or CT")
	}
	return side, nil
}

func proLimit(n int) (int, error) {
	if n < 0 || n > maxProRows {
		return 0, fmt.Errorf("limit must be from 1 to %d", maxProRows)
	}
	return cmp.Or(n, defaultProRows), nil
}

// proFilter is a checked find_pro_lineups input; places are lower case.
type proFilter struct {
	mapName, kind, side, landing, from string
	insta, video                       bool
	limit                              int
}

func (in findProLineupsIn) filter() (proFilter, error) {
	mapName, err := proMapName(in.Map)
	if err != nil {
		return proFilter{}, err
	}
	kind := strings.ToLower(strings.TrimSpace(in.Kind))
	if kind != "" && !slices.Contains(proLineupKinds, kind) {
		return proFilter{}, fmt.Errorf("kind must be one of %s", strings.Join(proLineupKinds, ", "))
	}
	side, err := proSide(in.Side)
	if err != nil {
		return proFilter{}, err
	}
	limit, err := proLimit(in.Limit)
	if err != nil {
		return proFilter{}, err
	}
	return proFilter{mapName: mapName, kind: kind, side: side, landing: strings.ToLower(strings.TrimSpace(in.Landing)),
		from: strings.ToLower(strings.TrimSpace(in.From)), insta: in.Insta, video: in.Video, limit: limit}, nil
}

func (f proFilter) keeps(l prolineups.Lineup) bool {
	return (f.kind == "" || string(l.Kind) == f.kind) && (f.side == "" || string(l.Side) == f.side) && (!f.insta || l.Insta) && (!f.video || l.Clip != "") &&
		strings.Contains(strings.ToLower(l.LandPlace), f.landing) && strings.Contains(strings.ToLower(l.ThrowPlace), f.from)
}

// findProLineups keeps the served order, which is most thrown recently first.
func findProLineups(idx tuiclient.ProLineupIndex, f proFilter) findProLineupsOut {
	out := findProLineupsOut{Map: cleanName(idx.Map), Fresh: idx.Fresh, RecentDays: idx.RecentDays, Lineups: []proLineupRowOut{}}
	for _, l := range idx.Lineups {
		if !f.keeps(l) {
			continue
		}
		out.Total++
		if len(out.Lineups) < f.limit {
			out.Lineups = append(out.Lineups, proLineupRowOf(l))
		}
	}
	out.Truncated = out.Total > len(out.Lineups)
	return out
}

func findProLineup(idx tuiclient.ProLineupIndex, ref string) (prolineups.Lineup, error) {
	ref = strings.TrimSpace(ref)
	for _, l := range idx.Lineups {
		if l.ID == ref || l.Slug == ref {
			return l, nil
		}
	}
	return prolineups.Lineup{}, fmt.Errorf("no pro lineup %q on %s; find_pro_lineups lists the ids", cleanName(ref), cleanName(idx.Map))
}

func proLineupRowOf(l prolineups.Lineup) proLineupRowOut {
	teams := make([]string, 0, proRowTeams)
	for _, u := range l.Teams[:min(len(l.Teams), proRowTeams)] {
		teams = append(teams, cleanName(u.Name))
	}
	return proLineupRowOut{ID: cleanName(l.ID), Slug: cleanName(l.Slug), Title: cleanText(l.Title, maxErrorRunes), Kind: string(l.Kind), Side: string(l.Side),
		LandPlace: cleanName(l.LandPlace), ThrowPlace: cleanName(l.ThrowPlace), Movement: cleanName(l.Movement), Jump: l.Jump, Insta: l.Insta,
		Throws: proCountsOut(l.Throws), Matches: proCountsOut(l.Matches), RoundShare: l.RoundShare, RoundWin: l.RoundWin,
		Landed: l.Landed, Attempts: l.Attempts, ClockMedian: l.Clock.Median, Teams: teams}
}

func proLineupOf(l prolineups.Lineup) proLineupOut {
	out := proLineupOut{proLineupRowOut: proLineupRowOf(l), Origin: l.Origin, Pitch: l.Pitch, Yaw: l.Yaw, Landing: l.Landing,
		Setpos: setposCommand(tuiclient.Lineup{Origin: l.Origin, Pitch: l.Pitch, Yaw: l.Yaw}), AirTime: l.AirTime, Trend: l.Trend,
		Clock: l.Clock.Buckets, TeamCount: l.TeamCount, TeamUsage: proUsages(l.Teams), Players: proUsages(l.Players), Events: proUsages(l.Events),
		PlantSites: proUsages(l.PlantSites), With: proUsages(l.With), Examples: make([]proExampleOut, 0, len(l.Examples))}
	if l.Rank.Of > 0 {
		out.Rank = fmt.Sprintf("%d of %d", l.Rank.Position, l.Rank.Of)
	}
	if !l.LastSeen.IsZero() {
		out.LastSeen = l.LastSeen.Format(time.DateOnly)
	}
	out.ClipURL = proClipURL(l.Clip)
	for _, e := range l.Examples {
		ex := proExampleOut{Match: cleanName(e.Match), Event: cleanName(e.Event), Team: cleanName(e.Team), Player: cleanName(e.Player),
			Round: e.Round, Clock: e.Clock, Tick: e.Tick, SteamID64: cleanName(e.SteamID64), ClipURL: proClipURL(e.Clip)}
		if !e.PlayedAt.IsZero() {
			ex.PlayedAt = e.PlayedAt.Format(time.DateOnly)
		}
		out.Examples = append(out.Examples, ex)
	}
	return out
}

func proUsages(us []prolineups.Usage) []proUsageOut {
	out := make([]proUsageOut, 0, len(us))
	for _, u := range us {
		out = append(out, proUsageOut{Name: cleanText(u.Name, maxErrorRunes), Key: cleanName(u.Key), Count: u.Count, Share: u.Share, Detail: cleanName(u.Detail)})
	}
	return out
}

func proCombos(idx tuiclient.ProLineupIndex, side string, limit int) proCombosOut {
	titles := map[string]string{}
	for _, l := range idx.Lineups {
		titles[l.ID] = l.Title
	}
	out := proCombosOut{Map: cleanName(idx.Map), Fresh: idx.Fresh, Combos: []proComboOut{}}
	for _, c := range idx.Combos {
		if side != "" && string(c.Side) != side {
			continue
		}
		out.Total++
		if len(out.Combos) == limit {
			continue
		}
		row := proComboOut{ID: cleanText(c.ID, maxErrorRunes), Title: cleanText(c.Title, maxErrorRunes), Side: string(c.Side), Rounds: c.Rounds.Recent,
			TeamCount: c.TeamCount, RoundWin: c.RoundWin, Clock: c.Clock, ClipURL: proClipURL(c.Clip)}
		for _, id := range c.Lineups {
			row.Lineups = append(row.Lineups, proComboLineupOut{ID: cleanName(id), Title: cleanText(titles[id], maxErrorRunes)})
		}
		for _, u := range c.Teams[:min(len(c.Teams), proRowTeams)] {
			row.Teams = append(row.Teams, cleanName(u.Name))
		}
		out.Combos = append(out.Combos, row)
	}
	out.Truncated = out.Total > len(out.Combos)
	return out
}

// proLineupSpots turns pro lineups of the map into practice spots.
func proLineupSpots(ctx context.Context, s *studio, in practiceIn) ([]tuiclient.PracticeSpot, error) {
	if len(in.ProLineupIDs) > maxPracticeSpots {
		return nil, fmt.Errorf("pro_lineup_ids takes 1 to %d lineups", maxPracticeSpots)
	}
	idx, err := proIndex(ctx, s, in.Map)
	if err != nil {
		return nil, err
	}
	lineups := make([]tuiclient.Lineup, 0, len(in.ProLineupIDs))
	for _, ref := range in.ProLineupIDs {
		l, err := findProLineup(idx, ref)
		if err != nil {
			return nil, err
		}
		lineups = append(lineups, tuiclient.Lineup{Title: l.Title, Map: in.Map, Side: string(l.Side), Grenade: string(l.Kind), Origin: l.Origin, Pitch: l.Pitch, Yaw: l.Yaw})
	}
	return lineupSpots(in.Map, lineups)
}

// proClipURL is a recorded clip on the community server Studio uses, which any browser opens.
func proClipURL(clip string) string {
	base := os.Getenv("ZV_COMMUNITY_URL")
	if base == "off" || !prolineups.ValidClipName(clip) {
		return ""
	}
	if base == "" {
		base = community.DefaultBaseURL
	}
	return strings.TrimRight(base, "/") + "/v1/pro-clips/" + clip
}
