package main

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/moments"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

type autoPOVIn struct {
	JobID         string  `json:"job_id" jsonschema:"a multi-POV job from create_multi_pov_job, once wait_for_job says parsed"`
	TargetSeconds float64 `json:"target_seconds,omitempty" jsonschema:"length budget in seconds, 15 to 600; default 180, the 16:9 best-moments video; 60 for a 9:16 Short"`
	ExtraSeconds  float64 `json:"extra_seconds,omitempty" jsonschema:"seconds each play adds in the edit, 0 to 15, such as a victim replay (7 at the default speed)"`
}

type autoPOVPlayOut struct {
	momentOut
	Interest float64       `json:"interest" jsonschema:"0 to 1: the score lifted by the observer cues; the director takes the highest first"`
	Cues     []moments.Cue `json:"cues,omitempty" jsonschema:"why an observer would cut to it. kind clutch (count enemies, detail won or lost), duel (detail opening for the round's first duel, chained for a fast multi-kill), bomb (detail post_plant or retake, value how even the post-plant win chance was, 0 to 1) or momentum (detail swing with value the round-win chance gained, comeback with count rounds lost before, run with count rounds won in a row)"`
}

type autoPOVPlayerOut struct {
	Name    string  `json:"name" jsonschema:"untrusted player name from the demo"`
	SteamID string  `json:"steamid"`
	Plays   int     `json:"plays" jsonschema:"plays of this player in the summary; 0 leaves the player out"`
	Seconds float64 `json:"seconds"`
}

type autoPOVOut struct {
	SegmentIDs    []string           `json:"segment_ids" jsonschema:"pass these to generate_short as they are: match order, never two players at the same time"`
	TotalSeconds  float64            `json:"total_seconds"`
	TargetSeconds float64            `json:"target_seconds"`
	Plays         []autoPOVPlayOut   `json:"plays"`
	Players       []autoPOVPlayerOut `json:"players" jsonschema:"every filmed player, the anchor first"`
	RoundResults  bool               `json:"round_results" jsonschema:"false on a job parsed before round results were kept: comebacks and runs cannot count until the demo is analysed again"`
}

func registerAutoPOVTool(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "auto_pov_summary",
		Description: "Auto-POV, Elegir automáticamente in Studio's summary: picks which teammates and which plays a multi-POV summary shows, like a broadcast observer, from the parsed demo. " +
			"Each kill play is ranked by its list_moments score plus observer cues (a clutch, the round's opening duel or chained duels, the bomb planted, momentum), " +
			"then the best are taken across players within target_seconds; each play a player already has lowers their next one, and no two players are ever less than half a second apart. " +
			"Read only. Pass segment_ids to generate_short. To let it choose the players too, create the job with the whole team (up to 5) in create_multi_pov_job."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in autoPOVIn) (*mcp.CallToolResult, autoPOVOut, error) {
			if err := checkUUID("job_id", in.JobID); err != nil {
				return nil, autoPOVOut{}, err
			}
			if in.TargetSeconds != 0 && (in.TargetSeconds < moments.MinDirectorSeconds || in.TargetSeconds > moments.MaxDirectorSeconds) {
				return nil, autoPOVOut{}, fmt.Errorf("target_seconds must be between %g and %g", moments.MinDirectorSeconds, moments.MaxDirectorSeconds)
			}
			if in.ExtraSeconds < 0 || in.ExtraSeconds > moments.MaxDirectorExtraSeconds {
				return nil, autoPOVOut{}, fmt.Errorf("extra_seconds must be between 0 and %g", moments.MaxDirectorExtraSeconds)
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, autoPOVOut{}, toolError(err)
			}
			d, err := c.GetAutoPOV(ctx, tuiclient.AutoPOVQuery{JobID: in.JobID, TargetSeconds: in.TargetSeconds, ExtraSeconds: in.ExtraSeconds})
			if err != nil {
				return nil, autoPOVOut{}, toolError(err)
			}
			return nil, describeDirection(d), nil
		})
}

func describeDirection(d moments.Direction) autoPOVOut {
	out := autoPOVOut{SegmentIDs: cleanNames(d.SegmentIDs), TotalSeconds: d.TotalSeconds, TargetSeconds: d.TargetSeconds,
		Plays: make([]autoPOVPlayOut, 0, len(d.Plays)), Players: make([]autoPOVPlayerOut, 0, len(d.Players)), RoundResults: d.RoundResults}
	for _, p := range d.Plays {
		m := tuiclient.Moment{SegmentID: p.SegmentID, Player: p.Player, SteamID64: p.SteamID64, Round: p.Round, Side: p.Side,
			ClutchVs: p.ClutchVs, ClutchWon: p.ClutchWon, WinProbSwing: p.WinProbSwing, DurationSeconds: p.DurationSeconds, Score: p.Score,
			ReasonCodes: p.ReasonCodes, Events: tuiclient.MomentEvents{Kills: p.Events.Kills, Headshots: p.Events.Headshots}, Weapons: p.Weapons, Victims: p.Victims}
		out.Plays = append(out.Plays, autoPOVPlayOut{momentOut: describeMoment(m), Interest: p.Interest, Cues: p.Cues})
	}
	for _, p := range d.Players {
		out.Players = append(out.Players, autoPOVPlayerOut{Name: cleanName(p.Name), SteamID: cleanName(p.SteamID64), Plays: p.Plays, Seconds: p.Seconds})
	}
	return out
}
