package importedvideos

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

const (
	jobA = "11111111-1111-4111-8111-111111111111"
	jobB = "22222222-2222-4222-8222-222222222222"
	jobC = "33333333-3333-4333-8333-333333333333"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestOpenKeepsDownloadedShortsAndDeletesTheClientStore(t *testing.T) {
	data := t.TempDir()
	results := filepath.Join(data, "cloud", "results")
	writeFile(t, filepath.Join(results, jobA, "short-01.mp4"), "video a")
	writeFile(t, filepath.Join(results, jobC, "short-01.mp4"), "orphan")
	client := filepath.Join(data, "cloud", "client.json")
	writeFile(t, client, `{
		"schema_version": "cliphub.cloud-client.v1",
		"device_token": "secret",
		"submissions": [
			{"cloud_job_id": "`+jobA+`", "local_job_id": "local-a", "title": "3 jugadas - Kill Feed", "created_at": "2026-10-20T10:00:00Z",
			 "videos": [{"name": "short-01.mp4", "variant": "viral-60-killfeed", "ready": true}]},
			{"cloud_job_id": "`+jobB+`", "title": "Never downloaded", "created_at": "2026-10-21T10:00:00Z",
			 "videos": [{"name": "short-01.mp4", "ready": false}]}
		]
	}`)
	writeFile(t, client+".damaged", "{")
	orphanTime := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(results, jobC, "short-01.mp4"), orphanTime, orphanTime); err != nil {
		t.Fatal(err)
	}

	store, err := Open(data)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if exists(client) || exists(client+".damaged") {
		t.Fatal("the client store with the device token is still on disk")
	}
	want := []Video{
		{CloudJobID: jobC, Name: "short-01.mp4", Title: fallbackTitle, CreatedAt: orphanTime},
		{CloudJobID: jobA, Name: "short-01.mp4", Title: "3 jugadas - Kill Feed", LocalJobID: "local-a", CreatedAt: time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)},
	}
	assertVideos(t, store.List(), want)

	reopened, err := Open(data)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	assertVideos(t, reopened.List(), want)
}

func TestOpenAdoptsTheFilesOfADamagedClientStore(t *testing.T) {
	data := t.TempDir()
	writeFile(t, filepath.Join(data, "cloud", "results", jobA, "short-01.mp4"), "video")
	client := filepath.Join(data, "cloud", "client.json")
	writeFile(t, client, "{not json")

	store, err := Open(data)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if exists(client) {
		t.Fatal("damaged client store kept")
	}
	videos := store.List()
	if len(videos) != 1 || videos[0].Title != fallbackTitle || videos[0].CloudJobID != jobA {
		t.Fatalf("videos = %+v, want the file under the fallback title", videos)
	}
}

func TestOpenWithoutCloudDataIsEmpty(t *testing.T) {
	data := t.TempDir()
	store, err := Open(data)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(store.List()) != 0 || exists(filepath.Join(data, "cloud")) {
		t.Fatalf("an install that never used the cloud got %+v or a cloud folder", store.List())
	}
}

func TestOpenOnlyServesIndexedFilesInsideTheResultsFolder(t *testing.T) {
	data := t.TempDir()
	writeFile(t, filepath.Join(data, "cloud", "results", jobA, "short-01.mp4"), "video")
	writeFile(t, filepath.Join(data, "secret.mp4"), "secret")
	store, err := Open(data)
	if err != nil {
		t.Fatal(err)
	}
	file, err := store.Open(jobA, "short-01.mp4")
	if err != nil || file.Name() != filepath.Join(data, "cloud", "results", jobA, "short-01.mp4") {
		t.Fatalf("Open = %v, %v", file, err)
	}
	_ = file.Close()
	for _, tc := range []struct{ id, name string }{
		{jobA, "../../../secret.mp4"},
		{jobA, "short-02.mp4"},
		{"..", "secret.mp4"},
		{"11111111-1111-4111-8111-11111111111X", "short-01.mp4"},
		{jobA, "short-01.txt"},
	} {
		if _, err := store.Open(tc.id, tc.name); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Open(%q, %q) error = %v, want ErrNotFound", tc.id, tc.name, err)
		}
	}
}

func TestDeleteRemovesTheFileTheFolderAndTheEntry(t *testing.T) {
	data := t.TempDir()
	dir := filepath.Join(data, "cloud", "results", jobA)
	writeFile(t, filepath.Join(dir, "short-01.mp4"), "video")
	store, err := Open(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(jobA, "short-01.mp4"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if exists(dir) || len(store.List()) != 0 {
		t.Fatalf("after Delete: folder kept=%v, list=%+v", exists(dir), store.List())
	}
	if err := store.Delete(jobA, "short-01.mp4"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second Delete error = %v, want ErrNotFound", err)
	}
	reopened, err := Open(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.List()) != 0 {
		t.Fatalf("deleted video came back: %+v", reopened.List())
	}
}

func assertVideos(t *testing.T, got, want []Video) {
	t.Helper()
	want = slices.Clone(want)
	if len(got) != len(want) {
		t.Fatalf("videos = %+v, want %+v", got, want)
	}
	for i := range want {
		if !got[i].CreatedAt.Equal(want[i].CreatedAt) {
			t.Fatalf("video %d created %v, want %v", i, got[i].CreatedAt, want[i].CreatedAt)
		}
		got[i].CreatedAt, want[i].CreatedAt = time.Time{}, time.Time{}
		if got[i] != want[i] {
			t.Fatalf("video %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
