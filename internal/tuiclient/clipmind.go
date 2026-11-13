package tuiclient

import (
	"context"
	"net/http"
)

// ClipMindStatus is the link between Studio and a ClipMind account.
type ClipMindStatus struct {
	Connected     bool   `json:"connected"`
	ServerURL     string `json:"server_url,omitempty"`
	Email         string `json:"email,omitempty"`
	WorkspaceID   string `json:"workspace_id,omitempty"`
	WorkspaceName string `json:"workspace_name,omitempty"`
	LastSyncAt    int64  `json:"last_sync_at,omitempty"`
	LastError     string `json:"last_error,omitempty"`
	Pending       int    `json:"pending_changes"`
	Syncing       bool   `json:"syncing"`
}

// ClipMindAccount reads the link's status.
func (c *Client) ClipMindAccount(ctx context.Context) (ClipMindStatus, error) {
	var out ClipMindStatus
	err := c.getJSON(ctx, "/api/clipmind/account", &out)
	return out, err
}

// ClipMindSync runs one sync and returns the status after it.
func (c *Client) ClipMindSync(ctx context.Context) (ClipMindStatus, error) {
	var out ClipMindStatus
	err := c.doJSON(ctx, http.MethodPost, "/api/clipmind/sync", nil, &out)
	return out, err
}

// ClipMindDisconnect signs Studio out of the account; local data stays.
func (c *Client) ClipMindDisconnect(ctx context.Context) (ClipMindStatus, error) {
	var out ClipMindStatus
	err := c.doJSON(ctx, http.MethodDelete, "/api/clipmind/account", nil, &out)
	return out, err
}

// WhoAmI is a ClipMind server's answer to "who is this key": the account and
// the workspace it reaches.
type WhoAmI struct {
	User struct {
		ID          string `json:"id"`
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
	} `json:"user"`
	Workspace *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Role string `json:"role"`
	} `json:"workspace"`
}

// WhoAmI asks a ClipMind server which account and workspace the credential reaches.
func (c *Client) WhoAmI(ctx context.Context) (WhoAmI, error) {
	var out WhoAmI
	err := c.getJSON(ctx, "/auth/me", &out)
	return out, err
}
