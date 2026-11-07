package main

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/job"
)

type createMultiPOVIn struct {
	DemoID   string   `json:"demo_id"`
	SteamIDs []string `json:"steamids" jsonschema:"SteamID64s of 2 to 5 teammates from get_match; the first is the anchor whose team comms the video keeps"`
}

func registerMultiPOVTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "create_multi_pov_job",
		Description: "Create one video job from a library demo for several teammates at once (2 to 5 players of one team), for a match summary with the best plays of each. " +
			"Follow with wait_for_job until \"parsed\", then list_moments: every play names its player. generate_short then mixes their plays in one Short; " +
			"format landscape-16x9 with preset gameplay-pov-60 makes the horizontal best-moments video. Full Demo needs a single-player job."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in createMultiPOVIn) (*mcp.CallToolResult, jobOut, error) {
			if err := checkUUID("demo_id", in.DemoID); err != nil {
				return nil, jobOut{}, err
			}
			if len(in.SteamIDs) < job.MinMultiPOVPlayers || len(in.SteamIDs) > job.MaxMultiPOVPlayers {
				return nil, jobOut{}, fmt.Errorf("steamids takes %d to %d teammates; use create_job for one player", job.MinMultiPOVPlayers, job.MaxMultiPOVPlayers)
			}
			for _, id := range in.SteamIDs {
				if err := checkSteamID(id); err != nil {
					return nil, jobOut{}, err
				}
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, jobOut{}, toolError(err)
			}
			res, err := c.CreateMultiPOVJob(ctx, in.DemoID, in.SteamIDs)
			if err != nil {
				return nil, jobOut{}, toolError(err)
			}
			return nil, jobOut{JobID: res.JobID, Status: cleanName(res.Status), Running: workerOwnsJob(res.Status)}, nil
		})
}
