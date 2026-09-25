package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/faceit"
	"github.com/rechedev9/cliphub/internal/httpapi"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/store"
)

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
	if !bytes.Contains(capsBody, []byte(`"faceit":{"enabled":true}`)) {
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
	if !list.Enabled || len(list.Players) < 1 || list.Players[0].ID != lookup.Player.ID {
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
	after := getJSON(t, httpClient, srv.URL+"/api/faceit/followed", http.StatusOK)
	var remaining struct {
		Players []struct {
			ID string `json:"id"`
		} `json:"players"`
	}
	if err := json.Unmarshal(after, &remaining); err != nil {
		t.Fatalf("decode followed after unfollow: %v", err)
	}
	for _, player := range remaining.Players {
		if player.ID == lookup.Player.ID {
			t.Fatalf("followed after unfollow still includes %s: %s", lookup.Player.ID, after)
		}
	}
	if len(remaining.Players) == 0 {
		t.Fatalf("followed after unfollow = %s, want the seeded default roster", after)
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
