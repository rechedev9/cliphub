package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/rechedev9/cliphub/internal/faceit"
	"github.com/rechedev9/cliphub/internal/httpapi"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/tasks"
)

const faceitZstdDemoPath = `C:\Users\reche\Downloads\1-b5604ae7-c676-454b-901a-0b02014abd94-1-2.dem.zst`

func TestFaceitStudioSidebarE2E(t *testing.T) {
	apiKey := strings.TrimSpace(os.Getenv("FACEIT_API_KEY"))
	if apiKey == "" {
		t.Skip("FACEIT_API_KEY is not set; skipping live FACEIT sidebar e2e")
	}

	dataDir := t.TempDir()
	jobs := store.NewMemoryJobRepository()
	files, err := storage.NewLocal(dataDir)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	client, err := faceit.New(faceit.Options{APIKey: apiKey, RequestTimeout: 20 * time.Second})
	if err != nil {
		t.Fatalf("faceit client: %v", err)
	}
	follows, err := faceit.NewFollowStore(filepath.Join(dataDir, "faceit", "followed.json"), time.Now)
	if err != nil {
		t.Fatalf("follow store: %v", err)
	}
	handlers := httpapi.NewHandlers(
		jobs,
		files,
		newInlineQueue(map[string]taskHandler{}, 1),
		httpapi.WithFaceit(client, follows),
		httpapi.WithCapabilities(httpapi.Capabilities{FaceitEnabled: true}),
	)

	srv := httptest.NewServer(httpapi.Routes(handlers))
	t.Cleanup(srv.Close)
	httpClient := srv.Client()
	httpClient.Timeout = 45 * time.Second

	capsBody := getJSON(t, httpClient, srv.URL+"/api/capabilities", http.StatusOK)
	assertNoCredential(t, capsBody, apiKey)
	if !bytes.Contains(capsBody, []byte(`"faceit":{"enabled":true}`)) && !bytes.Contains(capsBody, []byte(`"enabled":true`)) {
		t.Fatalf("capabilities missing faceit enabled: %s", capsBody)
	}

	lookupBody := getJSON(t, httpClient, srv.URL+"/api/faceit/players?nickname="+url.QueryEscape("m0NESY"), http.StatusOK)
	assertNoCredential(t, lookupBody, apiKey)
	var lookup struct {
		Player struct {
			ID         string `json:"id"`
			Nickname   string `json:"nickname"`
			ProfileURL string `json:"profile_url"`
			ELO        int    `json:"elo"`
		} `json:"player"`
	}
	if err := json.Unmarshal(lookupBody, &lookup); err != nil {
		t.Fatalf("decode lookup: %v", err)
	}
	if lookup.Player.ID == "" || !strings.EqualFold(lookup.Player.Nickname, "m0NESY") || lookup.Player.ProfileURL == "" {
		t.Fatalf("lookup player = %#v", lookup.Player)
	}

	urlLookup := getJSON(t, httpClient, srv.URL+"/api/faceit/players?nickname="+url.QueryEscape("https://www.faceit.com/en/players/m0NESY"), http.StatusOK)
	assertNoCredential(t, urlLookup, apiKey)
	if !bytes.Contains(urlLookup, []byte(lookup.Player.ID)) {
		t.Fatalf("url lookup did not return the same player")
	}

	missing := getJSON(t, httpClient, srv.URL+"/api/faceit/players?nickname=cliphub-no-such-player-xyz", http.StatusNotFound)
	assertNoCredential(t, missing, apiKey)

	invalid := getJSON(t, httpClient, srv.URL+"/api/faceit/players?nickname=", http.StatusBadRequest)
	assertNoCredential(t, invalid, apiKey)

	followBody := postJSON(t, httpClient, srv.URL+"/api/faceit/followed", `{"nickname":"m0NESY"}`, http.StatusOK)
	assertNoCredential(t, followBody, apiKey)
	if !bytes.Contains(followBody, []byte(lookup.Player.ID)) {
		t.Fatalf("follow response missing player id")
	}

	listed := getJSON(t, httpClient, srv.URL+"/api/faceit/followed", http.StatusOK)
	assertNoCredential(t, listed, apiKey)
	var list struct {
		Enabled bool `json:"enabled"`
		Players []struct {
			ID       string `json:"id"`
			Nickname string `json:"nickname"`
		} `json:"players"`
	}
	if err := json.Unmarshal(listed, &list); err != nil {
		t.Fatalf("decode followed: %v", err)
	}
	if !list.Enabled || len(list.Players) != 1 || list.Players[0].ID != lookup.Player.ID {
		t.Fatalf("followed = %#v", list)
	}

	reloaded, err := faceit.NewFollowStore(filepath.Join(dataDir, "faceit", "followed.json"), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := reloaded.List()
	if err != nil || len(persisted) != 1 || persisted[0].ID != lookup.Player.ID {
		t.Fatalf("persisted follows = %#v err=%v", persisted, err)
	}

	matchesBody := getJSON(t, httpClient, srv.URL+"/api/faceit/players/"+url.PathEscape(lookup.Player.ID)+"/matches?limit=5", http.StatusOK)
	assertNoCredential(t, matchesBody, apiKey)
	var matches struct {
		Matches []struct {
			ID      string `json:"id"`
			RoomURL string `json:"room_url"`
			Stats   *struct {
				Result string `json:"result"`
			} `json:"stats"`
		} `json:"matches"`
	}
	if err := json.Unmarshal(matchesBody, &matches); err != nil {
		t.Fatalf("decode matches: %v", err)
	}
	if len(matches.Matches) == 0 {
		t.Fatal("recent matches = 0, want at least one")
	}
	first := matches.Matches[0]
	if first.ID == "" || !strings.HasPrefix(first.RoomURL, "https://www.faceit.com/") {
		t.Fatalf("first match = %#v", first)
	}

	delReq, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/faceit/followed/"+url.PathEscape(lookup.Player.ID), nil)
	if err != nil {
		t.Fatal(err)
	}
	delRes, err := httpClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	delBody, _ := io.ReadAll(delRes.Body)
	_ = delRes.Body.Close()
	assertNoCredential(t, delBody, apiKey)
	if delRes.StatusCode != http.StatusNoContent {
		t.Fatalf("unfollow status = %d body=%s", delRes.StatusCode, delBody)
	}
	empty := getJSON(t, httpClient, srv.URL+"/api/faceit/followed", http.StatusOK)
	if !bytes.Contains(empty, []byte(`"players":[]`)) && !bytes.Contains(empty, []byte(`"players": []`)) {
		t.Fatalf("followed after unfollow = %s", empty)
	}
}

func TestFaceitZstdUploadE2E(t *testing.T) {
	info, err := os.Stat(faceitZstdDemoPath)
	if err != nil {
		t.Skip("FACEIT .dem.zst fixture is not on this machine")
	}

	dataDir := t.TempDir()
	repo := store.NewMemoryJobRepository()
	files, err := storage.NewLocal(dataDir)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	queue := newInlineQueue(map[string]taskHandler{
		tasks.TypeScanRoster: func(context.Context, *asynq.Task) error { return nil },
	}, 1)
	queueCtx, cancelQueue := context.WithCancel(context.Background())
	t.Cleanup(cancelQueue)
	queue.Start(queueCtx)
	handlers := httpapi.NewHandlers(repo, files, queue)

	srv := httptest.NewServer(httpapi.Routes(handlers))
	t.Cleanup(srv.Close)
	httpClient := srv.Client()
	httpClient.Timeout = 3 * time.Minute

	src, err := os.Open(faceitZstdDemoPath)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("demo", filepath.Base(faceitZstdDemoPath))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(part, src); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/jobs", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	res, err := httpClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("upload status = %d body=%s", res.StatusCode, redact(payload))
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(payload, &created); err != nil {
		t.Fatal(err)
	}
	id, err := uuid.Parse(created.ID)
	if err != nil {
		t.Fatalf("job id: %v", err)
	}
	stored, err := repo.Get(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	if stored.DemoFileName != "1-b5604ae7-c676-454b-901a-0b02014abd94-1-2.dem" {
		t.Fatalf("DemoFileName = %q, want stripped .zst", stored.DemoFileName)
	}
	rc, err := files.Open(stored.DemoPath)

	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	header := make([]byte, 7)
	if _, err := io.ReadFull(rc, header); err != nil {
		t.Fatal(err)
	}
	if string(header) != "PBDEMS2" {
		t.Fatalf("stored magic = %q, want PBDEMS2", header)
	}
	rest, err := io.Copy(io.Discard, rc)
	if err != nil {
		t.Fatal(err)
	}
	decompressed := rest + 7
	if decompressed <= info.Size() {
		t.Fatalf("decompressed %d bytes, compressed %d; want expansion", decompressed, info.Size())
	}
}

func getJSON(t *testing.T, client *http.Client, url string, want int) []byte {
	t.Helper()
	res, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != want {
		t.Fatalf("GET %s status = %d, want %d body=%s", url, res.StatusCode, want, redact(body))
	}
	return body
}

func postJSON(t *testing.T, client *http.Client, url, payload string, want int) []byte {
	t.Helper()
	res, err := client.Post(url, "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != want {
		t.Fatalf("POST %s status = %d, want %d body=%s", url, res.StatusCode, want, redact(body))
	}
	return body
}

func assertNoCredential(t *testing.T, body []byte, credential string) {
	t.Helper()
	if credential != "" && bytes.Contains(body, []byte(credential)) {
		t.Fatal("response contained FACEIT_API_KEY")
	}
}

func redact(body []byte) string {
	if len(body) > 400 {
		return string(body[:400]) + "…"
	}
	return string(body)
}
