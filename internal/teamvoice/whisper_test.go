package teamvoice

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/library"
)

func TestParseWhisperJSONKeepsSpokenLines(t *testing.T) {
	out := `{"result":{"language":"es"},"transcription":[
		{"offsets":{"from":1000,"to":3000},"text":"  stack   B  "},
		{"offsets":{"from":20000,"to":24000},"text":" Subtítulos realizados por la comunidad de Amara.org"},
		{"offsets":{"from":5000,"to":5000},"text":" "}]}`
	lines, err := parseWhisperJSON(strings.NewReader(out))
	if err != nil || len(lines) != 2 || lines[0].Text != "stack B" || lines[0].EndMS != 3000 {
		t.Fatalf("lines = %+v, %v", lines, err)
	}
	kept := keepSpoken(lines, []library.VoiceSpan{{StartMS: 900, EndMS: 2500}})
	if len(kept) != 1 || kept[0].Text != "stack B" {
		t.Fatalf("kept = %+v; want the phrase over silence dropped", kept)
	}
}

func TestReadProgressReportsPercentAndKeepsTheTail(t *testing.T) {
	var got []float64
	tail := readProgress(strings.NewReader("whisper_print_progress_callback: progress =  10%\nload failed\nprogress = 55%\n"), func(p float64) { got = append(got, p) })
	if len(got) != 2 || got[1] != 0.55 || tail != "load failed" {
		t.Fatalf("progress = %v, tail %q", got, tail)
	}
}

func TestFetchPinnedChecksSizeAndDigest(t *testing.T) {
	body := "whisper"
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
	if err := unzipTo(write("ok.zip", "Release/whisper-cli.exe", "Release/ggml.dll"), out); err != nil {
		t.Fatal(err)
	}
	if findFile(out, "WHISPER-CLI.EXE") == "" {
		t.Fatal("whisper-cli.exe was not extracted")
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
	cli, model := filepath.Join(dir, "whisper-cli.exe"), filepath.Join(dir, "model.bin")
	for _, p := range []string{cli, model} {
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if s := NewTranscriber(TranscriberOptions{CLI: cli, Model: model}).Status(); s.State != TranscriberFailed || s.ErrorCode != TranscriberStartFailed || !strings.Contains(s.Error, "Visual C++") {
		t.Fatalf("status of a whisper-cli that cannot start = %+v, want failed with the runtime hint", s)
	}
	// The test binary answers --help with exit 0, as a working whisper-cli does.
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if s := NewTranscriber(TranscriberOptions{CLI: self, Model: model}).Status(); s.State != TranscriberInstalled || s.DownloadBytes != Pinned.BinBytes+Pinned.ModelBytes {
		t.Fatalf("status with an override = %+v", s)
	}
}
