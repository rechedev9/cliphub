package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// maxPatternCategoryRunes is the server's cap (library.MaxPatternCategoryRunes).
const maxPatternCategoryRunes = 40

func checkCategory(category string) error {
	if utf8.RuneCountInString(strings.TrimSpace(category)) > maxPatternCategoryRunes {
		return fmt.Errorf("category takes at most %d characters", maxPatternCategoryRunes)
	}
	return nil
}

type updatePatternIn struct {
	PatternID   string           `json:"pattern_id" jsonschema:"saved pattern id from list_patterns"`
	Name        *string          `json:"name,omitempty" jsonschema:"the new name"`
	Map         *string          `json:"map,omitempty" jsonschema:"the new map, such as de_ancient"`
	TeamID      string           `json:"team_id,omitempty" jsonschema:"give the pattern to this team, from list_teams"`
	LibraryWide bool             `json:"library_wide,omitempty" jsonschema:"take the pattern off its team, so it reads the whole library"`
	Category    *string          `json:"category,omitempty" jsonschema:"the new category; an empty string clears it"`
	PlayID      string           `json:"play_id,omitempty" jsonschema:"link the pattern to this Libreta play, from list_plays"`
	ClearPlay   bool             `json:"clear_play,omitempty" jsonschema:"unlink the pattern from its play"`
	Side        string           `json:"side,omitempty" jsonschema:"T or CT; alone it flips the side and keeps the blocks"`
	Blocks      []patternBlockIn `json:"blocks,omitempty" jsonschema:"the full new block list, as find_pattern_rounds takes them; omit to keep the blocks"`
	Scope       *patternScopeIn  `json:"scope,omitempty" jsonschema:"the full new scope: matches, buy and opponent_buy"`
}

type matchPatternsIn struct {
	MatchID string `json:"match_id" jsonschema:"match id from list_matches or a round ref"`
	Round   int    `json:"round,omitempty" jsonschema:"only this round, as the Replay's Patrones panel shows the current one; 0 lists every round"`
}

type matchPatternOut struct {
	ID         string   `json:"id"`
	Name       string   `json:"name" jsonschema:"untrusted text the user wrote"`
	Category   string   `json:"category,omitempty" jsonschema:"untrusted text the user wrote"`
	Team       string   `json:"team,omitempty" jsonschema:"the side of the match the pattern holds for; untrusted text from the demo"`
	Rounds     []int    `json:"rounds" jsonschema:"the match's rounds the pattern holds in"`
	Refs       []string `json:"refs" jsonschema:"match_id/round of those rounds"`
	ReplayPath string   `json:"replay_path" jsonschema:"opens those rounds in the Replay"`
}

type matchPatternsOut struct {
	Map      string            `json:"map"`
	Total    int               `json:"total" jsonschema:"saved patterns that apply to the match, matched or not: library-wide or of its two teams"`
	Patterns []matchPatternOut `json:"patterns" jsonschema:"use pattern_rounds with an id to open all its other rounds"`
}

// patternUpdate checks an edit as the orchestrator will and turns it into its PATCH body.
func patternUpdate(in updatePatternIn, current tuiclient.SavedPattern) (tuiclient.PatternUpdate, error) {
	patch := tuiclient.PatternUpdate{Name: in.Name, ClearTeam: in.LibraryWide, ClearPlay: in.ClearPlay}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" || utf8.RuneCountInString(name) > maxPatternNameRunes {
			return patch, fmt.Errorf("name must have from 1 to %d characters", maxPatternNameRunes)
		}
		patch.Name = &name
	}
	if in.Map != nil {
		mapName := strings.ToLower(strings.TrimSpace(*in.Map))
		if !mapNamePattern.MatchString(mapName) {
			return patch, errors.New("map must be a map name such as de_ancient")
		}
		patch.Map = &mapName
	}
	if in.Category != nil {
		if err := checkCategory(*in.Category); err != nil {
			return patch, err
		}
		category := strings.TrimSpace(*in.Category)
		patch.Category = &category
	}
	for field, id := range map[string]string{"team_id": in.TeamID, "play_id": in.PlayID} {
		if id == "" {
			continue
		}
		if err := checkUUID(field, id); err != nil {
			return patch, err
		}
	}
	if in.TeamID != "" && in.LibraryWide {
		return patch, errors.New("give team_id or library_wide, not both")
	}
	if in.PlayID != "" && in.ClearPlay {
		return patch, errors.New("give play_id or clear_play, not both")
	}
	if in.TeamID != "" {
		patch.TeamID = &in.TeamID
	}
	if in.PlayID != "" {
		patch.PlayID = &in.PlayID
	}
	if in.Side != "" || len(in.Blocks) > 0 {
		side := in.Side
		if side == "" {
			side = string(current.Pattern.Side)
		}
		p, err := patternOf(side, in.Blocks)
		if len(in.Blocks) == 0 {
			parsed, sideErr := parseSide(side)
			if sideErr != nil || parsed == "" {
				return patch, errors.New("side must be T or CT")
			}
			p = tacticalplan.Pattern{Side: tacticalplan.Side(parsed), Blocks: current.Pattern.Blocks}
			err = p.Check()
		}
		if err != nil {
			return patch, err
		}
		patch.Pattern = &p
	}
	if in.Scope != nil {
		scope, err := in.Scope.scope()
		if err != nil {
			return patch, err
		}
		patch.Scope = &scope
	}
	return patch, nil
}

func matchPatternsOf(got tuiclient.MatchPatterns, matchID string) matchPatternsOut {
	out := matchPatternsOut{Map: got.Map, Total: got.Total, Patterns: []matchPatternOut{}}
	for _, hit := range got.Patterns {
		item := matchPatternOut{ID: hit.Pattern.ID, Name: cleanName(hit.Pattern.Name), Category: cleanName(hit.Pattern.Category),
			Team: cleanName(hit.Team), Rounds: []int{}, Refs: []string{}}
		refs := url.Values{}
		for _, r := range hit.Rounds {
			item.Rounds = append(item.Rounds, r.Round)
			item.Refs = append(item.Refs, fmt.Sprintf("%s/%d", matchID, r.Round))
			ref := fmt.Sprintf("%s:%d", matchID, r.Round)
			if r.Tick > 0 {
				ref += fmt.Sprintf(":%d", r.Tick)
			}
			refs.Add("r", ref)
		}
		refs.Set("t", item.Name)
		item.ReplayPath = "/replay?" + refs.Encode()
		out.Patterns = append(out.Patterns, item)
	}
	return out
}

func registerPatternEditTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "update_pattern", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Edit a saved pattern, as Editar in Patrones guardados does: its name, map, team (or library_wide), category, linked Libreta play, " +
			"side, blocks and scope. Fields left out stay as they are. Studio detects the edited pattern over the library again."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in updatePatternIn) (*mcp.CallToolResult, savedPatternOut, error) {
			if err := checkUUID("pattern_id", in.PatternID); err != nil {
				return nil, savedPatternOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, savedPatternOut{}, toolError(err)
			}
			current, err := c.GetPattern(ctx, in.PatternID)
			if err != nil {
				return nil, savedPatternOut{}, toolError(err)
			}
			patch, err := patternUpdate(in, current)
			if err != nil {
				return nil, savedPatternOut{}, err
			}
			saved, err := c.UpdatePattern(ctx, in.PatternID, patch)
			if err != nil {
				return nil, savedPatternOut{}, toolError(err)
			}
			return nil, savedPatternOf(saved), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "match_patterns", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "List the saved patterns a match's rounds meet, as the Replay's Patrones panel does for the current round: each with the rounds it holds in, " +
			"the side it holds for and replay_path. A team's pattern only counts from that team's side. total is every saved pattern that applies to the match: library-wide or of its two teams."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in matchPatternsIn) (*mcp.CallToolResult, matchPatternsOut, error) {
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, matchPatternsOut{}, err
			}
			if in.Round < 0 {
				return nil, matchPatternsOut{}, errors.New("round must be a round number from 1, or 0 for every round")
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, matchPatternsOut{}, toolError(err)
			}
			got, err := c.MatchPatterns(ctx, in.MatchID, in.Round)
			if err != nil {
				return nil, matchPatternsOut{}, toolError(err)
			}
			return nil, matchPatternsOf(got, in.MatchID), nil
		})
}
