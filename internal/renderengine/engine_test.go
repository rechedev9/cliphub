package renderengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

const fakeRenderModeEnv = "CLIPHUB_TEST_FAKE_RENDER_MODE"

func TestMain(m *testing.M) {
	if mode := os.Getenv(fakeRenderModeEnv); mode != "" {
		os.Exit(runFakeRenderCLI(mode, os.Args[1:]))
	}
	os.Exit(m.Run())
}

func runFakeRenderCLI(mode string, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "missing subcommand")
		return 2
	}
	switch args[0] {
	case "info":
		switch mode {
		case "info-full":
			fmt.Println(`{"gpu_adapter":"NVIDIA GeForce RTX 4080","backend":"Vulkan","device_type":"DiscreteGpu","is_discrete_gpu":true,"nvenc_encoders":["h264_nvenc","hevc_nvenc"],"hardware_encoders":["h264_nvenc","hevc_nvenc","h264_amf"],"recommended_codec":"h264_nvenc"}`)
			return 0
		case "info-legacy":
			fmt.Println(`{"gpu_adapter":"NVIDIA GeForce RTX 3070","backend":"Dx12","device_type":"DiscreteGpu","is_discrete_gpu":true,"nvenc_encoders":["h264_nvenc"]}`)
			return 0
		case "info-bad-json":
			fmt.Println("not-json")
			return 0
		case "fail":
			fmt.Fprintln(os.Stderr, "no compatible GPU adapter found")
			return 1
		}
	case "validate":
		if mode == "validate-ok" {
			fmt.Printf("OK: %s (project 1920x1080 @ 60 fps, 5.000s = 300 frames, 2 layers, 0 audio clips)\n", args[1])
			fmt.Println("WARN: font fallback active")
			return 0
		}
		fmt.Fprintln(os.Stderr, "invalid spec: missing duration")
		return 1
	case "snapshot":
		if mode == "snapshot-ok" {
			outDir := ""
			for i := 0; i < len(args)-1; i++ {
				if args[i] == "-o" {
					outDir = args[i+1]
				}
			}
			fmt.Printf("ARGS: %s\n", strings.Join(args, "|"))
			fmt.Println("Rendered 2 snapshots in 42 ms:")
			fmt.Printf("  %s\n", filepath.Join(outDir, "snap_0.500s.png"))
			fmt.Printf("  %s\n", filepath.Join(outDir, "snap_2.500s.png"))
			return 0
		}
		fmt.Fprintln(os.Stderr, "snapshot failed: discrete GPU required")
		return 1
	case "render":
		if mode == "render-ok" {
			outPath := ""
			codec := "auto"
			for i := 0; i < len(args)-1; i++ {
				if args[i] == "-o" {
					outPath = args[i+1]
				}
				if args[i] == "--codec" {
					codec = args[i+1]
				}
			}
			if argDump := os.Getenv("CLIPHUB_TEST_ARG_DUMP"); argDump != "" {
				_ = os.WriteFile(argDump, []byte(strings.Join(args, "\n")), 0o644)
			}
			stats := RenderStats{
				Output:         outPath,
				GPUName:        "NVIDIA GeForce RTX 4080",
				GPUBackend:     "Vulkan",
				IsDiscreteGPU:  true,
				Codec:          codec,
				Width:          1920,
				Height:         1080,
				FPS:            60,
				TotalFrames:    300,
				DurationSec:    5.0,
				InitMS:         15,
				RenderMS:       120,
				EncodeFlushMS:  30,
				AudioMuxMS:     10,
				TotalMS:        175,
				EffectiveFPS:   1714.2,
				RealtimeFactor: 28.5,
				OutputBytes:    1048576,
			}
			encoded, _ := json.Marshal(stats)
			fmt.Println(string(encoded))
			return 0
		}
		if mode == "render-bad-json" {
			fmt.Println("{broken")
			return 0
		}
		fmt.Fprintln(os.Stderr, "render failed: encoder open failed")
		return 1
	}
	fmt.Fprintf(os.Stderr, "unexpected command %q in mode %q\n", args[0], mode)
	return 2
}

func makeFakeBinary(t *testing.T, dir string) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, binaryName())
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, data, 0o755); err != nil {
		t.Fatal(err)
	}
	return dest
}

func touchExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("stub"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestFindBinaryResolutionOrder(t *testing.T) {
	t.Run("ZV_RENDER_BIN wins", func(t *testing.T) {
		root := t.TempDir()
		envBin := filepath.Join(root, "custom", binaryName())
		touchExecutable(t, envBin)
		exePath := filepath.Join(root, "app", "zv-orchestrator.exe")
		touchExecutable(t, filepath.Join(root, "app", binaryName()))

		got, err := findBinary(envBin, exePath, root)
		if err != nil {
			t.Fatalf("findBinary() error = %v", err)
		}
		if got != envBin {
			t.Fatalf("findBinary() = %q, want %q", got, envBin)
		}
	})

	t.Run("missing ZV_RENDER_BIN returns ErrBinaryNotFound", func(t *testing.T) {
		root := t.TempDir()
		_, err := findBinary(filepath.Join(root, "missing.exe"), "", root)
		if !errors.Is(err, ErrBinaryNotFound) {
			t.Fatalf("findBinary() error = %v, want ErrBinaryNotFound", err)
		}
	})

	t.Run("next to executable wins over release target", func(t *testing.T) {
		root := t.TempDir()
		exeDir := filepath.Join(root, "bin")
		siblingBin := filepath.Join(exeDir, binaryName())
		touchExecutable(t, siblingBin)
		releaseBin := filepath.Join(root, "tools", "render", "target", "release", binaryName())
		touchExecutable(t, releaseBin)

		got, err := findBinary("", filepath.Join(exeDir, "zv-orchestrator.exe"), root)
		if err != nil {
			t.Fatalf("findBinary() error = %v", err)
		}
		if got != siblingBin {
			t.Fatalf("findBinary() = %q, want %q", got, siblingBin)
		}
	})

	t.Run("tools/render/target/release fallback", func(t *testing.T) {
		root := t.TempDir()
		subDir := filepath.Join(root, "internal", "renderengine")
		if err := os.MkdirAll(subDir, 0o755); err != nil {
			t.Fatal(err)
		}
		releaseBin := filepath.Join(root, "tools", "render", "target", "release", binaryName())
		touchExecutable(t, releaseBin)
		t.Setenv("PATH", t.TempDir())

		got, err := findBinary("", filepath.Join(t.TempDir(), "zv.exe"), subDir)
		if err != nil {
			t.Fatalf("findBinary() error = %v", err)
		}
		if got != releaseBin {
			t.Fatalf("findBinary() = %q, want %q", got, releaseBin)
		}
	})

	t.Run("PATH fallback", func(t *testing.T) {
		pathDir := t.TempDir()
		pathBin := filepath.Join(pathDir, binaryName())
		touchExecutable(t, pathBin)
		t.Setenv("PATH", pathDir)
		if runtime.GOOS == "windows" {
			t.Setenv("PATHEXT", ".EXE")
		}

		got, err := findBinary("", filepath.Join(t.TempDir(), "zv.exe"), t.TempDir())
		if err != nil {
			t.Fatalf("findBinary() error = %v", err)
		}
		if !strings.EqualFold(filepath.Clean(got), filepath.Clean(pathBin)) {
			t.Fatalf("findBinary() = %q, want %q", got, pathBin)
		}
	})

	t.Run("not found anywhere", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		_, err := findBinary("", filepath.Join(t.TempDir(), "zv.exe"), t.TempDir())
		if !errors.Is(err, ErrBinaryNotFound) {
			t.Fatalf("findBinary() error = %v, want ErrBinaryNotFound", err)
		}
	})
}

func TestProbeGPU(t *testing.T) {
	bin := makeFakeBinary(t, t.TempDir())
	t.Setenv(envRenderBin, bin)

	t.Run("full payload", func(t *testing.T) {
		t.Setenv(fakeRenderModeEnv, "info-full")
		info, err := ProbeGPU(context.Background())
		if err != nil {
			t.Fatalf("ProbeGPU() error = %v", err)
		}
		if info.Adapter != "NVIDIA GeForce RTX 4080" || info.Backend != "Vulkan" || info.DeviceType != "DiscreteGpu" || !info.IsDiscreteGPU {
			t.Fatalf("ProbeGPU() = %+v", info)
		}
		if !slices.Equal(info.NVENCEncoders, []string{"h264_nvenc", "hevc_nvenc"}) {
			t.Fatalf("NVENCEncoders = %v", info.NVENCEncoders)
		}
		if !slices.Equal(info.HardwareEncoders, []string{"h264_nvenc", "hevc_nvenc", "h264_amf"}) {
			t.Fatalf("HardwareEncoders = %v", info.HardwareEncoders)
		}
		if info.RecommendedCodec != "h264_nvenc" {
			t.Fatalf("RecommendedCodec = %q, want h264_nvenc", info.RecommendedCodec)
		}
	})

	t.Run("legacy payload mirrors nvenc into hardware_encoders", func(t *testing.T) {
		t.Setenv(fakeRenderModeEnv, "info-legacy")
		info, err := ProbeGPU(context.Background())
		if err != nil {
			t.Fatalf("ProbeGPU() error = %v", err)
		}
		if !slices.Equal(info.HardwareEncoders, []string{"h264_nvenc"}) {
			t.Fatalf("HardwareEncoders = %v, want [h264_nvenc]", info.HardwareEncoders)
		}
	})

	t.Run("invalid JSON fails clearly", func(t *testing.T) {
		t.Setenv(fakeRenderModeEnv, "info-bad-json")
		if _, err := ProbeGPU(context.Background()); err == nil || !strings.Contains(err.Error(), "parse cliphub-render info JSON") {
			t.Fatalf("ProbeGPU() error = %v", err)
		}
	})

	t.Run("process failure returns stderr detail", func(t *testing.T) {
		t.Setenv(fakeRenderModeEnv, "fail")
		if _, err := ProbeGPU(context.Background()); err == nil || !strings.Contains(err.Error(), "no compatible GPU adapter found") {
			t.Fatalf("ProbeGPU() error = %v", err)
		}
	})
}

func TestValidateSpec(t *testing.T) {
	bin := makeFakeBinary(t, t.TempDir())
	t.Setenv(envRenderBin, bin)

	t.Run("empty spec path rejected", func(t *testing.T) {
		if _, err := ValidateSpec(context.Background(), "   "); err == nil {
			t.Fatal("ValidateSpec(empty) succeeded, want error")
		}
	})

	t.Run("valid spec parses summary and warnings", func(t *testing.T) {
		t.Setenv(fakeRenderModeEnv, "validate-ok")
		res, err := ValidateSpec(context.Background(), "scene.json")
		if err != nil {
			t.Fatalf("ValidateSpec() error = %v", err)
		}
		if !res.OK || !strings.Contains(res.Summary, "OK: scene.json") {
			t.Fatalf("ValidateSpec() = %+v", res)
		}
		if !slices.Equal(res.Warnings, []string{"font fallback active"}) {
			t.Fatalf("Warnings = %v", res.Warnings)
		}
	})

	t.Run("invalid spec surfaces error", func(t *testing.T) {
		t.Setenv(fakeRenderModeEnv, "fail")
		if _, err := ValidateSpec(context.Background(), "bad.json"); err == nil || !strings.Contains(err.Error(), "missing duration") {
			t.Fatalf("ValidateSpec() error = %v", err)
		}
	})
}

func TestSnapshot(t *testing.T) {
	bin := makeFakeBinary(t, t.TempDir())
	t.Setenv(envRenderBin, bin)
	t.Setenv(fakeRenderModeEnv, "snapshot-ok")

	outDir := filepath.Join(t.TempDir(), "snaps")
	res, err := Snapshot(context.Background(), SnapshotRequest{
		SpecPath:   "essay.json",
		At:         []float64{0.5, 2.5},
		OutDir:     outDir,
		Scene:      "intro",
		RequireGPU: true,
	})
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if res.OutputDir != outDir || len(res.Files) != 2 {
		t.Fatalf("Snapshot() = %+v", res)
	}
	for _, wantArg := range []string{"snapshot", "essay.json", "--at", "0.5,2.5", "-o", outDir, "--scene", "intro", "--require-gpu"} {
		if !strings.Contains(res.RawOutput, wantArg) {
			t.Fatalf("Snapshot() args missing %q in %q", wantArg, res.RawOutput)
		}
	}

	if _, err := Snapshot(context.Background(), SnapshotRequest{SpecPath: "essay.json", OutDir: outDir}); err == nil {
		t.Fatal("Snapshot(no timestamps) succeeded, want error")
	}
}

func TestRender(t *testing.T) {
	bin := makeFakeBinary(t, t.TempDir())
	t.Setenv(envRenderBin, bin)
	t.Setenv(fakeRenderModeEnv, "render-ok")

	argDump := filepath.Join(t.TempDir(), "args.txt")
	t.Setenv("CLIPHUB_TEST_ARG_DUMP", argDump)

	cq := 19
	workers := 4
	lufs := -14.0
	outPath := filepath.Join(t.TempDir(), "out.mp4")

	stats, err := Render(context.Background(), RenderRequest{
		SpecPath:   "essay.json",
		OutputPath: outPath,
		Codec:      "auto",
		Preset:     "p5",
		CQ:         &cq,
		Workers:    &workers,
		LUFS:       &lufs,
		RequireGPU: true,
		CudaDecode: true,
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if stats.Output != outPath || stats.Codec != "auto" || stats.TotalFrames != 300 || !stats.IsDiscreteGPU {
		t.Fatalf("Render() stats = %+v", stats)
	}

	rawArgs, err := os.ReadFile(argDump)
	if err != nil {
		t.Fatalf("read arg dump: %v", err)
	}
	gotArgs := strings.Split(strings.TrimSpace(string(rawArgs)), "\n")
	wantArgs := []string{
		"render", "essay.json", "-o", outPath, "--stats-json",
		"--codec", "auto",
		"--preset", "p5",
		"--cq", "19",
		"--workers", "4",
		"--lufs", "-14",
		"--require-gpu",
		"--cuda-decode",
	}
	if !slices.Equal(gotArgs, wantArgs) {
		t.Fatalf("render args = %v, want %v", gotArgs, wantArgs)
	}

	t.Run("bad stats JSON fails clearly", func(t *testing.T) {
		t.Setenv(fakeRenderModeEnv, "render-bad-json")
		if _, err := Render(context.Background(), RenderRequest{SpecPath: "essay.json", OutputPath: outPath}); err == nil || !strings.Contains(err.Error(), "parse cliphub-render stats JSON") {
			t.Fatalf("Render() error = %v", err)
		}
	})

	t.Run("process error surfaces stderr", func(t *testing.T) {
		t.Setenv(fakeRenderModeEnv, "fail")
		if _, err := Render(context.Background(), RenderRequest{SpecPath: "essay.json", OutputPath: outPath}); err == nil || !strings.Contains(err.Error(), "encoder open failed") {
			t.Fatalf("Render() error = %v", err)
		}
	})
}
