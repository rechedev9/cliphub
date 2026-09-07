package cloudbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// client is the outbound half of the bridge: every call the poller and the
// watcher make to the portal goes through here, authenticated with the one
// static shared secret.
type client struct {
	baseURL string
	token   string
	http    *http.Client
}

func newClient(baseURL, token string) *client {
	return &client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		// Long enough to push a multi-hundred-megabyte reel over a home
		// upstream link without tripping over itself.
		http: &http.Client{Timeout: 30 * time.Minute},
	}
}

func (c *client) authorize(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
}

type claimedRequest struct {
	ID               string `json:"id"`
	Note             string `json:"note"`
	DemoOriginalName string `json:"demoOriginalName"`
	SubmitterLabel   string `json:"submitterLabel"`
}

type claimResponse struct {
	Claimed bool `json:"claimed"`
	claimedRequest
}

// claim asks the portal for the single oldest approved request, if any.
func (c *client) claim(ctx context.Context) (claimedRequest, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/bridge/claim", nil)
	if err != nil {
		return claimedRequest{}, false, err
	}
	c.authorize(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return claimedRequest{}, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return claimedRequest{}, false, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var out claimResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return claimedRequest{}, false, fmt.Errorf("decode response: %w", err)
	}
	return out.claimedRequest, out.Claimed, nil
}

// downloadDemo streams the claimed request's raw .dem. The caller closes it.
func (c *client) downloadDemo(ctx context.Context, requestID string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/bridge/requests/"+requestID+"/demo", nil)
	if err != nil {
		return nil, err
	}
	c.authorize(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
	return resp.Body, nil
}

func (c *client) reportLocalJob(ctx context.Context, requestID, localJobID string) error {
	return c.postJSON(ctx, "/api/bridge/requests/"+requestID+"/local-job", map[string]string{
		"localJobId": localJobID,
	})
}

// requestStatus reads the portal's own status for a request, so the watcher
// can stop following one the owner has already closed out.
func (c *client) requestStatus(ctx context.Context, requestID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/bridge/requests/"+requestID+"/status", nil)
	if err != nil {
		return "", err
	}
	c.authorize(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var out struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return out.Status, nil
}

func (c *client) reportFailure(ctx context.Context, requestID, reason string) error {
	return c.postJSON(ctx, "/api/bridge/requests/"+requestID+"/status", map[string]string{
		"status": "failed",
		"reason": reason,
	})
}

// uploadArtifact streams one finished reel to the portal as a candidate
// deliverable. The owner picks which candidate is the real one in /admin.
func (c *client) uploadArtifact(ctx context.Context, requestID, variant, name string, body io.Reader, size int64) error {
	target := fmt.Sprintf(
		"%s/api/bridge/requests/%s/artifacts?variant=%s&name=%s",
		c.baseURL,
		requestID,
		url.QueryEscape(variant),
		url.QueryEscape(name),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	// Set explicitly so the portal can reject an oversized reel before
	// streaming it to disk rather than after.
	req.ContentLength = size
	c.authorize(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *client) postJSON(ctx context.Context, path string, payload map[string]string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.authorize(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
