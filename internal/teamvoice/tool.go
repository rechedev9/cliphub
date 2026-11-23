package teamvoice

import (
	"context"
	"errors"
	"log"
	"maps"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Spec pins a local speech program and its model, fetched on request, as Studio pins HLAE.
type Spec struct {
	Version     string
	CPU         Build
	GPU         Build
	Exe         string
	ModelName   string
	ModelURL    string
	ModelSHA256 string
	ModelBytes  int64
}

type Build struct {
	Dir    string
	URL    string
	SHA256 string
	Bytes  int64
}

// Pinned is the transcriber: NVIDIA's NeMo-Speech.cpp v0.3.0 for Windows x64, its Vulkan build and the CPU build that runs
// where Vulkan cannot (ggml.dll links vulkan-1.dll), and Parakeet TDT 0.6b v3 at q8_0, about 700 MB, 25 languages.
var Pinned = Spec{
	Version: "0.3.0",
	CPU: Build{Dir: "0.3.0", URL: "https://github.com/NVIDIA/NeMo-Speech.cpp/releases/download/v0.3.0/nemo-speech-0.3.0-windows-x86_64-cpu.zip",
		SHA256: "4ed9740e361e4653f56c3dce7d1e81883a1c8b831c1bd7d01567fff1130b5b05", Bytes: 8_694_509},
	GPU: Build{Dir: "0.3.0-vulkan", URL: "https://github.com/NVIDIA/NeMo-Speech.cpp/releases/download/v0.3.0/nemo-speech-0.3.0-windows-x86_64-vulkan.zip",
		SHA256: "72b3b8b41bfe2be0320eff4e9d310594e4f99deefcde4a157799a039656963ce", Bytes: 22_417_032},
	Exe:         "nemo-speech.exe",
	ModelName:   "parakeet-tdt-0.6b-v3.q8_0.gguf",
	ModelURL:    "https://huggingface.co/nvidia/parakeet-tdt-0.6b-v3/resolve/541d1f99c6b0c3cd0b11a95167540bb8edefd82b/parakeet-tdt-0.6b-v3.q8_0.gguf",
	ModelSHA256: "e3880d0aaaaf2c308ea2c35016b2b895c423eb3fda924c1b463d1c19b7f4d32e",
	ModelBytes:  713_975_456,
}

// Transcriber states.
const (
	TranscriberMissing     = "not_installed"
	TranscriberInstalling  = "installing"
	TranscriberInstalled   = "installed"
	TranscriberFailed      = "failed"
	TranscriberUnsupported = "unsupported"
)

// TranscriberStatus is what the UI shows about a local speech program.
type TranscriberStatus struct {
	State         string `json:"state"`
	Version       string `json:"version"`
	Model         string `json:"model"`
	DownloadBytes int64  `json:"download_bytes"`
	DoneBytes     int64  `json:"done_bytes,omitempty"`
	ErrorCode     string `json:"error_code,omitempty"`
	Error         string `json:"error,omitempty"`
}

// Install failure codes the UI words on its own.
const (
	TranscriberRuntimeMissing = "runtime_missing"
	TranscriberStartFailed    = "start_failed"
	TranscriberDownloadFailed = "download_failed"
)

// installError is an install failure with the code the UI words and a Spanish message for MCP.
type installError struct {
	code, msg string
	err       error
}

func (e *installError) Error() string { return e.msg }
func (e *installError) Unwrap() error { return e.err }

// statusDLLNotFound is Windows' STATUS_DLL_NOT_FOUND: a build without the VC++ runtime.
const statusDLLNotFound = 0xc0000135

const vcRuntimeHint = "Instala Microsoft Visual C++ Redistributable 2015-2022 (x64) desde https://aka.ms/vs/17/release/vc_redist.x64.exe y vuelve a intentarlo."

// Decoder reads a recording as the 16 kHz mono s16le PCM the local programs take.
type Decoder interface {
	RawPCM(ctx context.Context, job PCMJob) error
}

// TranscriberOptions configures a local speech program; CLI and Model override the fetched install.
type TranscriberOptions struct {
	Dir    string
	Media  Decoder
	Client *http.Client
	CLI    string
	Model  string
}

// tool installs and checks one pinned program and model; no audio leaves the PC.
type tool struct {
	opts        TranscriberOptions
	spec        Spec
	mu          sync.Mutex
	installDone chan struct{}
	total, done int64
	lastErr     *installError
	smoked      map[string]*installError
	gpuFailed   bool
}

func newTool(opts TranscriberOptions, spec Spec) *tool {
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: 30 * time.Minute}
	}
	return &tool{opts: opts, spec: spec, smoked: map[string]*installError{}}
}

type programs struct{ cpu, gpu, model string }

func (t *tool) overridden() bool { return t.opts.CLI != "" && t.opts.Model != "" }

func (t *tool) buildDir(b Build) string { return filepath.Join(t.opts.Dir, b.Dir) }

func (t *tool) findExe(b Build) string {
	return findFile(filepath.Join(t.buildDir(b), "bin"), t.spec.Exe)
}

func (t *tool) modelPath() string { return filepath.Join(t.opts.Dir, "models", t.spec.ModelName) }

// paths returns the builds and model to run, empty when not installed; an override has no GPU build.
func (t *tool) paths() programs {
	if t.overridden() {
		return programs{cpu: t.opts.CLI, model: t.opts.Model}
	}
	if t.opts.Dir == "" {
		return programs{}
	}
	if _, err := os.Stat(t.modelPath()); err != nil {
		return programs{}
	}
	p := programs{cpu: t.findExe(t.spec.CPU), gpu: t.findExe(t.spec.GPU), model: t.modelPath()}
	if p.cpu == "" || p.gpu == "" {
		return programs{}
	}
	return p
}

func (t *tool) cpuBuild() programs {
	if t.overridden() || t.opts.Dir == "" {
		return programs{}
	}
	if _, err := os.Stat(t.modelPath()); err != nil {
		return programs{}
	}
	cpu := t.findExe(t.spec.CPU)
	if cpu == "" {
		return programs{}
	}
	return programs{cpu: cpu, model: t.modelPath()}
}

func (t *tool) missingBytes() int64 {
	if t.overridden() || t.opts.Dir == "" {
		return 0
	}
	var n int64
	for _, b := range []Build{t.spec.CPU, t.spec.GPU} {
		if t.findExe(b) == "" {
			n += b.Bytes
		}
	}
	if _, err := os.Stat(t.modelPath()); err != nil {
		n += t.spec.ModelBytes
	}
	return n
}

func (t *tool) supported() bool {
	return t.overridden() || (runtime.GOOS == "windows" && runtime.GOARCH == "amd64" && t.opts.Dir != "")
}

func (t *tool) Status() TranscriberStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := TranscriberStatus{Version: t.spec.Version, Model: t.spec.ModelName}
	cli := t.paths().cpu
	switch {
	case !t.supported():
		s.State = TranscriberUnsupported
	case t.installDone != nil:
		s.State, s.DownloadBytes, s.DoneBytes = TranscriberInstalling, t.total, t.done
	case cli != "" && t.smokeLocked(cli) == nil:
		s.State = TranscriberInstalled
	case cli != "":
		smoke := t.smokeLocked(cli)
		s.State, s.ErrorCode, s.Error = TranscriberFailed, smoke.code, smoke.msg
	case t.lastErr != nil:
		s.State, s.ErrorCode, s.Error, s.DownloadBytes = TranscriberFailed, t.lastErr.code, t.lastErr.msg, t.missingBytes()
	default:
		s.State, s.DownloadBytes = TranscriberMissing, t.missingBytes()
	}
	return s
}

// smokeLocked runs the program's --help once per binary, so a build that cannot start never reads as installed.
func (t *tool) smokeLocked(cli string) *installError {
	if err, ok := t.smoked[cli]; ok {
		return err
	}
	err := smokeTest(cli)
	t.smoked[cli] = err
	return err
}

func smokeTest(cli string) *installError {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// #nosec G204 -- the pinned program, run with a fixed flag.
	cmd := exec.CommandContext(ctx, cli, "--help")
	cmd.Dir = filepath.Dir(cli)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	log.Printf("teamvoice: %s smoke test failed: %v: %s", cli, err, lastChars(string(out), 400))
	var exit *exec.ExitError
	if errors.As(err, &exit) && uint32(exit.ExitCode()) == statusDLLNotFound {
		return &installError{code: TranscriberRuntimeMissing, err: err,
			msg: "La transcripción no arranca en este PC porque falta el runtime de Visual C++. " + vcRuntimeHint}
	}
	return &installError{code: TranscriberStartFailed, err: err,
		msg: "La transcripción no arranca en este PC; suele faltar el runtime de Visual C++. " + vcRuntimeHint}
}

func lastChars(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}

// ErrTranscriberUnsupported is a PC the pinned build does not run on.
var ErrTranscriberUnsupported = errors.New("teamvoice: local transcription runs on Windows x64 only")

// ErrTranscriberMissing is a run asked for before the install.
var ErrTranscriberMissing = errors.New("teamvoice: the local speech program is not installed")

// Install fetches in the background what the pinned builds and model lack, and checks the programs again.
func (t *tool) Install(ctx context.Context) error {
	if !t.supported() {
		return ErrTranscriberUnsupported
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.installDone != nil {
		return nil
	}
	t.lastErr, t.gpuFailed = nil, false
	clear(t.smoked)
	if t.paths().cpu != "" {
		return nil
	}
	done := make(chan struct{})
	t.installDone, t.total, t.done = done, t.missingBytes(), 0
	go t.installInBackground(ctx, done)
	return nil
}

func (t *tool) installInBackground(ctx context.Context, done chan struct{}) {
	defer close(done)
	err := t.install(ctx)
	smoked := map[string]*installError{}
	if err == nil {
		p := t.paths()
		for _, cli := range []string{p.cpu, p.gpu} {
			if cli != "" {
				smoked[cli] = smokeTest(cli)
			}
		}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.installDone = nil
	maps.Copy(t.smoked, smoked)
	if err != nil {
		log.Printf("teamvoice: install %s: %v", t.spec.Exe, err)
		t.lastErr = &installError{code: TranscriberDownloadFailed, err: err,
			msg: "No se pudo descargar la transcripción local; comprueba la conexión y vuelve a intentarlo."}
	}
}

func (t *tool) install(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(t.modelPath()), 0o750); err != nil {
		return err
	}
	var fetched int64
	progress := func(done int64) {
		t.mu.Lock()
		t.done = fetched + done
		t.mu.Unlock()
	}
	for _, b := range []Build{t.spec.CPU, t.spec.GPU} {
		if t.findExe(b) != "" {
			continue
		}
		if err := t.installBuild(ctx, b, progress); err != nil {
			return err
		}
		fetched += b.Bytes
	}
	if _, err := os.Stat(t.modelPath()); err == nil {
		return nil
	}
	return fetchPinned(ctx, t.opts.Client, download{URL: t.spec.ModelURL, SHA256: t.spec.ModelSHA256, Bytes: t.spec.ModelBytes, Dst: t.modelPath(), Progress: progress})
}

func (t *tool) installBuild(ctx context.Context, b Build, progress func(int64)) error {
	archive := filepath.Join(t.opts.Dir, strings.TrimSuffix(t.spec.Exe, ".exe")+"-"+b.Dir+".zip")
	if err := fetchPinned(ctx, t.opts.Client, download{URL: b.URL, SHA256: b.SHA256, Bytes: b.Bytes, Dst: archive, Progress: progress}); err != nil {
		return err
	}
	defer os.Remove(archive)
	staging := t.buildDir(b) + ".staging"
	_ = os.RemoveAll(staging)
	if err := unzipTo(archive, filepath.Join(staging, "bin")); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	if findFile(staging, t.spec.Exe) == "" {
		_ = os.RemoveAll(staging)
		return errors.New("the archive has no " + t.spec.Exe)
	}
	_ = os.RemoveAll(t.buildDir(b))
	return os.Rename(staging, t.buildDir(b))
}

// ready returns the installed builds and model after their smoke tests, once an install in progress ends.
func (t *tool) ready(ctx context.Context) (programs, error) {
	t.mu.Lock()
	installing := t.installDone
	t.mu.Unlock()
	if installing != nil {
		select {
		case <-installing:
		case <-ctx.Done():
			return programs{}, ctx.Err()
		}
	}
	p := t.paths()
	if p.cpu == "" {
		p = t.cpuBuild()
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if p.cpu == "" && t.lastErr != nil {
		return programs{}, t.lastErr
	}
	if p.cpu == "" {
		return programs{}, ErrTranscriberMissing
	}
	if smoke := t.smokeLocked(p.cpu); smoke != nil {
		return programs{}, smoke
	}
	if p.gpu != "" && t.smokeLocked(p.gpu) != nil {
		p.gpu = ""
	}
	return p, nil
}

func (t *tool) gpuUsable() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return !t.gpuFailed
}

func (t *tool) dropGPU() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.gpuFailed = true
}

// threads leaves one core to Studio and the game.
func threads() string {
	return strconv.Itoa(max(1, min(8, runtime.NumCPU()-1)))
}
