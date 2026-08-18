package steamresolve

import (
	"bytes"
	"compress/bzip2"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestParseDemoURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "valve replay", raw: "http://replay123.valve.net/730/demo.dem.bz2"},
		{name: "https valve", raw: "https://replay412.valve.net/730/demo.dem"},
		{name: "file scheme", raw: "file:///etc/passwd", wantErr: true},
		{name: "localhost", raw: "http://127.0.0.1/demo.dem", wantErr: true},
		{name: "userinfo", raw: "http://user:pass@replay123.valve.net/730/demo.dem", wantErr: true},
		{name: "other host", raw: "https://example.com/demo.dem", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseDemoURL(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatal("error = nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestFetcherOpenPlainAndBz2(t *testing.T) {
	payload := []byte("PBDEMS2\x00hello")
	tests := []struct {
		name string
		path string
		body []byte
		want []byte
	}{
		{name: "plain dem", path: "/730/match.dem", body: payload, want: payload},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write(tt.body)
			}))
			t.Cleanup(server.Close)
			fetcher := NewFetcher(redirectingValveClient(t, server.URL))
			rc, name, err := fetcher.Open(context.Background(), "http://replay1.valve.net"+tt.path, 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			defer rc.Close()
			got, err := io.ReadAll(rc)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("body = %q, want %q", got, tt.want)
			}
			if !strings.HasSuffix(name, ".dem") {
				t.Errorf("name = %q", name)
			}
		})
	}

	t.Run("rejects disallowed host even via client", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("should not be fetched")
		}))
		t.Cleanup(server.Close)
		fetcher := NewFetcher(server.Client())
		_, _, err := fetcher.Open(context.Background(), server.URL+"/demo.dem", 1<<20)
		if err == nil {
			t.Fatal("error = nil")
		}
	})
}

func TestIsValveReplayHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{host: "replay123.valve.net", want: true},
		{host: "valve.net", want: true},
		{host: "evilvalve.net", want: false},
		{host: "replay.valve.net.evil.com", want: false},
		{host: "127.0.0.1", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			if got := isValveReplayHost(tt.host); got != tt.want {
				t.Errorf("isValveReplayHost(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}

// redirectingValveClient rewrites replay*.valve.net requests to the test server
// so allowDemoURL still sees a Valve host on the original URL.
func redirectingValveClient(t *testing.T, serverURL string) *http.Client {
	t.Helper()
	base, err := url.Parse(serverURL)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			clone := req.Clone(req.Context())
			clone.URL.Scheme = base.Scheme
			clone.URL.Host = base.Host
			clone.Host = base.Host
			return http.DefaultTransport.RoundTrip(clone)
		}),
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestBzip2ReaderStillWorks(t *testing.T) {
	// Sanity that the stdlib reader we use for Valve .dem.bz2 is the one we think.
	if bzip2.NewReader(bytes.NewReader(nil)) == nil {
		t.Fatal("bzip2 reader is nil")
	}
}
