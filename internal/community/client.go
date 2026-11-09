package community

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// maxReferenceBytes bounds the full pro reference download.
	maxReferenceBytes = 8 << 20
	maxResponseBytes  = 1 << 20
	userAgent         = "ClipHub-Studio-community/1"
)

// ErrNotModified means the server still has the reference the ETag names.
var ErrNotModified = errors.New("community: not modified")

// Client talks to the community server over HTTPS (plain HTTP only on loopback, for tests).
type Client struct {
	base *url.URL
	http *http.Client
}

// NewClient checks the base URL and builds a client with short timeouts.
func NewClient(baseURL string) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" {
		return nil, fmt.Errorf("community: invalid server URL %q", baseURL)
	}
	if u.Scheme != "https" && (u.Scheme != "http" || !isLoopback(u.Hostname())) {
		return nil, fmt.Errorf("community: server URL must use https, got %q", baseURL)
	}
	return &Client{base: u, http: &http.Client{Timeout: 30 * time.Second}}, nil
}

func isLoopback(host string) bool {
	ip := net.ParseIP(host)
	return host == "localhost" || (ip != nil && ip.IsLoopback())
}

// Host is the server host, for log lines.
func (c *Client) Host() string { return c.base.Host }

// ProReference downloads the full reference; ErrNotModified when etag still matches.
func (c *Client) ProReference(ctx context.Context, etag string) ([]byte, string, error) {
	req, err := c.request(ctx, http.MethodGet, "/v1/pro-reference?map="+AllMaps, nil)
	if err != nil {
		return nil, "", err
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("community: pro reference: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return nil, etag, ErrNotModified
	}
	body, err := readBody(resp, maxReferenceBytes)
	if err != nil {
		return nil, "", err
	}
	return body, resp.Header.Get("ETag"), nil
}

// Delete removes everything the install submitted.
func (c *Client) Delete(ctx context.Context, installID string) error {
	if !InstallIDPattern.MatchString(installID) {
		return errors.New("community: invalid install id")
	}
	return c.send(ctx, http.MethodDelete, "/v1/rankings/installs/"+installID, nil)
}

func (c *Client) send(ctx context.Context, method, path string, body []byte) error {
	req, err := c.request(ctx, method, path, body)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("community: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	_, err = readBody(resp, maxResponseBytes)
	return err
}

func (c *Client) request(ctx context.Context, method, path string, body []byte) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base.String()+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// readBody returns a 2xx body up to limit bytes, or the server's status as an error.
func readBody(resp *http.Response, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("community: read response: %w", err)
	}
	if int64(len(body)) > limit {
		return nil, errors.New("community: response too large")
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &e)
		return nil, fmt.Errorf("community: server answered %d %s", resp.StatusCode, e.Error)
	}
	return body, nil
}
