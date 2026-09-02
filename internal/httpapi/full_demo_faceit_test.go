package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/faceit"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/parser"
)

func TestStoreFullDemoFaceitRequiresAndPersistsCompleteRoster(t *testing.T) {
	t.Parallel()
	const (
		steamOne = "76561198000000001"
		steamTwo = "76561198000000002"
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		steamID := r.URL.Query().Get("game_player_id")
		switch {
		case r.URL.Path == "/players" && (steamID == steamOne || steamID == steamTwo):
			n := 1
			if steamID == steamTwo {
				n = 2
			}
			_, _ = fmt.Fprintf(w, `{"player_id":"player-%d","nickname":"player%d","country":"es","steam_id_64":"%s","games":{"cs2":{"region":"EU","skill_level":10,"faceit_elo":%d}}}`, n, n, steamID, 3000+n)
		case strings.Contains(r.URL.Path, "/history"), strings.Contains(r.URL.Path, "/games/cs2/stats"):
			_, _ = w.Write([]byte(`{"items":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := faceit.New(faceit.Options{APIKey: "faceit-test-key", BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	store := newFakeStorage()
	id := uuid.New()
	roster := parser.RosterResult{Players: []parser.PlayerStat{
		{SteamID64: steamOne, Name: "one", Team: "CT"},
		{SteamID64: steamTwo, Name: "two", Team: "T"},
	}}
	body, err := json.Marshal(roster)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(artifacts.RosterKey(id), bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithFaceit(client, nil))
	if err := h.storeFullDemoFaceit(context.Background(), job.Job{ID: id}); err != nil {
		t.Fatal(err)
	}

	rc, err := store.Open(artifacts.FullDemoFaceitKey(id))
	if err != nil {
		t.Fatalf("open enrichment: %v", err)
	}
	defer rc.Close()
	var got map[string]demooverlay.Enrichment
	if err := json.NewDecoder(rc).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[steamOne].Nickname != "player1" || got[steamTwo].ELO != 3002 {
		t.Fatalf("enrichment = %#v", got)
	}
}

func TestStoreFullDemoFaceitRejectsMissingRosterPlayer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()
	client, err := faceit.New(faceit.Options{APIKey: "faceit-test-key", BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	store := newFakeStorage()
	id := uuid.New()
	body := []byte(`{"players":[{"steamid64":"76561198000000001","name":"missing","team":"CT"}]}`)
	if err := store.Put(artifacts.RosterKey(id), bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithFaceit(client, nil))
	err = h.storeFullDemoFaceit(context.Background(), job.Job{ID: id})
	if !errors.Is(err, faceit.ErrPlayerNotFound) {
		t.Fatalf("error = %v, want ErrPlayerNotFound", err)
	}
	if _, openErr := store.Open(artifacts.FullDemoFaceitKey(id)); openErr == nil {
		t.Fatal("stored a partial FACEIT roster")
	}
}
