package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const botReplayMatch = "9be6d5a6-abda-4110-908d-deae67dee533"

func TestLaunchBotReplayAsksThenLaunches(t *testing.T) {
	rec := &recorder{seen: map[string]string{}}
	mux := http.NewServeMux()
	rec.handle(mux, "POST /api/library/matches/{id}/tactical/rounds/{round}/bot-replay", `{"launched":true,"exec_command":"exec cliphub_practice",
		"keys":{"prepare":"KP_INS","restart":"KP_ENTER","next":"KP_PLUS","start":"KP_MINUS","base":"KP_DEL"},"bots":[{"name":"carl","index":2}],
		"starts":[{"name":"ana","steamid64":"1"}],"current":0,
		"moments":[{"seconds":0,"label":"Salida","notes":[]},{"seconds":9.4,"label":"Granadas","notes":["Humo de carl"]}],
		"lineups":[{"seconds":9.4,"kind":"smoke","player":"carl","key":"KP_END"}],"limits":["Los bots no repiten el recorrido."]}`)
	cs := serveTools(t, mux, func(server *mcp.Server, s *studio) { registerBotReplayTools(server, s) })

	args := map[string]any{"match_id": botReplayMatch, "round": 3, "side": "ct", "replace_steamid": "76561198000000001"}
	var pending botReplayOut
	callTool(t, cs, "launch_bot_replay", args, &pending)
	if !pending.NeedsApproval || !strings.Contains(pending.Summary, "ronda 3, jugando de CT") || rec.get("POST /api/library/matches/{id}/tactical/rounds/{round}/bot-replay") != "" {
		t.Fatalf("pending = %+v", pending)
	}
	args["approve"] = pending.ApprovalCode
	var out botReplayOut
	callTool(t, cs, "launch_bot_replay", args, &out)
	got := rec.get("POST /api/library/matches/{id}/tactical/rounds/{round}/bot-replay")
	if !strings.Contains(got, "/rounds/3/bot-replay") || !strings.HasSuffix(got, `{"side":"CT","replace_steamid":"76561198000000001"}`) {
		t.Errorf("request = %s", got)
	}
	if !out.Launched || out.StartsAs != "ana" || len(out.Bots) != 1 || len(out.Keys) != 6 || out.Keys[5] != "KP_END: smoke de carl (9.4 s)" || !strings.HasPrefix(out.Keys[3], "KP_DEL:") {
		t.Errorf("out = %+v", out)
	}
	if len(out.Moments) != 2 || out.Moments[1] != "9.4 s Granadas: Humo de carl" || len(out.Limits) != 1 {
		t.Errorf("moments = %v limits = %v", out.Moments, out.Limits)
	}
	toolFails(t, cs, "launch_bot_replay", map[string]any{"match_id": "x", "round": 1, "side": "T"}, "match_id")
	toolFails(t, cs, "launch_bot_replay", map[string]any{"match_id": botReplayMatch, "round": 0, "side": "T"}, "round")
	toolFails(t, cs, "launch_bot_replay", map[string]any{"match_id": botReplayMatch, "round": 1, "side": "spec"}, "side")
}
