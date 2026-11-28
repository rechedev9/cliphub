package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/renderengine"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const gpuRenderHTTPTimeout = 10 * time.Minute

type renderGPUInfoOut struct {
	Available             bool                  `json:"available"`
	BinaryPath            string                `json:"binary_path,omitempty"`
	GPU                   *renderengine.GPUInfo `json:"gpu,omitempty"`
	EditorHardwareEncoder string                `json:"editor_hardware_encoder,omitempty"`
	Error                 string                `json:"error,omitempty"`
}

type validateRenderSpecIn struct {
	SpecPath string `json:"spec_path" jsonschema:"absolute local path to a cliphub-render JSON scene or essay spec"`
}

type gpuRenderSnapshotIn struct {
	SpecPath   string    `json:"spec_path" jsonschema:"absolute local path to a cliphub-render JSON scene or essay spec"`
	At         []float64 `json:"at" jsonschema:"timestamps in seconds to render as PNG snapshots"`
	OutDir     string    `json:"out_dir" jsonschema:"absolute local directory path where PNG snapshots will be written"`
	Scene      string    `json:"scene,omitempty" jsonschema:"scene id when snapshotting a multi-scene essay"`
	RequireGPU bool      `json:"require_gpu,omitempty" jsonschema:"fail unless a discrete GPU adapter is available"`
}

type gpuRenderVideoIn struct {
	SpecPath   string   `json:"spec_path" jsonschema:"absolute local path to a cliphub-render JSON scene or essay spec"`
	OutputPath string   `json:"output_path" jsonschema:"absolute local path of the new .mp4 file, in an existing folder; an existing file is refused"`
	Codec      string   `json:"codec,omitempty" jsonschema:"video encoder codec (auto, h264_nvenc, hevc_nvenc, av1_nvenc, h264_amf, hevc_amf, h264_qsv, hevc_qsv, libx264)"`
	Preset     string   `json:"preset,omitempty" jsonschema:"encoder preset override"`
	CQ         *int     `json:"cq,omitempty" jsonschema:"quality target override"`
	Workers    *int     `json:"workers,omitempty" jsonschema:"parallel scene workers for multi-scene essays"`
	LUFS       *float64 `json:"lufs,omitempty" jsonschema:"audio loudness normalization target in LUFS"`
	RequireGPU bool     `json:"require_gpu,omitempty" jsonschema:"fail unless a discrete GPU and hardware encoder are active"`
	CudaDecode bool     `json:"cuda_decode,omitempty" jsonschema:"enable CUDA hardware video decoding"`
	Approve    string   `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type gpuRenderVideoOut struct {
	NeedsApproval bool                      `json:"needs_approval,omitempty"`
	ApprovalCode  string                    `json:"approval_code,omitempty"`
	Summary       string                    `json:"summary"`
	Stats         *renderengine.RenderStats `json:"stats,omitempty"`
}

func registerRenderGPUTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "gpu_render_info",
		Annotations: readOnly,
		Description: "Inspect the GPU adapter, backend, hardware video encoders (NVENC/AMF/QSV), recommended codec, and FFmpeg editor hardware encoder available to ClipHub Studio.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, renderGPUInfoOut, error) {
		var out renderGPUInfoOut
		if err := studioJSONCall(ctx, s, http.MethodGet, "/api/render/gpu-info", nil, &out); err != nil {
			return nil, renderGPUInfoOut{}, toolError(err)
		}
		return nil, out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "validate_gpu_render_spec",
		Annotations: readOnly,
		Description: "Validate a local cliphub-render JSON scene or essay specification without rendering.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in validateRenderSpecIn) (*mcp.CallToolResult, renderengine.ValidationResult, error) {
		specPath, err := checkRenderLocalPath("spec_path", in.SpecPath)
		if err != nil {
			return nil, renderengine.ValidationResult{}, err
		}
		var out renderengine.ValidationResult
		payload := map[string]string{"spec_path": specPath}
		if err := studioJSONCall(ctx, s, http.MethodPost, "/api/render/validate", payload, &out); err != nil {
			return nil, renderengine.ValidationResult{}, toolError(err)
		}
		return nil, out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "gpu_render_snapshot",
		Description: "Render PNG visual-QA snapshots at specific timestamps from a local cliphub-render JSON spec.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in gpuRenderSnapshotIn) (*mcp.CallToolResult, renderengine.SnapshotResult, error) {
		specPath, err := checkRenderLocalPath("spec_path", in.SpecPath)
		if err != nil {
			return nil, renderengine.SnapshotResult{}, err
		}
		outDir, err := checkRenderLocalPath("out_dir", in.OutDir)
		if err != nil {
			return nil, renderengine.SnapshotResult{}, err
		}
		if len(in.At) == 0 {
			return nil, renderengine.SnapshotResult{}, errors.New("at must contain at least one timestamp")
		}
		reqBody := renderengine.SnapshotRequest{
			SpecPath:   specPath,
			At:         in.At,
			OutDir:     outDir,
			Scene:      strings.TrimSpace(in.Scene),
			RequireGPU: in.RequireGPU,
		}
		var out renderengine.SnapshotResult
		if err := studioJSONCall(ctx, s, http.MethodPost, "/api/render/snapshot", reqBody, &out); err != nil {
			return nil, renderengine.SnapshotResult{}, toolError(err)
		}
		return nil, out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "gpu_render_video",
		Description: "Render a local cliphub-render JSON scene or essay spec to a new MP4 file using the GPU render engine. It never replaces a file. " + startApprovalHint,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in gpuRenderVideoIn) (*mcp.CallToolResult, gpuRenderVideoOut, error) {
		specPath, err := checkRenderLocalPath("spec_path", in.SpecPath)
		if err != nil {
			return nil, gpuRenderVideoOut{}, err
		}
		outputPath := strings.TrimSpace(in.OutputPath)
		if err := checkNewLocalFile(outputPath, ".mp4"); err != nil {
			return nil, gpuRenderVideoOut{}, fmt.Errorf("output_path: %w", err)
		}
		payload := renderengine.RenderRequest{
			SpecPath:   specPath,
			OutputPath: outputPath,
			Codec:      strings.TrimSpace(in.Codec),
			Preset:     strings.TrimSpace(in.Preset),
			CQ:         in.CQ,
			Workers:    in.Workers,
			LUFS:       in.LUFS,
			RequireGPU: in.RequireGPU,
			CudaDecode: in.CudaDecode,
		}
		codecLabel := payload.Codec
		if codecLabel == "" {
			codecLabel = "auto"
		}
		summary := fmt.Sprintf(
			"Renderizar en GPU %s y crear el vídeo %s en la carpeta %s con códec %s.",
			quotedName(filepath.Base(specPath), "la especificación"),
			quotedName(filepath.Base(outputPath), "el archivo MP4"),
			quotedPath(filepath.Dir(outputPath)),
			cleanName(codecLabel),
		)
		decision, err := sessionApprovals.approve(req, approvalSpec{
			Tool:    "gpu_render_video",
			JobID:   specPath,
			Request: payload,
			Summary: summary,
			Given:   in.Approve,
			Auto:    autoStart,
		})
		if err != nil {
			return nil, gpuRenderVideoOut{}, err
		}
		if decision.Ask != nil {
			return decision.Ask, gpuRenderVideoOut{}, nil
		}
		if !decision.Approved {
			return nil, gpuRenderVideoOut{NeedsApproval: true, ApprovalCode: decision.Code, Summary: summary}, nil
		}
		var stats renderengine.RenderStats
		if err := studioJSONCall(ctx, s, http.MethodPost, "/api/render/run", payload, &stats); err != nil {
			return nil, gpuRenderVideoOut{}, toolError(err)
		}
		return nil, gpuRenderVideoOut{Summary: summary, Stats: &stats}, nil
	})
}

func checkRenderLocalPath(field, raw string) (string, error) {
	p := strings.TrimSpace(raw)
	if p == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	if strings.HasPrefix(p, `\`) || strings.HasPrefix(p, "//") {
		return "", fmt.Errorf("%s must be a local drive path, not a network UNC path", field)
	}
	if runtime.GOOS == "windows" && !localDrivePath(p) {
		return "", fmt.Errorf(`%s must start with a local drive letter, such as C:\`, field)
	}
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("%s must be an absolute path", field)
	}
	return p, nil
}

func studioJSONCall(ctx context.Context, s *studio, method, path string, body any, out any) error {
	c, err := s.client(ctx)
	if err != nil {
		return err
	}
	cfg, err := s.config()
	if err != nil && s.upstream == "" {
		return err
	}
	token := cfg.Token
	if token == "" {
		token = os.Getenv("ZV_MUTATION_TOKEN")
	}
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL()+path, reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("X-ClipHub-Token", token)
	}
	hc := &http.Client{Timeout: gpuRenderHTTPTimeout}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		rawErr, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		msg := strings.TrimSpace(string(rawErr))
		var payload struct {
			Code  string `json:"code"`
			Error string `json:"error"`
		}
		if json.Unmarshal(rawErr, &payload) == nil && payload.Error != "" {
			msg = payload.Error
		}
		return &tuiclient.APIError{StatusCode: resp.StatusCode, Code: payload.Code, Message: msg, Method: method, Path: path}
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (8<<20)+1))
	if err != nil {
		return fmt.Errorf("%s %s: read response: %w", method, path, err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%s %s: decode response: %w", method, path, err)
	}
	return nil
}
