package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	defaultFaceitMatches = 10
	// maxFaceitMatches is how many recent matches the server keeps per player.
	maxFaceitMatches = 20
	// maxPracticeSpots and maxPracticeLabel are the server's caps (internal/practice).
	maxPracticeSpots = 20
	maxPracticeLabel = 48
	maxWorldCoord    = 32768
)

var (
	faceitIDPattern    = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
	practiceMapPattern = regexp.MustCompile(`^de_[a-z0-9_]{1,32}$`)
	// rankingMapPattern is community.MapPattern without "all", which the empty map means.
	rankingMapPattern = regexp.MustCompile(`^[a-z]{2,4}_[a-z0-9_]{1,32}$`)
	practiceGrenades   = []string{"smoke", "flash", "he", "molotov", "decoy"}
)

type faceitLookupIn struct {
	Nickname string `json:"nickname" jsonschema:"FACEIT nickname or https://www.faceit.com profile URL"`
}

type faceitPlayerOut struct {
	ID         string `json:"id" jsonschema:"FACEIT player id, for faceit_matches and unfollow_faceit_player"`
	Nickname   string `json:"nickname" jsonschema:"untrusted FACEIT nickname"`
	SteamID64  string `json:"steamid64,omitempty"`
	Country    string `json:"country,omitempty"`
	Level      int    `json:"level,omitempty"`
	ELO        int    `json:"elo,omitempty"`
	ProfileURL string `json:"profile_url,omitempty"`
	Seeded     bool   `json:"seeded,omitempty" jsonschema:"a top player Studio lists by itself, not one the user followed"`
}

type faceitMatchesIn struct {
	PlayerID string `json:"player_id" jsonschema:"FACEIT player id from faceit_player or list_faceit_follows"`
	Limit    int    `json:"limit,omitempty" jsonschema:"default 10, at most 20"`
}

type faceitMatchOut struct {
	ID          string  `json:"id"`
	Finished    string  `json:"finished,omitempty" jsonschema:"YYYY-MM-DD"`
	Competition string  `json:"competition,omitempty"`
	Map         string  `json:"map,omitempty"`
	Result      string  `json:"result,omitempty"`
	Score       string  `json:"score" jsonschema:"the player's team rounds, then the rival's"`
	Kills       int     `json:"kills"`
	Deaths      int     `json:"deaths"`
	Assists     int     `json:"assists"`
	ADR         float64 `json:"adr,omitempty"`
	HeadshotPct float64 `json:"headshot_pct,omitempty"`
	RoomURL     string  `json:"room_url,omitempty"`
}

type faceitMatchesOut struct {
	Matches []faceitMatchOut `json:"matches"`
}

type faceitFollowsOut struct {
	Enabled bool              `json:"enabled" jsonschema:"false when FACEIT is not configured in this Studio"`
	Players []faceitPlayerOut `json:"players"`
}

type faceitIDIn struct {
	PlayerID string `json:"player_id" jsonschema:"FACEIT player id from list_faceit_follows"`
}

type steamIDIn struct {
	SteamID string `json:"steamid" jsonschema:"SteamID64 of a library player"`
}

type practiceSpotIn struct {
	Label  string     `json:"label" jsonschema:"what CS2 shows for the spot, at most 48 characters"`
	Origin [3]float64 `json:"origin" jsonschema:"where the thrower stood, the origin of a team_grenades row"`
	View   [2]float64 `json:"view" jsonschema:"yaw and pitch, the view of the same row"`
	Kind   string     `json:"kind,omitempty" jsonschema:"smoke, flash, he, molotov or decoy to hand the player; empty hands none"`
	Side   string     `json:"side,omitempty" jsonschema:"CT gets an incendiary for a molotov"`
}

type practiceIn struct {
	Map     string           `json:"map" jsonschema:"such as de_mirage"`
	Spots     []practiceSpotIn `json:"spots,omitempty" jsonschema:"1 to 20 spots, in the order the keys cycle through them"`
	LineupIDs []string         `json:"lineup_ids,omitempty" jsonschema:"instead of spots: 1 to 20 Libreta lineup ids from list_lineups, all on map"`
	Approve   string           `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type practiceOut struct {
	pendingOut
	Launched       bool     `json:"launched,omitempty"`
	AlreadyRunning bool     `json:"already_running,omitempty" jsonschema:"CS2 was open: the user runs exec_command in its console"`
	ExecCommand    string   `json:"exec_command,omitempty"`
	NextKey        string   `json:"next_key,omitempty" jsonschema:"the key that cycles to the next spot"`
	Keys           []string `json:"keys,omitempty" jsonschema:"spot label and its key"`
}

type communityIn struct {
	Map string `json:"map,omitempty" jsonschema:"one map, such as de_mirage; empty means all maps"`
}

type rankingMetricOut struct {
	Metric     string   `json:"metric"`
	Value      *float64 `json:"value,omitempty" jsonschema:"the user's own value"`
	Percentile *float64 `json:"percentile,omitempty" jsonschema:"share of installs the user beats, 0 to 100"`
	Median     *float64 `json:"median,omitempty"`
	Installs   int      `json:"installs"`
}

type communityOut struct {
	RankingsEnabled bool               `json:"rankings_enabled" jsonschema:"the user shares their numbers for the ranking"`
	ServerEnabled   bool               `json:"server_enabled"`
	SteamIDSet      bool               `json:"steam_id_set" jsonschema:"false means the user's Steam account is not set in Studio, so there is no own line"`
	LastSubmitted   string             `json:"last_submitted,omitempty"`
	SubmitError     string             `json:"submit_error,omitempty"`
	Map             string             `json:"map,omitempty"`
	Rounds          int                `json:"rounds"`
	MinInstalls     int                `json:"min_installs" jsonschema:"a metric needs this many installs to rank"`
	Metrics         []rankingMetricOut `json:"metrics"`
}

type setRankingIn struct {
	Enabled bool   `json:"enabled" jsonschema:"true shares the user's per-map numbers with the ClipHub community server and submits them now"`
	Approve string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type rankingSettingsOut struct {
	pendingOut
	RankingsEnabled bool   `json:"rankings_enabled"`
	Maps            int    `json:"maps,omitempty" jsonschema:"maps submitted"`
	SubmitError     string `json:"submit_error,omitempty"`
}

type approveIn struct {
	Approve string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

func registerCommunityTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: "faceit_player", Annotations: readOnly,
		Description: "Find a FACEIT CS2 account by nickname or profile URL: id, level, ELO, country and SteamID64."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in faceitLookupIn) (*mcp.CallToolResult, faceitPlayerOut, error) {
			nick, err := checkFaceitNickname(in.Nickname)
			if err != nil {
				return nil, faceitPlayerOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, faceitPlayerOut{}, toolError(err)
			}
			p, err := c.LookupFaceitPlayer(ctx, nick)
			if err != nil {
				return nil, faceitPlayerOut{}, toolError(err)
			}
			return nil, faceitPlayerOf(p), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "faceit_matches", Annotations: readOnly,
		Description: "List a FACEIT player's recent matches with map, result, score, K/D/A, ADR and the match room link."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in faceitMatchesIn) (*mcp.CallToolResult, faceitMatchesOut, error) {
			if !faceitIDPattern.MatchString(in.PlayerID) {
				return nil, faceitMatchesOut{}, errors.New("player_id must be a FACEIT player id from faceit_player")
			}
			if in.Limit < 0 || in.Limit > maxFaceitMatches {
				return nil, faceitMatchesOut{}, fmt.Errorf("limit must be from 1 to %d", maxFaceitMatches)
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, faceitMatchesOut{}, toolError(err)
			}
			matches, err := c.FaceitMatches(ctx, in.PlayerID, cmp.Or(in.Limit, defaultFaceitMatches))
			if err != nil {
				return nil, faceitMatchesOut{}, toolError(err)
			}
			out := faceitMatchesOut{Matches: make([]faceitMatchOut, 0, len(matches))}
			for _, m := range matches {
				out.Matches = append(out.Matches, faceitMatchOf(m))
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_faceit_follows", Annotations: readOnly,
		Description: "List the FACEIT players of the Jugadores page: the ones the user follows and the top players Studio lists by itself."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, faceitFollowsOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, faceitFollowsOut{}, toolError(err)
			}
			f, err := c.ListFaceitFollows(ctx)
			if err != nil {
				return nil, faceitFollowsOut{}, toolError(err)
			}
			out := faceitFollowsOut{Enabled: f.Enabled, Players: make([]faceitPlayerOut, 0, len(f.Players))}
			for _, p := range f.Players {
				out.Players = append(out.Players, faceitPlayerOf(p))
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "follow_faceit_player", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Follow a FACEIT player by nickname or profile URL, as Seguir does on the Jugadores page."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in faceitLookupIn) (*mcp.CallToolResult, faceitPlayerOut, error) {
			nick, err := checkFaceitNickname(in.Nickname)
			if err != nil {
				return nil, faceitPlayerOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, faceitPlayerOut{}, toolError(err)
			}
			p, err := c.FollowFaceitPlayer(ctx, nick)
			if err != nil {
				return nil, faceitPlayerOut{}, toolError(err)
			}
			return nil, faceitPlayerOf(p), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "unfollow_faceit_player", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Remove a player from the Jugadores list: a followed player is unfollowed, a listed top player is hidden."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in faceitIDIn) (*mcp.CallToolResult, empty, error) {
			if !faceitIDPattern.MatchString(in.PlayerID) {
				return nil, empty{}, errors.New("player_id must be a FACEIT player id from list_faceit_follows")
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, empty{}, toolError(err)
			}
			return nil, empty{}, toolError(c.UnfollowFaceitPlayer(ctx, in.PlayerID))
		})

	mcp.AddTool(server, &mcp.Tool{Name: "link_player_faceit", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Look a library player's SteamID64 up on FACEIT and store the account on their profile, as the player page does."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in steamIDIn) (*mcp.CallToolResult, playerSummaryOut, error) {
			if err := checkSteamID(in.SteamID); err != nil {
				return nil, playerSummaryOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, playerSummaryOut{}, toolError(err)
			}
			p, err := c.LinkPlayerFaceit(ctx, in.SteamID)
			if err != nil {
				return nil, playerSummaryOut{}, toolError(err)
			}
			return nil, playerSummaryOf(p), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "launch_practice", Annotations: &mcp.ToolAnnotations{},
		Description: "Open CS2 on a local practice server standing on grenade spots, as Practicar does on the grenades page: take origin and view from team_grenades rows, " +
			"or pass lineup_ids of Libreta lineups. " +
			"It opens the game on the user's PC, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in practiceIn) (*mcp.CallToolResult, practiceOut, error) {
			spots, err := in.practiceSpots(ctx, s)
			if err != nil {
				return nil, practiceOut{}, err
			}
			summary := fmt.Sprintf("Abrir CS2 en un servidor de práctica en %s con %s.", cleanName(in.Map), plural(len(spots), "posición", "posiciones"))
			ask, pending, err := gate(req, approvalSpec{Tool: "launch_practice", JobID: in.Map, Request: spots, Summary: summary, Given: in.Approve, Question: "¿Abrir ahora?"})
			if err != nil || ask != nil {
				return ask, practiceOut{}, err
			}
			if pending != nil {
				return nil, practiceOut{pendingOut: *pending}, nil
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, practiceOut{}, toolError(err)
			}
			res, err := c.LaunchPractice(ctx, in.Map, spots)
			if err != nil {
				return nil, practiceOut{}, toolError(err)
			}
			out := practiceOut{pendingOut: pendingOut{Summary: summary}, Launched: res.Launched, AlreadyRunning: res.AlreadyRunning,
				ExecCommand: cleanName(res.ExecCommand), NextKey: cleanName(res.NextKey)}
			for _, sp := range res.Spots {
				out.Keys = append(out.Keys, cleanName(sp.Label)+": "+cleanName(sp.Key))
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "community_ranking", Annotations: readOnly,
		Description: "Read the community ranking: whether the user shares their numbers, and their percentile on each metric against every Studio install that shares, overall or on one map."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in communityIn) (*mcp.CallToolResult, communityOut, error) {
			if in.Map != "" && !rankingMapPattern.MatchString(in.Map) {
				return nil, communityOut{}, errors.New("map must be a map name such as de_mirage")
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, communityOut{}, toolError(err)
			}
			st, err := c.GetCommunitySettings(ctx)
			if err != nil {
				return nil, communityOut{}, toolError(err)
			}
			rk, err := c.GetCommunityRankings(ctx, in.Map)
			if err != nil {
				return nil, communityOut{}, toolError(err)
			}
			return nil, communityOf(st, rk), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "set_community_ranking", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Turn sharing for the community ranking on (sends the user's per-map numbers to the ClipHub community server now and after new matches) or off. " +
			"Turning it on sends data off this PC, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in setRankingIn) (*mcp.CallToolResult, rankingSettingsOut, error) {
			if in.Enabled {
				summary := "Compartir tus números por mapa con el servidor de la comunidad de ClipHub para el ranking. Se envían ahora y tras cada partida nueva."
				ask, pending, err := gate(req, approvalSpec{Tool: "set_community_ranking", JobID: "on", Request: true, Summary: summary, Given: in.Approve, Question: "¿Activar?"})
				if err != nil || ask != nil {
					return ask, rankingSettingsOut{}, err
				}
				if pending != nil {
					return nil, rankingSettingsOut{pendingOut: *pending}, nil
				}
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, rankingSettingsOut{}, toolError(err)
			}
			st, err := c.SetCommunityRankings(ctx, in.Enabled)
			if err != nil {
				return nil, rankingSettingsOut{}, toolError(err)
			}
			return nil, rankingSettingsOut{RankingsEnabled: st.RankingsEnabled, SubmitError: cleanName(st.SubmitError)}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "submit_community_ranking", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Send the user's current per-map numbers to the community ranking now, as Enviar ahora does. Sharing must be on (set_community_ranking)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, rankingSettingsOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, rankingSettingsOut{}, toolError(err)
			}
			st, err := c.SubmitCommunityRankings(ctx)
			if err != nil {
				return nil, rankingSettingsOut{}, toolError(err)
			}
			return nil, rankingSettingsOut{RankingsEnabled: st.RankingsEnabled, Maps: st.Maps}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "delete_community_ranking", Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true)},
		Description: "Make the community server forget this install's numbers and turn sharing off. It cannot be undone, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in approveIn) (*mcp.CallToolResult, rankingSettingsOut, error) {
			summary := "Borrar tus números del servidor de la comunidad y dejar de compartirlos. No se puede deshacer."
			ask, pending, err := gate(req, approvalSpec{Tool: "delete_community_ranking", JobID: "install", Request: true, Summary: summary, Given: in.Approve, Question: "¿Borrar ahora?"})
			if err != nil || ask != nil {
				return ask, rankingSettingsOut{}, err
			}
			if pending != nil {
				return nil, rankingSettingsOut{pendingOut: *pending}, nil
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, rankingSettingsOut{}, toolError(err)
			}
			st, err := c.DeleteCommunityRankings(ctx)
			if err != nil {
				return nil, rankingSettingsOut{}, toolError(err)
			}
			return nil, rankingSettingsOut{pendingOut: pendingOut{Summary: summary}, RankingsEnabled: st.RankingsEnabled}, nil
		})
}

func checkFaceitNickname(raw string) (string, error) {
	nick := strings.TrimSpace(raw)
	if nick == "" || len(nick) > 200 || strings.ContainsFunc(nick, func(r rune) bool { return r < ' ' }) {
		return "", errors.New("nickname must be a FACEIT nickname or profile URL")
	}
	return nick, nil
}

func (in practiceIn) spots() ([]tuiclient.PracticeSpot, error) {
	if !practiceMapPattern.MatchString(in.Map) {
		return nil, errors.New("map must be a map name such as de_mirage")
	}
	if len(in.Spots) == 0 || len(in.Spots) > maxPracticeSpots {
		return nil, fmt.Errorf("spots takes 1 to %d spots", maxPracticeSpots)
	}
	out := make([]tuiclient.PracticeSpot, 0, len(in.Spots))
	for i, sp := range in.Spots {
		label := strings.TrimSpace(sp.Label)
		if label == "" || len([]rune(label)) > maxPracticeLabel {
			return nil, fmt.Errorf("spot %d: label takes 1 to %d characters", i+1, maxPracticeLabel)
		}
		for _, v := range sp.Origin {
			if math.IsNaN(v) || math.Abs(v) > maxWorldCoord {
				return nil, fmt.Errorf("spot %d: origin is outside the map", i+1)
			}
		}
		yaw, pitch := sp.View[0], sp.View[1]
		if math.Abs(pitch) > 90 || math.Abs(yaw) > 360 {
			return nil, fmt.Errorf("spot %d: view must be yaw and pitch in degrees", i+1)
		}
		grenade := sp.Kind
		if grenade != "" && !slices.Contains(practiceGrenades, grenade) {
			return nil, fmt.Errorf("spot %d: kind must be one of %s", i+1, strings.Join(practiceGrenades, ", "))
		}
		if grenade == "molotov" && strings.EqualFold(sp.Side, "CT") {
			grenade = "incendiary"
		}
		out = append(out, tuiclient.PracticeSpot{Label: label, Pos: sp.Origin, Ang: [2]float64{pitch, yaw}, Grenade: grenade})
	}
	return out, nil
}

func faceitPlayerOf(p tuiclient.FaceitPlayer) faceitPlayerOut {
	return faceitPlayerOut{ID: cleanName(p.ID), Nickname: cleanName(p.Nickname), SteamID64: cleanName(p.SteamID64), Country: cleanName(p.Country),
		Level: p.SkillLevel, ELO: p.ELO, ProfileURL: cleanText(p.ProfileURL, maxErrorRunes), Seeded: p.Seeded}
}

func faceitMatchOf(m tuiclient.FaceitMatch) faceitMatchOut {
	out := faceitMatchOut{ID: cleanName(m.ID), Competition: cleanName(m.Competition), Score: fmt.Sprintf("%d-%d", m.Score.For, m.Score.Against),
		RoomURL: cleanText(m.RoomURL, maxErrorRunes)}
	if !m.FinishedAt.IsZero() {
		out.Finished = m.FinishedAt.Format(time.DateOnly)
	}
	if st := m.Stats; st != nil {
		out.Map, out.Result, out.Kills, out.Deaths, out.Assists = cleanName(st.Map), cleanName(st.Result), st.Kills, st.Deaths, st.Assists
		out.ADR, out.HeadshotPct = st.ADR, st.HeadshotsPercent
	}
	return out
}

func communityOf(st tuiclient.CommunitySettings, rk tuiclient.CommunityRankings) communityOut {
	out := communityOut{RankingsEnabled: st.RankingsEnabled, ServerEnabled: st.ServerEnabled, SteamIDSet: st.SteamIDSet,
		SubmitError: cleanName(st.SubmitError), Map: cleanName(rk.Map), Rounds: rk.Rounds, MinInstalls: rk.MinInstalls, Metrics: []rankingMetricOut{}}
	if st.LastSubmittedAt != nil {
		out.LastSubmitted = st.LastSubmittedAt.Format(time.DateOnly)
	}
	for _, m := range rk.Metrics {
		out.Metrics = append(out.Metrics, rankingMetricOut{Metric: cleanName(m.Metric), Value: m.Value, Percentile: m.Percentile, Median: m.Median, Installs: m.Installs})
	}
	return out
}
