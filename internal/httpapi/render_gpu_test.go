package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/renderengine"
)

func writeFakeRenderScript(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, "cliphub-render.cmd")
		body := strings.Join([]string{
			"@echo off",
			"if \"%~1\"==\"info\" goto do_info",
			"if \"%~1\"==\"validate\" goto do_validate",
			"if \"%~1\"==\"snapshot\" goto do_snapshot",
			"if \"%~1\"==\"render\" goto do_render",
			"echo unknown command 1>&2",
			"exit /b 1",
			":do_info",
			"echo {\"gpu_adapter\":\"Mock RTX 4090\",\"backend\":\"Vulkan\",\"device_type\":\"DiscreteGpu\",\"is_discrete_gpu\":true,\"nvenc_encoders\":[\"h264_nvenc\"],\"hardware_encoders\":[\"h264_nvenc\",\"h264_amf\"],\"recommended_codec\":\"h264_nvenc\"}",
			"exit /b 0",
			":do_validate",
			"echo OK: %~2 (project 1920x1080 @ 60 fps, 2.000s = 120 frames, 1 layers, 0 audio clips)",
			"exit /b 0",
			":do_snapshot",
			"echo Rendered 1 snapshots in 10 ms:",
			"echo   %~6\\snap_0.500s.png",
			"exit /b 0",
			":do_render",
			"echo {\"output\":\"mock.mp4\",\"gpu_name\":\"Mock RTX 4090\",\"gpu_backend\":\"Vulkan\",\"is_discrete_gpu\":true,\"codec\":\"h264_nvenc\",\"width\":1920,\"height\":1080,\"fps\":60,\"total_frames\":120,\"duration_sec\":2.0,\"audio_mux_ms\":5,\"total_ms\":50,\"realtime_factor\":40.0,\"output_bytes\":12345}",
			"exit /b 0",
		}, "\r\n") + "\r\n"
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	path := filepath.Join(dir, "cliphub-render")
	body := strings.Join([]string{
		"#!/bin/sh",
		"case \"$1\" in",
		"  info)",
		"    echo '{\"gpu_adapter\":\"Mock RTX 4090\",\"backend\":\"Vulkan\",\"device_type\":\"DiscreteGpu\",\"is_discrete_gpu\":true,\"nvenc_encoders\":[\"h264_nvenc\"],\"hardware_encoders\":[\"h264_nvenc\",\"h264_amf\"],\"recommended_codec\":\"h264_nvenc\"}'",
		"    exit 0 ;;",
		"  validate)",
		"    echo \"OK: $2 (project 1920x1080 @ 60 fps, 2.000s = 120 frames, 1 layers, 0 audio clips)\"",
		"    exit 0 ;;",
		"  snapshot)",
		"    echo 'Rendered 1 snapshots in 10 ms:'",
		"    echo \"  $6/snap_0.500s.png\"",
		"    exit 0 ;;",
		"  render)",
		"    echo '{\"output\":\"mock.mp4\",\"gpu_name\":\"Mock RTX 4090\",\"gpu_backend\":\"Vulkan\",\"is_discrete_gpu\":true,\"codec\":\"h264_nvenc\",\"width\":1920,\"height\":1080,\"fps\":60,\"total_frames\":120,\"duration_sec\":2.0,\"audio_mux_ms\":5,\"total_ms\":50,\"realtime_factor\":40.0,\"output_bytes\":12345}'",
		"    exit 0 ;;",
		"esac",
		"echo 'unknown command' >&2",
		"exit 1",
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFakeNoGPUFFmpeg(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, "ffmpeg.cmd")
		if err := os.WriteFile(path, []byte("@echo off\r\nexit /b 1\r\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	path := filepath.Join(dir, "ffmpeg")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRenderGPUHandlers(t *testing.T) {
	fakeBin := writeFakeRenderScript(t)
	fakeFFmpeg := writeFakeNoGPUFFmpeg(t)
	t.Setenv("ZV_RENDER_BIN", fakeBin)

	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithCapabilities(Capabilities{
		RenderEnabled: true,
		RenderTools:   []CaptureTool{{Name: "ZV_FFMPEG_PATH", Path: fakeFFmpeg}},
	}))
	router := Routes(h)

	t.Run("GET /api/render/gpu-info returns structured GPU info", func(t *testing.T) {
		rw := httptest.NewRecorder()
		router.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/render/gpu-info", nil))
		if rw.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rw.Code, rw.Body.String())
		}
		var got RenderGPUInfoResponse
		if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !got.Available || got.GPU == nil || got.GPU.Adapter != "Mock RTX 4090" || got.GPU.RecommendedCodec != "h264_nvenc" {
			t.Fatalf("got = %+v", got)
		}
	})

	t.Run("GET /api/render/gpu-info reports unavailable cleanly", func(t *testing.T) {
		t.Setenv("ZV_RENDER_BIN", filepath.Join(t.TempDir(), "missing-render.exe"))
		rw := httptest.NewRecorder()
		router.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/render/gpu-info", nil))
		if rw.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rw.Code)
		}
		var got RenderGPUInfoResponse
		if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.Available || got.Error == "" {
			t.Fatalf("got = %+v, want unavailable with error", got)
		}
	})

	t.Run("POST /api/render/validate validates spec", func(t *testing.T) {
		specPath := filepath.Join(t.TempDir(), "scene.json")
		body, _ := json.Marshal(map[string]string{"spec_path": specPath})
		rw := httptest.NewRecorder()
		router.ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/render/validate", bytes.NewReader(body)))
		if rw.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rw.Code, rw.Body.String())
		}
		var got renderengine.ValidationResult
		if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !got.OK || !strings.Contains(got.Summary, "OK:") {
			t.Fatalf("got = %+v", got)
		}
	})

	t.Run("POST /api/render/validate rejects relative path", func(t *testing.T) {
		body := []byte(`{"spec_path":"relative.json"}`)
		rw := httptest.NewRecorder()
		router.ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/render/validate", bytes.NewReader(body)))
		if rw.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rw.Code)
		}
	})

	t.Run("POST /api/render/snapshot runs snapshots", func(t *testing.T) {
		dir := t.TempDir()
		body, _ := json.Marshal(renderengine.SnapshotRequest{
			SpecPath: filepath.Join(dir, "scene.json"),
			At:       []float64{0.5},
			OutDir:   filepath.Join(dir, "snaps"),
		})
		rw := httptest.NewRecorder()
		router.ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/render/snapshot", bytes.NewReader(body)))
		if rw.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rw.Code, rw.Body.String())
		}
		var got renderengine.SnapshotResult
		if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(got.Files) != 1 {
			t.Fatalf("got = %+v", got)
		}
	})

	t.Run("POST /api/render/run executes render", func(t *testing.T) {
		dir := t.TempDir()
		body, _ := json.Marshal(renderengine.RenderRequest{
			SpecPath:   filepath.Join(dir, "scene.json"),
			OutputPath: filepath.Join(dir, "out.mp4"),
			Codec:      "auto",
		})
		rw := httptest.NewRecorder()
		router.ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/render/run", bytes.NewReader(body)))
		if rw.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rw.Code, rw.Body.String())
		}
		var got renderengine.RenderStats
		if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.Codec != "h264_nvenc" || got.TotalFrames != 120 {
			t.Fatalf("got = %+v", got)
		}
	})
}
