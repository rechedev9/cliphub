package streamcli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/rechedev9/cliphub/internal/capturetools"
	"github.com/rechedev9/cliphub/internal/vodfetch"
)

type vodFetcher interface {
	Download(ctx context.Context, rawURL, destPath string) (vodfetch.Result, error)
}

type streamFetchResult struct {
	OK       bool   `json:"ok"`
	DryRun   bool   `json:"dry_run"`
	Executed bool   `json:"executed"`
	URL      string `json:"url"`
	Kind     string `json:"kind"`
	Output   string `json:"output"`
	Bytes    int64  `json:"bytes,omitempty"`
	Title    string `json:"title,omitempty"`
	MaxBytes int64  `json:"max_bytes"`
}

func runStreamFetch(args []string, stdout, stderr io.Writer, fetch vodFetcher) int {
	if isSingleHelp(args) {
		fmt.Fprint(stdout, streamFetchUsage)
		return exitSuccess
	}
	fs := flag.NewFlagSet("stream fetch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	rawURL := fs.String("url", "", "Twitch or YouTube clip/VOD URL")
	out := fs.String("out", "", "destination MP4 path")
	maxBytes := fs.Int64("max-bytes", vodfetch.DefaultMaxBytes, "download size ceiling in bytes")
	ytdlp := fs.String("ytdlp", "", "yt-dlp executable; defaults to ZV_YTDLP_PATH or PATH")
	timeout := fs.String("timeout", "20m", "download timeout")
	format := fs.String("format", "text", "text or json")
	dryRun := fs.Bool("dry-run", false, "validate the URL and destination without downloading")
	if err := fs.Parse(args); err != nil {
		return writeStreamCommandError(args, stdout, stderr, err, streamFetchUsage)
	}
	if fs.NArg() != 0 {
		return writeStreamCommandError(args, stdout, stderr, fmt.Errorf("unexpected positional arg %q", fs.Arg(0)), streamFetchUsage)
	}
	if *rawURL == "" || *out == "" {
		return writeStreamCommandError(args, stdout, stderr, fmt.Errorf("--url and --out are required"), streamFetchUsage)
	}
	if *format != "text" && *format != "json" {
		return writeStreamCommandError(args, stdout, stderr, fmt.Errorf("unsupported format %q", *format), streamFetchUsage)
	}
	if *maxBytes <= 0 {
		return writeStreamCommandError(args, stdout, stderr, fmt.Errorf("--max-bytes must be positive"), streamFetchUsage)
	}
	downloadTimeout, err := time.ParseDuration(*timeout)
	if err != nil || downloadTimeout <= 0 {
		if err == nil {
			err = fmt.Errorf("must be positive")
		}
		return writeStreamCommandError(args, stdout, stderr, fmt.Errorf("invalid --timeout: %w", err), streamFetchUsage)
	}
	source, err := vodfetch.ValidateSource(*rawURL)
	if err != nil {
		return writeStreamCommandError(args, stdout, stderr, fmt.Errorf("invalid --url: %w", err), streamFetchUsage)
	}
	if err := rejectStreamOutputAliases(*out); err != nil {
		return writeStreamCommandError(args, stdout, stderr, err, streamFetchUsage)
	}
	absOut, err := filepath.Abs(*out)
	if err != nil {
		return writeStreamRuntimeError(args, stdout, stderr, fmt.Errorf("resolve destination: %w", err))
	}
	result := streamFetchResult{
		OK:       true,
		DryRun:   *dryRun,
		Executed: !*dryRun,
		URL:      source.PublicURL,
		Kind:     source.Kind.String(),
		Output:   absOut,
		MaxBytes: *maxBytes,
	}
	if *dryRun {
		return writeStreamFetchResult(args, stdout, stderr, *format, result)
	}
	if fetch == nil {
		binary := *ytdlp
		if binary == "" {
			paths, _ := capturetools.Detect(capturetools.FromEnvironment())
			binary = paths.Ytdlp
		}
		if binary == "" {
			return writeStreamRuntimeError(args, stdout, stderr, fmt.Errorf("yt-dlp is not configured; install it on PATH or set ZV_YTDLP_PATH"))
		}
		fetch = vodfetch.Fetcher{BinaryPath: binary, MaxBytes: *maxBytes}
	}
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()
	downloaded, err := fetch.Download(ctx, source.AcquisitionURL, absOut)
	if err != nil {
		return writeStreamRuntimeError(args, stdout, stderr, err)
	}
	result.Bytes = downloaded.Bytes
	result.Title = downloaded.Title
	result.Output = downloaded.Path
	return writeStreamFetchResult(args, stdout, stderr, *format, result)
}

func writeStreamFetchResult(args []string, stdout, stderr io.Writer, format string, result streamFetchResult) int {
	if format == "json" {
		if err := writeJSON(stdout, result); err != nil {
			fmt.Fprintf(stderr, "error: write stream fetch result: %v\n", err)
			return exitUnexpected
		}
		return exitSuccess
	}
	if result.DryRun {
		fmt.Fprintf(stdout, "valid stream fetch: %s -> %s (not downloaded)\n", result.URL, result.Output)
		return exitSuccess
	}
	fmt.Fprintf(stdout, "downloaded stream: %s (%d bytes)\n", result.Output, result.Bytes)
	return exitSuccess
}
