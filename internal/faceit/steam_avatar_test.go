package faceit

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestResolveSteamProfileAvatarUsesPublicProfileAndSuppressesPrivateAvatar(t *testing.T) {
	t.Parallel()
	client := &http.Client{Transport: steamAvatarTransport{roundTrip: func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "steamcommunity.com" {
			t.Fatalf("profile host = %q", r.URL.Host)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`<profile><privacyState>private</privacyState><avatarFull>https://avatars.akamai.steamstatic.com/private.jpg</avatarFull></profile>`))}, nil
	}}}
	got, err := ResolveSteamProfileAvatar(context.Background(), client, "76561198000000001")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Private || got.URL != "" {
		t.Fatalf("private profile leaked avatar: %+v", got)
	}
}

func TestSteamAvatarServiceTreatsNetworkFailureAsMissing(t *testing.T) {
	t.Parallel()
	service := SteamAvatarService{HTTPClient: &http.Client{Transport: steamAvatarTransport{roundTrip: func(*http.Request) (*http.Response, error) {
		return nil, errors.New("offline")
	}}}}
	if got := service.ResolveSteamAvatars(context.Background(), []string{"76561198000000001"}); len(got) != 0 {
		t.Fatalf("network failure returned avatar data: %+v", got)
	}
}

func TestResolveSteamProfileAvatarRejectsNonPublicAndRedirectedProfiles(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		`<profile><privacyState>friendsOnly</privacyState><avatarFull>https://avatars.akamai.steamstatic.com/friends.jpg</avatarFull></profile>`,
		`<profile><avatarFull>https://avatars.akamai.steamstatic.com/missing-state.jpg</avatarFull></profile>`,
	} {
		client := &http.Client{Transport: steamAvatarTransport{roundTrip: func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
		}}}
		got, err := ResolveSteamProfileAvatar(context.Background(), client, "76561198000000001")
		if err != nil || got.URL != "" || got.Private {
			t.Fatalf("non-public profile = %+v, %v", got, err)
		}
	}
	client := &http.Client{Transport: steamAvatarTransport{roundTrip: func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"https://example.invalid/profile"}}, Body: io.NopCloser(strings.NewReader("redirect"))}, nil
	}}}
	got, err := ResolveSteamProfileAvatar(context.Background(), client, "76561198000000001")
	if err != nil || got.URL != "" || got.Private {
		t.Fatalf("redirected profile = %+v, %v", got, err)
	}
}

func TestSteamAvatarServiceBoundsRosterAndKeepsOnlyAllowedPublicURLs(t *testing.T) {
	t.Parallel()
	client := &http.Client{Transport: steamAvatarTransport{roundTrip: func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "steamcommunity.com" || !strings.HasPrefix(r.URL.Path, "/profiles/") {
			t.Fatalf("unexpected request %s", r.URL.String())
		}
		id := strings.TrimPrefix(r.URL.Path, "/profiles/")
		body := `<profile><privacyState>public</privacyState><avatarFull>https://avatars.akamai.steamstatic.com/` + id + `.jpg</avatarFull></profile>`
		if strings.HasSuffix(id, "2") {
			body = `<profile><privacyState>public</privacyState><avatarFull>https://notsteamstatic.com/` + id + `.jpg</avatarFull></profile>`
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	}}}
	service := SteamAvatarService{HTTPClient: client}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got := service.ResolveSteamAvatars(ctx, []string{
		"76561198000000001", "76561198000000001", "not-a-steam-id", "76561198000000002",
	})
	if got["76561198000000001"].URL != "https://avatars.akamai.steamstatic.com/76561198000000001.jpg" {
		t.Fatalf("public avatar = %+v", got)
	}
	if _, ok := got["76561198000000002"]; ok {
		t.Fatalf("accepted a lookalike avatar host: %+v", got)
	}
}

type steamAvatarTransport struct {
	roundTrip func(*http.Request) (*http.Response, error)
}

func (t steamAvatarTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return t.roundTrip(r)
}
