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
	overlayAvatarMaxBytes = 2 * 1024 * 1024
	overlayAvatarTimeout  = 10 * time.Second
)

// FetchAvatar downloads a FACEIT or Steam CDN avatar. Other hosts and empty
// URLs return (nil, nil).
func FetchAvatar(ctx context.Context, httpClient *http.Client, rawURL string) ([]byte, error) {
	cleaned := cleanAvatarURL(rawURL)
	if cleaned == "" {
		return nil, nil
	}
	u, err := url.Parse(cleaned)
	if err != nil || !allowedAvatarURL(u) {
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
	// Accept CDN redirects only when every redirect remains inside the same
	// strict HTTPS allowlist. This preserves CDN compatibility without using an
	// avatar URL as a general outbound request primitive.
	requestClient := *client
	requestClient.CheckRedirect = func(next *http.Request, _ []*http.Request) error {
		if !allowedAvatarURL(next.URL) {
			return http.ErrUseLastResponse
		}
		return nil
	}
	res, err := requestClient.Do(req)
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

func allowedAvatarHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return host == "faceit-cdn.net" || strings.HasSuffix(host, ".faceit-cdn.net") ||
		host == "steamstatic.com" || strings.HasSuffix(host, ".steamstatic.com") ||
		host == "steamusercontent.com" || strings.HasSuffix(host, ".steamusercontent.com")
}

func allowedAvatarURL(u *url.URL) bool {
	return u != nil && strings.EqualFold(u.Scheme, "https") && u.User == nil && u.Port() == "" && allowedAvatarHost(u.Hostname())
}
