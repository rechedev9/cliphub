package telemetryalert

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// errNoEventsSource marks a collector that predates GET /v1/errors. The
// alerter keeps working from /v1/logs so it can be deployed first.
var errNoEventsSource = errors.New("collector has no /v1/errors endpoint")

// ErrorEvent is one row of the admin /v1/errors page (contracts.md §1).
type ErrorEvent struct {
	ID          string    `json:"id"`
	OccurredAt  time.Time `json:"occurred_at"`
	ReceivedAt  time.Time `json:"received_at"`
	Kind        string    `json:"kind"`
	SupportCode string    `json:"support_code"`
	SessionID   string    `json:"session_id"`
	Release     string    `json:"release"`
	Component   string    `json:"component"`
	Name        string    `json:"name"`
	Stage       string    `json:"stage"`
	Class       string    `json:"class"`
	Message     string    `json:"message"`
	JobID       string    `json:"job_id"`
}

type errorPage struct {
	Events    []ErrorEvent `json:"events"`
	NextAfter string       `json:"next_after"`
	HasMore   bool         `json:"has_more"`
}

// LogRecord is one row of the admin /v1/logs page.
type LogRecord struct {
	Cursor      int64     `json:"cursor"`
	ReceivedAt  time.Time `json:"received_at"`
	ID          string    `json:"id"`
	SupportCode string    `json:"support_code"`
	SessionID   string    `json:"session_id"`
	OccurredAt  time.Time `json:"occurred_at"`
	Release     string    `json:"release"`
	Source      string    `json:"source"`
	Level       string    `json:"level"`
	Event       string    `json:"event"`
	Message     string    `json:"message"`
	JobID       string    `json:"job_id"`
	Operation   string    `json:"operation"`
	Outcome     string    `json:"outcome"`
	DurationMS  int64     `json:"duration_ms"`
	ExitCode    *int64    `json:"exit_code"`
	LostRecords int64     `json:"lost_records"`
}

type logPage struct {
	Records    []LogRecord `json:"records"`
	NextCursor int64       `json:"next_cursor"`
	HasMore    bool        `json:"has_more"`
}

// Health is the admin /healthz body. Older collectors return only service and
// status, so every extended field is optional.
type Health struct {
	Service            string             `json:"service"`
	Status             string             `json:"status"`
	Version            string             `json:"version"`
	StartedAt          string             `json:"started_at"`
	DBOK               *bool              `json:"db_ok"`
	LastReceivedAt     map[string]*string `json:"last_received_at"`
	Rejections         map[string]int64   `json:"rejections"`
	EventsStorageRatio *float64           `json:"events_storage_ratio"`
	LogsStorageRatio   *float64           `json:"logs_storage_ratio"`
}

type source struct {
	base   string
	token  string
	client *http.Client
}

func (s source) get(ctx context.Context, path string, query url.Values, into any) error {
	target := s.base + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("admin %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound && path == "/v1/errors" {
		return errNoEventsSource
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("admin %s: status %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(into); err != nil {
		return fmt.Errorf("admin %s: decode: %w", path, err)
	}
	return nil
}

func (s source) fetchHealth(ctx context.Context) (Health, error) {
	var h Health
	err := s.get(ctx, "/healthz", nil, &h)
	return h, err
}

// fetchErrors pages error events after the cursor; maxPages 0 means until caught up.
func (s source) fetchErrors(ctx context.Context, after string, maxPages int) ([]ErrorEvent, string, bool, error) {
	var all []ErrorEvent
	for page := 0; maxPages == 0 || page < maxPages; page++ {
		query := url.Values{"limit": {"200"}}
		if after != "" {
			query.Set("after", after)
		}
		var body errorPage
		if err := s.get(ctx, "/v1/errors", query, &body); err != nil {
			return all, after, false, err
		}
		all = append(all, body.Events...)
		if body.NextAfter != "" {
			after = body.NextAfter
		}
		if !body.HasMore || len(body.Events) == 0 {
			return all, after, true, nil
		}
	}
	return all, after, false, nil
}

// fetchLogs pages every log record after the cursor (500 per page).
func (s source) fetchLogs(ctx context.Context, after int64, maxPages int) ([]LogRecord, int64, bool, error) {
	var all []LogRecord
	for page := 0; maxPages == 0 || page < maxPages; page++ {
		var body logPage
		query := url.Values{"limit": {"500"}, "after": {strconv.FormatInt(after, 10)}}
		if err := s.get(ctx, "/v1/logs", query, &body); err != nil {
			return all, after, false, err
		}
		all = append(all, body.Records...)
		if body.NextCursor > after {
			after = body.NextCursor
		}
		if !body.HasMore || len(body.Records) == 0 {
			return all, after, true, nil
		}
	}
	return all, after, false, nil
}
