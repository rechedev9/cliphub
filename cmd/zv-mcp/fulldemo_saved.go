package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

type getFullDemoPlanIn struct {
	JobID  string `json:"job_id"`
	PlanID string `json:"plan_id,omitempty" jsonschema:"plan_id of a Full Demo version from list_video_versions; empty reads the current plan"`
}

type fullDemoRoundOut struct {
	Round   int `json:"round" jsonschema:"source round number, for plan_full_demo rounds"`
	Seconds int `json:"seconds"`
	Kills   int `json:"kills" jsonschema:"kills of the POV player in the round"`
}

type fullDemoRoundsOut struct {
	JobID        string             `json:"job_id"`
	Rounds       []fullDemoRoundOut `json:"rounds"`
	TotalSeconds int                `json:"total_seconds" jsonschema:"length of every round together, before intro, clips and outro"`
}

// describeFullDemoRounds is the round list and length the Full Demo editor shows from the recap plan.
func describeFullDemoRounds(jobID string, plan tuiclient.Plan) fullDemoRoundsOut {
	tickrate := plan.Demo.Tickrate
	if tickrate <= 0 {
		tickrate = 64
	}
	out := fullDemoRoundsOut{JobID: jobID, Rounds: make([]fullDemoRoundOut, 0, len(plan.Segments))}
	ticks := 0
	for _, s := range plan.Segments {
		span := max(0, s.TickEnd-s.TickStart)
		ticks += span
		out.Rounds = append(out.Rounds, fullDemoRoundOut{Round: s.Round, Seconds: int(math.Round(float64(span) / float64(tickrate))), Kills: len(s.Kills)})
	}
	out.TotalSeconds = int(math.Round(float64(ticks) / float64(tickrate)))
	return out
}

// checkFullDemoCapture answers plan_full_demo check_only from POST /full-demo/capture-check.
func checkFullDemoCapture(ctx context.Context, c *tuiclient.Client, jobID string, options json.RawMessage) (*mcp.CallToolResult, fullDemoPlanOut, error) {
	raw, err := c.CheckFullDemoCapture(ctx, jobID, options)
	if err != nil {
		return nil, fullDemoPlanOut{}, toolError(err)
	}
	var capture fullDemoCapture
	if err := json.Unmarshal(raw, &capture); err != nil {
		return nil, fullDemoPlanOut{}, fmt.Errorf("Studio returned an unreadable capture check: %w", err)
	}
	return nil, describeCaptureCheck(jobID, capture), nil
}

func describeCaptureCheck(jobID string, capture fullDemoCapture) fullDemoPlanOut {
	out := fullDemoPlanOut{JobID: jobID, NeedsCapture: capture.Needed, MissingRounds: capture.MissingRounds,
		Summary: "Comprobación sin guardar el plan: las capturas de todas las rondas están guardadas, así que solo se vuelve a montar."}
	if capture.Needed {
		out.Summary = fmt.Sprintf("Comprobación sin guardar el plan: con estas opciones CS2 graba %s.", plural(len(capture.MissingRounds), "ronda", "rondas"))
	}
	if s := capture.Space; s != nil && s.Short {
		out.LowSpace = fmt.Sprintf("Puede faltar espacio: hacen falta unos %s y quedan %s libres. Almacenamiento libera espacio.", formatBytes(s.NeededBytes), formatBytes(s.FreeBytes))
		out.Summary += " " + out.LowSpace
	}
	return out
}

func registerSavedFullDemoTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "full_demo_rounds", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "List the rounds a Full Demo of a parsed job records, with each round's seconds and the POV's kills, and the length of them all, as the Full Demo editor shows them. " +
			"Use it to pick plan_full_demo rounds and tell the user how long the video runs; it changes nothing."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idIn) (*mcp.CallToolResult, fullDemoRoundsOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, fullDemoRoundsOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, fullDemoRoundsOut{}, toolError(err)
			}
			plan, err := c.GetRecapPlan(ctx, in.ID)
			if tuiclient.StatusCode(err) == http.StatusConflict {
				return nil, fullDemoRoundsOut{}, errors.New("the Full Demo rounds are not ready: wait_for_job until the job is parsed")
			}
			if err != nil {
				return nil, fullDemoRoundsOut{}, toolError(err)
			}
			return nil, describeFullDemoRounds(in.ID, plan), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_full_demo_plan", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read a saved Full Demo plan without changing it: the current one, or with plan_id the plan an older version rendered, as opening that version in the Full Demo editor does. " +
			"It returns the same summary as plan_full_demo; render_full_demo takes the current plan's plan_hash."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in getFullDemoPlanIn) (*mcp.CallToolResult, fullDemoPlanOut, error) {
			if err := checkUUID("job_id", in.JobID); err != nil {
				return nil, fullDemoPlanOut{}, err
			}
			if in.PlanID != "" {
				if err := checkUUID("plan_id", in.PlanID); err != nil {
					return nil, fullDemoPlanOut{}, err
				}
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, fullDemoPlanOut{}, toolError(err)
			}
			var raw json.RawMessage
			if in.PlanID == "" {
				raw, err = c.GetFullDemoPlan(ctx, in.JobID)
			} else {
				raw, err = c.GetFullDemoPlanVersion(ctx, in.JobID, in.PlanID)
			}
			if err != nil {
				return nil, fullDemoPlanOut{}, toolError(err)
			}
			state, err := decodeFullDemoState(raw)
			if err != nil {
				return nil, fullDemoPlanOut{}, err
			}
			if state.Document == nil {
				return nil, fullDemoPlanOut{}, fmt.Errorf("this job has no saved Full Demo plan: plan_full_demo makes one")
			}
			out := describeFullDemoPlan(in.JobID, *state.Document)
			describeFullDemoCapture(&out, state.Capture)
			return nil, out, nil
		})
}
