package faceit

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	steamProfileXMLSuffix = "?xml=1"
	steamAvatarMaxBytes   = 64 * 1024
	steamRequestTimeout   = 10 * time.Second
)

type steamProfile struct {
	AvatarFull string `xml:"avatarFull"`
}

func ResolveSteamAvatar(ctx context.Context, httpClient *http.Client, steamID64 string) (string, error) {
	if strings.TrimSpace(steamID64) == "" {
		return "", nil
	}
	profileURL := "https://steamcommunity.com/profiles/" + url.PathEscape(steamID64) + steamProfileXMLSuffix

	reqCtx, cancel := context.WithTimeout(ctx, steamRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, profileURL, nil)
	if err != nil {
		return "", fmt.Errorf("build Steam profile request: %w", err)
	}
	req.Header.Set("Accept", "text/xml")

	client := httpClient
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch Steam profile: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", nil
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, steamAvatarMaxBytes))
	if err != nil {
		return "", nil
	}
	var profile steamProfile
	if err := xml.Unmarshal(body, &profile); err != nil {
		return "", nil
	}
	avatarURL := strings.TrimSpace(profile.AvatarFull)
	if avatarURL == "" {
		return "", nil
	}
	u, err := url.Parse(avatarURL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" {
		return "", nil
	}
	return u.String(), nil
}

const overlayAvatarMaxBytes = 256 * 1024

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
	reqCtx, cancel := context.WithTimeout(ctx, steamRequestTimeout)
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

func IsDefaultFaceitAvatar(avatar string) bool {
	if avatar == "" {
		return true
	}
	return strings.Contains(avatar, "3b536dda-e3dd-40cd-baed-7e66ab050c8f")
}
