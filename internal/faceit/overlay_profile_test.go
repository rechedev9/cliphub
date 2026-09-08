package faceit

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/jpeg"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOverlayCareerMatchesUseLifetimeNotExtendedSample(t *testing.T) {
	for _, tc := range []struct {
		body string
		want *int
	}{
		{`{"lifetime":{"Matches":"7229","Total Matches":"1577"}}`, intPtr(7229)},
		{`{"lifetime":{"Total Matches":"1577"}}`, nil},
		{`{"lifetime":{"Matches":"0"}}`, intPtr(0)},
		{`{"lifetime":{"Matches":"-1"}}`, nil},
	} {
		t.Run(tc.body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/players":
					io.WriteString(w, `{"player_id":"p-1","nickname":"donk666","verified":true,"memberships":["premium"],"steam_id_64":"76561198000000001","games":{"cs2":{"region":"EU","skill_level":10,"faceit_elo":4370}}}`)
				case "/players/p-1/stats/cs2":
					io.WriteString(w, tc.body)
				case "/players/p-1/games/cs2/stats", "/players/p-1/history":
					if r.URL.Query().Get("limit") != "20" {
						t.Errorf("recent sample limit: %s", r.URL.RawQuery)
					}
					io.WriteString(w, `{"items":[]}`)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client, err := New(Options{APIKey: "test-key", BaseURL: server.URL, HTTPClient: server.Client()})
			if err != nil {
				t.Fatal(err)
			}
			players, err := client.OverlayPlayers(context.Background(), []string{"76561198000000001"})
			if err != nil {
				t.Fatal(err)
			}
			p := players["76561198000000001"]
			if !p.Verified || !p.Premium {
				t.Fatalf("profile flags: %+v", p)
			}
			if (tc.want == nil) != (p.LifetimeMatches == nil) || (tc.want != nil && *p.LifetimeMatches != *tc.want) {
				t.Fatalf("lifetime matches: %+v", p)
			}
		})
	}
}

func TestRecentHeadshotRateKeepsKnownZeroAndExcludesMissing(t *testing.T) {
	var missing, zero apiMatchStats
	if err := json.Unmarshal([]byte(`{}`), &missing); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"Headshots %":"0"}`), &zero); err != nil {
		t.Fatal(err)
	}
	m, z := convertStats(missing), convertStats(zero)
	if m.HasHSPct || !z.HasHSPct {
		t.Fatalf("presence: missing=%+v zero=%+v", m, z)
	}
	got := AggregateLast20([]RecentMatch{{Stats: &m}, {Stats: &z}, {Stats: &MatchStats{HeadshotsPercent: 80, HasHSPct: true}}, {Stats: &MatchStats{HeadshotsPercent: 200, HasHSPct: true}}})
	if got.HSPct == nil || *got.HSPct != 40 {
		t.Fatalf("recent headshots: %+v", got)
	}
	if AggregateLast20([]RecentMatch{{Stats: &m}}).HSPct != nil {
		t.Fatal("invented missing rate")
	}
}

type overlayAvatarTransport func(*http.Request) (*http.Response, error)

func (f overlayAvatarTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOverlayAvatarKeepsLargeValidPortraitAndBoundsDownload(t *testing.T) {
	pixels := image.NewRGBA(image.Rect(0, 0, 640, 640))
	rng := rand.New(rand.NewSource(42))
	_, _ = rng.Read(pixels.Pix)
	for i := 3; i < len(pixels.Pix); i += 4 {
		pixels.Pix[i] = 255
	}
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, pixels, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	if jpg.Len() <= 256<<10 {
		t.Fatal("fixture does not reproduce the discarded portrait")
	}
	for _, body := range [][]byte{jpg.Bytes(), bytes.Repeat([]byte{1}, overlayAvatarMaxBytes+1)} {
		client := &http.Client{Transport: overlayAvatarTransport(func(r *http.Request) (*http.Response, error) {
			if !strings.Contains(r.URL.Host, "faceit-cdn.net") {
				t.Fatal("unexpected host")
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
		})}
		got, err := FetchAvatar(context.Background(), client, "https://assets.faceit-cdn.net/avatars/player.jpg")
		if err != nil {
			t.Fatal(err)
		}
		if len(body) <= overlayAvatarMaxBytes {
			if !bytes.Equal(got, body) {
				t.Fatal("valid large portrait was discarded")
			}
		} else if len(got) != 0 {
			t.Fatal("oversized portrait accepted")
		}
	}
}
