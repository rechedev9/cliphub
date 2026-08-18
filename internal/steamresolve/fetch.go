package steamresolve

import (
	"compress/bzip2"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	demoFetchTimeout = 3 * time.Minute
	maxRedirects     = 5
)

// ErrDemoHostRejected is returned when a resolved demo URL is not on Valve's replay CDN.
var ErrDemoHostRejected = errors.New("demo URL is not a Valve replay host")

// Fetcher downloads a resolved demo URL and unwraps .bz2 when needed.
type Fetcher struct {
	http *http.Client
}

// NewFetcher builds a fetcher that only follows redirects onto Valve replay hosts.
func NewFetcher(httpClient *http.Client) *Fetcher {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: demoFetchTimeout, CheckRedirect: restrictDemoRedirect}
	}
	return &Fetcher{http: httpClient}
}

// Open downloads demoURL and returns a reader of the (possibly decompressed)
// demo bytes plus a display file name. The caller must Close the reader.
func (f *Fetcher) Open(ctx context.Context, demoURL string, maxBytes int64) (io.ReadCloser, string, error) {
	if f == nil || f.http == nil {
		return nil, "", errors.New("steam demo fetcher is not configured")
	}
	if maxBytes < 1 {
		return nil, "", errors.New("demo size limit must be positive")
	}
	parsed, err := parseDemoURL(demoURL)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("demo request: %w", err)
	}
	res, err := f.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download demo: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		_ = res.Body.Close()
		return nil, "", fmt.Errorf("download demo: status %d", res.StatusCode)
	}
	name := displayDemoName(parsed.Path)
	limited := io.LimitReader(res.Body, maxBytes+1)
	if strings.HasSuffix(strings.ToLower(parsed.Path), ".bz2") {
		return &fetchedDemo{src: bzip2.NewReader(limited), closer: res.Body}, strings.TrimSuffix(name, ".bz2"), nil
	}
	return &fetchedDemo{src: limited, closer: res.Body}, name, nil
}

type fetchedDemo struct {
	src    io.Reader
	closer io.Closer
}

func (f *fetchedDemo) Read(p []byte) (int, error) { return f.src.Read(p) }

func (f *fetchedDemo) Close() error {
	if f.closer == nil {
		return nil
	}
	return f.closer.Close()
}

func parseDemoURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("demo URL: %w", err)
	}
	if err := allowDemoURL(parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func allowDemoURL(parsed *url.URL) error {
	if parsed.User != nil {
		return ErrDemoHostRejected
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ErrDemoHostRejected
	}
	host := strings.ToLower(parsed.Hostname())
	if !isValveReplayHost(host) {
		return fmt.Errorf("%w: %s", ErrDemoHostRejected, host)
	}
	return nil
}

func isValveReplayHost(host string) bool {
	return host == "valve.net" || strings.HasSuffix(host, ".valve.net")
}

func restrictDemoRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return errors.New("too many demo redirects")
	}
	return allowDemoURL(req.URL)
}

func displayDemoName(path string) string {
	trimmed := strings.TrimSpace(path)
	if i := strings.LastIndexAny(trimmed, "/\\"); i >= 0 {
		trimmed = trimmed[i+1:]
	}
	if trimmed == "" {
		return "match.dem"
	}
	return trimmed
}
