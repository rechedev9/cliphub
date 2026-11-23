package main

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

type transcriberOut struct {
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

func registerInstallTranscriber(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "install_transcriber", Annotations: &mcp.ToolAnnotations{},
		Description: "Read Studio's local transcription (NVIDIA Parakeet TDT 0.6b v3, CC BY 4.0, on NeMo-Speech.cpp's Vulkan build for the GPU and its CPU build when Vulkan fails, about 745 MB in all; it detects the language) and retry it, as Reintentar in the Subtítulos panel does. " +
			"Studio downloads it on its own, without asking, the first time something needs it: subtitles on in a video, transcribe_voice or transcribe_demo_voice; a transcription asked meanwhile waits for the download. " +
			"When the state is not_installed or failed, this starts the install again: after download_failed once the connection is back, after runtime_missing or start_failed once the user installed the Visual C++ runtime. " +
			"It runs in the background, and download_bytes is what it still fetches: call it again until installed. Nothing leaves the PC when it runs."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, transcriberOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, transcriberOut{}, toolError(err)
			}
			now, err := c.VoiceTranscriberStatus(ctx)
			if err != nil {
				return nil, transcriberOut{}, toolError(err)
			}
			if now.State == "installed" || now.State == "installing" || now.State == "unsupported" {
				return nil, transcriberOutOf(now), nil
			}
			retried, err := c.InstallVoiceTranscriber(ctx)
			if err != nil {
				return nil, transcriberOut{}, toolError(err)
			}
			return nil, transcriberOutOf(retried), nil
		})
}
