package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/rechedev9/cliphub/internal/store"
)

func newSyncTestServer(t *testing.T) (*httptest.Server, *sql.DB) {
	t.Helper()
	repo, err := store.NewSQLiteJobRepository(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatalf("NewSQLiteJobRepository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithSyncLog(store.NewSyncLog(repo.DB())))
	srv := httptest.NewServer(Routes(h))
	t.Cleanup(srv.Close)
	return srv, repo.DB()
}

func TestSyncEndpoints(t *testing.T) {
	srv1, db1 := newSyncTestServer(t)
	if _, err := db1.ExecContext(context.Background(), `INSERT INTO library_teams (id, name, name_key, created_at, updated_at) VALUES ('t1', 'Team One', 'team one', 1, 1)`); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(srv1.URL + "/api/sync/changes?after=0&limit=1000")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d", resp.StatusCode)
	}
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	var batch store.SyncBatch
	if err := dec.Decode(&batch); err != nil {
		t.Fatal(err)
	}
	if len(batch.Changes) != 1 {
		t.Fatalf("changes = %d, want 1", len(batch.Changes))
	}

	body, err := json.Marshal(map[string]any{"changes": batch.Changes})
	if err != nil {
		t.Fatal(err)
	}
	srv2, db2 := newSyncTestServer(t)
	resp, err = http.Post(srv2.URL+"/api/sync/changes", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("POST status = %d, want 204", resp.StatusCode)
	}
	var count int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM library_teams WHERE id = 't1'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("team row not applied to second database")
	}
	var logCount int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM sync_changes`).Scan(&logCount); err != nil {
		t.Fatal(err)
	}
	if logCount != 0 {
		t.Fatalf("apply echoed %d rows into the second log", logCount)
	}

	for _, bad := range []struct {
		body string
	}{
		{`{"changes":[{"table":"nope","key":["x"],"op":"delete","seq":1,"changed_at":1}]}`},
		{`{"changes":[{"table":"library_teams","key":["t9"],"op":"upsert","seq":1,"changed_at":1,"row":{"id":"t9","name":"x","name_key":"x","created_at":1,"updated_at":1,"bogus":1}}]}`},
	} {
		resp, err := http.Post(srv2.URL+"/api/sync/changes", "application/json", bytes.NewReader([]byte(bad.body)))
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("bad input POST status = %d, want 400", resp.StatusCode)
		}
		if !bytes.Contains(body, []byte(`"code"`)) {
			t.Fatalf("bad input body = %s, want JSON error", body)
		}
	}

	resp, err = http.Get(srv1.URL + "/api/sync/changes?after=abc")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid after status = %d, want 400", resp.StatusCode)
	}
}
