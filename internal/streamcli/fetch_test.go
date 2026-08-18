package streamcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/vodfetch"
)

type fakeVODFetcher struct {
	result   vodfetch.Result
	err      error
	calls    int
	gotURL   string
	gotDest  string
	gotCtxOK bool
}

func (f *fakeVODFetcher) Download(ctx context.Context, rawURL, destPath string) (vodfetch.Result, error) {
	f.calls++
	f.gotURL = rawURL
	f.gotDest = destPath
	_, f.gotCtxOK = ctx.Deadline()
	if f.err != nil {
		return vodfetch.Result{}, f.err
	}
	return f.result, nil
}

func TestRunStreamFetchDryRunValidatesWithoutDownloading(t *testing.T) {
	out := filepath.Join(t.TempDir(), "clip.mp4")
	fetcher := &fakeVODFetcher{}
	args := []string{
		"--url", "https://clips.twitch.tv/SomeClipSlug",
		"--out", out,
		"--dry-run",
		"--format", "json",
	}
	var stdout, stderr bytes.Buffer
	code := runStreamFetch(args, &stdout, &stderr, fetcher)
	if code != exitSuccess || stderr.Len() != 0 {
		t.Fatalf("code = %d, stderr = %q", code, stderr.String())
	}
	if fetcher.calls != 0 {
		t.Fatalf("Download called %d times, want 0", fetcher.calls)
	}
	var result streamFetchResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if !result.OK || !result.DryRun || result.Executed || result.Kind != "twitch_clip" {
		t.Fatalf("result = %#v", result)
	}
	if result.URL != "https://clips.twitch.tv/SomeClipSlug" {
		t.Fatalf("url = %q", result.URL)
	}
	if result.MaxBytes != vodfetch.DefaultMaxBytes {
		t.Fatalf("max_bytes = %d, want default", result.MaxBytes)
	}
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run created dest; stat err = %v", err)
	}
}

func TestRunStreamFetchTable(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "vod.mp4")
	tests := []struct {
		name       string
		args       []string
		fetcher    *fakeVODFetcher
		wantCode   int
		wantCalls  int
		wantSubstr string
	}{
		{
			name:     "missing flags",
			args:     []string{"--format", "json"},
			wantCode: exitInvalidArgs,
		},
		{
			name:     "rejected host",
			args:     []string{"--url", "https://example.com/watch", "--out", dest, "--format", "json"},
			wantCode: exitInvalidArgs,
		},
		{
			name:     "zero max-bytes",
			args:     []string{"--url", "https://www.twitch.tv/videos/123456789", "--out", dest, "--max-bytes", "0", "--format", "json"},
			wantCode: exitInvalidArgs,
		},
		{
			name:     "invalid timeout",
			args:     []string{"--url", "https://www.twitch.tv/videos/123456789", "--out", dest, "--timeout", "nope", "--format", "json"},
			wantCode: exitInvalidArgs,
		},
		{
			name: "download success",
			args: []string{"--url", "https://www.twitch.tv/videos/123456789", "--out", dest, "--max-bytes", "4096", "--format", "json"},
			fetcher: &fakeVODFetcher{result: vodfetch.Result{
				Path: dest, Bytes: 32, Title: "Mirage",
			}},
			wantCode:  exitSuccess,
			wantCalls: 1,
		},
		{
			name:       "download failure",
			args:       []string{"--url", "https://www.twitch.tv/videos/123456789", "--out", dest, "--format", "json"},
			fetcher:    &fakeVODFetcher{err: vodfetch.ErrNotFound},
			wantCode:   exitUnexpected,
			wantCalls:  1,
			wantSubstr: "source not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetcher := tt.fetcher
			if fetcher == nil {
				fetcher = &fakeVODFetcher{}
			}
			var stdout, stderr bytes.Buffer
			code := runStreamFetch(tt.args, &stdout, &stderr, fetcher)
			if code != tt.wantCode {
				t.Fatalf("code = %d, want %d; stdout=%s stderr=%s", code, tt.wantCode, stdout.String(), stderr.String())
			}
			if fetcher.calls != tt.wantCalls {
				t.Fatalf("calls = %d, want %d", fetcher.calls, tt.wantCalls)
			}
			if tt.wantSubstr != "" && !strings.Contains(stdout.String()+stderr.String(), tt.wantSubstr) {
				t.Fatalf("output %q missing %q", stdout.String()+stderr.String(), tt.wantSubstr)
			}
			if tt.wantCode == exitSuccess {
				var result streamFetchResult
				if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
					t.Fatalf("decode: %v\n%s", err, stdout.String())
				}
				if !result.OK || result.DryRun || !result.Executed || result.Bytes != 32 || result.Title != "Mirage" {
					t.Fatalf("result = %#v", result)
				}
				if !fetcher.gotCtxOK {
					t.Fatal("download context had no deadline")
				}
			}
		})
	}
}

func TestRunStreamUnknownRejectsFetchTypo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runStreamWithService([]string{"fetchx"}, &stdout, &stderr, &fakeStreamService{})
	if code != exitInvalidArgs {
		t.Fatalf("code = %d, want %d", code, exitInvalidArgs)
	}
	if !strings.Contains(stderr.String(), "unknown stream command") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
