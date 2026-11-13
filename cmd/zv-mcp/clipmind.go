package main

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// clipMindStatusOut is Ajustes > Cuenta ClipMind: whether Studio is linked to
// an account and what is waiting to sync.
type clipMindStatusOut struct {
	Connected     bool   `json:"connected" jsonschema:"whether Studio is signed in to a ClipMind account"`
	ServerURL     string `json:"server_url,omitempty"`
	Login         string `json:"login,omitempty" jsonschema:"the username or email Studio signed in with"`
	WorkspaceName string `json:"workspace_name,omitempty" jsonschema:"untrusted text"`
	LastSyncAt    int64  `json:"last_sync_at,omitempty" jsonschema:"Unix milliseconds of the last good sync"`
	LastError     string `json:"last_error,omitempty" jsonschema:"why the last sync failed; the user signs in again when the session ended"`
	Pending       int    `json:"pending_changes" jsonschema:"local changes not sent yet"`
	Syncing       bool   `json:"syncing"`
}

func clipMindStatusFrom(s tuiclient.ClipMindStatus) clipMindStatusOut {
	return clipMindStatusOut{
		Connected: s.Connected, ServerURL: s.ServerURL, Login: s.Login, WorkspaceName: s.WorkspaceName,
		LastSyncAt: s.LastSyncAt, LastError: s.LastError, Pending: s.Pending, Syncing: s.Syncing,
	}
}

// registerClipMindTools adds Studio's account link. Signing in stays in the UI:
// a password must not pass through an assistant.
func registerClipMindTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "clipmind_status", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Ajustes > Cuenta ClipMind: whether Studio is signed in to a ClipMind account (the web edition of ClipHub), " +
			"its workspace, the last sync and how many local changes wait to be sent. Signing in is done by the user in Studio."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, clipMindStatusOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, clipMindStatusOut{}, toolError(err)
			}
			st, err := c.ClipMindAccount(ctx)
			if err != nil {
				return nil, clipMindStatusOut{}, toolError(err)
			}
			return nil, clipMindStatusFrom(st), nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "clipmind_sync", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Sincronizar ahora: sends this library's changes and match files to the ClipMind account and brings the team's changes down. " +
			"It fails when Studio is not signed in; ask the user to sign in under Ajustes > Cuenta ClipMind."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, clipMindStatusOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, clipMindStatusOut{}, toolError(err)
			}
			st, err := c.ClipMindSync(ctx)
			if err != nil {
				return nil, clipMindStatusOut{}, toolError(err)
			}
			return nil, clipMindStatusFrom(st), nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "clipmind_disconnect", Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false)},
		Description: "Desconectar: signs Studio out of the ClipMind account and stops syncing. The library on this PC stays as it is."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, clipMindStatusOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, clipMindStatusOut{}, toolError(err)
			}
			st, err := c.ClipMindDisconnect(ctx)
			if err != nil {
				return nil, clipMindStatusOut{}, toolError(err)
			}
			return nil, clipMindStatusFrom(st), nil
		})
}
