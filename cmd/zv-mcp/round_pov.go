package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

type recordRoundPOVIn struct {
	MatchID     string `json:"match_id" jsonschema:"library match id from get_match or find_rounds"`
	Round       int    `json:"round" jsonschema:"round number, as in the Replay"`
	SteamID     string `json:"steamid" jsonschema:"SteamID64 of the player whose point of view is recorded"`
	WaitSeconds int    `json:"wait_seconds,omitempty" jsonschema:"how long to wait for the player to be parsed, at most 55; call again with the same arguments if the result is still running"`
	Approve     string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type recordRoundPOVOut struct {
	generateOut
	Running bool   `json:"running,omitempty" jsonschema:"true while the player is still being parsed; call again with the same arguments"`
	Note    string `json:"note,omitempty"`
}

// roundPOVPlanIn is the plain plan of one round: Studio's defaults, that round
// only and no effects between rounds (there is no other round to cut to).
func roundPOVPlanIn(jobID string, round int) planFullDemoIn {
	off := false
	return planFullDemoIn{JobID: jobID, Rounds: []int{round}, BetweenRoundEffects: &off}
}

func registerRoundPOVTool(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "record_round_pov", Meta: appToolMeta(jobProgressAppURI),
		Description: "Record one player's point of view for one round of a library match, as the Replay's \"Grabar su POV de esta ronda\" does: " +
			"a Full Demo limited to that round, from the end of freeze time to the end of the round, with Studio's default look. " +
			"It creates or reuses the player's job, waits for it to be parsed, plans the round and starts the capture. This opens CS2 full screen, so the user must approve it: " +
			"if the result says needs_approval, show the summary, ask the user, and call again with approve set to approval_code only if they agree. " +
			"Follow with wait_for_job, passing the returned variant; the video lands in Mis vídeos (list_all_videos). " +
			"For a different look or several rounds use plan_full_demo with rounds and render_full_demo."},
		func(ctx context.Context, req *mcp.CallToolRequest, in recordRoundPOVIn) (*mcp.CallToolResult, recordRoundPOVOut, error) {
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, recordRoundPOVOut{}, err
			}
			if err := checkSteamID(in.SteamID); err != nil {
				return nil, recordRoundPOVOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, recordRoundPOVOut{}, toolError(err)
			}
			m, err := c.GetLibraryMatch(ctx, in.MatchID)
			if err != nil {
				return nil, recordRoundPOVOut{}, toolError(err)
			}
			if err := checkRoundPOV(m, in); err != nil {
				return nil, recordRoundPOVOut{}, err
			}
			job, err := c.CreatePlayerJob(ctx, m.DemoID, in.SteamID)
			if err != nil {
				return nil, recordRoundPOVOut{}, toolError(err)
			}
			status, err := waitParsed(ctx, c, job.JobID, in.WaitSeconds)
			if err != nil {
				return nil, recordRoundPOVOut{}, err
			}
			if status != "parsed" {
				return nil, recordRoundPOVOut{generateOut: generateOut{JobID: job.JobID, Status: cleanName(status)}, Running: true,
					Note: "The player is still being parsed. Call record_round_pov again with the same arguments."}, nil
			}
			doc, err := planRoundPOV(ctx, c, roundPOVPlanIn(job.JobID, in.Round))
			if err != nil {
				return nil, recordRoundPOVOut{}, err
			}
			res, out, err := startFullDemo(ctx, req, c, fullDemoStart{Tool: "record_round_pov", JobID: job.JobID, Doc: doc, Approve: in.Approve})
			return res, recordRoundPOVOut{generateOut: out}, err
		})
}

// checkRoundPOV refuses a round the match does not have and a player who did not play it.
func checkRoundPOV(m tuiclient.LibraryMatch, in recordRoundPOVIn) error {
	if in.Round < 1 || in.Round > m.RoundCount {
		return fmt.Errorf("round must be between 1 and %d, the rounds of this match, got %d", m.RoundCount, in.Round)
	}
	if _, ok := seriesPlayer(m, in.SteamID); !ok {
		return fmt.Errorf("SteamID %s did not play this match; read the players with get_match", in.SteamID)
	}
	return nil
}

// waitParsed returns "parsed" once the job is, or the last status read when the wait ends first.
func waitParsed(ctx context.Context, c *tuiclient.Client, jobID string, waitSeconds int) (string, error) {
	status := ""
	err := poll(ctx, waitSeconds, func() (bool, error) {
		st, err := c.GetJobStatus(ctx, jobID)
		if err != nil {
			return false, toolError(err)
		}
		status = st.Status
		if st.Status == "failed" {
			return false, fmt.Errorf("the player could not be parsed: %s", cleanText(st.FailureReason, maxErrorRunes))
		}
		return st.Status == "parsed", nil
	})
	if err == nil && status != "parsed" && !workerOwnsJob(status) {
		return status, fmt.Errorf("the job is %s, not parsed; create_job with reparse parses it again", cleanName(status))
	}
	return status, err
}

// planRoundPOV saves the plan of a parsed job and returns it as Studio stored it.
func planRoundPOV(ctx context.Context, c *tuiclient.Client, in planFullDemoIn) (recapplan.Document, error) {
	state, err := loadFullDemoState(ctx, c, in.JobID)
	if err != nil {
		return recapplan.Document{}, toolError(err)
	}
	options, err := applyFullDemoChoices(state.Defaults, in)
	if err != nil {
		return recapplan.Document{}, err
	}
	body, err := json.Marshal(options)
	if err != nil {
		return recapplan.Document{}, err
	}
	raw, err := c.PlanFullDemo(ctx, in.JobID, body)
	if err != nil {
		return recapplan.Document{}, toolError(err)
	}
	doc, err := decodeFullDemoDocument(raw)
	if err != nil {
		return recapplan.Document{}, err
	}
	if err := doc.Validate(); err != nil {
		return recapplan.Document{}, fmt.Errorf("Studio returned a plan that does not validate: %w", err)
	}
	return doc, nil
}
