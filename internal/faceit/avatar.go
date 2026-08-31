package faceit

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	overlayAvatarMaxBytes = 256 * 1024
	overlayAvatarTimeout  = 10 * time.Second
)

// FetchAvatar downloads a FACEIT or Steam CDN avatar. Other hosts and empty
// URLs return (nil, nil).
func FetchAvatar(ctx context.Context, httpClient *http.Client, rawURL string) ([]byte, error) {
	cleaned := cleanAvatarURL(rawURL)
	if cleaned == "" {
		return nil, nil
	}
	host := ""
	if u, err := url.Parse(cleaned); err == nil {
		host = strings.ToLower(u.Hostname())
	}
	if !strings.HasSuffix(host, "faceit-cdn.net") && !strings.HasSuffix(host, "steamstatic.com") && !strings.HasSuffix(host, "akamai.steamstatic.com") {
		return nil, nil
	}
	reqCtx, cancel := context.WithTimeout(ctx, overlayAvatarTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, cleaned, nil)
	if err != nil {
		return nil, fmt.Errorf("build FACEIT avatar request: %w", err)
	}
	client := httpClient
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch FACEIT avatar: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, nil
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, overlayAvatarMaxBytes+1))
	if err != nil || len(body) == 0 || len(body) > overlayAvatarMaxBytes {
		return nil, nil
	}
	return body, nil
}
