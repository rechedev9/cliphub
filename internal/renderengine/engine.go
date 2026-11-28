package renderengine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	envRenderBin          = "ZV_RENDER_BIN"
	envFFmpegPath         = "ZV_FFMPEG_PATH"
	defaultProbeTTL       = 15 * time.Second
	defaultSnapTTL        = 2 * time.Minute
	childWaitDelay        = 10 * time.Second
	partialRemoveAttempts = 20
	partialRemoveDelay    = 250 * time.Millisecond
)

var ErrBinaryNotFound = errors.New("cliphub-render binary not found")

type Engine struct {
	FFmpegPath string
}

type GPUInfo struct {
	Adapter          string   `json:"gpu_adapter"`
	Backend          string   `json:"backend"`
	DeviceType       string   `json:"device_type"`
	IsDiscreteGPU    bool     `json:"is_discrete_gpu"`
	NVENCEncoders    []string `json:"nvenc_encoders"`
	HardwareEncoders []string `json:"hardware_encoders,omitempty"`
	RecommendedCodec string   `json:"recommended_codec,omitempty"`
}

type ValidationResult struct {
	OK       bool     `json:"ok"`
	Summary  string   `json:"summary"`
	Warnings []string `json:"warnings,omitempty"`
}

type SnapshotRequest struct {
	SpecPath   string    `json:"spec_path"`
	At         []float64 `json:"at"`
	OutDir     string    `json:"out_dir"`
	Scene      string    `json:"scene,omitempty"`
	RequireGPU bool      `json:"require_gpu,omitempty"`
}

type SnapshotResult struct {
	OutputDir string   `json:"output_dir"`
	Files     []string `json:"files"`
	RawOutput string   `json:"raw_output"`
}

type RenderRequest struct {
	SpecPath   string   `json:"spec_path"`
	OutputPath string   `json:"output_path"`
	Codec      string   `json:"codec,omitempty"`
	Preset     string   `json:"preset,omitempty"`
	CQ         *int     `json:"cq,omitempty"`
	Workers    *int     `json:"workers,omitempty"`
	LUFS       *float64 `json:"lufs,omitempty"`
	RequireGPU bool     `json:"require_gpu,omitempty"`
	CudaDecode bool     `json:"cuda_decode,omitempty"`
}

type SceneStats struct {
	ID           string  `json:"id"`
	DurationSec  float64 `json:"duration_sec"`
	TotalFrames  int     `json:"total_frames"`
	RenderMS     int64   `json:"render_ms"`
	EffectiveFPS float64 `json:"effective_fps"`
}

type RenderStats struct {
	Output         string       `json:"output"`
	GPUName        string       `json:"gpu_name"`
	GPUBackend     string       `json:"gpu_backend"`
	IsDiscreteGPU  bool         `json:"is_discrete_gpu,omitempty"`
	Codec          string       `json:"codec"`
	Workers        int          `json:"workers,omitempty"`
	Width          int          `json:"width"`
	Height         int          `json:"height"`
	FPS            int          `json:"fps"`
	SceneCount     int          `json:"scene_count,omitempty"`
	TotalFrames    int          `json:"total_frames"`
	DurationSec    float64      `json:"duration_sec"`
	InitMS         int64        `json:"init_ms,omitempty"`
	RenderMS       int64        `json:"render_ms,omitempty"`
	ScenesWallMS   int64        `json:"scenes_wall_ms,omitempty"`
	ConcatMS       int64        `json:"concat_ms,omitempty"`
	EncodeFlushMS  int64        `json:"encode_flush_ms,omitempty"`
	AudioMuxMS     int64        `json:"audio_mux_ms"`
	TotalMS        int64        `json:"total_ms"`
	EffectiveFPS   float64      `json:"effective_fps,omitempty"`
	AggregateFPS   float64      `json:"aggregate_fps,omitempty"`
	RealtimeFactor float64      `json:"realtime_factor"`
	OutputBytes    uint64       `json:"output_bytes"`
	Scenes         []SceneStats `json:"scenes,omitempty"`
}

func FindBinary() (string, error) {
	exePath, _ := os.Executable()
	cwd, _ := os.Getwd()
	return findBinary(os.Getenv(envRenderBin), exePath, cwd)
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "cliphub-render.exe"
	}
	return "cliphub-render"
}

func findBinary(configured, exePath, cwd string) (string, error) {
	if trimmed := strings.TrimSpace(configured); trimmed != "" {
		if isRegularFile(trimmed) {
			return trimmed, nil
		}
		if resolved, err := exec.LookPath(trimmed); err == nil && isRegularFile(resolved) {
			return resolved, nil
		}
		return "", fmt.Errorf("%w: %s=%q", ErrBinaryNotFound, envRenderBin, trimmed)
	}

	name := binaryName()
	var candidates []string
	if exePath != "" {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, name),
			filepath.Join(exeDir, "bin", name),
		)
	}
	if cwd != "" {
		candidates = append(candidates, filepath.Join(cwd, "bin", name))
		for _, root := range ancestorDirs(cwd, 6) {
			candidates = append(candidates, filepath.Join(root, "tools", "render", "target", "release", name))
		}
	}
	if exePath != "" {
		for _, root := range ancestorDirs(filepath.Dir(exePath), 6) {
			candidates = append(candidates, filepath.Join(root, "tools", "render", "target", "release", name))
		}
	}
	for _, candidate := range candidates {
		if isRegularFile(candidate) {
			return candidate, nil
		}
	}
	if resolved, err := exec.LookPath(name); err == nil && isRegularFile(resolved) {
		return resolved, nil
	}
	return "", ErrBinaryNotFound
}

func ancestorDirs(start string, maxDepth int) []string {
	dir := filepath.Clean(start)
	var dirs []string
	for i := 0; i < maxDepth && dir != ""; i++ {
		dirs = append(dirs, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return dirs
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS != "windows" {
		return info.Mode()&0o111 != 0
	}
	return true
}

func withDefaultTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}

func (e Engine) command(ctx context.Context, bin string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.WaitDelay = childWaitDelay
	if path := strings.TrimSpace(e.FFmpegPath); path != "" {
		cmd.Env = append(cmd.Environ(), envFFmpegPath+"="+path)
	}
	return cmd
}

func runCommand(cmd *exec.Cmd) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if detail != "" {
			return "", fmt.Errorf("cliphub-render %s: %w: %s", cmd.Args[1], err, detail)
		}
		return "", fmt.Errorf("cliphub-render %s: %w", cmd.Args[1], err)
	}
	return stdout.String(), nil
}

func (e Engine) ProbeGPU(ctx context.Context) (GPUInfo, error) {
	bin, err := FindBinary()
	if err != nil {
		return GPUInfo{}, err
	}
	runCtx, cancel := withDefaultTimeout(ctx, defaultProbeTTL)
	defer cancel()
	out, err := runCommand(e.command(runCtx, bin, "info", "--json"))
	if err != nil {
		return GPUInfo{}, err
	}
	var info GPUInfo
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		return GPUInfo{}, fmt.Errorf("parse cliphub-render info JSON: %w", err)
	}
	if info.NVENCEncoders == nil {
		info.NVENCEncoders = []string{}
	}
	if len(info.HardwareEncoders) == 0 && len(info.NVENCEncoders) > 0 {
		info.HardwareEncoders = append([]string(nil), info.NVENCEncoders...)
	}
	if info.HardwareEncoders == nil {
		info.HardwareEncoders = []string{}
	}
	return info, nil
}

func (e Engine) ValidateSpec(ctx context.Context, specPath string) (ValidationResult, error) {
	specPath = strings.TrimSpace(specPath)
	if specPath == "" {
		return ValidationResult{}, errors.New("spec path is required")
	}
	bin, err := FindBinary()
	if err != nil {
		return ValidationResult{}, err
	}
	runCtx, cancel := withDefaultTimeout(ctx, defaultProbeTTL)
	defer cancel()
	out, err := runCommand(e.command(runCtx, bin, "validate", specPath))
	if err != nil {
		return ValidationResult{}, err
	}
	var summary string
	var warnings []string
	for _, rawLine := range strings.Split(out, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "WARN:") {
			warnings = append(warnings, strings.TrimSpace(strings.TrimPrefix(line, "WARN:")))
			continue
		}
		if summary == "" {
			summary = line
		}
	}
	if summary == "" {
		summary = strings.TrimSpace(out)
	}
	return ValidationResult{
		OK:       true,
		Summary:  summary,
		Warnings: warnings,
	}, nil
}

func (e Engine) Snapshot(ctx context.Context, req SnapshotRequest) (SnapshotResult, error) {
	specPath := strings.TrimSpace(req.SpecPath)
	if specPath == "" {
		return SnapshotResult{}, errors.New("spec path is required")
	}
	if len(req.At) == 0 {
		return SnapshotResult{}, errors.New("at least one snapshot timestamp is required")
	}
	outDir := strings.TrimSpace(req.OutDir)
	if outDir == "" {
		return SnapshotResult{}, errors.New("output directory is required")
	}
	bin, err := FindBinary()
	if err != nil {
		return SnapshotResult{}, err
	}
	timestamps := make([]string, len(req.At))
	for i, ts := range req.At {
		timestamps[i] = strconv.FormatFloat(ts, 'f', -1, 64)
	}
	args := []string{"snapshot", specPath, "--at", strings.Join(timestamps, ","), "-o", outDir}
	if scene := strings.TrimSpace(req.Scene); scene != "" {
		args = append(args, "--scene", scene)
	}
	if req.RequireGPU {
		args = append(args, "--require-gpu")
	}
	runCtx, cancel := withDefaultTimeout(ctx, defaultSnapTTL)
	defer cancel()
	out, err := runCommand(e.command(runCtx, bin, args...))
	if err != nil {
		return SnapshotResult{}, err
	}
	var files []string
	for _, rawLine := range strings.Split(out, "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasSuffix(strings.ToLower(line), ".png") {
			files = append(files, line)
		}
	}
	if files == nil {
		files = []string{}
	}
	return SnapshotResult{
		OutputDir: outDir,
		Files:     files,
		RawOutput: strings.TrimSpace(out),
	}, nil
}

func (e Engine) Render(ctx context.Context, req RenderRequest) (RenderStats, error) {
	specPath := strings.TrimSpace(req.SpecPath)
	if specPath == "" {
		return RenderStats{}, errors.New("spec path is required")
	}
	outputPath := strings.TrimSpace(req.OutputPath)
	if outputPath == "" {
		return RenderStats{}, errors.New("output path is required")
	}
	bin, err := FindBinary()
	if err != nil {
		return RenderStats{}, err
	}
	cmd := e.command(ctx, bin, buildRenderArgs(specPath, outputPath, req)...)
	out, err := runCommand(cmd)
	if err != nil {
		removePartialRender(outputPath, cmd.Process)
		return RenderStats{}, err
	}
	var stats RenderStats
	if err := json.Unmarshal([]byte(out), &stats); err != nil {
		return RenderStats{}, fmt.Errorf("parse cliphub-render stats JSON: %w", err)
	}
	return stats, nil
}

func removePartialRender(outputPath string, process *os.Process) {
	if process == nil {
		return
	}
	workDir := fmt.Sprintf("%s.%d.partial", outputPath, process.Pid)
	for range partialRemoveAttempts {
		if os.RemoveAll(workDir) == nil {
			return
		}
		time.Sleep(partialRemoveDelay)
	}
}

func buildRenderArgs(specPath, outputPath string, req RenderRequest) []string {
	args := []string{"render", specPath, "-o", outputPath, "--stats-json"}
	if codec := strings.TrimSpace(req.Codec); codec != "" {
		args = append(args, "--codec", codec)
	}
	if preset := strings.TrimSpace(req.Preset); preset != "" {
		args = append(args, "--preset", preset)
	}
	if req.CQ != nil {
		args = append(args, "--cq", strconv.Itoa(*req.CQ))
	}
	if req.Workers != nil {
		args = append(args, "--workers", strconv.Itoa(*req.Workers))
	}
	if req.LUFS != nil {
		args = append(args, "--lufs", strconv.FormatFloat(*req.LUFS, 'f', -1, 64))
	}
	if req.RequireGPU {
		args = append(args, "--require-gpu")
	}
	if req.CudaDecode {
		args = append(args, "--cuda-decode")
	}
	return args
}
