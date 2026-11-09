package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestCommunityServerReferenceFeedsTheProBaseline(t *testing.T) {
	ref := tacticalplan.DefaultProReference()
	ref.ID, ref.Description, ref.UpdatedAt = "server-x", "283 mapas profesionales de IEM Cologne Major 2026.", "2026-11-06T09:00:00Z"
	body, err := json.Marshal(ref)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/pro-reference" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	client, err := community.NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cache := community.OpenProReferenceCache(dir, client)
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	env := newLibraryTestEnv(t)
	WithCommunity(CommunityDeps{Client: client, Reference: cache})(env.h)

	var base baselineResponse
	env.json(http.MethodGet, "/api/library/baseline?scope=pro&map=de_nuke", nil, http.StatusOK, &base)
	if base.Source != referenceSourceServer || base.UpdatedAt != ref.UpdatedAt || base.Description != ref.Description {
		t.Fatalf("pro baseline = %+v, want the server reference", base)
	}
}
