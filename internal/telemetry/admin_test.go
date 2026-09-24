package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func adminGet(t *testing.T, api *API, path string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("Authorization", "Bearer "+testAdminKey)
	response := httptest.NewRecorder()
	api.AdminHandler().ServeHTTP(response, request)
	return response
}

func feedEvent(occurredAt time.Time, kind string) Event {
	event := testEvent(occurredAt, kind, "error-1")
	event.ID = uuid.NewString()
	if kind == KindSpan {
		event.Name, event.Stage, event.Class, event.Outcome, event.DurationMS = "desktop.boot", "boot", "", "ok", 10
	}
	return event
}

func TestErrorFeedPagesAcrossEqualReceiptTimes(t *testing.T) {
	store, api, now := logFixture(t)
	first := now.Add(-2 * time.Hour)
	second := now.Add(-time.Hour)
	var batchOne []Event
	for i := 0; i < 5; i++ {
		event := feedEvent(first, KindError)
		event.Message = fmt.Sprintf("failure %d", i)
		event.JobID = uuid.NewString()
		batchOne = append(batchOne, event)
	}
	batchOne = append(batchOne, feedEvent(first, KindSpan))
	batchTwo := []Event{feedEvent(second, KindError), feedEvent(second, KindError)}
	for _, batch := range []struct {
		events   []Event
		received time.Time
	}{{batchOne, first}, {batchTwo, second}} {
		if _, err := store.Insert(context.Background(), batch.events, batch.received, nil); err != nil {
			t.Fatal(err)
		}
	}
	var want []string
	for _, event := range batchOne[:5] {
		want = append(want, event.ID)
	}
	sort.Strings(want)
	secondIDs := []string{batchTwo[0].ID, batchTwo[1].ID}
	sort.Strings(secondIDs)
	want = append(want, secondIDs...)

	var got []string
	after := ""
	for pages := 0; ; pages++ {
		if pages > 10 {
			t.Fatal("error feed did not terminate")
		}
		response := adminGet(t, api, "/v1/errors?limit=2&after="+after)
		if response.Code != http.StatusOK {
			t.Fatalf("errors page: %d %s", response.Code, response.Body.String())
		}
		var page struct {
			Events []struct {
				Event
				ReceivedAt time.Time `json:"received_at"`
			} `json:"events"`
			NextAfter string `json:"next_after"`
			HasMore   bool   `json:"has_more"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		for _, event := range page.Events {
			if event.Kind != KindError || event.ReceivedAt.IsZero() || event.SupportCode == "" {
				t.Fatalf("feed row is incomplete: %+v", event)
			}
			got = append(got, event.ID)
		}
		if len(page.Events) > 0 {
			last := page.Events[len(page.Events)-1]
			if page.NextAfter != fmt.Sprintf("%d:%s", last.ReceivedAt.UnixMilli(), last.ID) {
				t.Fatalf("next_after = %q for last row %s", page.NextAfter, last.ID)
			}
		} else if page.NextAfter != after {
			t.Fatalf("empty page moved the cursor: %q -> %q", after, page.NextAfter)
		}
		after = page.NextAfter
		if !page.HasMore {
			break
		}
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("feed order = %v\nwant %v", got, want)
	}

	// The first page carries the stored diagnostic message and job id.
	response := adminGet(t, api, "/v1/errors?limit=1")
	if !strings.Contains(response.Body.String(), `"job_id":"`) || !strings.Contains(response.Body.String(), `"message":"failure `) {
		t.Fatalf("feed omits diagnostics: %s", response.Body.String())
	}
	// An exhausted cursor keeps returning itself, so a scanner can persist it.
	response = adminGet(t, api, "/v1/errors?after="+after)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"events":[]`) ||
		!strings.Contains(response.Body.String(), `"next_after":"`+after+`"`) || !strings.Contains(response.Body.String(), `"has_more":false`) {
		t.Fatalf("exhausted feed: %s", response.Body.String())
	}
}

func TestErrorFeedRejectsInvalidInput(t *testing.T) {
	_, api, _ := logFixture(t)
	for query, code := range map[string]string{
		"after=abc":                           "invalid_cursor",
		"after=12":                            "invalid_cursor",
		"after=-1:" + uuid.NewString():        "invalid_cursor",
		"after=1:not-a-uuid":                  "invalid_cursor",
		"after=%3A" + uuid.NewString():        "invalid_cursor",
		"limit=0":                             "invalid_limit",
		"limit=201":                           "invalid_limit",
		"limit=x&after=1:" + uuid.NewString(): "invalid_limit",
		"after=99999999999999999999:" + uuid.NewString(): "invalid_cursor",
	} {
		response := adminGet(t, api, "/v1/errors?"+query)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), code) {
			t.Fatalf("%s: %d %s", query, response.Code, response.Body.String())
		}
	}
	if response := adminGet(t, api, "/v1/errors?limit=200"); response.Code != http.StatusOK {
		t.Fatalf("maximum limit: %d", response.Code)
	}
	unauthorized := httptest.NewRecorder()
	api.AdminHandler().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/v1/errors", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated feed: %d", unauthorized.Code)
	}
}

type capturedLog struct {
	mu    sync.Mutex
	lines []string
}

func (c *capturedLog) logf(format string, args ...any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, fmt.Sprintf(format, args...))
}

func (c *capturedLog) take() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	lines := c.lines
	c.lines = nil
	return lines
}

var rejectionLine = regexp.MustCompile(`^telemetry stage=(ingest|logs) class=rejected status=[0-9]{3} code=[a-z_]+$`)

func TestIngestRejectionsAreCountedAndLoggedWithoutBodies(t *testing.T) {
	store, api, now := logFixture(t)
	logs := &capturedLog{}
	api.logf = logs.logf
	const secret = "C:\\Users\\Alice\\private-marker.dem"

	validEvent := testEvent(now, KindError, "error-1")
	validEvent.Message = secret
	eventBody, err := json.Marshal(Batch{Events: []Event{validEvent}})
	if err != nil {
		t.Fatal(err)
	}
	invalidEvent := validEvent
	invalidEvent.Name = "invented.event"
	invalidEventBody, _ := json.Marshal(Batch{Events: []Event{invalidEvent}})
	record := logRecord(now)
	record.Message = secret
	logBody, _ := json.Marshal(LogBatch{Records: []LogRecord{record}})
	invalidRecord := record
	invalidRecord.Source = "arbitrary"
	invalidLogBody, _ := json.Marshal(LogBatch{Records: []LogRecord{invalidRecord}})

	post := func(path, key, contentType string, body []byte) int {
		request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		request.Header.Set("Content-Type", contentType)
		request.Header.Set(IngestKeyHeader, key)
		response := httptest.NewRecorder()
		api.PublicHandler().ServeHTTP(response, request)
		return response.Code
	}
	steps := []struct {
		name   string
		key    string
		setup  func() func()
		path   string
		ctype  string
		body   []byte
		status int
		want   string
	}{
		{name: "events unauthorized", key: "wrong", path: "/v1/ingest", body: eventBody, status: 401, want: "events:401:unauthorized"},
		{name: "events rate limited", setup: func() func() { api.budget.sourceRequestLimit = 0; return func() { api.budget.sourceRequestLimit = 30 } }, path: "/v1/ingest", body: eventBody, status: 429, want: "events:429:rate_limited"},
		{name: "events content type", path: "/v1/ingest", ctype: "text/plain", body: eventBody, status: 415, want: "events:415:content_type_must_be_json"},
		{name: "events malformed", path: "/v1/ingest", body: append([]byte(secret), eventBody...), status: 400, want: "events:400:invalid_request"},
		{name: "events trailing value", path: "/v1/ingest", body: append(append([]byte{}, eventBody...), eventBody...), status: 400, want: "events:400:invalid_request"},
		{name: "events oversized", path: "/v1/ingest", body: []byte(`{"events":[],"x":"` + strings.Repeat("x", maxRequestBytes) + `"}`), status: 413, want: "events:413:invalid_request"},
		{name: "events not allowlisted", path: "/v1/ingest", body: invalidEventBody, status: 422, want: "events:422:invalid_event"},
		{name: "events budget", setup: func() func() { api.budget.sourceEventLimit = 0; return func() { api.budget.sourceEventLimit = 500 } }, path: "/v1/ingest", body: eventBody, status: 429, want: "events:429:event_budget_exhausted"},
		{name: "events high water", setup: func() func() {
			store.highWaterPages = 1
			return func() { store.highWaterPages = storageHighWaterPages }
		}, path: "/v1/ingest", body: eventBody, status: 507, want: "events:507:storage_capacity_reached"},
		{name: "logs unauthorized", key: "wrong", path: "/v1/logs", body: logBody, status: 401, want: "logs:401:unauthorized"},
		{name: "logs rate limited", setup: func() func() {
			api.logBudget.sourceRequestLimit = 0
			return func() { api.logBudget.sourceRequestLimit = 120 }
		}, path: "/v1/logs", body: logBody, status: 429, want: "logs:429:rate_limited"},
		{name: "logs content type", path: "/v1/logs", ctype: "text/plain", body: logBody, status: 415, want: "logs:415:content_type_must_be_json"},
		{name: "logs malformed", path: "/v1/logs", body: append([]byte(secret), logBody...), status: 400, want: "logs:400:invalid_request"},
		{name: "logs invalid record", path: "/v1/logs", body: invalidLogBody, status: 422, want: "logs:422:invalid_log"},
		{name: "logs budget", setup: func() func() {
			api.logBudget.sourceEventLimit = 1
			return func() { api.logBudget.sourceEventLimit = 32 << 20 }
		}, path: "/v1/logs", body: logBody, status: 429, want: "logs:429:log_budget_exhausted"},
	}
	want := map[string]int64{}
	for _, step := range steps {
		if step.setup != nil {
			restore := step.setup()
			code := post(step.path, keyOr(step.key), contentTypeOr(step.ctype), step.body)
			restore()
			if code != step.status {
				t.Fatalf("%s: status %d, want %d", step.name, code, step.status)
			}
		} else if code := post(step.path, keyOr(step.key), contentTypeOr(step.ctype), step.body); code != step.status {
			t.Fatalf("%s: status %d, want %d", step.name, code, step.status)
		}
		want[step.want]++
		lines := logs.take()
		if len(lines) != 1 || !rejectionLine.MatchString(lines[0]) {
			t.Fatalf("%s: rejection log = %q", step.name, lines)
		}
		if !strings.HasSuffix(lines[0], strings.SplitN(step.want, ":", 3)[2]) {
			t.Fatalf("%s: log %q does not carry the code", step.name, lines[0])
		}
	}
	// An identity conflict is a logs-only rejection.
	if code := post("/v1/logs", testIngestKey, "application/json", logBody); code != http.StatusAccepted {
		t.Fatalf("seed log: %d", code)
	}
	conflict := record
	conflict.Message = "different " + secret
	conflictBody, _ := json.Marshal(LogBatch{Records: []LogRecord{conflict}})
	if code := post("/v1/logs", testIngestKey, "application/json", conflictBody); code != http.StatusConflict {
		t.Fatalf("conflict: %d", code)
	}
	want["logs:409:log_identity_conflict"]++
	logs.take()

	var health AdminHealth
	response := adminGet(t, api, "/healthz")
	if err := json.Unmarshal(response.Body.Bytes(), &health); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(health.Rejections) != fmt.Sprint(want) {
		t.Fatalf("rejections = %v\nwant %v", health.Rejections, want)
	}

	// Store failures still carry their internal error line, plus the label line.
	_ = store.logs.db.Close()
	if code := post("/v1/logs", testIngestKey, "application/json", logBody); code != http.StatusServiceUnavailable {
		t.Fatalf("log store failure: %d", code)
	}
	_ = store.db.Close()
	if code := post("/v1/ingest", testIngestKey, "application/json", eventBody); code != http.StatusInternalServerError {
		t.Fatalf("event store failure: %d", code)
	}
	snapshot := api.rejections.snapshot()
	if snapshot["logs:503:storage_unavailable"] != 1 || snapshot["events:500:storage_unavailable"] != 1 {
		t.Fatalf("store failure counters: %v", snapshot)
	}
	for _, line := range logs.take() {
		if strings.Contains(line, "private-marker") || strings.Contains(line, "Alice") || strings.Contains(line, "192.0.2.1") {
			t.Fatalf("log line leaked request content: %q", line)
		}
	}
}

func keyOr(key string) string {
	if key == "" {
		return testIngestKey
	}
	return key
}

func contentTypeOr(contentType string) string {
	if contentType == "" {
		return "application/json"
	}
	return contentType
}

func TestAdminHealthReportsBuildStorageAndDegradedDatabase(t *testing.T) {
	store, api, now := logFixture(t)
	api.Version = "v5.2.1-12-gabcdef0"
	decode := func() (map[string]any, AdminHealth) {
		t.Helper()
		response := adminGet(t, api, "/healthz")
		if response.Code != http.StatusOK {
			t.Fatalf("admin health status %d", response.Code)
		}
		var raw map[string]any
		var health AdminHealth
		if err := json.Unmarshal(response.Body.Bytes(), &raw); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(response.Body.Bytes(), &health); err != nil {
			t.Fatal(err)
		}
		return raw, health
	}
	raw, health := decode()
	for _, key := range []string{"service", "status", "version", "started_at", "db_ok", "last_received_at", "rejections", "events_storage_ratio", "logs_storage_ratio"} {
		if _, ok := raw[key]; !ok {
			t.Fatalf("admin health lacks %s: %v", key, raw)
		}
	}
	if _, err := time.Parse(time.RFC3339, health.StartedAt); err != nil {
		t.Fatalf("started_at %q: %v", health.StartedAt, err)
	}
	if health.Service != "cliphub-telemetry-admin" || health.Status != "ok" || !health.DBOK || health.Version != api.Version ||
		health.LastReceivedAt.Events != nil || health.LastReceivedAt.Logs != nil || len(health.Rejections) != 0 {
		t.Fatalf("fresh admin health = %+v", health)
	}
	if health.EventsStorageRatio <= 0 || health.EventsStorageRatio >= 1 || health.LogsStorageRatio <= 0 || health.LogsStorageRatio >= 1 {
		t.Fatalf("storage ratios = %v %v", health.EventsStorageRatio, health.LogsStorageRatio)
	}

	eventsReceived := now.Add(-time.Minute)
	if _, err := store.Insert(context.Background(), []Event{feedEvent(now, KindError)}, eventsReceived, nil); err != nil {
		t.Fatal(err)
	}
	if code := postLogs(t, api, logRecord(now)).Code; code != http.StatusAccepted {
		t.Fatalf("log ingest: %d", code)
	}
	_, health = decode()
	if health.LastReceivedAt.Events == nil || !health.LastReceivedAt.Events.Equal(eventsReceived) ||
		health.LastReceivedAt.Logs == nil || !health.LastReceivedAt.Logs.Equal(now) {
		t.Fatalf("last_received_at = %+v", health.LastReceivedAt)
	}

	// The public health check stays static and minimal.
	public := httptest.NewRecorder()
	api.PublicHandler().ServeHTTP(public, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if public.Code != http.StatusOK || public.Body.String() != "{\"service\":\"cliphub-telemetry\",\"status\":\"ok\"}\n" {
		t.Fatalf("public health = %d %q", public.Code, public.Body.String())
	}

	_ = store.db.Close()
	_, health = decode()
	if health.DBOK || health.Status != "degraded" || health.Version != api.Version {
		t.Fatalf("closed database health = %+v", health)
	}
}

func TestNewAPIDefaultsVersionToDev(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "telemetry.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	api, err := NewAPI(store, testIngestKey, testAdminKey)
	if err != nil {
		t.Fatal(err)
	}
	if api.Version != "dev" || api.startedAt.IsZero() {
		t.Fatalf("default build info = %q %v", api.Version, api.startedAt)
	}
}
