package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/renderengine"
)

type RenderGPUInfoResponse struct {
	Available             bool                  `json:"available"`
	BinaryPath            string                `json:"binary_path,omitempty"`
	GPU                   *renderengine.GPUInfo `json:"gpu,omitempty"`
	EditorHardwareEncoder string                `json:"editor_hardware_encoder,omitempty"`
	Error                 string                `json:"error,omitempty"`
}

type validateRenderSpecRequest struct {
	SpecPath string `json:"spec_path"`
}

func (h *Handlers) renderFFmpegPath() string {
	for _, tool := range h.capabilities.RenderTools {
		if tool.Name == "ZV_FFMPEG_PATH" && strings.TrimSpace(tool.Path) != "" {
			return strings.TrimSpace(tool.Path)
		}
	}
	return ""
}

func validateAbsoluteLocalPath(field, raw string) (string, error) {
	p := strings.TrimSpace(raw)
	if p == "" {
		return "", errors.New(field + " is required")
	}
	if strings.HasPrefix(p, `\`) || strings.HasPrefix(p, "//") {
		return "", errors.New(field + " must be a local path, not a network UNC path")
	}
	if !filepath.IsAbs(p) {
		return "", errors.New(field + " must be an absolute path")
	}
	return p, nil
}

func (h *Handlers) GetRenderGPUInfo(w http.ResponseWriter, r *http.Request) {
	resp := RenderGPUInfoResponse{
		EditorHardwareEncoder: editor.DetectHardwareEncoder(h.renderFFmpegPath()),
	}
	bin, err := renderengine.FindBinary()
	if err != nil {
		resp.Error = err.Error()
		writeJSON(w, http.StatusOK, resp)
		return
	}
	resp.BinaryPath = bin
	info, err := renderengine.ProbeGPU(r.Context())
	if err != nil {
		resp.Error = err.Error()
		writeJSON(w, http.StatusOK, resp)
		return
	}
	resp.Available = true
	resp.GPU = &info
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handlers) ValidateRenderSpec(w http.ResponseWriter, r *http.Request) {
	var req validateRenderSpecRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	specPath, err := validateAbsoluteLocalPath("spec_path", req.SpecPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := renderengine.ValidateSpec(r.Context(), specPath)
	if err != nil {
		writeRenderEngineError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *Handlers) SnapshotRenderSpec(w http.ResponseWriter, r *http.Request) {
	var req renderengine.SnapshotRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	specPath, err := validateAbsoluteLocalPath("spec_path", req.SpecPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	outDir, err := validateAbsoluteLocalPath("out_dir", req.OutDir)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.At) == 0 {
		writeError(w, http.StatusBadRequest, "at must contain at least one timestamp")
		return
	}
	req.SpecPath = specPath
	req.OutDir = outDir
	res, err := renderengine.Snapshot(r.Context(), req)
	if err != nil {
		writeRenderEngineError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *Handlers) RunRenderSpec(w http.ResponseWriter, r *http.Request) {
	var req renderengine.RenderRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	specPath, err := validateAbsoluteLocalPath("spec_path", req.SpecPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	outputPath, err := validateAbsoluteLocalPath("output_path", req.OutputPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.SpecPath = specPath
	req.OutputPath = outputPath
	stats, err := renderengine.Render(r.Context(), req)
	if err != nil {
		writeRenderEngineError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func writeRenderEngineError(w http.ResponseWriter, err error) {
	if errors.Is(err, renderengine.ErrBinaryNotFound) {
		writeCodedError(w, http.StatusServiceUnavailable, "render_engine_unavailable", err.Error())
		return
	}
	writeCodedError(w, http.StatusUnprocessableEntity, "render_engine_error", err.Error())
}
