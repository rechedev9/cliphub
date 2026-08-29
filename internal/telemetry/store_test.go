package telemetry

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestOpenStoreRejectsUnexpectedPageSize(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "telemetry.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if _, err := db.Exec("PRAGMA page_size=8192; VACUUM; CREATE TABLE marker (id INTEGER)"); err != nil {
		t.Fatalf("set page size: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}
	if _, err := OpenStore(path); err == nil || !strings.Contains(err.Error(), "expected 4096") {
		t.Fatalf("OpenStore error = %v, want page-size rejection", err)
	}
}

func TestOpenStorePurgesLegacyFreeTextSchema(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "telemetry.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	legacySchema := `
CREATE TABLE telemetry_events (
 id TEXT PRIMARY KEY, received_at INTEGER NOT NULL, occurred_at INTEGER NOT NULL,
 kind TEXT NOT NULL, support_code TEXT NOT NULL, session_id TEXT NOT NULL,
 release TEXT NOT NULL, component TEXT NOT NULL, name TEXT NOT NULL,
 stage TEXT NOT NULL, class TEXT NOT NULL, summary TEXT NOT NULL,
 fingerprint TEXT NOT NULL, os TEXT NOT NULL, arch TEXT NOT NULL,
 outcome TEXT NOT NULL, duration_ms INTEGER NOT NULL
);
INSERT INTO telemetry_events VALUES (
 'legacy', 1, 1, 'error', 'CH-AAAA-BBBB-CCCC-DDDD-EEEE', 'session',
 '1.0.0', 'renderer', 'route.error', 'renderer', 'exception',
 'C:/Users/Luis/secret.dem token=abc', 'content-derived', 'win32', 'x64', '', 0
);`
	if _, err := db.Exec(legacySchema); err != nil {
		t.Fatalf("seed legacy database: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore legacy migration: %v", err)
	}
	defer store.Close()
	var count, version int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM telemetry_events").Scan(&count); err != nil {
		t.Fatalf("count migrated events: %v", err)
	}
	if err := store.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read migrated version: %v", err)
	}
	var tableSQL string
	if err := store.db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='telemetry_events'").Scan(&tableSQL); err != nil {
		t.Fatalf("read migrated schema: %v", err)
	}
	if count != 0 || version != 1 || !strings.Contains(tableSQL, "CHECK (summary = '')") {
		t.Fatalf("legacy migration = count:%d version:%d schema:%s", count, version, tableSQL)
	}
}

func TestStoreInsertQuerySummaryAndRetention(t *testing.T) {
	t.Parallel()
	store, err := OpenStore(filepath.Join(t.TempDir(), "telemetry.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	events := []Event{
		testEvent(now, KindError, "error-1"),
		testEvent(now.Add(time.Second), KindSpan, "span-1"),
		testEvent(now.Add(2*time.Second), KindSpan, "span-2"),
	}
	events[1].DurationMS = 100
	events[1].Class = ""
	events[1].Outcome = "ok"
	events[2].DurationMS = 300
	events[2].Class = ""
	events[2].Outcome = "ok"
	inserted, err := store.Insert(context.Background(), events, now, nil)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if inserted != 3 {
		t.Fatalf("inserted = %d, want 3", inserted)
	}
	inserted, err = store.Insert(context.Background(), events, now, nil)
	if err != nil {
		t.Fatalf("duplicate Insert: %v", err)
	}
	if inserted != 0 {
		t.Fatalf("duplicate inserted = %d, want 0", inserted)
	}
	got, err := store.Incidents(context.Background(), IncidentQuery{
		SupportCode: "CH-ABCD-1234-5678-90AB-CDEF",
		Since:       now.Add(-time.Hour),
		Limit:       20,
	})
	if err != nil {
		t.Fatalf("Incidents: %v", err)
	}
	if len(got) != 3 || got[0].ID != events[2].ID {
		t.Fatalf("Incidents = %#v", got)
	}
	summary, err := store.Summary(context.Background(), now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if summary.Storage.Events != 3 || summary.Storage.DatabaseBytes <= 0 || summary.Storage.MaxDatabaseBytes != 128<<20 {
		t.Fatalf("storage usage = %#v", summary.Storage)
	}
	if len(summary.Errors) != 1 || summary.Errors[0].Count != 1 {
		t.Fatalf("error groups = %#v", summary.Errors)
	}
	if len(summary.Spans) != 1 {
		t.Fatalf("span groups = %#v", summary.Spans)
	}
	span := summary.Spans[0]
	if span.Count != 2 || span.AverageMS != 200 || span.P95MS != 300 || span.MaximumMS != 300 {
		t.Fatalf("span group = %#v", span)
	}
	deleted, err := store.DeleteBefore(context.Background(), now.Add(time.Millisecond))
	if err != nil {
		t.Fatalf("DeleteBefore: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("deleted = %d, want 3", deleted)
	}
}

func TestCancelledSummaryReleasesSingleConnection(t *testing.T) {
	t.Parallel()
	store, err := OpenStore(filepath.Join(t.TempDir(), "telemetry.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.Summary(ctx, time.Now().Add(-time.Hour), time.Now()); err == nil {
		t.Fatal("Summary with cancelled context succeeded")
	}
	event := testEvent(time.Now(), KindError, "error-1")
	if inserted, err := store.Insert(context.Background(), []Event{event}, time.Now(), nil); err != nil || inserted != 1 {
		t.Fatalf("Insert after cancelled Summary = %d, %v", inserted, err)
	}
}

func TestRetentionFreelistReopensHighWaterAdmission(t *testing.T) {
	t.Parallel()
	store, err := OpenStore(filepath.Join(t.TempDir(), "telemetry.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	events := make([]Event, 2_000)
	for index := range events {
		events[index] = testEvent(now, KindError, "error-1")
		events[index].ID = uuid.NewString()
		events[index].Fingerprint = strings.Repeat("a", 64)
	}
	if _, err := store.Insert(context.Background(), events, now, nil); err != nil {
		t.Fatalf("bulk Insert: %v", err)
	}
	var pagesBefore, freeBefore int64
	if err := store.db.QueryRow("PRAGMA page_count").Scan(&pagesBefore); err != nil {
		t.Fatalf("page_count: %v", err)
	}
	if err := store.db.QueryRow("PRAGMA freelist_count").Scan(&freeBefore); err != nil {
		t.Fatalf("freelist_count: %v", err)
	}
	store.highWaterPages = pagesBefore - freeBefore
	if !storagePagesAtHighWater(pagesBefore, freeBefore, store.highWaterPages) {
		t.Fatal("fixture did not reach its injected high-water")
	}
	if _, err := store.DeleteBefore(context.Background(), now.Add(time.Second)); err != nil {
		t.Fatalf("DeleteBefore: %v", err)
	}
	var pagesAfter, freeAfter int64
	if err := store.db.QueryRow("PRAGMA page_count").Scan(&pagesAfter); err != nil {
		t.Fatalf("page_count after delete: %v", err)
	}
	if err := store.db.QueryRow("PRAGMA freelist_count").Scan(&freeAfter); err != nil {
		t.Fatalf("freelist_count after delete: %v", err)
	}
	if freeAfter <= freeBefore || storagePagesAtHighWater(pagesAfter, freeAfter, store.highWaterPages) {
		t.Fatalf("retention did not reopen capacity: before=%d/%d after=%d/%d", pagesBefore, freeBefore, pagesAfter, freeAfter)
	}
	event := testEvent(now, KindError, "error-1")
	event.ID = uuid.NewString()
	if inserted, err := store.Insert(context.Background(), []Event{event}, now, nil); err != nil || inserted != 1 {
		t.Fatalf("Insert after retention = %d, %v", inserted, err)
	}
}

func testEvent(occurredAt time.Time, kind, seed string) Event {
	ids := map[string]string{
		"error-1": "b838ff59-f6a1-49fc-b11d-7478550238d1",
		"span-1":  "d2d95675-16a6-4490-8145-c62e8eb26520",
		"span-2":  "d34a517d-27fa-4428-87b2-4b28bb3e456b",
	}
	return Event{
		SchemaVersion: SchemaVersion,
		ID:            ids[seed],
		OccurredAt:    occurredAt,
		Kind:          kind,
		SupportCode:   "CH-ABCD-1234-5678-90AB-CDEF",
		SessionID:     "a20a070d-c99f-4744-a613-5759d6ecc74c",
		Release:       "2.4.35",
		Component:     "electron",
		Name:          "desktop.boot_failed",
		Stage:         "boot",
		Class:         "boot_failed",
		OS:            "win32",
		Arch:          "x64",
	}
}
