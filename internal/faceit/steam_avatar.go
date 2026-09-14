package faceit

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	steamProfileXMLSuffix = "?xml=1"
	steamAvatarMaxBytes   = 64 * 1024
	steamRequestTimeout   = 10 * time.Second
	steamAvatarBatchLimit = 10
	steamAvatarWorkers    = 4
)

type steamProfile struct {
	PrivacyState string `xml:"privacyState"`
	AvatarFull   string `xml:"avatarFull"`
}

// SteamAvatar is the public profile-avatar state captured with a Full Demo.
// Private is deliberately retained even though a private profile never gets
// an avatar URL: Steam can include stale avatar metadata in a private profile.
type SteamAvatar struct {
	URL     string `json:"url,omitempty"`
	Private bool   `json:"private,omitempty"`
}

// SteamAvatarResolver is the small dependency used by the local Full Demo
// admission path. It makes tests independent of real Steam accounts.
type SteamAvatarResolver interface {
	ResolveSteamAvatars(context.Context, []string) map[string]SteamAvatar
}

// SteamAvatarService resolves only the official public Steam Community
// profile endpoint. A single caller deadline bounds the entire roster lookup;
// the per-profile timeout is only a shorter cap when that deadline permits it.
type SteamAvatarService struct {
	HTTPClient *http.Client
}

func (s SteamAvatarService) ResolveSteamAvatars(ctx context.Context, steamIDs []string) map[string]SteamAvatar {
	unique := make([]string, 0, len(steamIDs))
	seen := make(map[string]struct{}, len(steamIDs))
	for _, raw := range steamIDs {
		id := strings.TrimSpace(raw)
		if !validSteamID64(id) {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
		if len(unique) == steamAvatarBatchLimit {
			break
		}
	}
	if len(unique) == 0 || ctx.Err() != nil {
		return map[string]SteamAvatar{}
	}
	workers := steamAvatarWorkers
	if workers > len(unique) {
		workers = len(unique)
	}
	jobs := make(chan string)
	results := make(chan struct {
		id     string
		avatar SteamAvatar
	}, len(unique))
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				avatar, err := ResolveSteamProfileAvatar(ctx, s.HTTPClient, id)
				if err == nil && (avatar.URL != "" || avatar.Private) {
					results <- struct {
						id     string
						avatar SteamAvatar
					}{id, avatar}
				}
			}
		}()
	}
send:
	for _, id := range unique {
		select {
		case jobs <- id:
		case <-ctx.Done():
			break send
		}
	}
	close(jobs)
	wg.Wait()
	close(results)
	out := make(map[string]SteamAvatar, len(unique))
	for result := range results {
		out[result.id] = result.avatar
	}
	return out
}

func ResolveSteamAvatar(ctx context.Context, httpClient *http.Client, steamID64 string) (string, error) {
	avatar, err := ResolveSteamProfileAvatar(ctx, httpClient, steamID64)
	return avatar.URL, err
}

// ResolveSteamProfileAvatar reads the public Steam Community XML profile.
// A private profile is an explicit successful outcome, never a fallback that
// may expose its avatar metadata.
func ResolveSteamProfileAvatar(ctx context.Context, httpClient *http.Client, steamID64 string) (SteamAvatar, error) {
	steamID64 = strings.TrimSpace(steamID64)
	if !validSteamID64(steamID64) {
		return SteamAvatar{}, nil
	}
	profileURL := "https://steamcommunity.com/profiles/" + url.PathEscape(steamID64) + steamProfileXMLSuffix

	reqCtx, cancel := context.WithTimeout(ctx, steamRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, profileURL, nil)
	if err != nil {
		return SteamAvatar{}, fmt.Errorf("build Steam profile request: %w", err)
	}
	req.Header.Set("Accept", "text/xml")

	client := httpClient
	if client == nil {
		client = http.DefaultClient
	}
	// The profile origin is fixed. Following a redirect would turn this lookup
	// into a request to an arbitrary host, even though the initial URL is safe.
	requestClient := *client
	requestClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := requestClient.Do(req)
	if err != nil {
		return SteamAvatar{}, fmt.Errorf("fetch Steam profile: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return SteamAvatar{}, nil
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, steamAvatarMaxBytes))
	if err != nil {
		return SteamAvatar{}, nil
	}
	var profile steamProfile
	if err := xml.Unmarshal(body, &profile); err != nil {
		return SteamAvatar{}, nil
	}
	privacy := strings.ToLower(strings.TrimSpace(profile.PrivacyState))
	if privacy == "private" {
		return SteamAvatar{Private: true}, nil
	}
	if privacy != "public" {
		return SteamAvatar{}, nil
	}
	avatarURL := strings.TrimSpace(profile.AvatarFull)
	if avatarURL == "" {
		return SteamAvatar{}, nil
	}
	u, err := url.Parse(avatarURL)
	if err != nil || !isSteamAvatarURL(u) {
		return SteamAvatar{}, nil
	}
	return SteamAvatar{URL: u.String()}, nil
}

func validSteamID64(id string) bool {
	if len(id) != 17 {
		return false
	}
	for _, c := range id {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func isSteamAvatarURL(u *url.URL) bool {
	if u == nil || !strings.EqualFold(u.Scheme, "https") || u.User != nil || u.Port() != "" {
		return false
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	return host == "steamstatic.com" || strings.HasSuffix(host, ".steamstatic.com") ||
		host == "steamusercontent.com" || strings.HasSuffix(host, ".steamusercontent.com")
}

func IsDefaultFaceitAvatar(avatar string) bool {
	if avatar == "" {
		return true
	}
	return strings.Contains(avatar, "3b536dda-e3dd-40cd-baed-7e66ab050c8f")
}
