package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/faceit"
	"github.com/rechedev9/cliphub/internal/job"
)

func TestStoreFullDemoFaceitWritesFollowedSteamIDsOnly(t *testing.T) {
	t.Parallel()
	follows, err := faceit.NewFollowStore(filepath.Join(t.TempDir(), "followed.json"), func() time.Time {
		return time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := follows.Follow(faceit.Player{
		ID:         "player-1",
		Nickname:   "donk666",
		SteamID64:  "76561198000000001",
		Country:    "ru",
		ELO:        4370,
		SkillLevel: 10,
		ProfileURL: "https://www.faceit.com/en/players/donk666",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := follows.Follow(faceit.Player{
		ID:         "player-2",
		Nickname:   "no-steam",
		ELO:        3000,
		ProfileURL: "https://www.faceit.com/en/players/nosteam",
	}); err != nil {
		t.Fatal(err)
	}

	store := newFakeStorage()
	h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithFaceit(nil, follows))
	id := uuid.New()
	h.storeFullDemoFaceit(job.Job{ID: id})

	rc, err := store.Open(artifacts.FullDemoFaceitKey(id))
	if err != nil {
		t.Fatalf("open enrichment: %v", err)
	}
	defer rc.Close()
	var got map[string]demooverlay.Enrichment
	if err := json.NewDecoder(rc).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("enrichment = %#v, want only the followed SteamID", got)
	}
	en := got["76561198000000001"]
	if en.Nickname != "donk666" || en.Country != "ru" || en.ELO != 4370 || en.SkillLevel != 10 {
		t.Fatalf("enrichment = %+v", en)
	}
	if en.Last20 != nil {
		t.Fatalf("last-20 invented from follows: %+v", en.Last20)
	}
	if en.Ranking != nil {
		t.Fatalf("ranking invented from follows: %+v", en.Ranking)
	}
}

func TestStoreFullDemoFaceitLooksUpRosterViaDataAPI(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/players" && r.URL.Query().Get("game_player_id") == "76561198000000001":
			_, _ = w.Write([]byte(`{"player_id":"player-1","nickname":"donk666","country":"ru","avatar":"https://assets.faceit-cdn.net/avatars/donk.png","steam_id_64":"76561198000000001","games":{"cs2":{"region":"EU","skill_level":10,"faceit_elo":4370}}}`))
		case strings.HasPrefix(r.URL.Path, "/players/player-1/history"), strings.HasPrefix(r.URL.Path, "/players/player-1/games/cs2/stats"):
			_, _ = w.Write([]byte(`{"items":[]}`))
		case r.URL.Path == "/rankings/games/cs2/regions/EU/players/player-1":
			_, _ = w.Write([]byte(`{"position":1}`))
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
	_ = store.Put(artifacts.RosterKey(id), bytes.NewReader([]byte(`{"players":[{"steamid64":"76561198000000001","name":"donk666","team":"CT"}]}`)))
	h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithFaceit(client, nil))
	h.storeFullDemoFaceit(job.Job{ID: id})

	rc, err := store.Open(artifacts.FullDemoFaceitKey(id))
	if err != nil {
		t.Fatalf("open enrichment: %v", err)
	}
	defer rc.Close()
	var got map[string]demooverlay.Enrichment
	if err := json.NewDecoder(rc).Decode(&got); err != nil {
		t.Fatal(err)
	}
	en := got["76561198000000001"]
	if en.Nickname != "donk666" || en.ELO != 4370 || en.Ranking == nil || *en.Ranking != 1 {
		t.Fatalf("enrichment = %+v", en)
	}
	if en.Last20 != nil {
		t.Fatalf("empty match history invented last-20: %+v", en.Last20)
	}
}

func TestStoreFullDemoFaceitSkipsWhenNothingFollowed(t *testing.T) {
	t.Parallel()
	follows, err := faceit.NewFollowStore(filepath.Join(t.TempDir(), "followed.json"), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	store := newFakeStorage()
	h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithFaceit(nil, follows))
	id := uuid.New()
	h.storeFullDemoFaceit(job.Job{ID: id})
	if _, err := store.Open(artifacts.FullDemoFaceitKey(id)); err == nil {
		t.Fatal("wrote FACEIT sidecar with no followed players")
	}
}
