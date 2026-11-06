package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func decodeEdit(t *testing.T, edit tuiclient.ShortEdit) renderplan.EditRequest {
	t.Helper()
	raw, err := json.Marshal(edit)
	if err != nil {
		t.Fatal(err)
	}
	var req renderplan.EditRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatal(err)
	}
	return req
}

func TestResolveEditDefaultsMatchProduct(t *testing.T) {
	edit, err := resolveEdit(shortChoices{})
	if err != nil {
		t.Fatal(err)
	}
	got, want := decodeEdit(t, edit), renderplan.DefaultEditRequest()
	if got.Format != want.Format || got.KillEffect != want.KillEffect || got.Transition != want.Transition || got.CoverStrategy != want.CoverStrategy {
		t.Errorf("defaults drifted: got %+v, want %+v", got, want)
	}
	if got.Intro || got.Outro || got.HookText || got.KillCounter || got.VoiceComms || got.NativeHUD {
		t.Errorf("toggles must default off: %+v", got)
	}
}

func TestEveryOptionIsAcceptedByRenderplan(t *testing.T) {
	for _, f := range formatOptions {
		for _, k := range killEffectOptions {
			for _, tr := range transitionOptions {
				edit, err := resolveEdit(shortChoices{Format: f.ID, KillEffect: k.ID, Transition: tr.ID})
				if err != nil {
					t.Fatal(err)
				}
				if err := decodeEdit(t, edit).Validate(); err != nil {
					t.Errorf("%s/%s/%s rejected by renderplan: %v", f.ID, k.ID, tr.ID, err)
				}
			}
		}
	}
}

func TestResolveEditRejectsUnknownIDs(t *testing.T) {
	_, err := resolveEdit(shortChoices{KillEffect: "explosion"})
	if err == nil || !strings.Contains(err.Error(), "punch-in") || !strings.Contains(err.Error(), "kill_effect") {
		t.Errorf("error should name the field and list valid ids: %v", err)
	}
}

func TestResolveEditToggles(t *testing.T) {
	on, off := true, false
	edit, err := resolveEdit(shortChoices{Intro: &on, KillCounter: &on, Cover: &off})
	if err != nil {
		t.Fatal(err)
	}
	if !edit.Intro || !edit.KillCounter || edit.Outro || edit.CoverStrategy != coverNone {
		t.Errorf("toggles not applied: %+v", edit)
	}
}

func TestCheckMusic(t *testing.T) {
	songs := []tuiclient.Song{{ID: "phonk-01"}, {ID: "synthwave-1"}}
	if err := checkMusic("phonk-01", songs); err != nil {
		t.Errorf("known track refused: %v", err)
	}
	if err := checkMusic("nope", songs); err == nil || !strings.Contains(err.Error(), "synthwave-1") {
		t.Errorf("error should list valid ids: %v", err)
	}
}

func TestDescribeShortRequestNamesChoices(t *testing.T) {
	edit, _ := resolveEdit(shortChoices{})
	got := describeShortRequest(tuiclient.GenerateRequest{Music: &tuiclient.ShortMusic{Key: "phonk-01"}, Edit: edit})
	for _, want := range []string{"short-9x16", "punch-in", "flash", "phonk-01", "cover"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary %q misses %q", got, want)
		}
	}
}

func TestEveryBannerStyleIsAcceptedByRenderplan(t *testing.T) {
	for _, f := range bannerFamilies {
		for _, st := range f.Styles {
			edit, err := resolveEdit(shortChoices{BannerFamily: f.ID, BannerStyle: st.ID, BannerCode: "zack_1"})
			if err != nil {
				t.Fatalf("%s %s: %v", f.ID, st.ID, err)
			}
			got := decodeEdit(t, edit)
			if err := got.Validate(); err != nil {
				t.Errorf("%s %s rejected by renderplan: %v", f.ID, st.ID, err)
			}
			if got.KeyDropCode != "ZACK_1" || *got.KeyDropStartSeconds != 0 || *got.KeyDropEndSeconds != 4 {
				t.Errorf("%s %s: code or window = %q %v %v", f.ID, st.ID, got.KeyDropCode, *got.KeyDropStartSeconds, *got.KeyDropEndSeconds)
			}
		}
	}
}

func TestResolveEditRejectsBadBannerAndCardText(t *testing.T) {
	end := 1.0
	on := true
	for name, c := range map[string]shortChoices{
		"fields without a style":  {BannerCode: "ABC"},
		"unknown style":           {BannerStyle: "tigerr", BannerFamily: "CSGOSKINS"},
		"bad code":                {BannerStyle: "classic", BannerCode: "-bad"},
		"window ends early":       {BannerStyle: "classic", BannerEnd: &end, BannerStart: &end},
		"intro text without card": {IntroText: "Hola"},
		"outro text too long":     {Outro: &on, OutroText: strings.Repeat("x", 81)},
	} {
		if _, err := resolveEdit(c); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	edit, err := resolveEdit(shortChoices{Intro: &on, IntroText: "  El ace de la final  "})
	if err != nil || edit.IntroText != "El ace de la final" {
		t.Errorf("intro text = %q, %v", edit.IntroText, err)
	}
}

func TestResolveMusicMapsPercentsToGains(t *testing.T) {
	full, half, mute := 100, 50, 0
	music, err := resolveMusic(shortChoices{Music: "phonk-01", MusicVolume: &full, GameVolume: &mute})
	if err != nil || music.Volume != 0 || music.GameVolume == nil || *music.GameVolume != 0 {
		t.Fatalf("full music, muted game = %+v, %v", music, err)
	}
	music, err = resolveMusic(shortChoices{Music: "phonk-01", MusicVolume: &half})
	if err != nil || music.Volume != 0.5 || music.GameVolume != nil {
		t.Fatalf("half music = %+v, %v", music, err)
	}
	if _, err := resolveMusic(shortChoices{MusicVolume: &half}); err == nil {
		t.Error("a volume without a track was accepted")
	}
	if music, _ := resolveMusic(shortChoices{}); music != nil {
		t.Errorf("no track gave %+v", music)
	}
}
