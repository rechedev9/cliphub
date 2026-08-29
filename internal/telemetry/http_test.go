package telemetry

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	testIngestKey = "ingest-key-with-at-least-24-characters"
	testAdminKey  = "admin-token-with-at-least-32-characters-long"
)

func TestPublicIngestAndPrivateQuery(t *testing.T) {
	t.Parallel()
	store, err := OpenStore(filepath.Join(t.TempDir(), "telemetry.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	api, err := NewAPI(store, testIngestKey, testAdminKey)
	if err != nil {
		t.Fatalf("NewAPI: %v", err)
	}
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	api.now = func() time.Time { return now }
	api.logf = func(string, ...any) {}

	body, err := json.Marshal(Batch{Events: []Event{testEvent(now, KindError, "error-1")}})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/ingest", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(IngestKeyHeader, testIngestKey)
	response := httptest.NewRecorder()
	api.PublicHandler().ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || !strings.Contains(response.Body.String(), `"inserted":1`) {
		t.Fatalf("ingest response = %d %s", response.Code, response.Body.String())
	}

	unauthorized := httptest.NewRecorder()
	api.AdminHandler().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/v1/incidents?support_code=CH-ABCD-1234-5678-90AB-CDEF", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}

	query := httptest.NewRequest(http.MethodGet, "/v1/incidents?support_code=CH-ABCD-1234-5678-90AB-CDEF", nil)
	query.Header.Set("Authorization", "Bearer "+testAdminKey)
	queried := httptest.NewRecorder()
	api.AdminHandler().ServeHTTP(queried, query)
	if queried.Code != http.StatusOK || !strings.Contains(queried.Body.String(), `"support_code":"CH-ABCD-1234-5678-90AB-CDEF"`) {
		t.Fatalf("query response = %d %s", queried.Code, queried.Body.String())
	}
}

func TestPublicLimiterRunsOnlyAfterIngestAuthentication(t *testing.T) {
	t.Parallel()
	store, err := OpenStore(filepath.Join(t.TempDir(), "telemetry.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	api, err := NewAPI(store, testIngestKey, testAdminKey)
	if err != nil {
		t.Fatalf("NewAPI: %v", err)
	}
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	api.now = func() time.Time { return now }
	api.budget.globalRequestLimit = 1
	api.budget.sourceRequestLimit = 1
	body, err := json.Marshal(Batch{Events: []Event{testEvent(now, KindError, "error-1")}})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	request := func(key string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/v1/ingest", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set(IngestKeyHeader, key)
		return r
	}
	unauthorized := httptest.NewRecorder()
	api.PublicHandler().ServeHTTP(unauthorized, request("wrong"))
	accepted := httptest.NewRecorder()
	api.PublicHandler().ServeHTTP(accepted, request(testIngestKey))
	limited := httptest.NewRecorder()
	api.PublicHandler().ServeHTTP(limited, request(testIngestKey))
	if unauthorized.Code != http.StatusUnauthorized || accepted.Code != http.StatusAccepted || limited.Code != http.StatusTooManyRequests {
		t.Fatalf("statuses = unauthorized:%d accepted:%d limited:%d", unauthorized.Code, accepted.Code, limited.Code)
	}
}

func TestIngestBudgetChargesOnlyNewRowsAndRejectsSourceOverflow(t *testing.T) {
	t.Parallel()
	store, err := OpenStore(filepath.Join(t.TempDir(), "telemetry.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	api, err := NewAPI(store, testIngestKey, testAdminKey)
	if err != nil {
		t.Fatalf("NewAPI: %v", err)
	}
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	api.now = func() time.Time { return now }
	api.budget.globalEventLimit = 1
	api.budget.sourceEventLimit = 1
	request := func(event Event) *http.Request {
		body, marshalErr := json.Marshal(Batch{Events: []Event{event}})
		if marshalErr != nil {
			t.Fatalf("Marshal: %v", marshalErr)
		}
		r := httptest.NewRequest(http.MethodPost, "/v1/ingest", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set(IngestKeyHeader, testIngestKey)
		return r
	}
	firstEvent := testEvent(now, KindError, "error-1")
	first := httptest.NewRecorder()
	api.PublicHandler().ServeHTTP(first, request(firstEvent))
	replay := httptest.NewRecorder()
	api.PublicHandler().ServeHTTP(replay, request(firstEvent))
	secondEvent := firstEvent
	secondEvent.ID = "07c0be0c-bcc9-47bf-87a9-08275151f28c"
	blocked := httptest.NewRecorder()
	api.PublicHandler().ServeHTTP(blocked, request(secondEvent))
	if first.Code != http.StatusAccepted || replay.Code != http.StatusAccepted || blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("statuses = first:%d replay:%d blocked:%d", first.Code, replay.Code, blocked.Code)
	}
	if !strings.Contains(replay.Body.String(), `"inserted":0`) {
		t.Fatalf("replay body = %s", replay.Body.String())
	}
}

func TestIngestBudgetSeparatesSourcesAndRollsBackReservations(t *testing.T) {
	t.Parallel()
	budget := newIngestBudget([32]byte{4, 5, 6})
	budget.sourceRequestLimit = 1
	budget.globalRequestLimit = 2
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	first := budget.sourceKey("192.0.2.1:1000")
	second := budget.sourceKey("192.0.2.2:1000")
	if !budget.AllowRequest(now, first) || budget.AllowRequest(now, first) || !budget.AllowRequest(now, second) {
		t.Fatal("request budget did not isolate source quotas")
	}
	budget.globalEventLimit = 1
	budget.sourceEventLimit = 1
	rollback, ok := budget.ReserveEvents(now, first, 1)
	if !ok || rollback == nil {
		t.Fatal("could not reserve event budget")
	}
	rollback()
	if _, ok := budget.ReserveEvents(now, second, 1); !ok {
		t.Fatal("rolled-back event reservation remained charged")
	}
}

func TestSourceKeyNormalizesIPv6ToPrefix(t *testing.T) {
	t.Parallel()
	budget := newIngestBudget([32]byte{1, 2, 3})
	first := budget.sourceKey("[2001:db8:1234:5678::1]:443")
	samePrefix := budget.sourceKey("[2001:db8:1234:5678:ffff::9]:8443")
	otherPrefix := budget.sourceKey("[2001:db8:1234:5679::1]:443")
	if first != samePrefix {
		t.Fatal("addresses in one IPv6 /64 did not share a limiter key")
	}
	if first == otherPrefix {
		t.Fatal("different IPv6 /64 prefixes shared a limiter key")
	}
}

func TestPublicIngestRejectsUnsafeShapes(t *testing.T) {
	t.Parallel()
	store, err := OpenStore(filepath.Join(t.TempDir(), "telemetry.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	api, err := NewAPI(store, testIngestKey, testAdminKey)
	if err != nil {
		t.Fatalf("NewAPI: %v", err)
	}
	api.now = func() time.Time { return time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC) }
	tests := []struct {
		name        string
		contentType string
		key         string
		body        string
		wantStatus  int
	}{
		{name: "missing key", contentType: "application/json", body: `{}`, wantStatus: http.StatusUnauthorized},
		{name: "wrong content type", key: testIngestKey, contentType: "text/plain", body: `{}`, wantStatus: http.StatusUnsupportedMediaType},
		{name: "unknown field", key: testIngestKey, contentType: "application/json", body: `{"events":[],"demo":"x"}`, wantStatus: http.StatusBadRequest},
		{name: "oversized", key: testIngestKey, contentType: "application/json", body: `{"events":[],"padding":"` + strings.Repeat("x", maxRequestBytes) + `"}`, wantStatus: http.StatusRequestEntityTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/v1/ingest", strings.NewReader(tt.body))
			request.Header.Set("Content-Type", tt.contentType)
			request.Header.Set(IngestKeyHeader, tt.key)
			response := httptest.NewRecorder()
			api.PublicHandler().ServeHTTP(response, request)
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d body=%s, want %d", response.Code, response.Body.String(), tt.wantStatus)
			}
		})
	}
}
