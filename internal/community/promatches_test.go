package community

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"
)

func TestMatchProSidesPairsClanNamesWithTeams(t *testing.T) {
	teams := []ProTeam{{ID: 1, Name: "Team Vitality"}, {ID: 2, Name: "FaZe"}}
	if got := MatchProSides([]string{"FaZe Clan", "Vitality"}, teams); !slices.Equal(got, []int{1, 0}) {
		t.Fatalf("crossed sides = %v", got)
	}
	if got := MatchProSides([]string{"Vitality", "xyz"}, teams); !slices.Equal(got, []int{0, 1}) {
		t.Fatalf("one named side places the other: %v", got)
	}
	if got := MatchProSides([]string{"abc", "xyz"}, teams); !slices.Equal(got, []int{-1, -1}) {
		t.Fatalf("unknown clans = %v", got)
	}
}

func TestParseProMatchQuery(t *testing.T) {
	q, err := ParseProMatchQuery(url.Values{"days": {"30"}, "team": {" FaZe "}, "map": {"de_nuke"}, "page": {"2"}})
	if err != nil || q.Days != 30 || q.Team != "FaZe" || q.Page != 2 {
		t.Fatalf("query = %+v %v", q, err)
	}
	if got := q.Values().Encode(); got != "days=30&map=de_nuke&page=2&team=FaZe" {
		t.Fatalf("values = %s", got)
	}
	for _, bad := range []url.Values{{"days": {"0"}}, {"page": {"x"}}, {"map": {"../x"}}} {
		if _, err := ParseProMatchQuery(bad); err == nil {
			t.Fatalf("%v must be refused", bad)
		}
	}
}

func TestProClientTellsAMissingCatalog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/pro-matches/7/a-vs-b-m1-nuke" {
			_, _ = w.Write([]byte("zst"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	c, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.ProMatches(context.Background(), ProMatchQuery{}); !errors.Is(err, ErrNoProCatalog) {
		t.Fatalf("missing catalog = %v", err)
	}
	if _, err := c.ProMapFileBytes(context.Background(), 7, "a-vs-b-m2-x"); !errors.Is(err, ErrNoProMap) {
		t.Fatalf("missing map = %v", err)
	}
	body, err := c.ProMapFileBytes(context.Background(), 7, "a-vs-b-m1-nuke")
	if err != nil || !bytes.Equal(body, []byte("zst")) {
		t.Fatalf("file = %q %v", body, err)
	}
	if _, err := c.ProMapFileBytes(context.Background(), 7, "../db"); err == nil {
		t.Fatal("a path outside the entry pattern must be refused before any request")
	}
}
