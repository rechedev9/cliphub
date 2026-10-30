package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/tasks"
)

type libraryTestEnv struct {
	t       *testing.T
	repo    *store.MemoryLibraryRepository
	storage *fakeStorage
	queue   *fakeQueue
	router  http.Handler
}

func newLibraryTestEnv(t *testing.T) libraryTestEnv {
	t.Helper()
	repo := store.NewMemoryLibraryRepository()
	files := newFakeStorage()
	queue := &fakeQueue{}
	h := NewHandlers(newFakeRepo(), files, queue, WithLibrary(repo))
	return libraryTestEnv{t: t, repo: repo, storage: files, queue: queue, router: Routes(h)}
}

func (e libraryTestEnv) do(method, path string, body []byte, contentType string) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rw := httptest.NewRecorder()
	e.router.ServeHTTP(rw, req)
	return rw
}

func (e libraryTestEnv) json(method, path string, body any, wantStatus int, out any) {
	e.t.Helper()
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	rw := e.do(method, path, raw, "application/json")
	if rw.Code != wantStatus {
		e.t.Fatalf("%s %s = %d, want %d; body=%s", method, path, rw.Code, wantStatus, rw.Body.String())
	}
	if out != nil {
		if err := json.Unmarshal(rw.Body.Bytes(), out); err != nil {
			e.t.Fatalf("decode %s %s: %v", method, path, err)
		}
	}
}

// seedLibraryMatch registers a demo, stores the tactical fixture under its
// library keys and saves the analysis the worker would have produced.
func (e libraryTestEnv) seedLibraryMatch(sha, ctClan, tClan string) library.Match {
	e.t.Helper()
	ctx := context.Background()
	d := library.Demo{ID: uuid.New(), SHA256: sha, FileName: sha + ".dem"}
	d.Path = artifacts.LibraryDemoKey(d.ID)
	if _, err := e.repo.CreateDemo(ctx, &d); err != nil {
		e.t.Fatal(err)
	}
	doc, blob := tacticalFixture(e.t, uuid.Nil)
	doc.Teams = []tacticalplan.Team{
		{Key: "ct-start", Name: ctClan, StartSide: tacticalplan.SideCT},
		{Key: "t-start", Name: tClan, StartSide: tacticalplan.SideT},
	}
	doc.Players = []tacticalplan.Player{
		{Slot: 0, SteamID64: "76561198000000001", Name: "one", TeamKey: "ct-start", StartSide: tacticalplan.SideCT},
		{Slot: 1, SteamID64: "76561198000000002", Name: "two", TeamKey: "t-start", StartSide: tacticalplan.SideT},
	}
	b, _ := json.Marshal(doc)
	_ = e.storage.Put(artifacts.LibraryTacticalIndexKey(d.ID), bytes.NewReader(b))
	_ = e.storage.Put(artifacts.LibraryTacticalPositionsKey(d.ID), bytes.NewReader(blob.Data))
	m, err := e.repo.SaveAnalysis(ctx, library.AnalysisFromDocument(d.ID, doc))
	if err != nil {
		e.t.Fatal(err)
	}
	return m
}

func TestImportLibraryDemosStoresDeduplicatesAndRejectsPerFile(t *testing.T) {
	env := newLibraryTestEnv(t)
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	_ = mw.WriteField("tag", "rival")
	_ = mw.WriteField("played_at", "2026-10-01T18:00:00Z")
	demo := append(append([]byte{}, demoMagic...), []byte("same bytes")...)
	for _, file := range []struct {
		name string
		data []byte
	}{
		{"1-9be6d5a6-abda-4110-908d-deae67dee533-1-1.dem", demo},
		{"copy.dem", demo},
		{"old.dem", []byte("HL2DEMO\x00rest")},
		{"notes.txt", []byte("hello")},
	} {
		part, _ := mw.CreateFormFile("demo", file.name)
		_, _ = part.Write(file.data)
	}
	_ = mw.Close()

	rw := env.do(http.MethodPost, "/api/library/demos", body.Bytes(), mw.FormDataContentType())
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rw.Code, rw.Body.String())
	}
	var resp struct {
		Demos []libraryImportResult `json:"demos"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	wantStatus := []string{libraryImportImported, libraryImportDuplicate, libraryImportRejected, libraryImportRejected}
	wantCode := []string{"", "", codeCSGODemo, codeNotADemo}
	if len(resp.Demos) != len(wantStatus) {
		t.Fatalf("results = %+v", resp.Demos)
	}
	for i, r := range resp.Demos {
		if r.Status != wantStatus[i] || r.ErrorCode != wantCode[i] {
			t.Fatalf("result %d = %+v, want %s %s", i, r, wantStatus[i], wantCode[i])
		}
	}
	imported := resp.Demos[0].Demo
	if imported == nil || resp.Demos[1].Demo == nil || resp.Demos[1].Demo.ID != imported.ID {
		t.Fatalf("duplicate does not point at the imported demo: %+v", resp.Demos[:2])
	}
	if imported.Source != library.SourceFaceit || imported.State != library.ParseQueued || len(imported.Tags) != 1 || imported.Tags[0] != "rival" {
		t.Fatalf("imported demo = %+v", imported)
	}
	if imported.PlayedAt == nil || !imported.PlayedAt.Equal(time.Date(2026, 10, 1, 18, 0, 0, 0, time.UTC)) {
		t.Fatalf("played_at = %v", imported.PlayedAt)
	}
	if imported.SizeBytes != int64(len(demo)) || imported.SHA256 == "" {
		t.Fatalf("size/hash = %d/%q", imported.SizeBytes, imported.SHA256)
	}
	if !bytes.Equal(env.storage.puts[artifacts.LibraryDemoKey(imported.ID)], demo) {
		t.Fatal("demo bytes not stored under the library key")
	}
	stored := 0
	for key := range env.storage.puts {
		if strings.HasPrefix(key, "library/") {
			stored++
		}
	}
	if stored != 1 {
		t.Fatalf("library files = %d, want only the imported demo (the duplicate is discarded)", stored)
	}
	if len(env.queue.enqueued) != 1 || env.queue.enqueued[0].Type() != tasks.TypeAnalyzeLibraryDemo {
		t.Fatalf("enqueued = %+v, want one library scan", env.queue.enqueued)
	}
}

func TestImportLibraryDemosRejectsABodyWithoutDemos(t *testing.T) {
	env := newLibraryTestEnv(t)
	if rw := env.do(http.MethodPost, "/api/library/demos", []byte(`{}`), "application/json"); rw.Code != http.StatusBadRequest {
		t.Fatalf("JSON body = %d, want 400", rw.Code)
	}
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	_ = mw.WriteField("tag", "x")
	_ = mw.Close()
	if rw := env.do(http.MethodPost, "/api/library/demos", body.Bytes(), mw.FormDataContentType()); rw.Code != http.StatusBadRequest {
		t.Fatalf("no demo part = %d, want 400", rw.Code)
	}
}

func TestLibraryMatchEndpointsListFilterCorrectAndServeEvidence(t *testing.T) {
	env := newLibraryTestEnv(t)
	first := env.seedLibraryMatch("a", "team_apEX", "MOUZ")
	second := env.seedLibraryMatch("b", "team_apex", "FaZe")

	var list struct {
		Matches []library.Match `json:"matches"`
	}
	env.json(http.MethodGet, "/api/library/matches?map=de_mirage&limit=10", nil, http.StatusOK, &list)
	if len(list.Matches) != 2 {
		t.Fatalf("matches = %+v", list.Matches)
	}
	mouz := teamOfMatch(t, first, "t-start").TeamID
	env.json(http.MethodGet, "/api/library/matches?team_id="+mouz.String(), nil, http.StatusOK, &list)
	if len(list.Matches) != 1 || list.Matches[0].ID != first.ID {
		t.Fatalf("team filter = %+v", list.Matches)
	}
	for _, bad := range []string{"?team_id=x", "?steamid=abc", "?from=yesterday", "?limit=0", "?source=hltv2", "?map=De%20Mirage"} {
		if rw := env.do(http.MethodGet, "/api/library/matches"+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Fatalf("GET matches%s = %d, want 400", bad, rw.Code)
		}
	}

	var fixed library.Match
	env.json(http.MethodPut, "/api/library/matches/"+first.ID.String()+"/teams/ct-start",
		map[string]any{"team_name": "Team Vitality", "remember_alias": true}, http.StatusOK, &fixed)
	ct := teamOfMatch(t, fixed, "ct-start")
	if ct.TeamName != "Team Vitality" || ct.Assignment != library.AssignmentManual {
		t.Fatalf("corrected side = %+v", ct)
	}
	var other library.Match
	env.json(http.MethodGet, "/api/library/matches/"+second.ID.String(), nil, http.StatusOK, &other)
	if got := teamOfMatch(t, other, "ct-start"); got.TeamID == nil || *got.TeamID != *ct.TeamID {
		t.Fatalf("remembered alias did not follow to the other match: %+v", got)
	}
	if len(other.Players) != 2 || other.Players[0].SteamID64 != "76561198000000001" {
		t.Fatalf("match players = %+v", other.Players)
	}
	if rw := env.do(http.MethodPut, "/api/library/matches/"+first.ID.String()+"/teams/t-start",
		[]byte(`{"team_id":"`+ct.TeamID.String()+`"}`), "application/json"); rw.Code != http.StatusConflict {
		t.Fatalf("same team on both sides = %d, want 409", rw.Code)
	}

	var series library.Series
	env.json(http.MethodPost, "/api/library/series", map[string]any{"format": "bo3", "event": "IEM"}, http.StatusCreated, &series)
	var patched library.Match
	env.json(http.MethodPatch, "/api/library/matches/"+first.ID.String(),
		map[string]any{"series_id": series.ID, "played_at": "2026-09-30T20:00:00Z"}, http.StatusOK, &patched)
	if patched.SeriesID == nil || *patched.SeriesID != series.ID || !patched.PlayedAt.Equal(time.Date(2026, 9, 30, 20, 0, 0, 0, time.UTC)) {
		t.Fatalf("patched = %+v", patched)
	}
	var detached library.Match
	env.json(http.MethodPatch, "/api/library/matches/"+first.ID.String(), map[string]any{"series_id": nil}, http.StatusOK, &detached)
	if detached.SeriesID != nil || !detached.PlayedAt.Equal(patched.PlayedAt) {
		t.Fatalf("series_id null did not detach: %+v", detached)
	}
	if rw := env.do(http.MethodPost, "/api/library/series", []byte(`{"format":"bo2"}`), "application/json"); rw.Code != http.StatusBadRequest {
		t.Fatalf("bo2 series = %d, want 400", rw.Code)
	}

	var doc tacticalplan.Document
	env.json(http.MethodGet, "/api/library/matches/"+first.ID.String()+"/tactical", nil, http.StatusOK, &doc)
	if len(doc.Rounds) != 2 {
		t.Fatalf("tactical rounds = %d", len(doc.Rounds))
	}
	var round tacticalRoundResponse
	env.json(http.MethodGet, "/api/library/matches/"+first.ID.String()+"/tactical/rounds/2", nil, http.StatusOK, &round)
	if round.Round.Number != 2 || len(round.Frames) != 1 || round.Frames[0].Tick != 500 {
		t.Fatalf("round 2 = %+v", round)
	}
	if rw := env.do(http.MethodGet, "/api/library/matches/"+first.ID.String()+"/tactical/aggregate", nil, ""); rw.Code != http.StatusOK {
		t.Fatalf("aggregate = %d; body=%s", rw.Code, rw.Body.String())
	}
	if rw := env.do(http.MethodGet, "/api/library/matches/"+uuid.NewString(), nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("unknown match = %d, want 404", rw.Code)
	}
}

func teamOfMatch(t *testing.T, m library.Match, key string) library.MatchTeam {
	t.Helper()
	for _, team := range m.Teams {
		if team.Key == key {
			return team
		}
	}
	t.Fatalf("no side %s in %+v", key, m.Teams)
	return library.MatchTeam{}
}

func TestLibraryTeamAndPlayerEndpoints(t *testing.T) {
	env := newLibraryTestEnv(t)
	m := env.seedLibraryMatch("a", "Vitality", "MOUZ")
	vit := teamOfMatch(t, m, "ct-start").TeamID

	if rw := env.do(http.MethodPost, "/api/library/teams", []byte(`{"name":"Otro","aliases":["mouz"]}`), "application/json"); rw.Code != http.StatusConflict {
		t.Fatalf("taken alias = %d, want 409", rw.Code)
	}
	if rw := env.do(http.MethodPost, "/api/library/teams", []byte(`{"name":"   "}`), "application/json"); rw.Code != http.StatusBadRequest {
		t.Fatalf("empty name = %d, want 400", rw.Code)
	}
	var spirit library.Team
	env.json(http.MethodPost, "/api/library/teams", map[string]any{"name": "Spirit", "aliases": []string{"Team Spirit"}}, http.StatusCreated, &spirit)
	var merged library.Team
	env.json(http.MethodPost, "/api/library/teams/"+spirit.ID.String()+"/merge", map[string]any{"into_team_id": vit}, http.StatusOK, &merged)
	if merged.ID != *vit || len(merged.Aliases) != 2 || len(merged.Roster) != 1 || merged.Roster[0].SteamID64 != "76561198000000001" {
		t.Fatalf("merged = %+v", merged)
	}
	var renamed library.Team
	env.json(http.MethodPatch, "/api/library/teams/"+vit.String(), map[string]any{"name": "Team Vitality"}, http.StatusOK, &renamed)
	if renamed.Name != "Team Vitality" {
		t.Fatalf("renamed = %+v", renamed)
	}
	var teams struct {
		Teams []library.Team `json:"teams"`
	}
	env.json(http.MethodGet, "/api/library/teams?q=spirit", nil, http.StatusOK, &teams)
	if len(teams.Teams) != 1 || teams.Teams[0].ID != *vit {
		t.Fatalf("team search by alias = %+v", teams.Teams)
	}

	var players struct {
		Players []library.Player `json:"players"`
	}
	env.json(http.MethodGet, "/api/library/players?team_id="+vit.String(), nil, http.StatusOK, &players)
	if len(players.Players) != 1 || players.Players[0].Nick != "one" || players.Players[0].MatchCount != 1 {
		t.Fatalf("players on team = %+v", players.Players)
	}
	var player library.Player
	env.json(http.MethodGet, "/api/library/players/76561198000000001", nil, http.StatusOK, &player)
	if len(player.Teams) != 1 || player.Teams[0].TeamName != "Team Vitality" || len(player.Nicks) != 1 {
		t.Fatalf("player = %+v", player)
	}
	if rw := env.do(http.MethodGet, "/api/library/players/not-a-steamid", nil, ""); rw.Code != http.StatusBadRequest {
		t.Fatalf("bad steamid = %d, want 400", rw.Code)
	}
	if rw := env.do(http.MethodPost, "/api/library/players/76561198000000001/faceit", nil, ""); rw.Code != http.StatusServiceUnavailable {
		t.Fatalf("FACEIT link without a client = %d, want 503", rw.Code)
	}
}

func TestLibraryDemoRetryDeleteAndResume(t *testing.T) {
	env := newLibraryTestEnv(t)
	ctx := context.Background()
	failed := library.Demo{SHA256: "f", FileName: "f.dem"}
	stuck := library.Demo{SHA256: "s", FileName: "s.dem"}
	for _, d := range []*library.Demo{&failed, &stuck} {
		if _, err := env.repo.CreateDemo(ctx, d); err != nil {
			t.Fatal(err)
		}
		_ = env.storage.Put(artifacts.LibraryDemoKey(d.ID), bytes.NewReader([]byte("demo")))
	}
	_ = env.repo.SetDemoState(ctx, failed.ID, library.ParseFailed, "demo_unreadable", "boom")
	_ = env.repo.SetDemoState(ctx, stuck.ID, library.ParseParsing, "", "")

	var listed struct {
		Demos []library.Demo `json:"demos"`
	}
	env.json(http.MethodGet, "/api/library/demos?state=failed", nil, http.StatusOK, &listed)
	if len(listed.Demos) != 1 || listed.Demos[0].ID != failed.ID || listed.Demos[0].ErrorCode != "demo_unreadable" {
		t.Fatalf("failed demos = %+v", listed.Demos)
	}
	if rw := env.do(http.MethodGet, "/api/library/demos?state=done", nil, ""); rw.Code != http.StatusBadRequest {
		t.Fatalf("unknown state = %d, want 400", rw.Code)
	}

	var retried library.Demo
	env.json(http.MethodPost, "/api/library/demos/"+failed.ID.String()+"/analyze", nil, http.StatusAccepted, &retried)
	if retried.State != library.ParseQueued || retried.Error != "" || len(env.queue.enqueued) != 1 {
		t.Fatalf("retried = %+v, enqueued %d", retried, len(env.queue.enqueued))
	}

	var patched library.Demo
	env.json(http.MethodPatch, "/api/library/demos/"+failed.ID.String(), map[string]any{"source": "scrim", "tags": []string{"b", "a"}}, http.StatusOK, &patched)
	if patched.Source != library.SourceScrim || !patched.SourceManual || len(patched.Tags) != 2 {
		t.Fatalf("patched = %+v", patched)
	}
	if rw := env.do(http.MethodPatch, "/api/library/demos/"+failed.ID.String(), []byte(`{"source":"esea"}`), "application/json"); rw.Code != http.StatusBadRequest {
		t.Fatalf("unknown source = %d, want 400", rw.Code)
	}

	h := NewHandlers(newFakeRepo(), env.storage, env.queue, WithLibrary(env.repo))
	resumed, err := h.ResumeLibraryScans(ctx)
	if err != nil || resumed != 2 {
		t.Fatalf("ResumeLibraryScans = %d, %v; want the queued and the parsing demo", resumed, err)
	}
	if d, _ := env.repo.GetDemo(ctx, stuck.ID); d.State != library.ParseQueued {
		t.Fatalf("stuck demo state = %s, want queued", d.State)
	}

	if rw := env.do(http.MethodDelete, "/api/library/demos/"+failed.ID.String(), nil, ""); rw.Code != http.StatusNoContent {
		t.Fatalf("delete = %d", rw.Code)
	}
	if _, ok := env.storage.puts[artifacts.LibraryDemoKey(failed.ID)]; ok {
		t.Fatal("deleted demo file is still stored")
	}
	if rw := env.do(http.MethodGet, "/api/library/demos/"+failed.ID.String(), nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("get deleted = %d, want 404", rw.Code)
	}
}

func TestLibraryRoutesWithoutARepositoryAreNotImplemented(t *testing.T) {
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{})
	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/library/matches", nil))
	if rw.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", rw.Code)
	}
}
