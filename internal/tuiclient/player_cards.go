package tuiclient

import (
	"context"
	"net/http"
)

// PlayerCard is GET /api/jobs/{id}/player-card: the card catalog and the player a one-player Short
// of the job shows, or why its Shorts cannot carry a card.
type PlayerCard struct {
	Templates []PlayerCardTemplate `json:"templates"`
	Available bool                 `json:"available"`
	Reason    string               `json:"reason,omitempty"`
	Player    *PlayerCardPlayer    `json:"player,omitempty"`
}

// PlayerCardTemplate is one card design of the catalog.
type PlayerCardTemplate struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Needs       []string `json:"needs"`
}

// PlayerCardPlayer is the player on the card; Image is "upload", "faceit" or empty for the initials.
type PlayerCardPlayer struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name"`
	Image     string `json:"image"`
}

// PlayerCard reads the card catalog and the player of one job.
func (c *Client) PlayerCard(ctx context.Context, jobID string) (PlayerCard, error) {
	var out PlayerCard
	err := c.getJSON(ctx, jobPath(jobID)+"/player-card", &out)
	return out, err
}

// SetPlayerCardImage uploads a PNG or JPEG that every card of the job's player shows.
func (c *Client) SetPlayerCardImage(ctx context.Context, jobID, filePath string) error {
	return c.uploadMultipart(ctx, upload{Path: jobPath(jobID) + "/player-card/image", FileField: "image", FilePath: filePath}, nil)
}

// RemovePlayerCardImage puts the job's player back on the FACEIT avatar or the initials.
func (c *Client) RemovePlayerCardImage(ctx context.Context, jobID string) error {
	return c.doJSON(ctx, http.MethodDelete, jobPath(jobID)+"/player-card/image", nil, nil)
}
