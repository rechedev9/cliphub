package telemetryalert

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// PingDeadman reports the run to healthchecks check A: GET <url> when the run
// is green, POST <url>/fail with a short label-only reason otherwise.
func PingDeadman(ctx context.Context, client *http.Client, baseURL string, failures []string) error {
	if baseURL == "" {
		return nil
	}
	method, target, body := http.MethodGet, baseURL, ""
	if len(failures) > 0 {
		method, target = http.MethodPost, baseURL+"/fail"
		body = strings.Join(failures, "; ")
		if len(body) > 200 {
			body = body[:200]
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, target, strings.NewReader(body))
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("deadman ping failed")
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("deadman ping: status %d", resp.StatusCode)
	}
	return nil
}
