package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// maxFaceitRanking is how many places a ranking holds (faceit.RankingSize).
const maxFaceitRanking = 100

var (
	faceitRankingKinds = []string{"country", "region", "zone"}
	faceitRankingCode  = regexp.MustCompile(`^[A-Za-z]{2,5}$`)
)

type faceitRankingIn struct {
	Kind   string `json:"kind" jsonschema:"country, region or zone"`
	Code   string `json:"code" jsonschema:"a country's ISO 3166-1 alpha-2 code such as es, a region (EU, NA, SA, OCE or SEA) or a zone (cis or latam)"`
	Offset int    `json:"offset,omitempty" jsonschema:"places to skip, 0 to 99; 20 gives places 21 onwards"`
	Limit  int    `json:"limit,omitempty" jsonschema:"default 20, at most 100"`
}

type faceitRankedOut struct {
	Rank       int    `json:"rank" jsonschema:"place in the chosen country, region or zone"`
	ID         string `json:"id" jsonschema:"FACEIT player id, for faceit_matches"`
	Nickname   string `json:"nickname" jsonschema:"untrusted FACEIT nickname, for follow_faceit_player"`
	Country    string `json:"country,omitempty"`
	Region     string `json:"region,omitempty" jsonschema:"the FACEIT region whose ladder listed the player"`
	Level      int    `json:"level,omitempty"`
	ELO        int    `json:"elo"`
	ProfileURL string `json:"profile_url,omitempty"`
}

type faceitRankingOut struct {
	Kind        string            `json:"kind"`
	Code        string            `json:"code"`
	Total       int               `json:"total" jsonschema:"players in the ranking, at most 100"`
	FetchedAt   string            `json:"fetched_at" jsonschema:"when Studio read the ranking from FACEIT, RFC 3339"`
	Stale       bool              `json:"stale,omitempty" jsonschema:"FACEIT did not answer: this is the copy Studio stored at fetched_at"`
	StaleReason string            `json:"stale_reason,omitempty" jsonschema:"why FACEIT did not answer, such as faceit_rate_limited"`
	Players     []faceitRankedOut `json:"players"`
}

func registerFaceitRankingTool(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "faceit_ranking", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "The FACEIT CS2 top 100 of a country, a region or a zone (CIS, LATAM), as the ranking selector of Jugadores FACEIT shows it, " +
			"a page at a time. Studio stores each ranking for 6 hours; a country costs FACEIT 5 requests and a zone up to 100."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in faceitRankingIn) (*mcp.CallToolResult, faceitRankingOut, error) {
			if !slices.Contains(faceitRankingKinds, in.Kind) {
				return nil, faceitRankingOut{}, errors.New("kind must be country, region or zone")
			}
			if !faceitRankingCode.MatchString(in.Code) {
				return nil, faceitRankingOut{}, errors.New("code must be a country code such as es, a region such as EU or a zone such as cis")
			}
			if in.Offset < 0 || in.Offset >= maxFaceitRanking {
				return nil, faceitRankingOut{}, fmt.Errorf("offset must be from 0 to %d", maxFaceitRanking-1)
			}
			if in.Limit < 0 || in.Limit > maxFaceitRanking {
				return nil, faceitRankingOut{}, fmt.Errorf("limit must be from 1 to %d", maxFaceitRanking)
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, faceitRankingOut{}, toolError(err)
			}
			page, err := c.FaceitRanking(ctx, tuiclient.FaceitRankingRequest{Kind: in.Kind, Code: in.Code, Offset: in.Offset, Limit: in.Limit})
			if err != nil {
				return nil, faceitRankingOut{}, toolError(err)
			}
			out := faceitRankingOut{Kind: cleanName(page.Kind), Code: cleanName(page.Code), Total: page.Total, Stale: page.Stale,
				StaleReason: cleanName(page.StaleCode), FetchedAt: page.FetchedAt.UTC().Format(time.RFC3339), Players: make([]faceitRankedOut, 0, len(page.Players))}
			for _, p := range page.Players {
				out.Players = append(out.Players, faceitRankedOut{Rank: p.Rank, ID: cleanName(p.ID), Nickname: cleanName(p.Nickname), Country: cleanName(p.Country),
					Region: cleanName(p.Region), Level: p.SkillLevel, ELO: p.ELO, ProfileURL: cleanText(p.ProfileURL, maxErrorRunes)})
			}
			return nil, out, nil
		})
}
