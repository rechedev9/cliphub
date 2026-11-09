package main

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// replayForms are the four ways the Replay reads its rounds, as Studio names
// them in the URL.
var replayForms = []string{"repeticion", "calor", "recorridos", "granadas"}

type replayRoundsIn struct {
	MatchID      string   `json:"match_id" jsonschema:"match id from list_matches"`
	TeamKey      string   `json:"team_key,omitempty" jsonschema:"follow one team across the side swap: ct-start or t-start, from get_match"`
	Side         string   `json:"side,omitempty" jsonschema:"only rounds seen from CT or T"`
	Result       string   `json:"result,omitempty" jsonschema:"won or lost, from the team's or the side's view; needs team_key or side"`
	Buy          []string `json:"buy,omitempty" jsonschema:"own buys: pistol, eco, semi, force, full"`
	OpponentBuy  []string `json:"opponent_buy,omitempty" jsonschema:"the other side's buys, same values"`
	Site         []string `json:"site,omitempty" jsonschema:"where the round went: A, B, mid or none"`
	AttackShape  []string `json:"attack_shape,omitempty" jsonschema:"T round shapes, such as execute, default, split or fast"`
	DefenseShape []string `json:"defense_shape,omitempty" jsonschema:"CT round shapes, as get_round reports them"`
	Tag          []string `json:"tag,omitempty" jsonschema:"round tags, such as postplant, pistol, ace or retake_won"`
	Players      []string `json:"players,omitempty" jsonschema:"SteamID64s: only rounds these players were in"`
	Focus        []string `json:"focus,omitempty" jsonschema:"SteamID64s of the players the Replay puts in the foreground: the rest go translucent in Repetición, and Mapa de calor, Recorridos and Granadas count only them. It never drops a round"`
	RoundFrom    int      `json:"round_from,omitempty"`
	RoundTo      int      `json:"round_to,omitempty"`
	Phase        string   `json:"phase,omitempty" jsonschema:"regulation or overtime"`
	Form         string   `json:"form,omitempty" jsonschema:"which form studio_path opens: repeticion (default), calor, recorridos or granadas"`
}

type replayRoundOut struct {
	Ref          string `json:"ref" jsonschema:"match_id/round, as get_round takes it"`
	Round        int    `json:"round"`
	Winner       string `json:"winner" jsonschema:"CT or T"`
	CTBuy        string `json:"ct_buy"`
	TBuy         string `json:"t_buy"`
	Site         string `json:"site,omitempty"`
	AttackShape  string `json:"attack_shape,omitempty"`
	DefenseShape string `json:"defense_shape,omitempty"`
}

type replayRoundsOut struct {
	MatchID    string           `json:"match_id"`
	Map        string           `json:"map"`
	Rounds     []replayRoundOut `json:"rounds" jsonschema:"the rounds the scope keeps, exactly the ones the Replay's scope bar shows"`
	Kept       int              `json:"kept"`
	Total      int              `json:"total"`
	StudioPath string           `json:"studio_path" jsonschema:"the path that opens the Replay in Studio with this scope and form"`
}

// values turns the tool input into the query the Replay keeps in its URL, the
// same one tacticalplan.FilterFromValues reads.
func (in replayRoundsIn) values(doc tacticalplan.Document) (url.Values, error) {
	v := url.Values{}
	if in.TeamKey != "" && in.TeamKey != "ct-start" && in.TeamKey != "t-start" {
		return nil, fmt.Errorf("team_key must be ct-start or t-start, not %q", cleanName(in.TeamKey))
	}
	side, err := parseSide(in.Side)
	if err != nil {
		return nil, err
	}
	outcome, ok := map[string]string{"": "", roundResultWon: "win", roundResultLost: "loss"}[in.Result]
	if !ok {
		return nil, fmt.Errorf("result must be won or lost, not %q", cleanName(in.Result))
	}
	if outcome != "" && in.TeamKey == "" && side == "" {
		return nil, fmt.Errorf("result needs team_key or side: a round is won or lost by someone")
	}
	for key, value := range map[string]string{"team": in.TeamKey, "side": side, "outcome": outcome, "phase": strings.ToLower(strings.TrimSpace(in.Phase))} {
		if value != "" {
			v.Set(key, value)
		}
	}
	for key, list := range map[string][]string{"buy": in.Buy, "opponent_buy": in.OpponentBuy, "site": in.Site, "t_pattern": in.AttackShape, "ct_pattern": in.DefenseShape, "tag": in.Tag} {
		for _, value := range list {
			v.Add(key, strings.TrimSpace(value))
		}
	}
	for key, ids := range map[string][]string{"slot": in.Players, "subject": in.Focus} {
		for _, id := range ids {
			if err := checkSteamID(id); err != nil {
				return nil, err
			}
			i := slices.IndexFunc(doc.Players, func(p tacticalplan.Player) bool { return p.SteamID64 == id })
			if i < 0 {
				return nil, fmt.Errorf("player %s did not play this match", id)
			}
			v.Add(key, strconv.Itoa(int(doc.Players[i].Slot)))
		}
	}
	if in.RoundFrom < 0 || in.RoundTo < 0 {
		return nil, fmt.Errorf("round_from and round_to must be positive")
	}
	if in.RoundFrom > 0 {
		v.Set("round_from", strconv.Itoa(in.RoundFrom))
	}
	if in.RoundTo > 0 {
		v.Set("round_to", strconv.Itoa(in.RoundTo))
	}
	return v, nil
}

type playerSetposIn struct {
	MatchID string  `json:"match_id" jsonschema:"match id from list_matches"`
	Round   int     `json:"round" jsonschema:"round number, as in a round ref"`
	SteamID string  `json:"steamid" jsonschema:"the player's SteamID64"`
	Second  float64 `json:"second,omitempty" jsonschema:"seconds after the round went live (the end of freeze time); 0 by default"`
}

type playerSetposOut struct {
	Player string  `json:"player"`
	Round  int     `json:"round"`
	Second float64 `json:"second" jsonschema:"the sampled moment the line was read at, in seconds after the round went live"`
	Tick   int     `json:"tick"`
	Alive  bool    `json:"alive"`
	Setpos string  `json:"setpos" jsonschema:"the console line for CS2 with sv_cheats 1: setpos x y z;setang pitch yaw 0"`
}

// setposAt reads the setpos of one slot from the frame nearest to a tick.
func setposAt(frames []tacticalplan.Frame, slot uint8, tick int) (tacticalplan.Frame, tacticalplan.Sample, bool) {
	best := -1
	for i, f := range frames {
		if best < 0 || abs(f.Tick-tick) < abs(frames[best].Tick-tick) {
			best = i
		}
	}
	if best < 0 {
		return tacticalplan.Frame{}, tacticalplan.Sample{}, false
	}
	for _, sample := range frames[best].Samples {
		if sample.Slot == slot {
			return frames[best], sample, true
		}
	}
	return tacticalplan.Frame{}, tacticalplan.Sample{}, false
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func setposLine(s tacticalplan.Sample) string {
	return fmt.Sprintf("setpos %.2f %.2f %.2f;setang %.2f %.2f 0", s.X, s.Y, s.Z, tacticalplan.SetangPitch(s.Pitch), tacticalplan.SetangYaw(s.Yaw))
}

type roundGrenadeOut struct {
	Second  float64 `json:"second" jsonschema:"seconds after the round went live"`
	Kind    string  `json:"kind"`
	Side    string  `json:"side,omitempty"`
	Player  string  `json:"player,omitempty"`
	Callout string  `json:"callout,omitempty" jsonschema:"where it landed"`
	Status  string  `json:"status" jsonschema:"whether the catalog has a lineup for it"`
	Lineup  string  `json:"lineup,omitempty" jsonschema:"the catalog lineup it matches: from area to destination"`
}

type roundLineupsOut struct {
	Ref         string            `json:"ref"`
	Map         string            `json:"map"`
	CatalogSize int               `json:"catalog_size" jsonschema:"lineups the catalog has for this map"`
	Smokes      int               `json:"smokes"`
	Matched     int               `json:"matched" jsonschema:"smokes that match a catalog lineup"`
	Grenades    []roundGrenadeOut `json:"grenades"`
}

type replayPlaylistIn struct {
	Rounds []string `json:"rounds" jsonschema:"round refs (match_id/round) in play order, from find_rounds, replay_rounds or the reports; at most 200"`
	Title  string   `json:"title,omitempty" jsonschema:"the playlist's name, shown over the rounds"`
}

type replayPlaylistOut struct {
	Rounds     int    `json:"rounds"`
	StudioPath string `json:"studio_path" jsonschema:"the path that opens these rounds in the Replay as one playlist"`
}

// maxPlaylistRounds mirrors REPLAY_MAX_REFS in the web: a longer URL is cut there.
const maxPlaylistRounds = 200

// playlistPath is the Replay URL of a list of round refs, as the web's replayHref writes it.
func playlistPath(refs []string, title string) (string, error) {
	if len(refs) == 0 || len(refs) > maxPlaylistRounds {
		return "", fmt.Errorf("rounds must have from 1 to %d round refs", maxPlaylistRounds)
	}
	v := url.Values{}
	for _, ref := range refs {
		matchID, round, ok := strings.Cut(strings.TrimSpace(ref), "/")
		n, err := strconv.Atoi(round)
		if !ok || err != nil || n < 1 || n > maxPlayRound {
			return "", fmt.Errorf("%q is not a round ref (match_id/round)", cleanName(ref))
		}
		if err := checkUUID("match_id", matchID); err != nil {
			return "", err
		}
		v.Add("r", matchID+":"+strconv.Itoa(n))
	}
	if title = strings.TrimSpace(title); title != "" {
		v.Set("t", title)
	}
	return "/replay?" + v.Encode(), nil
}

type overlayRoundOut struct {
	Ref     string   `json:"ref"`
	Side    string   `json:"side,omitempty"`
	Winner  string   `json:"winner"`
	Buy     string   `json:"buy,omitempty"`
	Site    string   `json:"site,omitempty"`
	Seconds float64  `json:"seconds" jsonschema:"how long the round ran after going live"`
	Players []string `json:"players" jsonschema:"whose paths the round draws: the focus, or the team or side in scope"`
}

type replayOverlayOut struct {
	MatchID    string            `json:"match_id"`
	Map        string            `json:"map"`
	Superposed int               `json:"superposed" jsonschema:"rounds drawn at once"`
	Total      int               `json:"total" jsonschema:"rounds the scope keeps; more than superposed when the cap cut the list"`
	Cap        int               `json:"cap" jsonschema:"most rounds one overlay draws"`
	Rounds     []overlayRoundOut `json:"rounds"`
	StudioPath string            `json:"studio_path" jsonschema:"the path that opens these rounds superposed in the Replay"`
}

// overlayOut names the superposed rounds and whose paths each draws, and the path that opens them in Studio.
func overlayOut(matchID string, got tuiclient.MatchOverlay, scope url.Values) replayOverlayOut {
	out := replayOverlayOut{MatchID: matchID, Map: cleanName(got.Map), Rounds: []overlayRoundOut{}}
	if got.Overlay != nil {
		out.Total, out.Cap = got.Overlay.Total, got.Overlay.Limit
		for _, r := range got.Overlay.Rounds {
			row := overlayRoundOut{Ref: fmt.Sprintf("%s/%d", matchID, r.Round), Side: string(r.Side), Winner: string(r.Winner),
				Buy: string(r.Buy), Site: string(r.Site), Seconds: r.DurationSeconds, Players: []string{}}
			for _, track := range r.Tracks {
				row.Players = append(row.Players, cleanName(track.Name))
			}
			out.Rounds = append(out.Rounds, row)
		}
	}
	out.Superposed = len(out.Rounds)
	path := url.Values{}
	for key, list := range scope {
		path[key] = list
	}
	path.Set("tab", "replay")
	path.Set("forma", "recorridos")
	out.StudioPath = "/partidas/" + url.PathEscape(matchID) + "?" + path.Encode()
	return out
}

func registerReplayTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "replay_overlay", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Superpose the rounds of one match that a Replay scope keeps, as Superponer does: every round starts at the end of freeze time. " +
			"It takes the scope of replay_rounds and the same focus: with players in focus only their paths are drawn, in every round they played. " +
			"The answer lists the rounds and whose paths each draws; studio_path opens them superposed in Studio."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in replayRoundsIn) (*mcp.CallToolResult, replayOverlayOut, error) {
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, replayOverlayOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, replayOverlayOut{}, toolError(err)
			}
			doc, err := c.LibraryTacticalPlan(ctx, in.MatchID)
			if err != nil {
				return nil, replayOverlayOut{}, toolError(err)
			}
			values, err := in.values(doc)
			if err != nil {
				return nil, replayOverlayOut{}, err
			}
			got, err := c.LibraryMatchOverlay(ctx, in.MatchID, values)
			if err != nil {
				return nil, replayOverlayOut{}, toolError(err)
			}
			return nil, overlayOut(in.MatchID, got, values), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "round_lineups", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Compare one round's grenades with the lineup catalog, as Comparar granadas does in the Replay's round panel: each smoke, flash, HE and molotov, " +
			"who threw it, where it landed and the catalog lineup it matches."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in roundIn) (*mcp.CallToolResult, roundLineupsOut, error) {
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, roundLineupsOut{}, err
			}
			if in.Round < 1 || in.Round > maxPlayRound {
				return nil, roundLineupsOut{}, fmt.Errorf("round must be from 1 to %d", maxPlayRound)
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, roundLineupsOut{}, toolError(err)
			}
			got, err := c.LibraryRoundLineups(ctx, in.MatchID, in.Round)
			if err != nil {
				return nil, roundLineupsOut{}, toolError(err)
			}
			out := roundLineupsOut{Ref: fmt.Sprintf("%s/%d", in.MatchID, in.Round), Map: cleanName(got.Map), CatalogSize: got.CatalogSize,
				Smokes: got.Smokes, Matched: got.Matched, Grenades: []roundGrenadeOut{}}
			for _, g := range got.Grenades {
				row := roundGrenadeOut{Second: g.Seconds, Kind: cleanName(g.Kind), Side: cleanName(g.Side), Player: cleanName(g.Player),
					Callout: cleanName(g.Callout), Status: cleanName(g.Status)}
				if g.Lineup != nil {
					row.Lineup = cleanName(strings.TrimSpace(g.Lineup.FromArea + " to " + g.Lineup.Destination))
				}
				out.Grenades = append(out.Grenades, row)
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "replay_playlist", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Put rounds of any matches in one playlist, as B does on each round in the Replay: studio_path opens them in the Replay in the order given, " +
			"to play one after another or to superpose. Nothing is saved; the playlist is the path."},
		func(_ context.Context, _ *mcp.CallToolRequest, in replayPlaylistIn) (*mcp.CallToolResult, replayPlaylistOut, error) {
			path, err := playlistPath(in.Rounds, in.Title)
			if err != nil {
				return nil, replayPlaylistOut{}, err
			}
			return nil, replayPlaylistOut{Rounds: len(in.Rounds), StudioPath: path}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "player_setpos", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read the setpos line of one player at a moment of a round, as the S key over a player in the Replay copies it: paste it in the CS2 console " +
			"with sv_cheats 1 to stand where they stood, looking where they looked. Positions are sampled a few times a second; the answer says which moment it read."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in playerSetposIn) (*mcp.CallToolResult, playerSetposOut, error) {
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, playerSetposOut{}, err
			}
			if err := checkSteamID(in.SteamID); err != nil {
				return nil, playerSetposOut{}, err
			}
			if in.Round < 1 || in.Round > maxPlayRound {
				return nil, playerSetposOut{}, fmt.Errorf("round must be from 1 to %d", maxPlayRound)
			}
			if in.Second < 0 || in.Second > 600 {
				return nil, playerSetposOut{}, fmt.Errorf("second must be from 0 to 600")
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, playerSetposOut{}, toolError(err)
			}
			doc, err := c.LibraryTacticalPlan(ctx, in.MatchID)
			if err != nil {
				return nil, playerSetposOut{}, toolError(err)
			}
			i := slices.IndexFunc(doc.Players, func(p tacticalplan.Player) bool { return p.SteamID64 == in.SteamID })
			if i < 0 {
				return nil, playerSetposOut{}, fmt.Errorf("player %s did not play this match", in.SteamID)
			}
			round, err := c.LibraryTacticalRound(ctx, in.MatchID, in.Round)
			if err != nil {
				return nil, playerSetposOut{}, toolError(err)
			}
			rate := doc.Demo.Tickrate
			if rate <= 0 {
				rate = 64
			}
			frame, sample, ok := setposAt(round.Frames, doc.Players[i].Slot, round.Round.TickFreezeEnd+int(in.Second*rate))
			if !ok {
				return nil, playerSetposOut{}, fmt.Errorf("round %d has no saved position for that player", in.Round)
			}
			return nil, playerSetposOut{
				Player: cleanName(doc.Players[i].Name), Round: in.Round, Tick: frame.Tick,
				Second: float64(int(float64(frame.Tick-round.Round.TickFreezeEnd)/rate*10)) / 10,
				Alive:  sample.Flags.Has(tacticalplan.FlagAlive), Setpos: setposLine(sample),
			}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "replay_rounds", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read the rounds of one match that a Replay scope keeps, exactly as the scope bar of the Replay does: team, side, result, buys, site, " +
			"round shapes, tags, players, round range and phase. Every form of the Replay (Repetición, Mapa de calor, Recorridos, Granadas) reads these same rounds. " +
			"focus puts players in the foreground without dropping rounds, as a click on a player does. " +
			"studio_path opens the Replay in Studio on that scope and form."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in replayRoundsIn) (*mcp.CallToolResult, replayRoundsOut, error) {
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, replayRoundsOut{}, err
			}
			form := strings.ToLower(strings.TrimSpace(in.Form))
			if form != "" && !slices.Contains(replayForms, form) {
				return nil, replayRoundsOut{}, fmt.Errorf("form must be one of %s, not %q", strings.Join(replayForms, ", "), cleanName(in.Form))
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, replayRoundsOut{}, toolError(err)
			}
			doc, err := c.LibraryTacticalPlan(ctx, in.MatchID)
			if err != nil {
				return nil, replayRoundsOut{}, toolError(err)
			}
			values, err := in.values(doc)
			if err != nil {
				return nil, replayRoundsOut{}, err
			}
			filter, err := tacticalplan.FilterFromValues(values)
			if err != nil {
				return nil, replayRoundsOut{}, err
			}
			out := replayRoundsOut{MatchID: in.MatchID, Map: cleanName(doc.Demo.Map), Rounds: []replayRoundOut{}, Total: len(doc.Rounds)}
			for _, r := range filter.Apply(doc) {
				out.Rounds = append(out.Rounds, replayRoundOut{
					Ref: fmt.Sprintf("%s/%d", in.MatchID, r.Number), Round: r.Number, Winner: string(r.Winner),
					CTBuy: string(r.Economy.CTBuy), TBuy: string(r.Economy.TBuy), Site: string(r.Class.Site),
					AttackShape: string(r.Class.TSide), DefenseShape: string(r.Class.CTSide),
				})
			}
			out.Kept = len(out.Rounds)
			values.Set("tab", "replay")
			if form != "" && form != replayForms[0] {
				values.Set("forma", form)
			}
			out.StudioPath = "/partidas/" + url.PathEscape(in.MatchID) + "?" + values.Encode()
			return nil, out, nil
		})
}
