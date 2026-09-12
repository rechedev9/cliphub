package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func logFixture(t *testing.T) (*Store, *API, time.Time) {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "telemetry.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	api, err := NewAPI(store, testIngestKey, testAdminKey)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	api.now = func() time.Time { return now }
	api.logf = func(string, ...any) {}
	return store, api, now
}

func logRecord(now time.Time) LogRecord {
	return LogRecord{SchemaVersion: 1, ID: uuid.NewString(), SupportCode: "CH-ABCD-1234-5678-90AB-CDEF", SessionID: uuid.NewString(), Sequence: 1, OccurredAt: now, Release: "3.0.2", Source: "orchestrator", Level: "error", Event: "process.stderr", Message: "encoder failed", JobID: uuid.NewString(), AttemptID: uuid.NewString(), Operation: "render:variant", Attempt: 1}
}

func postLogs(t *testing.T, api *API, records ...LogRecord) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(LogBatch{Records: records})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(IngestKeyHeader, testIngestKey)
	w := httptest.NewRecorder()
	api.PublicHandler().ServeHTTP(w, req)
	return w
}

func TestLogsDurableReceiptRetryAndPaginatedQuery(t *testing.T) {
	store, api, now := logFixture(t)
	first := logRecord(now)
	first.Message = "Beginning: preparing FFmpeg\n" + strings.Repeat("progress observation\n", 190) + "Final cause: encoder device unavailable; token=secret-value; input=C:\\Users\\Alice\\private.dem"
	second := first
	second.ID, second.Sequence, second.Event, second.Message = uuid.NewString(), 2, "attempt.finished", "failed after encoder device unavailable"
	second.Outcome = "error"
	for attempt := 0; attempt < 2; attempt++ {
		w := postLogs(t, api, first, second)
		if w.Code != http.StatusAccepted {
			t.Fatalf("ingest %d: %d %s", attempt, w.Code, w.Body.String())
		}
		var receipt struct {
			AcceptedIDs []string `json:"accepted_ids"`
			Inserted    int      `json:"inserted"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &receipt); err != nil {
			t.Fatal(err)
		}
		if len(receipt.AcceptedIDs) != 2 || receipt.AcceptedIDs[0] != first.ID || receipt.AcceptedIDs[1] != second.ID || receipt.Inserted != 2*(1-attempt) {
			t.Fatalf("receipt: %+v", receipt)
		}
	}
	query := LogQuery{JobID: first.JobID, Since: now.Add(-time.Hour), Limit: 1}
	page, err := store.logs.Query(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Records) != 1 || !page.HasMore || page.Records[0].ID != first.ID {
		t.Fatalf("first page: %+v", page)
	}
	message := page.Records[0].Message
	if len(message) < 2048 || !strings.Contains(message, "Final cause: encoder device unavailable") || strings.Contains(message, "secret-value") || strings.Contains(message, "Alice") {
		t.Fatalf("lost cause or leaked private text: %s", message)
	}
	query.After = page.NextCursor
	page, err = store.logs.Query(context.Background(), query)
	if err != nil || len(page.Records) != 1 || page.HasMore || page.Records[0].ID != second.ID {
		t.Fatalf("second page: %+v %v", page, err)
	}
	if page.Records[0].AttemptID != first.AttemptID || page.Records[0].ReceivedAt != now {
		t.Fatalf("correlation/receipt time lost: %+v", page.Records[0])
	}
	query.After = page.NextCursor
	page, err = store.logs.Query(context.Background(), query)
	if err != nil || len(page.Records) != 0 || page.HasMore {
		t.Fatalf("end page: %+v %v", page, err)
	}
}

func TestLogsIdentityConflictDoesNotAcknowledgeOrPartiallyInsert(t *testing.T) {
	store, api, now := logFixture(t)
	first := logRecord(now)
	if w := postLogs(t, api, first); w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	conflict := first
	conflict.Message = "different content"
	newRecord := first
	newRecord.ID, newRecord.Sequence = uuid.NewString(), 2
	if w := postLogs(t, api, newRecord, conflict); w.Code != http.StatusConflict || strings.Contains(w.Body.String(), "accepted_ids") {
		t.Fatalf("conflict accepted: %d %s", w.Code, w.Body.String())
	}
	usage, err := store.logs.Usage(context.Background())
	if err != nil || usage.Records != 1 {
		t.Fatalf("partial transaction: %+v %v", usage, err)
	}
	conflict.ID = uuid.NewString()
	if w := postLogs(t, api, conflict); w.Code != http.StatusConflict {
		t.Fatalf("sequence conflict: %d", w.Code)
	}
}

func TestLogAuthValidationBudgetAndStorageFailures(t *testing.T) {
	store, api, now := logFixture(t)
	record := logRecord(now)
	for _, tc := range []struct {
		method, path string
		private      bool
		status       int
	}{
		{http.MethodPost, "/v1/logs", false, 401},
		{http.MethodGet, "/v1/logs", false, 405},
		{http.MethodGet, "/v1/logs", true, 401},
	} {
		w := httptest.NewRecorder()
		handler := api.PublicHandler()
		if tc.private {
			handler = api.AdminHandler()
		}
		handler.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.status {
			t.Fatalf("%s %s private=%v: %d", tc.method, tc.path, tc.private, w.Code)
		}
	}
	for _, mutate := range []func(*LogRecord){
		func(r *LogRecord) { r.Source = "arbitrary" }, func(r *LogRecord) { r.JobID = "not-a-job" },
		func(r *LogRecord) { r.Message = strings.Repeat("x", MaxLogMessageBytes+1) },
		func(r *LogRecord) { r.Sequence = 0 }, func(r *LogRecord) { r.Outcome = "invented" },
	} {
		invalid := record
		mutate(&invalid)
		if w := postLogs(t, api, invalid); w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("invalid accepted: %d %s", w.Code, w.Body.String())
		}
	}
	api.logBudget.sourceEventLimit = 1
	if w := postLogs(t, api, record); w.Code != http.StatusTooManyRequests {
		t.Fatalf("budget: %d", w.Code)
	}
	usage, err := store.logs.Usage(context.Background())
	if err != nil || usage.Records != 0 {
		t.Fatalf("budget wrote data: %+v %v", usage, err)
	}
	api.logBudget.sourceEventLimit = 32 << 20
	_ = store.logs.db.Close()
	if w := postLogs(t, api, record); w.Code != http.StatusServiceUnavailable || strings.Contains(w.Body.String(), "accepted_ids") {
		t.Fatalf("storage failure acknowledged: %d %s", w.Code, w.Body.String())
	}
}

func TestLogRetentionAndRestartPreserveOldEventAPI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telemetry.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	old := logRecord(now.Add(-45 * 24 * time.Hour)) // Client clock differs; receipt owns retention.
	records, err := validateLogBatch(LogBatch{Records: []LogRecord{old}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.logs.Insert(context.Background(), records, now, nil); err != nil {
		t.Fatal(err)
	}
	gap := logRecord(now)
	gap.Event, gap.LostRecords = "delivery.gap", 7
	if _, err = store.logs.Insert(context.Background(), []LogRecord{gap}, now.Add(-31*24*time.Hour), nil); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if n, err := store.DeleteBefore(context.Background(), now.Add(-30*24*time.Hour)); err != nil || n != 1 {
		t.Fatalf("retention: %d %v", n, err)
	}
	page, err := store.logs.Query(context.Background(), LogQuery{SupportCode: old.SupportCode, Since: now.Add(-time.Hour), Limit: 10})
	if err != nil || len(page.Records) != 1 || page.Records[0].ID != old.ID {
		t.Fatalf("receipt based retention/restart: %+v %v", page, err)
	}
	api, err := NewAPI(store, testIngestKey, testAdminKey)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/logs?job_id="+old.JobID, nil)
	req.Header.Set("Authorization", "Bearer "+testAdminKey)
	api.AdminHandler().ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), old.ID) {
		t.Fatalf("admin query: %d %s", w.Code, w.Body.String())
	}
	var version int
	if err := store.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 2 {
		t.Fatalf("legacy schema changed: %d %v", version, err)
	}
}
