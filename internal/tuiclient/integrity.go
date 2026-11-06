package tuiclient

import (
	"context"
	"net/http"
)

// AnticheatDocument mirrors GET /api/jobs/{id}/anticheat (internal/anticheat.Document).
type AnticheatDocument struct {
	Status        string           `json:"status"`
	FailureReason string           `json:"failure_reason,omitempty"`
	Report        *AnticheatReport `json:"report,omitempty"`
}

type AnticheatReport struct {
	Players     []AnticheatPlayer `json:"players"`
	Limitations []string          `json:"limitations"`
}

type AnticheatPlayer struct {
	SteamID64  string            `json:"steamid64"`
	Name       string            `json:"name"`
	Team       string            `json:"team"`
	Rounds     int               `json:"rounds"`
	Kills      int               `json:"kills"`
	Deaths     int               `json:"deaths"`
	Score      float64           `json:"score"`
	Confidence float64           `json:"confidence"`
	Verdict    string            `json:"verdict"`
	Metrics    []AnticheatMetric `json:"metrics"`
}

type AnticheatMetric struct {
	Label     string  `json:"label"`
	Unit      string  `json:"unit"`
	Value     float64 `json:"value"`
	Suspicion float64 `json:"suspicion"`
}

// GetAnticheat reads the integrity screening of a job's demo; 409 means never started.
func (c *Client) GetAnticheat(ctx context.Context, jobID string) (AnticheatDocument, error) {
	var out AnticheatDocument
	err := c.getJSON(ctx, jobPath(jobID)+"/anticheat", &out)
	return out, err
}

// StartAnticheat queues the integrity screening; a running one is left alone.
func (c *Client) StartAnticheat(ctx context.Context, jobID string) error {
	return c.doJSON(ctx, http.MethodPost, jobPath(jobID)+"/anticheat", nil, nil)
}
