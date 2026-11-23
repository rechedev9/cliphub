package teamvoice

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/library"
)

func TestKeepSpokenDropsPhrasesOverSilence(t *testing.T) {
	lines := []Line{{StartMS: 1000, EndMS: 3000, Text: "stack B"}, {StartMS: 20000, EndMS: 24000, Text: "Subtítulos realizados por la comunidad de Amara.org"}}
	kept := keepSpoken(lines, []library.VoiceSpan{{StartMS: 900, EndMS: 2500}})
	if len(kept) != 1 || kept[0].Text != "stack B" {
		t.Fatalf("kept = %+v; want the phrase over silence dropped", kept)
	}
}

func TestFetchPinnedChecksSizeAndDigest(t *testing.T) {
	body := "parakeet"
	sum := sha256.Sum256([]byte(body))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
	defer srv.Close()
	dir := t.TempDir()
	good := download{URL: srv.URL, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(body)), Dst: filepath.Join(dir, "ok.bin")}
	if err := fetchPinned(t.Context(), srv.Client(), good); err != nil {
		t.Fatal(err)
	}
	bad := good
	bad.Dst, bad.SHA256 = filepath.Join(dir, "bad.bin"), strings.Repeat("0", 64)
	if err := fetchPinned(t.Context(), srv.Client(), bad); err == nil {
		t.Fatal("a wrong digest was accepted")
	}
	if _, err := os.Stat(bad.Dst); !os.IsNotExist(err) {
		t.Fatalf("a refused download left %s", bad.Dst)
	}
}

func TestUnzipRefusesPathsOutsideTheFolder(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, files ...string) string {
		path := filepath.Join(dir, name)
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		zw := zip.NewWriter(f)
		for _, file := range files {
			w, err := zw.Create(file)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte("x"))
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
		return path
	}
	out := filepath.Join(dir, "out")
	if err := unzipTo(write("ok.zip", "Release/nemo-speech.exe", "Release/ggml.dll"), out); err != nil {
		t.Fatal(err)
	}
	if findFile(out, "NEMO-SPEECH.EXE") == "" {
		t.Fatal("nemo-speech.exe was not extracted")
	}
	if err := unzipTo(write("evil.zip", "../evil.exe"), filepath.Join(dir, "out2")); err == nil {
		t.Fatal("an archive path leaving the folder was extracted")
	}
}

func TestTranscriberStatusFollowsTheInstall(t *testing.T) {
	tr := NewTranscriber(TranscriberOptions{})
	if s := tr.Status(); s.State != TranscriberUnsupported && s.State != TranscriberMissing {
		t.Fatalf("state without a folder = %q", s.State)
	}
	dir := t.TempDir()
	cli, model := filepath.Join(dir, "nemo-speech.exe"), filepath.Join(dir, "model.bin")
	for _, p := range []string{cli, model} {
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if s := NewTranscriber(TranscriberOptions{CLI: cli, Model: model}).Status(); s.State != TranscriberFailed || s.ErrorCode != TranscriberStartFailed || !strings.Contains(s.Error, "Visual C++") {
		t.Fatalf("status of a nemo-speech that cannot start = %+v, want failed with the runtime hint", s)
	}
	// The test binary answers --help with exit 0, as a working nemo-speech does.
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if s := NewTranscriber(TranscriberOptions{CLI: self, Model: model}).Status(); s.State != TranscriberInstalled || s.DownloadBytes != 0 {
		t.Fatalf("status with an override = %+v, want installed with nothing to fetch", s)
	}
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

type requestLog struct {
	mu    sync.Mutex
	paths []string
}

func (l *requestLog) add(path string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.paths = append(l.paths, path)
}

func (l *requestLog) list() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.paths)
}

func skipUnlessWindowsX64(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		t.Skip("the pinned builds run on Windows x64 only")
	}
}

func pinnedServer(t *testing.T, release <-chan struct{}) (Spec, *requestLog, *http.Client) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	exe, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: "Release/nemo-speech.exe", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(exe); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	model := []byte("parakeet weights")
	requests := &requestLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.add(r.URL.Path)
		switch r.URL.Path {
		case "/cpu.zip", "/gpu.zip":
			_, _ = w.Write(archive.Bytes())
		case "/model.gguf":
			<-release
			_, _ = w.Write(model)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	build := func(dir, path string) Build {
		return Build{Dir: dir, URL: srv.URL + path, SHA256: sha256Hex(archive.Bytes()), Bytes: int64(archive.Len())}
	}
	spec := Spec{Version: "9.9.9", CPU: build("9.9.9", "/cpu.zip"), GPU: build("9.9.9-vulkan", "/gpu.zip"), Exe: "nemo-speech.exe",
		ModelName: "model.gguf", ModelURL: srv.URL + "/model.gguf", ModelSHA256: sha256Hex(model), ModelBytes: int64(len(model))}
	return spec, requests, srv.Client()
}

type pinnedInstall struct {
	tr       *Transcriber
	spec     Spec
	requests *requestLog
	dir      string
}

func installPinned(t *testing.T, media Decoder) pinnedInstall {
	t.Helper()
	release := make(chan struct{})
	close(release)
	spec, requests, client := pinnedServer(t, release)
	dir := t.TempDir()
	tr := &Transcriber{newTool(TranscriberOptions{Dir: dir, Media: media, Client: client}, spec)}
	if err := tr.Install(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.ready(t.Context()); err != nil {
		t.Fatal(err)
	}
	return pinnedInstall{tr: tr, spec: spec, requests: requests, dir: dir}
}

func TestTranscribeWaitsForTheInstallItNeeds(t *testing.T) {
	skipUnlessWindowsX64(t)
	release := make(chan struct{})
	spec, requests, client := pinnedServer(t, release)
	speech := []library.VoiceSpan{{StartMS: 1000, EndMS: 2600}}
	words, _ := json.Marshal([][]fakeWord{{{"Rotad", 0.25, 0.6}}})
	t.Setenv(fakeNemoEnv, string(words))
	tr := &Transcriber{newTool(TranscriberOptions{Dir: t.TempDir(), Media: fakeDecoder(pcm(4_000, speech)), Client: client}, spec)}
	total := spec.CPU.Bytes + spec.GPU.Bytes + spec.ModelBytes
	if s := tr.Status(); s.State != TranscriberMissing || s.DownloadBytes != total {
		t.Fatalf("status before any install = %+v, want not_installed with %d bytes to fetch", s, total)
	}
	if err := tr.Install(t.Context()); err != nil {
		t.Fatal(err)
	}
	if s := tr.Status(); s.State != TranscriberInstalling || s.DownloadBytes != total {
		t.Fatalf("status while the model downloads = %+v", s)
	}
	type result struct {
		lines []Line
		err   error
	}
	got := make(chan result, 1)
	go func() {
		lines, err := tr.Transcribe(t.Context(), TranscribeJob{Audio: "a.ogg", Speech: speech})
		got <- result{lines, err}
	}()
	select {
	case r := <-got:
		t.Fatalf("a transcription ran before the install ended: %+v", r)
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	select {
	case r := <-got:
		if r.err != nil || len(r.lines) != 1 || r.lines[0].Text != "Rotad" {
			t.Fatalf("transcription after the install = %+v", r)
		}
	case <-time.After(time.Minute):
		t.Fatal("the transcription never ran after the install")
	}
	if s := tr.Status(); s.State != TranscriberInstalled || s.DownloadBytes != 0 {
		t.Fatalf("status after the install = %+v", s)
	}
	before := len(requests.list())
	if err := tr.Install(t.Context()); err != nil {
		t.Fatal(err)
	}
	if s, after := tr.Status(), requests.list(); s.State != TranscriberInstalled || len(after) != before {
		t.Fatalf("an install with the builds and model in place = %q after requests %q; want nothing downloaded", s.State, after[before:])
	}
}

func TestAnInstallWithTheCPUBuildFetchesOnlyTheVulkanBuild(t *testing.T) {
	skipUnlessWindowsX64(t)
	in := installPinned(t, nil)
	if err := os.RemoveAll(filepath.Join(in.dir, in.spec.GPU.Dir)); err != nil {
		t.Fatal(err)
	}
	if s := in.tr.Status(); s.State != TranscriberMissing || s.DownloadBytes != in.spec.GPU.Bytes {
		t.Fatalf("status without the Vulkan build = %+v, want not_installed with only its %d bytes to fetch", s, in.spec.GPU.Bytes)
	}
	before := len(in.requests.list())
	if err := in.tr.Install(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := in.tr.ready(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := in.requests.list()[before:]; !slices.Equal(got, []string{"/gpu.zip"}) {
		t.Fatalf("requests of the second install = %q, want only the Vulkan build", got)
	}
	if s := in.tr.Status(); s.State != TranscriberInstalled {
		t.Fatalf("status after fetching the Vulkan build = %+v", s)
	}
}

func TestTranscribeKeepsTheCPUBuildWhenTheVulkanDownloadFails(t *testing.T) {
	skipUnlessWindowsX64(t)
	speech := []library.VoiceSpan{{StartMS: 1000, EndMS: 2600}}
	words, _ := json.Marshal([][]fakeWord{{{"Rotad", 0.25, 0.6}}})
	t.Setenv(fakeNemoEnv, string(words))
	runs := filepath.Join(t.TempDir(), "runs.txt")
	t.Setenv(fakeNemoRunsEnv, runs)
	in := installPinned(t, fakeDecoder(pcm(4_000, speech)))
	if err := os.RemoveAll(filepath.Join(in.dir, in.spec.GPU.Dir)); err != nil {
		t.Fatal(err)
	}
	in.tr.spec.GPU.SHA256 = strings.Repeat("0", 64)
	if err := in.tr.Install(t.Context()); err != nil {
		t.Fatal(err)
	}
	lines, err := in.tr.Transcribe(t.Context(), TranscribeJob{Audio: "a.ogg", Speech: speech})
	if err != nil || len(lines) != 1 || lines[0].Text != "Rotad" {
		t.Fatalf("transcription without the Vulkan build = %+v, %v; want the CPU build to transcribe", lines, err)
	}
	raw, err := os.ReadFile(runs)
	if err != nil {
		t.Fatal(err)
	}
	if used := strings.TrimSpace(string(raw)); strings.Count(used, "\n") != 0 || !strings.HasSuffix(used, " cpu") {
		t.Fatalf("runs = %q, want one run on the CPU", used)
	}
}

func TestTranscribeFallsBackToTheCPUBuildOnceVulkanFails(t *testing.T) {
	skipUnlessWindowsX64(t)
	speech := []library.VoiceSpan{{StartMS: 1000, EndMS: 2600}}
	words, _ := json.Marshal([][]fakeWord{{{"Rotad", 0.25, 0.6}}})
	t.Setenv(fakeNemoEnv, string(words))
	runs := filepath.Join(t.TempDir(), "runs.txt")
	t.Setenv(fakeNemoRunsEnv, runs)
	in := installPinned(t, fakeDecoder(pcm(4_000, speech)))
	build := func(exe string) string {
		for _, dir := range []string{in.spec.GPU.Dir, in.spec.CPU.Dir} {
			if strings.Contains(filepath.ToSlash(exe), "/"+dir+"/") {
				return dir
			}
		}
		return exe
	}
	transcribe := func() []string {
		t.Helper()
		_ = os.Remove(runs)
		lines, err := in.tr.Transcribe(t.Context(), TranscribeJob{Audio: "a.ogg", Speech: speech})
		if err != nil || len(lines) != 1 || lines[0].Text != "Rotad" {
			t.Fatalf("transcription = %+v, %v", lines, err)
		}
		raw, err := os.ReadFile(runs)
		if err != nil {
			t.Fatal(err)
		}
		var used []string
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
			exe, device, _ := strings.Cut(line, " ")
			used = append(used, build(exe)+" "+device)
		}
		return used
	}
	gpu, cpu := in.spec.GPU.Dir+" vulkan", in.spec.CPU.Dir+" cpu"
	if used := transcribe(); !slices.Equal(used, []string{gpu}) {
		t.Fatalf("runs with a working Vulkan = %q, want the Vulkan build alone", used)
	}
	t.Setenv(fakeNemoVulkanEnv, "fail")
	if used := transcribe(); !slices.Equal(used, []string{gpu, cpu}) {
		t.Fatalf("runs when Vulkan fails = %q, want the Vulkan build, then the CPU build", used)
	}
	if used := transcribe(); !slices.Equal(used, []string{cpu}) {
		t.Fatalf("runs after Vulkan failed once = %q, want the CPU build alone", used)
	}
}
