package teamvoice

import (
	"context"
	"errors"
	"log"
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
	BinURL      string
	BinSHA256   string
	BinBytes    int64
	Exe         string
	ModelName   string
	ModelURL    string
	ModelSHA256 string
	ModelBytes  int64
}

// Pinned is the transcriber: NVIDIA's NeMo-Speech.cpp v0.3.0 (CPU build for Windows x64, with
// its own VC++ runtime) and Parakeet TDT 0.6b v3 at q8_0, about 700 MB covering 25 European languages.
var Pinned = Spec{
	Version:     "0.3.0",
	BinURL:      "https://github.com/NVIDIA/NeMo-Speech.cpp/releases/download/v0.3.0/nemo-speech-0.3.0-windows-x86_64-cpu.zip",
	BinSHA256:   "4ed9740e361e4653f56c3dce7d1e81883a1c8b831c1bd7d01567fff1130b5b05",
	BinBytes:    8_694_509,
	Exe:         "nemo-speech.exe",
	ModelName:   "parakeet-tdt-0.6b-v3.q8_0.gguf",
	ModelURL:    "https://huggingface.co/nvidia/parakeet-tdt-0.6b-v3/resolve/541d1f99c6b0c3cd0b11a95167540bb8edefd82b/parakeet-tdt-0.6b-v3.q8_0.gguf",
	ModelSHA256: "e3880d0aaaaf2c308ea2c35016b2b895c423eb3fda924c1b463d1c19b7f4d32e",
	ModelBytes:  713_975_456,
}

// PinnedTranslator is the optional English translation: whisper.cpp v1.9.5 (CPU build for
// Windows x64) with the multilingual medium model at q5_0, run in translate mode.
var PinnedTranslator = Spec{
	Version:     "1.9.5",
	BinURL:      "https://github.com/ggml-org/whisper.cpp/releases/download/b5454/whisper-bin-x64.zip",
	BinSHA256:   "6ba69e3482d7826214f90a6a9c84ca07782aec1e1d0c6a7c30c994fd5d816ccb",
	BinBytes:    8_928_640,
	Exe:         "whisper-cli.exe",
	ModelName:   "ggml-medium-q5_0.bin",
	ModelURL:    "https://huggingface.co/ggerganov/whisper.cpp/resolve/5359861c739e955e79d9a303bcbc70fb988958b1/ggml-medium-q5_0.bin",
	ModelSHA256: "19fea4b380c3a618ec4723c3eef2eb785ffba0d0538cf43f8f235e7b3b34220f",
	ModelBytes:  539_212_467,
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
	RawPCM(ctx context.Context, src, dst string) error
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
	opts TranscriberOptions
	spec Spec
	// what names the feature in Spanish messages: "La transcripción" or "La traducción".
	what       string
	mu         sync.Mutex
	installing bool
	done       int64
	lastErr    *installError
	// checked is the program that last ran its smoke test; checkErr is that test's result.
	checked  string
	checkErr *installError
}

func newTool(opts TranscriberOptions, spec Spec, what string) *tool {
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: 30 * time.Minute}
	}
	return &tool{opts: opts, spec: spec, what: what}
}

func (t *tool) versionDir() string { return filepath.Join(t.opts.Dir, t.spec.Version) }

// paths returns the program and model to run, empty when not installed.
func (t *tool) paths() (string, string) {
	if t.opts.CLI != "" && t.opts.Model != "" {
		return t.opts.CLI, t.opts.Model
	}
	if t.opts.Dir == "" {
		return "", ""
	}
	model := filepath.Join(t.opts.Dir, "models", t.spec.ModelName)
	if _, err := os.Stat(model); err != nil {
		return "", ""
	}
	cli := findFile(filepath.Join(t.versionDir(), "bin"), t.spec.Exe)
	if cli == "" {
		return "", ""
	}
	return cli, model
}

func (t *tool) supported() bool {
	return (t.opts.CLI != "" && t.opts.Model != "") || (runtime.GOOS == "windows" && runtime.GOARCH == "amd64" && t.opts.Dir != "")
}

func (t *tool) Status() TranscriberStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := TranscriberStatus{Version: t.spec.Version, Model: t.spec.ModelName, DownloadBytes: t.spec.BinBytes + t.spec.ModelBytes}
	cli, _ := t.paths()
	switch {
	case !t.supported():
		s.State = TranscriberUnsupported
	case t.installing:
		s.State, s.DoneBytes = TranscriberInstalling, t.done
	case cli != "" && t.smokeLocked(cli) == nil:
		s.State = TranscriberInstalled
	case cli != "":
		s.State, s.ErrorCode, s.Error = TranscriberFailed, t.checkErr.code, t.checkErr.msg
	case t.lastErr != nil:
		s.State, s.ErrorCode, s.Error = TranscriberFailed, t.lastErr.code, t.lastErr.msg
	default:
		s.State = TranscriberMissing
	}
	return s
}

// smokeLocked runs the program's --help once per binary, so a build that cannot start never reads as installed.
func (t *tool) smokeLocked(cli string) *installError {
	if t.checked == cli {
		return t.checkErr
	}
	t.checked, t.checkErr = cli, smokeTest(cli, t.what)
	return t.checkErr
}

func smokeTest(cli, what string) *installError {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// #nosec G204 -- the pinned program, run with a fixed flag.
	cmd := exec.CommandContext(ctx, cli, "--help")
	cmd.Dir = filepath.Dir(cli)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	log.Printf("teamvoice: %s smoke test failed: %v: %s", filepath.Base(cli), err, lastChars(string(out), 400))
	var exit *exec.ExitError
	if errors.As(err, &exit) && uint32(exit.ExitCode()) == statusDLLNotFound {
		return &installError{code: TranscriberRuntimeMissing, err: err,
			msg: what + " no arranca en este PC porque falta el runtime de Visual C++. " + vcRuntimeHint}
	}
	return &installError{code: TranscriberStartFailed, err: err,
		msg: what + " no arranca en este PC; suele faltar el runtime de Visual C++. " + vcRuntimeHint}
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

// Install fetches the pinned build and model in the background; Status follows it.
func (t *tool) Install(ctx context.Context) error {
	if !t.supported() {
		return ErrTranscriberUnsupported
	}
	t.mu.Lock()
	if t.installing {
		t.mu.Unlock()
		return nil
	}
	t.installing, t.done, t.lastErr, t.checked, t.checkErr = true, 0, nil, "", nil
	t.mu.Unlock()
	go func() {
		err := t.install(ctx)
		cli, _ := t.paths()
		var smoke *installError
		if err == nil && cli != "" {
			smoke = smokeTest(cli, t.what)
		}
		t.mu.Lock()
		defer t.mu.Unlock()
		t.installing = false
		if err == nil && cli != "" {
			t.checked, t.checkErr = cli, smoke
		}
		if err != nil {
			log.Printf("teamvoice: install %s: %v", t.spec.Exe, err)
			t.lastErr = &installError{code: TranscriberDownloadFailed, err: err,
				msg: "No se pudo descargar " + strings.ToLower(t.what) + " local; comprueba la conexión y vuelve a intentarlo."}
		}
	}()
	return nil
}

func (t *tool) install(ctx context.Context) error {
	models := filepath.Join(t.opts.Dir, "models")
	if err := os.MkdirAll(models, 0o750); err != nil {
		return err
	}
	progress := func(base int64) func(int64) {
		return func(done int64) {
			t.mu.Lock()
			t.done = base + done
			t.mu.Unlock()
		}
	}
	if cli := findFile(filepath.Join(t.versionDir(), "bin"), t.spec.Exe); cli == "" {
		archive := filepath.Join(t.opts.Dir, strings.TrimSuffix(t.spec.Exe, ".exe")+"-"+t.spec.Version+".zip")
		if err := fetchPinned(ctx, t.opts.Client, download{URL: t.spec.BinURL, SHA256: t.spec.BinSHA256, Bytes: t.spec.BinBytes, Dst: archive, Progress: progress(0)}); err != nil {
			return err
		}
		defer os.Remove(archive)
		staging := t.versionDir() + ".staging"
		_ = os.RemoveAll(staging)
		if err := unzipTo(archive, filepath.Join(staging, "bin")); err != nil {
			_ = os.RemoveAll(staging)
			return err
		}
		if findFile(staging, t.spec.Exe) == "" {
			_ = os.RemoveAll(staging)
			return errors.New("the archive has no " + t.spec.Exe)
		}
		_ = os.RemoveAll(t.versionDir())
		if err := os.Rename(staging, t.versionDir()); err != nil {
			return err
		}
	}
	model := filepath.Join(models, t.spec.ModelName)
	if _, err := os.Stat(model); err == nil {
		return nil
	}
	return fetchPinned(ctx, t.opts.Client, download{URL: t.spec.ModelURL, SHA256: t.spec.ModelSHA256, Bytes: t.spec.ModelBytes, Dst: model, Progress: progress(t.spec.BinBytes)})
}

// ready returns the installed program and model, after its smoke test.
func (t *tool) ready() (string, string, error) {
	cli, model := t.paths()
	if cli == "" {
		return "", "", ErrTranscriberMissing
	}
	t.mu.Lock()
	smoke := t.smokeLocked(cli)
	t.mu.Unlock()
	if smoke != nil {
		return "", "", smoke
	}
	return cli, model, nil
}

// threads leaves one core to Studio and the game.
func threads() string {
	return strconv.Itoa(max(1, min(8, runtime.NumCPU()-1)))
}
