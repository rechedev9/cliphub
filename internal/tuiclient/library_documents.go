package tuiclient

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"
)

// TeamDocument is an HTML document attached to a team, such as an anti-strat report.
type TeamDocument struct {
	ID          string    `json:"id"`
	TeamID      string    `json:"team_id"`
	TeamName    string    `json:"team_name"`
	Title       string    `json:"title"`
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TeamDocumentUpload is one document to attach; Title may be empty to use the file name.
type TeamDocumentUpload struct {
	TeamID   string
	Title    string
	FileName string
	Content  []byte
}

func teamDocumentPath(id string) string { return "/api/library/documents/" + url.PathEscape(id) }

// ListTeamDocuments lists the documents, newest first, optionally of one team.
func (c *Client) ListTeamDocuments(ctx context.Context, teamID string) ([]TeamDocument, error) {
	var out struct {
		Documents []TeamDocument `json:"documents"`
	}
	err := c.getJSON(ctx, "/api/library/documents"+query(map[string]string{"team_id": teamID, "limit": "200"}), &out)
	return out.Documents, err
}

// GetTeamDocument returns one document's details, not its content.
func (c *Client) GetTeamDocument(ctx context.Context, id string) (TeamDocument, error) {
	var out TeamDocument
	err := c.getJSON(ctx, teamDocumentPath(id), &out)
	return out, err
}

// UploadTeamDocument attaches a document to a team. It is at most a few MiB, so
// the form is built in memory.
func (c *Client) UploadTeamDocument(ctx context.Context, in TeamDocumentUpload) (TeamDocument, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("team_id", in.TeamID); err != nil {
		return TeamDocument{}, err
	}
	if in.Title != "" {
		if err := mw.WriteField("title", in.Title); err != nil {
			return TeamDocument{}, err
		}
	}
	part, err := mw.CreateFormFile("file", in.FileName)
	if err != nil {
		return TeamDocument{}, err
	}
	if _, err := part.Write(in.Content); err != nil {
		return TeamDocument{}, err
	}
	if err := mw.Close(); err != nil {
		return TeamDocument{}, err
	}
	const path = "/api/library/document-uploads"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, &body)
	if err != nil {
		return TeamDocument{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	var out TeamDocument
	err = c.send(req, http.MethodPost, path, &out)
	return out, err
}

// DeleteTeamDocument removes a document.
func (c *Client) DeleteTeamDocument(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, teamDocumentPath(id), nil, nil)
}
