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
	if s := NewTranscriber(TranscriberOptions{CLI: self, Model: model}).Status(); s.State != TranscriberInstalled || s.DownloadBytes != Pinned.BinBytes+Pinned.ModelBytes {
		t.Fatalf("status with an override = %+v", s)
	}
}
