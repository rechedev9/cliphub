package teamvoice

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rechedev9/cliphub/internal/library"
)

// Spec pins the whisper.cpp build and model Studio fetches on request, as it pins HLAE.
type Spec struct {
	Version     string
	BinURL      string
	BinSHA256   string
	BinBytes    int64
	ModelName   string
	ModelURL    string
	ModelSHA256 string
	ModelBytes  int64
}

// Pinned is whisper.cpp v1.9.5 (CPU build for Windows x64) with the multilingual small model,
// quantized to q5_1: about 200 MB that transcribes Spanish and English comms on the CPU.
var Pinned = Spec{
	Version:     "1.9.5",
	BinURL:      "https://github.com/ggml-org/whisper.cpp/releases/download/b5454/whisper-bin-x64.zip",
	BinSHA256:   "6ba69e3482d7826214f90a6a9c84ca07782aec1e1d0c6a7c30c994fd5d816ccb",
	BinBytes:    8_928_640,
	ModelName:   "ggml-small-q5_1.bin",
	ModelURL:    "https://huggingface.co/ggerganov/whisper.cpp/resolve/5359861c739e955e79d9a303bcbc70fb988958b1/ggml-small-q5_1.bin",
	ModelSHA256: "ae85e4a935d7a567bd102fe55afc16bb595bdb618e11b2fc7591bc08120411bb",
	ModelBytes:  190_085_487,
}

// Transcriber states.
const (
	TranscriberMissing     = "not_installed"
	TranscriberInstalling  = "installing"
	TranscriberInstalled   = "installed"
	TranscriberFailed      = "failed"
	TranscriberUnsupported = "unsupported"
)

// Languages a transcription may ask for; auto lets whisper guess from the first seconds.
var Languages = []string{"es", "en", "pt", "fr", "de", "auto"}

// TranscriberStatus is what the UI shows about local transcription.
type TranscriberStatus struct {
	State         string `json:"state"`
	Version       string `json:"version"`
	Model         string `json:"model"`
	DownloadBytes int64  `json:"download_bytes"`
	DoneBytes     int64  `json:"done_bytes,omitempty"`
	Error         string `json:"error,omitempty"`
}

// TranscriberOptions configures a Transcriber; CLI and Model override the fetched install.
type TranscriberOptions struct {
	Dir    string
	Media  Media
	Client *http.Client
	CLI    string
	Model  string
}

// Transcriber runs whisper.cpp on this PC; no audio leaves it.
type Transcriber struct {
	opts       TranscriberOptions
	spec       Spec
	mu         sync.Mutex
	installing bool
	done       int64
	lastErr    string
}

func NewTranscriber(opts TranscriberOptions) *Transcriber {
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: 30 * time.Minute}
	}
	return &Transcriber{opts: opts, spec: Pinned}
}

func (t *Transcriber) versionDir() string { return filepath.Join(t.opts.Dir, t.spec.Version) }

// paths returns the whisper-cli and model to run, empty when not installed.
func (t *Transcriber) paths() (string, string) {
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
	cli := findFile(filepath.Join(t.versionDir(), "bin"), "whisper-cli.exe")
	if cli == "" {
		return "", ""
	}
	return cli, model
}

func (t *Transcriber) supported() bool {
	return (t.opts.CLI != "" && t.opts.Model != "") || (runtime.GOOS == "windows" && runtime.GOARCH == "amd64" && t.opts.Dir != "")
}

func (t *Transcriber) Status() TranscriberStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := TranscriberStatus{Version: t.spec.Version, Model: t.spec.ModelName, DownloadBytes: t.spec.BinBytes + t.spec.ModelBytes}
	switch cli, _ := t.paths(); {
	case !t.supported():
		s.State = TranscriberUnsupported
	case t.installing:
		s.State, s.DoneBytes = TranscriberInstalling, t.done
	case cli != "":
		s.State = TranscriberInstalled
	case t.lastErr != "":
		s.State, s.Error = TranscriberFailed, t.lastErr
	default:
		s.State = TranscriberMissing
	}
	return s
}

// ErrTranscriberUnsupported is a PC the pinned build does not run on.
var ErrTranscriberUnsupported = errors.New("teamvoice: local transcription runs on Windows x64 only")

// Install fetches the pinned build and model in the background; Status follows it.
func (t *Transcriber) Install(ctx context.Context) error {
	if !t.supported() {
		return ErrTranscriberUnsupported
	}
	t.mu.Lock()
	if t.installing {
		t.mu.Unlock()
		return nil
	}
	t.installing, t.done, t.lastErr = true, 0, ""
	t.mu.Unlock()
	go func() {
		err := t.install(ctx)
		t.mu.Lock()
		defer t.mu.Unlock()
		t.installing = false
		if err != nil {
			t.lastErr = err.Error()
		}
	}()
	return nil
}

func (t *Transcriber) install(ctx context.Context) error {
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
	if cli := findFile(filepath.Join(t.versionDir(), "bin"), "whisper-cli.exe"); cli == "" {
		archive := filepath.Join(t.opts.Dir, "whisper-bin-x64-"+t.spec.Version+".zip")
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
		if findFile(staging, "whisper-cli.exe") == "" {
			_ = os.RemoveAll(staging)
			return errors.New("the whisper.cpp archive has no whisper-cli.exe")
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

// ErrTranscriberMissing is a transcription asked for before the install.
var ErrTranscriberMissing = errors.New("teamvoice: local transcription is not installed")

// TranscribeJob is one track to transcribe; Speech drops phrases whisper invents over silence.
type TranscribeJob struct {
	Audio    string
	Language string
	Speech   []library.VoiceSpan
	Progress func(share float64)
}

// Line is one transcribed phrase in audio time.
type Line struct {
	StartMS int64
	EndMS   int64
	Text    string
}

// Transcribe runs whisper-cli over one track and returns its phrases.
func (t *Transcriber) Transcribe(ctx context.Context, job TranscribeJob) ([]Line, error) {
	cli, model := t.paths()
	if cli == "" {
		return nil, ErrTranscriberMissing
	}
	dir, err := os.MkdirTemp("", "cliphub-voice-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	wav := filepath.Join(dir, "track.wav")
	if err := t.opts.Media.WAV(ctx, job.Audio, wav); err != nil {
		return nil, err
	}
	base := filepath.Join(dir, "track")
	threads := strconv.Itoa(max(1, min(8, runtime.NumCPU()-1)))
	// #nosec G204 -- the pinned whisper-cli receives generated local paths and a checked language code.
	cmd := exec.CommandContext(ctx, cli, "-m", model, "-f", wav, "-l", job.Language, "-t", threads, "-oj", "-of", base, "-pp")
	cmd.Dir = filepath.Dir(cli)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	tail := readProgress(stderr, job.Progress)
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("whisper-cli: %w: %s", err, tail)
	}
	f, err := os.Open(base + ".json")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	lines, err := parseWhisperJSON(f)
	if err != nil {
		return nil, err
	}
	return keepSpoken(lines, job.Speech), nil
}

var progressPattern = regexp.MustCompile(`progress\s*=\s*(\d+)%`)

// readProgress reports whisper's percent lines and keeps the last other lines for an error.
func readProgress(r io.Reader, report func(float64)) string {
	sc := bufio.NewScanner(r)
	var tail []string
	for sc.Scan() {
		line := sc.Text()
		if m := progressPattern.FindStringSubmatch(line); m != nil {
			if pct, err := strconv.Atoi(m[1]); err == nil && report != nil {
				report(float64(pct) / 100)
			}
			continue
		}
		tail = append(tail, line)
		if len(tail) > 5 {
			tail = tail[1:]
		}
	}
	return strings.Join(tail, " | ")
}

// parseWhisperJSON reads whisper-cli's -oj output.
func parseWhisperJSON(r io.Reader) ([]Line, error) {
	var doc struct {
		Transcription []struct {
			Offsets struct {
				From int64 `json:"from"`
				To   int64 `json:"to"`
			} `json:"offsets"`
			Text string `json:"text"`
		} `json:"transcription"`
	}
	if err := json.NewDecoder(io.LimitReader(r, 64<<20)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("read whisper output: %w", err)
	}
	out := []Line{}
	for _, s := range doc.Transcription {
		text := strings.Join(strings.Fields(s.Text), " ")
		if text == "" || s.Offsets.To < s.Offsets.From {
			continue
		}
		if runes := []rune(text); len(runes) > library.MaxVoiceLineRunes {
			text = string(runes[:library.MaxVoiceLineRunes])
		}
		out = append(out, Line{StartMS: s.Offsets.From, EndMS: s.Offsets.To, Text: text})
	}
	return out, nil
}

// keepSpoken drops lines under a third covered by detected speech: whisper fills silence with stock phrases.
func keepSpoken(lines []Line, speech []library.VoiceSpan) []Line {
	if speech == nil {
		return lines
	}
	out := []Line{}
	for _, l := range lines {
		var covered int64
		for _, s := range speech {
			covered += max(0, min(l.EndMS, s.EndMS)-max(l.StartMS, s.StartMS))
		}
		if length := l.EndMS - l.StartMS; length <= 0 || covered*3 >= length {
			out = append(out, l)
		}
	}
	return out
}
