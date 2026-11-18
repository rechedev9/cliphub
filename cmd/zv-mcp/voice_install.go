package main

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

type installTranscriberIn struct {
	Approve string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type transcriberOut struct {
	pendingOut
	State         string `json:"state" jsonschema:"not_installed, installing, installed, failed or unsupported"`
	Version       string `json:"version"`
	Model         string `json:"model"`
	DownloadBytes int64  `json:"download_bytes"`
	DoneBytes     int64  `json:"done_bytes,omitempty"`
	ErrorCode     string `json:"error_code,omitempty" jsonschema:"runtime_missing (install the Visual C++ runtime), start_failed or download_failed"`
	Error         string `json:"error,omitempty"`
}

func transcriberOutOf(t tuiclient.VoiceTranscriber) transcriberOut {
	return transcriberOut{State: t.State, Version: t.Version, Model: t.Model, DownloadBytes: t.DownloadBytes, DoneBytes: t.DoneBytes, ErrorCode: t.ErrorCode, Error: cleanText(t.Error, maxErrorRunes)}
}

// installTool is a local speech download Studio fetches once, after the user approves its size.
// What words it for the user with the version and the size in MB.
type installTool struct {
	Name, Label, What, About string
	Status, Install          func(*tuiclient.Client, context.Context) (tuiclient.VoiceTranscriber, error)
}

func registerInstallTool(server *mcp.Server, s *studio, t installTool) {
	mcp.AddTool(server, &mcp.Tool{Name: t.Name, Annotations: &mcp.ToolAnnotations{},
		Description: "Install " + t.About + " as " + t.Label + " does. The download is large, so the user must approve it. " + deleteApprovalHint +
			" It runs in the background: read this tool's state until installed. Nothing leaves the PC when it runs."},
		func(ctx context.Context, req *mcp.CallToolRequest, in installTranscriberIn) (*mcp.CallToolResult, transcriberOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, transcriberOut{}, toolError(err)
			}
			now, err := t.Status(c, ctx)
			if err != nil {
				return nil, transcriberOut{}, toolError(err)
			}
			if now.State == "installed" || now.State == "installing" || now.State == "unsupported" {
				return nil, transcriberOutOf(now), nil
			}
			summary := "Descargar " + fmt.Sprintf(t.What, now.Version, max(1, now.DownloadBytes>>20)) + " en la carpeta de Studio."
			ask, pending, err := gate(req, approvalSpec{Tool: t.Name, JobID: now.Version, Request: now.Model, Summary: summary, Given: in.Approve, Question: "¿Descargar ahora?"})
			if err != nil || ask != nil {
				return ask, transcriberOut{}, err
			}
			if pending != nil {
				out := transcriberOutOf(now)
				out.pendingOut = *pending
				return nil, out, nil
			}
			installed, err := t.Install(c, ctx)
			if err != nil {
				return nil, transcriberOut{}, toolError(err)
			}
			return nil, transcriberOutOf(installed), nil
		})
}
