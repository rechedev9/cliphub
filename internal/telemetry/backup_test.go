package telemetry

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestBackupDailyKeepsSevenReadableCopiesPerDatabase(t *testing.T) {
	store, api, now := logFixture(t)
	if _, err := store.Insert(context.Background(), []Event{feedEvent(now, KindError)}, now, nil); err != nil {
		t.Fatal(err)
	}
	if code := postLogs(t, api, logRecord(now)).Code; code != 202 {
		t.Fatalf("log ingest: %d", code)
	}
	dir := filepath.Join(t.TempDir(), "backups")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(dir, "operator-note.txt")
	if err := os.WriteFile(unrelated, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 9, 1, 3, 0, 0, 0, time.UTC)
	// A copy interrupted on the last day must not block or pass for that day.
	stale := filepath.Join(dir, "events-20260909.db.tmp")
	if err := os.WriteFile(stale, []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	for day := 0; day < 9; day++ {
		copies, err := store.BackupDaily(context.Background(), dir, start.AddDate(0, 0, day))
		if err != nil || copies != 2 {
			t.Fatalf("day %d: copies=%d err=%v", day, copies, err)
		}
	}
	if copies, err := store.BackupDaily(context.Background(), dir, start.AddDate(0, 0, 8).Add(12*time.Hour)); err != nil || copies != 0 {
		t.Fatalf("same-day rerun rewrote copies=%d err=%v", copies, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	var want []string
	for _, name := range []string{"events", "logs"} {
		for day := 2; day < 9; day++ {
			want = append(want, name+"-"+start.AddDate(0, 0, day).Format("20060102")+".db")
		}
	}
	want = append(want, "operator-note.txt")
	sort.Strings(want)
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("backup files = %v\nwant %v", names, want)
	}
	for table, file := range map[string]string{"telemetry_events": "events-20260909.db", "diagnostic_logs": "logs-20260909.db"} {
		db, err := sql.Open("sqlite", filepath.Join(dir, file))
		if err != nil {
			t.Fatal(err)
		}
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		_ = db.Close()
		if err != nil || count != 1 {
			t.Fatalf("%s copy: count=%d err=%v", file, count, err)
		}
	}
}
