package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/faceit"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/renderplan"
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

func TestStoreFullDemoSteamAvatarsSnapshotsPublicAndPrivateWithoutFACEITFields(t *testing.T) {
	t.Parallel()
	const (
		publicID  = "76561198000000001"
		privateID = "76561198000000002"
		missingID = "76561198000000003"
	)
	store := newFakeStorage()
	id := uuid.New()
	roster, err := json.Marshal(parser.RosterResult{Players: []parser.PlayerStat{
		{SteamID64: publicID, Name: "demo-name", Team: "CT"},
		{SteamID64: privateID, Name: "private-name", Team: "T"},
		{SteamID64: missingID, Name: "missing-name", Team: "T"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(artifacts.RosterKey(id), bytes.NewReader(roster)); err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithSteamAvatarResolver(&fakeSteamAvatarResolver{avatars: map[string]faceit.SteamAvatar{
		publicID:  {URL: "https://avatars.akamai.steamstatic.com/public.jpg"},
		privateID: {URL: "https://avatars.akamai.steamstatic.com/private.jpg", Private: true},
	}}))
	if err := h.storeFullDemoSteamAvatars(context.Background(), job.Job{ID: id}); err != nil {
		t.Fatal(err)
	}
	rc, err := store.Open(artifacts.FullDemoSteamAvatarsKey(id))
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	var got map[string]faceit.SteamAvatar
	if err := json.NewDecoder(rc).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got[publicID].URL == "" || got[publicID].Private {
		t.Fatalf("public snapshot = %+v", got)
	}
	if !got[privateID].Private || got[privateID].URL != "" {
		t.Fatalf("private snapshot = %+v", got)
	}
	if _, ok := got[missingID]; ok {
		t.Fatalf("missing profile was invented: %+v", got)
	}
}

type fakeSteamAvatarResolver struct {
	avatars map[string]faceit.SteamAvatar
	calls   [][]string
}

func (f *fakeSteamAvatarResolver) ResolveSteamAvatars(_ context.Context, ids []string) map[string]faceit.SteamAvatar {
	f.calls = append(f.calls, append([]string(nil), ids...))
	return f.avatars
}

func TestStartGenerateLocalFullDemoSnapshotsSteamAvatars(t *testing.T) {
	const (
		publicID  = "76561198000000001"
		privateID = "76561198000000002"
		missingID = "76561198000000003"
	)
	h, j, store, queue, options := fullDemoAPIFixture(t)
	resolver := &fakeSteamAvatarResolver{avatars: map[string]faceit.SteamAvatar{
		publicID:  {URL: "https://avatars.akamai.steamstatic.com/public.jpg"},
		privateID: {URL: "https://avatars.akamai.steamstatic.com/private.jpg", Private: true},
	}}
	h = NewHandlers(h.repo, store, queue,
		WithCapabilities(Capabilities{RecordEnabled: true}),
		WithSteamAvatarResolver(resolver),
	)
	roster, err := json.Marshal(parser.RosterResult{Players: []parser.PlayerStat{
		{SteamID64: publicID, Name: "demo-public", Team: "CT"},
		{SteamID64: privateID, Name: "demo-private", Team: "T"},
		{SteamID64: missingID, Name: "demo-missing", Team: "T"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(artifacts.RosterKey(j.ID), bytes.NewReader(roster)); err != nil {
		t.Fatal(err)
	}
	snapshot := fullDemoAPIPlan(t, h, j, options)
	body := map[string]any{"preset": "gameplay-pov-60", "edit": renderplan.FullDemoEditRequest(snapshot)}
	if rw := fullDemoAPIRequest(t, h, j, "/generate", body); rw.Code != http.StatusAccepted {
		t.Fatalf("generate = %d: %s", rw.Code, rw.Body.String())
	}
	if len(resolver.calls) != 1 || strings.Join(resolver.calls[0], ",") != strings.Join([]string{publicID, privateID, missingID}, ",") {
		t.Fatalf("Steam avatar calls = %#v", resolver.calls)
	}
	rc, err := store.Open(artifacts.FullDemoSteamAvatarsKey(j.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	var got map[string]faceit.SteamAvatar
	if err := json.NewDecoder(rc).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got[publicID].URL == "" || got[publicID].Private || !got[privateID].Private || got[privateID].URL != "" {
		t.Fatalf("stored local avatar snapshot = %#v", got)
	}
	if _, ok := got[missingID]; ok {
		t.Fatalf("missing profile was invented: %#v", got)
	}
	if _, err := store.Open(artifacts.FullDemoFaceitKey(j.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("local generate stored FACEIT enrichment: %v", err)
	}
	queue.err = asynq.ErrDuplicateTask
	if rw := fullDemoAPIRequest(t, h, j, "/generate", body); rw.Code != http.StatusAccepted {
		t.Fatalf("duplicate generate = %d: %s", rw.Code, rw.Body.String())
	}
	if len(resolver.calls) != 1 {
		t.Fatalf("duplicate lookup ignored durable snapshot: %#v", resolver.calls)
	}
}

func TestStartGenerateShortDoesNotResolveSteamAvatars(t *testing.T) {
	repo, store, queue := newFakeRepo(), newFakeStorage(), &fakeQueue{}
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, KillPlan: &plan}
	repo.jobs[j.ID] = j
	resolver := &fakeSteamAvatarResolver{}
	h := NewHandlers(repo, store, queue,
		WithCapabilities(Capabilities{RecordEnabled: true}),
		WithSteamAvatarResolver(resolver),
	)
	if rw := postGenerate(t, h, j.ID, `{"preset":"clean-pov-60","edit":{"intro":true}}`); rw.Code != http.StatusAccepted {
		t.Fatalf("short generate = %d: %s", rw.Code, rw.Body.String())
	}
	if len(resolver.calls) != 0 {
		t.Fatalf("short generate resolved Steam avatars: %#v", resolver.calls)
	}
}

func TestStartGenerateLocalFullDemoAllowsUnavailableSteamAvatars(t *testing.T) {
	h, j, store, queue, options := fullDemoAPIFixture(t)
	resolver := &fakeSteamAvatarResolver{}
	h = NewHandlers(h.repo, store, queue,
		WithCapabilities(Capabilities{RecordEnabled: true}),
		WithSteamAvatarResolver(resolver),
	)
	roster := []byte(`{"players":[{"steamid64":"76561198000000001","name":"demo-player","team":"CT"}]}`)
	if err := store.Put(artifacts.RosterKey(j.ID), bytes.NewReader(roster)); err != nil {
		t.Fatal(err)
	}
	snapshot := fullDemoAPIPlan(t, h, j, options)
	if rw := fullDemoAPIRequest(t, h, j, "/generate", map[string]any{
		"preset": "gameplay-pov-60", "edit": renderplan.FullDemoEditRequest(snapshot),
	}); rw.Code != http.StatusAccepted {
		t.Fatalf("unavailable Steam generate = %d: %s", rw.Code, rw.Body.String())
	}
	if len(resolver.calls) != 1 {
		t.Fatalf("Steam avatar calls = %#v", resolver.calls)
	}
	rc, err := store.Open(artifacts.FullDemoSteamAvatarsKey(j.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	var got map[string]faceit.SteamAvatar
	if err := json.NewDecoder(rc).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("unavailable Steam lookup invented avatars: %#v", got)
	}
}
