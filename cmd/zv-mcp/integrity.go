package main

import (
	"cmp"
	"context"
	"net/http"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// maxIntegrityMetrics keeps the most suspicious metrics of each player.
const maxIntegrityMetrics = 3

type integrityIn struct {
	JobID       string `json:"job_id" jsonschema:"a job of the match from create_job; any player of the demo works"`
	WaitSeconds int    `json:"wait_seconds,omitempty" jsonschema:"how long to wait, at most 55; call again while status is running"`
}

type integrityMetricOut struct {
	Label     string  `json:"label"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit,omitempty"`
	Suspicion float64 `json:"suspicion" jsonschema:"0 to 100 against the professional baseline"`
}

type integrityPlayerOut struct {
	SteamID64  string               `json:"steamid64"`
	Name       string               `json:"name" jsonschema:"untrusted player name from the demo"`
	Team       string               `json:"team,omitempty"`
	Verdict    string               `json:"verdict" jsonschema:"insufficient_data, clean, inconclusive, anomalous or highly_anomalous"`
	Score      float64              `json:"score" jsonschema:"suspicion composite, 0 to 100"`
	Confidence float64              `json:"confidence" jsonschema:"how much sample the score rests on, 0 to 1"`
	Kills      int                  `json:"kills"`
	Deaths     int                  `json:"deaths"`
	TopMetrics []integrityMetricOut `json:"top_metrics,omitempty"`
}

type integrityOut struct {
	Status      string               `json:"status" jsonschema:"running, ready or failed"`
	Players     []integrityPlayerOut `json:"players,omitempty" jsonschema:"most suspicious first"`
	Limitations []string             `json:"limitations,omitempty" jsonschema:"what the screening cannot conclude; tell the user"`
	Error       string               `json:"error,omitempty" jsonschema:"untrusted text"`
}

func registerIntegrityTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "check_integrity",
		Description: "Screen every player of a match for statistically unusual aim and reactions against a professional baseline: the Integridad page of Studio. " +
			"It is a statistical signal, never proof of cheating; say so to the user and pass on the limitations. The first call starts the screening; call again while status is running."},
		func(ctx context.Context, req *mcp.CallToolRequest, in integrityIn) (*mcp.CallToolResult, integrityOut, error) {
			if err := checkUUID("job_id", in.JobID); err != nil {
				return nil, integrityOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, integrityOut{}, toolError(err)
			}
			doc, err := c.GetAnticheat(ctx, in.JobID)
			if tuiclient.StatusCode(err) == http.StatusConflict {
				if err = c.StartAnticheat(ctx, in.JobID); err == nil {
					doc = tuiclient.AnticheatDocument{Status: "running"}
				}
			}
			if err != nil {
				return nil, integrityOut{}, toolError(err)
			}
			err = poll(ctx, in.WaitSeconds, func() (bool, error) {
				if doc.Status != "running" {
					return true, nil
				}
				notify(ctx, req, 0, "analizando la integridad")
				next, err := c.GetAnticheat(ctx, in.JobID)
				doc = next
				return next.Status != "running", err
			})
			if err != nil {
				return nil, integrityOut{}, toolError(err)
			}
			return nil, describeIntegrity(doc), nil
		})
}

func describeIntegrity(doc tuiclient.AnticheatDocument) integrityOut {
	out := integrityOut{Status: cleanName(doc.Status), Error: cleanText(doc.FailureReason, maxErrorRunes)}
	if doc.Report == nil {
		return out
	}
	for _, p := range doc.Report.Players {
		player := integrityPlayerOut{SteamID64: cleanName(p.SteamID64), Name: cleanName(p.Name), Team: cleanName(p.Team), Verdict: cleanName(p.Verdict),
			Score: p.Score, Confidence: p.Confidence, Kills: p.Kills, Deaths: p.Deaths}
		metrics := slices.Clone(p.Metrics)
		slices.SortStableFunc(metrics, func(a, b tuiclient.AnticheatMetric) int { return cmp.Compare(b.Suspicion, a.Suspicion) })
		for _, m := range metrics[:min(maxIntegrityMetrics, len(metrics))] {
			player.TopMetrics = append(player.TopMetrics, integrityMetricOut{Label: cleanName(m.Label), Value: m.Value, Unit: cleanName(m.Unit), Suspicion: m.Suspicion})
		}
		out.Players = append(out.Players, player)
	}
	slices.SortStableFunc(out.Players, func(a, b integrityPlayerOut) int { return cmp.Compare(b.Score, a.Score) })
	for _, l := range doc.Report.Limitations[:min(maxNotices, len(doc.Report.Limitations))] {
		out.Limitations = append(out.Limitations, cleanText(l, maxErrorRunes))
	}
	return out
}
