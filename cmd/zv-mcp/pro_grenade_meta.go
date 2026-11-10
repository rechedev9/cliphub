package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/prolineups"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// A lineup is rising or falling when its rate moved by half and by at least one
// throw per 100 side rounds; metaTrend in web/lib/pro-lineups-meta.ts mirrors it.
const (
	metaTrendFactor = 1.5
	metaTrendMin    = 1.0
	// metaSmallSample marks a row with fewer throws in the period, as the web's «Poca muestra».
	metaSmallSample = 5
)

type proGrenadeMetaIn struct {
	proMapIn
	Side   string `json:"side" jsonschema:"T or CT"`
	Kind   string `json:"kind,omitempty" jsonschema:"smoke, flash, he or molotov; empty means all"`
	Period string `json:"period,omitempty" jsonschema:"recent (the index's recent window, 90 days) or all; default recent"`
	Limit  int    `json:"limit,omitempty" jsonschema:"default 20, at most 100"`
}

type metaTeamOut struct {
	Name string `json:"name" jsonschema:"untrusted team name from pro demos"`
	Pct  string `json:"pct,omitempty" jsonschema:"percent of that team's recent side rounds with the throw"`
}

type metaTrendOut struct {
	Last30 float64 `json:"last_30" jsonschema:"throws per 100 side rounds in the last 30 days"`
	Before float64 `json:"before" jsonschema:"throws per 100 side rounds in the 60 days before"`
	Label  string  `json:"label" jsonschema:"new (entering the meta), rising, falling or steady"`
}

type proGrenadeMetaRowOut struct {
	Rank       int           `json:"rank"`
	ID         string        `json:"id" jsonschema:"for get_pro_lineup"`
	Title      string        `json:"title"`
	Kind       string        `json:"kind"`
	LandPlace  string        `json:"land_place"`
	ThrowPlace string        `json:"throw_place"`
	Throws     int           `json:"throws" jsonschema:"throws in the period"`
	Small      bool          `json:"small_sample,omitempty" jsonschema:"fewer than 5 throws in the period: read its rank and trend with care"`
	Per100     float64       `json:"per_100_rounds" jsonschema:"throws per 100 side rounds in the period: the ranking"`
	RoundShare float64       `json:"round_share" jsonschema:"share of the side's recent rounds with the throw, 0 to 1"`
	Teams      []metaTeamOut `json:"teams" jsonschema:"recent teams, most rounds first, at most 3"`
	TeamCount  int           `json:"team_count" jsonschema:"teams that ever threw it"`
	Trend      *metaTrendOut `json:"trend,omitempty" jsonschema:"missing until both trend windows have rounds"`
}

type proGrenadeMetaOut struct {
	Map        string                 `json:"map"`
	Side       string                 `json:"side"`
	Period     string                 `json:"period"`
	PeriodDays int                    `json:"period_days,omitempty" jsonschema:"days of the recent period; missing for all"`
	SideRounds int                    `json:"side_rounds" jsonschema:"pro side rounds read in the period, the denominator of per_100_rounds"`
	Fresh      bool                   `json:"fresh"`
	Total      int                    `json:"total"`
	Entering   []string               `json:"entering" jsonschema:"ids of the ranked lineups whose trend is new or rising"`
	Grenades   []proGrenadeMetaRowOut `json:"grenades"`
	Truncated  bool                   `json:"truncated,omitempty"`
}

func registerProGrenadeMetaTool(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "pro_grenade_meta", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Rank a map side's pro grenades by throws per 100 side rounds in a period, as the Meta view of Lineups pro does: which teams use each " +
			"and the trend of the last 30 days against the 60 before (new, rising, falling or steady), so a grenade entering the meta shows. get_pro_lineup reads one."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in proGrenadeMetaIn) (*mcp.CallToolResult, proGrenadeMetaOut, error) {
			f, err := in.filter()
			if err != nil {
				return nil, proGrenadeMetaOut{}, err
			}
			idx, err := proIndex(ctx, s, f.mapName)
			if err != nil {
				return nil, proGrenadeMetaOut{}, err
			}
			return nil, grenadeMeta(idx, f), nil
		})
}

type metaFilter struct {
	mapName, side, kind string
	all                 bool
	limit               int
}

func (in proGrenadeMetaIn) filter() (metaFilter, error) {
	mapName, err := proMapName(in.Map)
	if err != nil {
		return metaFilter{}, err
	}
	side, err := proSide(in.Side)
	if err != nil {
		return metaFilter{}, err
	}
	if side == "" {
		return metaFilter{}, errors.New("side is required: the meta ranks one side")
	}
	kind := strings.ToLower(strings.TrimSpace(in.Kind))
	if kind != "" && !slices.Contains(proLineupKinds, kind) {
		return metaFilter{}, fmt.Errorf("kind must be one of %s", strings.Join(proLineupKinds, ", "))
	}
	period := cmp.Or(strings.ToLower(strings.TrimSpace(in.Period)), "recent")
	if period != "recent" && period != "all" {
		return metaFilter{}, errors.New("period must be recent or all")
	}
	limit, err := proLimit(in.Limit)
	if err != nil {
		return metaFilter{}, err
	}
	return metaFilter{mapName: mapName, side: side, kind: kind, all: period == "all", limit: limit}, nil
}

func grenadeMeta(idx tuiclient.ProLineupIndex, f metaFilter) proGrenadeMetaOut {
	rounds := idx.RoundsT
	if f.side == "CT" {
		rounds = idx.RoundsCT
	}
	out := proGrenadeMetaOut{Map: cleanName(idx.Map), Side: f.side, Period: "recent", PeriodDays: idx.RecentDays, SideRounds: rounds.Recent,
		Fresh: idx.Fresh, Entering: []string{}, Grenades: []proGrenadeMetaRowOut{}}
	if f.all {
		out.Period, out.PeriodDays, out.SideRounds = "all", 0, rounds.All
	}
	rows := []proGrenadeMetaRowOut{}
	for _, l := range idx.Lineups {
		if string(l.Side) != f.side || (f.kind != "" && string(l.Kind) != f.kind) {
			continue
		}
		throws := l.Throws.Recent
		if f.all {
			throws = l.Throws.All
		}
		if throws == 0 {
			continue
		}
		rows = append(rows, metaRow(l, throws, out.SideRounds))
	}
	slices.SortStableFunc(rows, func(a, b proGrenadeMetaRowOut) int {
		return cmp.Or(cmp.Compare(b.Per100, a.Per100), cmp.Compare(b.Throws, a.Throws), cmp.Compare(a.ID, b.ID))
	})
	out.Total = len(rows)
	for i, r := range rows[:min(len(rows), f.limit)] {
		r.Rank = i + 1
		if r.Trend != nil && (r.Trend.Label == "new" || r.Trend.Label == "rising") {
			out.Entering = append(out.Entering, r.ID)
		}
		out.Grenades = append(out.Grenades, r)
	}
	out.Truncated = out.Total > len(out.Grenades)
	return out
}

func metaRow(l prolineups.Lineup, throws, rounds int) proGrenadeMetaRowOut {
	row := proGrenadeMetaRowOut{ID: cleanName(l.ID), Title: cleanText(l.Title, maxErrorRunes), Kind: string(l.Kind), LandPlace: cleanName(l.LandPlace),
		ThrowPlace: cleanName(l.ThrowPlace), Throws: throws, Small: throws < metaSmallSample, RoundShare: l.RoundShare, TeamCount: l.TeamCount, Teams: []metaTeamOut{}}
	if rounds > 0 {
		row.Per100 = math.Round(1000*float64(throws)/float64(rounds)) / 10
	}
	for _, u := range l.Teams[:min(len(l.Teams), proRowTeams)] {
		row.Teams = append(row.Teams, metaTeamOut{Name: cleanText(u.Name, maxErrorRunes), Pct: cleanName(u.Detail)})
	}
	if l.Trend != nil {
		row.Trend = &metaTrendOut{Last30: l.Trend[0], Before: l.Trend[1], Label: metaTrend(l.Trend[0], l.Trend[1])}
	}
	return row
}

// metaTrend reads throws per 100 side rounds now against before.
func metaTrend(now, before float64) string {
	switch {
	case before == 0 && now >= metaTrendMin:
		return "new"
	case now >= before*metaTrendFactor && now-before >= metaTrendMin:
		return "rising"
	case now*metaTrendFactor <= before && before-now >= metaTrendMin:
		return "falling"
	}
	return "steady"
}
