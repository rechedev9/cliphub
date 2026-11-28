package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/renderengine"
)

func TestRenderGPUToolsOverMCP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/render/gpu-info":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(renderGPUInfoOut{
				Available:             true,
				BinaryPath:            "C:\\bin\\cliphub-render.exe",
				EditorHardwareEncoder: "nvenc-h264",
				GPU: &renderengine.GPUInfo{
					Adapter:          "NVIDIA GeForce RTX 4080",
					Backend:          "Vulkan",
					DeviceType:       "DiscreteGpu",
					IsDiscreteGPU:    true,
					NVENCEncoders:    []string{"h264_nvenc"},
					HardwareEncoders: []string{"h264_nvenc"},
					RecommendedCodec: "h264_nvenc",
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/render/validate":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(renderengine.ValidationResult{
				OK:      true,
				Summary: "OK: scene.json",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/render/snapshot":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(renderengine.SnapshotResult{
				OutputDir: filepath.Join(t.TempDir(), "snaps"),
				Files:     []string{"snap_0.500s.png"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/render/run":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(renderengine.RenderStats{
				Output:        "out.mp4",
				GPUName:       "NVIDIA GeForce RTX 4080",
				Codec:         "h264_nvenc",
				TotalFrames:   120,
				IsDiscreteGPU: true,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv("ORCHESTRATOR_URL", srv.URL)
	t.Setenv("ZV_MUTATION_TOKEN", "")

	ctx := context.Background()
	s := &studio{sessionPath: filepath.Join(t.TempDir(), "missing-session.json")}
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	registerTools(server, s)
	serverT, clientT := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "gpu_render_info", Arguments: map[string]any{}})
	if err != nil || res.IsError {
		t.Fatalf("gpu_render_info: res=%+v err=%v", res, err)
	}
	var gpuOut renderGPUInfoOut
	raw, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(raw, &gpuOut); err != nil {
		t.Fatalf("unmarshal gpu_render_info: %v", err)
	}
	if !gpuOut.Available || gpuOut.GPU == nil || gpuOut.GPU.RecommendedCodec != "h264_nvenc" {
		t.Fatalf("gpu_render_info = %+v", gpuOut)
	}

	dir := t.TempDir()
	specPath := filepath.Join(dir, "scene.json")
	outDir := filepath.Join(dir, "snaps")
	outMP4 := filepath.Join(dir, "out.mp4")

	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "validate_gpu_render_spec",
		Arguments: map[string]any{"spec_path": specPath},
	})
	if err != nil || res.IsError {
		t.Fatalf("validate_gpu_render_spec: res=%+v err=%v", res, err)
	}

	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "gpu_render_snapshot",
		Arguments: map[string]any{"spec_path": specPath, "at": []float64{0.5}, "out_dir": outDir},
	})
	if err != nil || res.IsError {
		t.Fatalf("gpu_render_snapshot: res=%+v err=%v", res, err)
	}

	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "gpu_render_video",
		Arguments: map[string]any{"spec_path": specPath, "output_path": outMP4, "codec": "auto"},
	})
	if err != nil || res.IsError {
		t.Fatalf("gpu_render_video first call: res=%+v err=%v", res, err)
	}
	var firstOut gpuRenderVideoOut
	raw, _ = json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(raw, &firstOut); err != nil {
		t.Fatalf("unmarshal gpu_render_video: %v", err)
	}
	if !firstOut.NeedsApproval || firstOut.ApprovalCode == "" {
		t.Fatalf("gpu_render_video without approval = %+v, want needs_approval + code", firstOut)
	}
	if !strings.Contains(firstOut.Summary, "«"+dir+"»") {
		t.Fatalf("approval summary %q does not name the output folder %q", firstOut.Summary, dir)
	}

	existing := filepath.Join(dir, "existing.mp4")
	if err := os.WriteFile(existing, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, refused := range []string{existing, filepath.Join(dir, "scene.json"), filepath.Join(dir, "missing", "out.mp4")} {
		res, err = cs.CallTool(ctx, &mcp.CallToolParams{
			Name:      "gpu_render_video",
			Arguments: map[string]any{"spec_path": specPath, "output_path": refused},
		})
		if err != nil || !res.IsError {
			t.Fatalf("gpu_render_video into %q: res=%+v err=%v, want a tool error", refused, res, err)
		}
	}

	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "gpu_render_video",
		Arguments: map[string]any{"spec_path": specPath, "output_path": outMP4, "codec": "auto", "approve": firstOut.ApprovalCode},
	})
	if err != nil || res.IsError {
		t.Fatalf("gpu_render_video approved call: res=%+v err=%v", res, err)
	}
	var approvedOut gpuRenderVideoOut
	raw, _ = json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(raw, &approvedOut); err != nil {
		t.Fatalf("unmarshal approved gpu_render_video: %v", err)
	}
	if approvedOut.Stats == nil || approvedOut.Stats.TotalFrames != 120 {
		t.Fatalf("approved gpu_render_video = %+v", approvedOut)
	}
}
