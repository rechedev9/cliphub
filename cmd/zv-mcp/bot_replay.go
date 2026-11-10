package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

type botReplayIn struct {
	MatchID        string `json:"match_id" jsonschema:"match id from list_matches, or the match part of a round ref"`
	Round          int    `json:"round" jsonschema:"round number, as in round refs"`
	Side           string `json:"side" jsonschema:"T or CT: the side the user plays; the bots are the other side"`
	ReplaceSteamID string `json:"replace_steamid,omitempty" jsonschema:"SteamID64 of a player of side in that round whose spawn and positions the user takes; empty takes the first"`
	Approve        string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type botReplayOut struct {
	pendingOut
	Launched       bool     `json:"launched,omitempty"`
	AlreadyRunning bool     `json:"already_running,omitempty" jsonschema:"CS2 was open: the user runs exec_command in its console"`
	ExecCommand    string   `json:"exec_command,omitempty"`
	Bots           []string `json:"bots,omitempty" jsonschema:"the players the bots stand in for, in the order they join"`
	StartsAs       string   `json:"starts_as,omitempty" jsonschema:"the player whose spawn and positions the user takes"`
	Keys           []string `json:"keys,omitempty" jsonschema:"the keypad keys of the session and what each does"`
	Moments        []string `json:"moments,omitempty" jsonschema:"the snapshots the next key walks through, with the bots' grenades of each"`
	Limits         []string `json:"limits,omitempty" jsonschema:"what plain CS2 cannot do here; tell the user"`
}

func registerBotReplayTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "launch_bot_replay", Annotations: &mcp.ToolAnnotations{},
		Description: "Open CS2 on a local practice server against one library round, as Practicar contra esta ronda does in the Replay's round panel: " +
			"the user plays side, bots stand where each player of the other side stood at the round's key moments (start, grenades, first contact, plant), " +
			"frozen and not shooting; a dead rival's bot waits at its spawn. In game the user presses KP_ENTER twice to set the server up and add the bots, " +
			"then KP_ENTER places them at moment 1, KP_PLUS steps, KP_MINUS changes the spawn (otra salida), KP_DEL shifts the bot numbers when they do not move, " +
			"and the keypad digits put the user on the rivals' grenade throws. The keypad binds replace the user's own. " +
			"Plain CS2 cannot make bots walk the recorded paths, look where the rival looked or throw grenades without a server plugin; pass limits on to the user. " +
			"It opens the game on the user's PC, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in botReplayIn) (*mcp.CallToolResult, botReplayOut, error) {
			request, err := in.request()
			if err != nil {
				return nil, botReplayOut{}, err
			}
			summary := fmt.Sprintf("Abrir CS2 en un servidor de práctica contra la ronda %d, jugando de %s.", in.Round, request.Side)
			ask, pending, err := gate(req, approvalSpec{Tool: "launch_bot_replay", JobID: in.MatchID, Request: request, Summary: summary, Given: in.Approve, Question: "¿Abrir ahora?"})
			if err != nil || ask != nil {
				return ask, botReplayOut{}, err
			}
			if pending != nil {
				return nil, botReplayOut{pendingOut: *pending}, nil
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, botReplayOut{}, toolError(err)
			}
			res, err := c.LaunchBotReplay(ctx, request)
			if err != nil {
				return nil, botReplayOut{}, toolError(err)
			}
			return nil, botReplayOutOf(res, summary), nil
		})
}

func (in botReplayIn) request() (tuiclient.BotReplayRequest, error) {
	if err := checkUUID("match_id", in.MatchID); err != nil {
		return tuiclient.BotReplayRequest{}, err
	}
	if in.Round < 1 || in.Round > maxPlayRound {
		return tuiclient.BotReplayRequest{}, fmt.Errorf("round must be from 1 to %d", maxPlayRound)
	}
	side := strings.ToUpper(strings.TrimSpace(in.Side))
	if side != "T" && side != "CT" {
		return tuiclient.BotReplayRequest{}, fmt.Errorf("side must be T or CT")
	}
	if in.ReplaceSteamID != "" {
		if err := checkSteamID(in.ReplaceSteamID); err != nil {
			return tuiclient.BotReplayRequest{}, err
		}
	}
	return tuiclient.BotReplayRequest{MatchID: in.MatchID, Round: in.Round, Side: side, ReplaceSteamID: in.ReplaceSteamID}, nil
}

func botReplayOutOf(res tuiclient.BotReplay, summary string) botReplayOut {
	out := botReplayOut{pendingOut: pendingOut{Summary: summary}, Launched: res.Launched, AlreadyRunning: res.AlreadyRunning,
		ExecCommand: cleanName(res.ExecCommand), Limits: cleanNames(res.Limits)}
	for _, b := range res.Bots {
		out.Bots = append(out.Bots, cleanName(b.Name))
	}
	if res.Current >= 0 && res.Current < len(res.Starts) {
		out.StartsAs = cleanName(res.Starts[res.Current].Name)
	}
	out.Keys = []string{
		cleanName(res.Keys.Restart) + ": dos veces para preparar el servidor y crear los bots; después coloca a los bots en el momento 1",
		cleanName(res.Keys.Next) + ": siguiente momento",
		cleanName(res.Keys.Start) + ": otra salida",
		cleanName(res.Keys.Base) + ": cambia el número del primer bot si no se colocan; vuelve a pulsar " + cleanName(res.Keys.Restart),
		cleanName(res.Keys.Prepare) + ": prepara el servidor otra vez",
	}
	for _, l := range res.Lineups {
		out.Keys = append(out.Keys, fmt.Sprintf("%s: %s de %s (%.1f s)", cleanName(l.Key), cleanName(l.Kind), cleanName(l.Player), l.Seconds))
	}
	for _, m := range res.Moments {
		line := fmt.Sprintf("%.1f s %s", m.Seconds, cleanName(m.Label))
		if len(m.Notes) > 0 {
			line += ": " + strings.Join(cleanNames(m.Notes), ", ")
		}
		out.Moments = append(out.Moments, line)
	}
	return out
}
