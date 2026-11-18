package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/faceit"
)

type rankingBody struct {
	Kind      string    `json:"kind"`
	Code      string    `json:"code"`
	Total     int       `json:"total"`
	FetchedAt time.Time `json:"fetched_at"`
	Stale     bool      `json:"stale"`
	StaleCode string    `json:"stale_code"`
	Players   []struct {
		Rank       int    `json:"rank"`
		ID         string `json:"id"`
		Nickname   string `json:"nickname"`
		Country    string `json:"country"`
		ELO        int    `json:"elo"`
		SkillLevel int    `json:"skill_level"`
		ProfileURL string `json:"profile_url"`
	} `json:"players"`
}

// rankingRouter serves a FACEIT whose Spanish players sit on EU and NA, with
// the failure status the test sets, behind a ranking store on a fixed clock.
func rankingRouter(t *testing.T, failWith *atomic.Int32) (http.Handler, *atomic.Int32, *time.Time) {
	t.Helper()
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if status := int(failWith.Load()); status != 0 {
			w.WriteHeader(status)
			return
		}
		items := `[]`
		switch {
		case strings.HasSuffix(r.URL.Path, "/EU") && r.URL.Query().Get("country") == "es":
			items = `[{"player_id":"es-eu","nickname":"mopoz","country":"es","position":1,"faceit_elo":3900,"game_skill_level":10},
				{"player_id":"es-low","nickname":"low","country":"es","position":2,"faceit_elo":2100,"game_skill_level":8}]`
		case strings.HasSuffix(r.URL.Path, "/NA") && r.URL.Query().Get("country") == "es":
			items = `[{"player_id":"es-na","nickname":"dav1g","country":"es","position":1,"faceit_elo":4100,"game_skill_level":10}]`
		}
		_, _ = w.Write([]byte(`{"items":` + items + `}`))
	}))
	t.Cleanup(server.Close)
	client, err := faceit.New(faceit.Options{APIKey: "faceit-test-key", BaseURL: server.URL, HTTPClient: server.Client(), RequestTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.November, 18, 12, 0, 0, 0, time.UTC)
	rankings, err := faceit.NewRankingStore(t.TempDir(), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithFaceit(client, nil), WithFaceitRankings(rankings))
	return Routes(h), &hits, &now
}

func getRanking(t *testing.T, router http.Handler, query string) (int, rankingBody, string) {
	t.Helper()
	rw := httptest.NewRecorder()
	router.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/faceit/rankings?"+query, nil))
	var body rankingBody
	if rw.Code == http.StatusOK {
		if err := json.Unmarshal(rw.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode %s: %v", rw.Body.String(), err)
		}
	}
	return rw.Code, body, rw.Body.String()
}

func TestFaceitRankingPagesACountryFromOneRead(t *testing.T) {
	t.Parallel()
	var failWith atomic.Int32
	router, hits, now := rankingRouter(t, &failWith)

	status, first, raw := getRanking(t, router, "kind=country&code=ES&offset=0&limit=2")
	if status != http.StatusOK {
		t.Fatalf("status = %d body=%s", status, raw)
	}
	if first.Kind != "country" || first.Code != "es" || first.Total != 3 || first.Stale || !first.FetchedAt.Equal(*now) || len(first.Players) != 2 {
		t.Fatalf("first page = %+v", first)
	}
	if p := first.Players[0]; p.Rank != 1 || p.ID != "es-na" || p.ELO != 4100 || p.Country != "es" || p.SkillLevel != 10 ||
		p.ProfileURL != "https://www.faceit.com/en/players/dav1g" {
		t.Fatalf("first row = %+v, want dav1g at rank 1", p)
	}
	if first.Players[1].Rank != 2 || first.Players[1].ID != "es-eu" {
		t.Fatalf("second row = %+v", first.Players[1])
	}
	reads := hits.Load()

	_, second, _ := getRanking(t, router, "kind=country&code=es&offset=2&limit=2")
	if len(second.Players) != 1 || second.Players[0].Rank != 3 || second.Players[0].ID != "es-low" || second.Total != 3 {
		t.Fatalf("second page = %+v", second)
	}
	if hits.Load() != reads {
		t.Fatalf("second page reached FACEIT: %d -> %d requests", reads, hits.Load())
	}
	if _, past, _ := getRanking(t, router, "kind=country&code=es&offset=40"); len(past.Players) != 0 || past.Total != 3 {
		t.Fatalf("page past the end = %+v, want no rows", past)
	}
}

func TestFaceitRankingServesTheStoredCopyOrSaysWhyFACEITFailed(t *testing.T) {
	t.Parallel()
	var failWith atomic.Int32
	router, _, now := rankingRouter(t, &failWith)
	if status, _, raw := getRanking(t, router, "kind=country&code=es"); status != http.StatusOK {
		t.Fatalf("status = %d body=%s", status, raw)
	}
	fetchedAt := *now

	failWith.Store(http.StatusInternalServerError)
	*now = now.Add(7 * time.Hour)
	status, stale, raw := getRanking(t, router, "kind=country&code=es")
	if status != http.StatusOK || !stale.Stale || stale.StaleCode != "faceit_unavailable" || !stale.FetchedAt.Equal(fetchedAt) || len(stale.Players) != 3 {
		t.Fatalf("FACEIT down: status=%d body=%s, want the copy of %v marked stale", status, raw, fetchedAt)
	}

	failWith.Store(http.StatusTooManyRequests)
	status, _, raw = getRanking(t, router, "kind=region&code=EU")
	if status != http.StatusTooManyRequests || !strings.Contains(raw, "faceit_rate_limited") {
		t.Fatalf("rate limited without a copy: status=%d body=%s, want 429 faceit_rate_limited", status, raw)
	}
}

func TestFaceitRankingRejectsInvalidRequests(t *testing.T) {
	t.Parallel()
	var failWith atomic.Int32
	router, hits, _ := rankingRouter(t, &failWith)
	for _, query := range []string{
		"code=es",
		"kind=team&code=es",
		"kind=country",
		"kind=country&code=esp",
		"kind=country&code=zz",
		"kind=country&code=eu",
		"kind=region&code=GLOBAL",
		"kind=zone&code=eu",
		"kind=country&code=es&offset=-1",
		"kind=country&code=es&offset=100",
		"kind=country&code=es&offset=x",
		"kind=country&code=es&limit=0",
		"kind=country&code=es&limit=101",
	} {
		if status, _, raw := getRanking(t, router, query); status != http.StatusBadRequest {
			t.Errorf("%s: status = %d body=%s, want 400", query, status, raw)
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("rejected requests reached FACEIT %d times", hits.Load())
	}
}

func TestFaceitCountriesAndFlagsComeFromTheBundledFlagSet(t *testing.T) {
	t.Parallel()
	router := Routes(NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}))
	rw := httptest.NewRecorder()
	router.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/faceit/countries", nil))
	var body struct {
		Countries []string `json:"countries"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &body); err != nil || rw.Code != http.StatusOK {
		t.Fatalf("countries status=%d body=%s err=%v", rw.Code, rw.Body.String(), err)
	}
	listed := strings.Join(body.Countries, ",")
	if len(body.Countries) < 240 || !strings.Contains(listed, "es,") || strings.Contains(listed, "eu") || strings.Contains(listed, "xx") {
		t.Fatalf("countries = %v, want every ISO country with a flag and no eu or xx", body.Countries)
	}

	flag := httptest.NewRecorder()
	router.ServeHTTP(flag, httptest.NewRequest(http.MethodGet, "/api/faceit/flags/es", nil))
	if flag.Code != http.StatusOK || flag.Header().Get("Content-Type") != "image/svg+xml" || !strings.Contains(flag.Body.String(), "<svg") {
		t.Fatalf("flag status=%d type=%q", flag.Code, flag.Header().Get("Content-Type"))
	}
	for _, code := range []string{"zz", "ES", "es.svg", "eu"} {
		missing := httptest.NewRecorder()
		router.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/api/faceit/flags/"+code, nil))
		if missing.Code != http.StatusNotFound {
			t.Errorf("flag %q status = %d, want 404", code, missing.Code)
		}
	}
}
