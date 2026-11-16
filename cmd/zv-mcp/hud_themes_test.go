package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const userThemeID = "4b1f6c2e-8d3a-4f5b-9c7e-1a2b3c4d5e6f"

func TestPlanChoicesResolveAUserThemeAndTheHUDLook(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/full-demo/hud-themes" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"max":10,"themes":[{"id":"` + userThemeID + `","base":"prism","name":"Mío","ct":"112233","t":"445566","background":"000000","radius":4,"created_at":"2026-11-01T00:00:00Z","updated_at":"2026-11-01T00:00:00Z"}]}`))
	}))
	defer srv.Close()
	c := tuiclient.New(tuiclient.Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
	in := planFullDemoIn{HUD: userThemeID, hudLookIn: hudLookIn{CompactHeader: ptrTo(true), RoundWinBanner: ptrTo(true), WinProbability: ptrTo(true), TeamTStart: ptrTo("  Spirit ")}}
	if err := resolveUserHUD(t.Context(), c, &in); err != nil {
		t.Fatal(err)
	}
	got, err := applyFullDemoChoices(recapplan.DefaultOptions(), in)
	if err != nil {
		t.Fatal(err)
	}
	o := got.Overlays
	if o.HUDTheme != "prism" || o.HUDCustom == nil || o.HUDCustom.CT != "112233" || *o.HUDCustom.Radius != 4 {
		t.Fatalf("hud = %q, custom = %+v; want the theme on its base", o.HUDTheme, o.HUDCustom)
	}
	if !o.HUDOptions.CompactHeader || o.HUDOptions.HideLoadout || !o.HUDOptions.RoundWinBanner || !o.HUDOptions.WinProbability || o.TeamNames.TStart != "Spirit" {
		t.Fatalf("look = %+v / %+v", o.HUDOptions, o.TeamNames)
	}
	if summary := describeHUDLook(o); !strings.Contains(summary, "cabecera compacta") || !strings.Contains(summary, "Spirit") {
		t.Fatalf("summary = %q", summary)
	}
	missing := planFullDemoIn{HUD: "9b1f6c2e-8d3a-4f5b-9c7e-1a2b3c4d5e6f"}
	if err := resolveUserHUD(t.Context(), c, &missing); err == nil {
		t.Fatal("an unknown theme id was accepted")
	}
	if _, err := applyFullDemoChoices(recapplan.DefaultOptions(), planFullDemoIn{HUD: "native", hudLookIn: hudLookIn{HideInventory: ptrTo(true)}}); err == nil {
		t.Fatal("the native HUD accepted a custom HUD switch")
	}
	catalog, err := applyFullDemoChoices(got, planFullDemoIn{HUD: "arena"})
	if err != nil || catalog.Overlays.HUDCustom != nil || !catalog.Overlays.HUDOptions.CompactHeader {
		t.Fatalf("choosing a catalog design = %+v, %v; want the theme dropped and the switches kept", catalog.Overlays, err)
	}
}
