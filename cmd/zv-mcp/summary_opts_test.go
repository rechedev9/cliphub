package main

import (
	"strconv"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestEveryPreRollAndCaptureStyleIsAcceptedByRenderplan(t *testing.T) {
	for _, p := range preRollOptions {
		for _, h := range captureHUDOptions {
			seconds, err := strconv.Atoi(p.ID)
			if err != nil {
				t.Fatal(err)
			}
			edit, err := resolveEdit(shortChoices{Format: "landscape-16x9", PreRollSeconds: &seconds, CaptureHUD: h.ID})
			if err != nil {
				t.Fatalf("%s s, %s: %v", p.ID, h.ID, err)
			}
			got := decodeEdit(t, edit)
			if err := got.Validate(); err != nil {
				t.Errorf("%s s, %s rejected by renderplan: %v", p.ID, h.ID, err)
			}
			if want := map[string]int{"3": 0, "5": 5, "8": 8}[p.ID]; got.PreRollSeconds != want || got.CaptureHUD != h.ID {
				t.Errorf("%s s, %s: sent pre-roll %d and capture %q, want %d and %q", p.ID, h.ID, got.PreRollSeconds, got.CaptureHUD, want, h.ID)
			}
		}
	}
}

func TestResolveEditRejectsAPreRollOrCaptureStyleStudioRefuses(t *testing.T) {
	four, on := 4, true
	for name, c := range map[string]shortChoices{
		"pre-roll 4":                {PreRollSeconds: &four},
		"unknown capture style":     {Format: "landscape-16x9", CaptureHUD: "radar"},
		"capture style in a Short":  {CaptureHUD: "clean"},
		"capture style and the HUD": {Format: "landscape-16x9", CaptureHUD: "clean", NativeHUD: &on},
	} {
		if _, err := resolveEdit(c); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestDescribeShortRequestNamesThePreRollAndCaptureStyle(t *testing.T) {
	five := 5
	edit, err := resolveEdit(shortChoices{Format: "landscape-16x9", PreRollSeconds: &five, CaptureHUD: "deathnotices"})
	if err != nil {
		t.Fatal(err)
	}
	got := describeShortRequest(tuiclient.GenerateRequest{Edit: edit})
	for _, want := range []string{"5 s antes de cada kill", "captura Killfeed"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary %q misses %q", got, want)
		}
	}
}
