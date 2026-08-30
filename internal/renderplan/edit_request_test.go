package renderplan

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEditRequestSerializesAutomaticTextControls(t *testing.T) {
	b, err := json.Marshal(EditRequest{HookText: true, KillCounter: false, CoverStrategy: CoverStrategyNone})
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if !strings.Contains(got, `"hook_text":true`) || !strings.Contains(got, `"kill_counter":false`) {
		t.Fatalf("EditRequest JSON = %s, want explicit automatic text booleans", got)
	}
	if !strings.Contains(got, `"cover_strategy":"no-cover"`) {
		t.Fatalf("EditRequest JSON = %s, want explicit cover strategy", got)
	}
}

func TestEditRequestSerializesOptionalRecapControls(t *testing.T) {
	b, err := json.Marshal(EditRequest{MatchRecap: true, VoiceComms: true, NativeHUD: false})
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if !strings.Contains(got, `"match_recap":true`) || !strings.Contains(got, `"voice_comms":true`) || !strings.Contains(got, `"native_hud":false`) {
		t.Fatalf("EditRequest JSON = %s, want explicit recap extras", got)
	}
}

func TestRecapEditRequestLocksFullDemoTreatment(t *testing.T) {
	got := RecapEditRequest()
	if got.Format != FormatLandscape16x9 || !got.MatchRecap || !got.NativeHUD || !got.VoiceComms {
		t.Fatalf("recap edit = %#v, want landscape recap with native HUD and comms", got)
	}
	if got.KillEffect != KillEffectClean || got.Transition != TransitionCut {
		t.Fatalf("recap garnish = effect %q transition %q, want clean/cut", got.KillEffect, got.Transition)
	}
	if got.VoiceVolume == nil || *got.VoiceVolume != DefaultRecapVoiceVolume {
		t.Fatalf("recap voice volume = %v, want %v", got.VoiceVolume, DefaultRecapVoiceVolume)
	}
	if got.Intro || got.Outro || got.HookText || got.KillCounter {
		t.Fatalf("recap edit gained Shorts garnish: %#v", got)
	}
}

func TestNormalizeEditRequestDefaultsUnsetFields(t *testing.T) {
	got := NormalizeEditRequest(EditRequest{Intro: true})
	want := EditRequest{
		Format:        FormatShort9x16,
		KillEffect:    KillEffectPunchIn,
		Transition:    TransitionFlash,
		Intro:         true,
		CoverStrategy: CoverStrategyGenerated,
	}
	if got != want {
		t.Fatalf("edit request = %#v, want %#v", got, want)
	}
}

func TestEditRequestValidateRejectsUnknownFields(t *testing.T) {
	cases := []struct {
		name string
		req  EditRequest
		want string
	}{
		{name: "format", req: EditRequest{Format: "square", KillEffect: KillEffectPunchIn, Transition: TransitionFlash}, want: "unknown render format"},
		{name: "effect", req: EditRequest{Format: FormatShort9x16, KillEffect: "explode", Transition: TransitionFlash}, want: "unknown kill effect"},
		{name: "transition", req: EditRequest{Format: FormatShort9x16, KillEffect: KillEffectPunchIn, Transition: "spin"}, want: "unknown transition"},
		{name: "cover strategy", req: EditRequest{Format: FormatShort9x16, KillEffect: KillEffectPunchIn, Transition: TransitionFlash, CoverStrategy: "uploaded"}, want: "unknown cover strategy"},
		{
			name: "intro text too long",
			req: EditRequest{
				Format: FormatShort9x16, KillEffect: KillEffectPunchIn, Transition: TransitionFlash,
				IntroText: strings.Repeat("a", 81),
			},
			want: "intro text exceeds 80 characters",
		},
		{
			name: "outro text too long",
			req: EditRequest{
				Format: FormatShort9x16, KillEffect: KillEffectPunchIn, Transition: TransitionFlash,
				OutroText: strings.Repeat("a", 81),
			},
			want: "outro text exceeds 80 characters",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if err == nil {
				t.Fatal("Validate error = nil, want error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want %q", err.Error(), tc.want)
			}
		})
	}
}

func TestEditRequestValidateAcceptsViralStyles(t *testing.T) {
	cases := []EditRequest{
		{Format: FormatShort9x16, KillEffect: KillEffectShake, Transition: TransitionGlitch},
		{Format: FormatLandscape16x9, KillEffect: KillEffectGlitch, Transition: TransitionZoomWhip},
	}
	for _, req := range cases {
		if err := req.Validate(); err != nil {
			t.Fatalf("Validate(%#v) = %v, want nil", req, err)
		}
	}
}

func TestEditRequestValidateVoiceVolume(t *testing.T) {
	ok := 0.85
	bad := 1.5
	neg := -0.1
	mute := 0.0
	tests := []struct {
		name    string
		volume  *float64
		wantErr string
	}{
		{name: "unset", volume: nil},
		{name: "default", volume: &ok},
		{name: "mute", volume: &mute},
		{name: "above one", volume: &bad, wantErr: "voice volume must be between 0 and 1"},
		{name: "negative", volume: &neg, wantErr: "voice volume must be between 0 and 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := EditRequest{Format: FormatShort9x16, KillEffect: KillEffectPunchIn, Transition: TransitionFlash, VoiceVolume: tt.volume}
			err := req.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestEditRequestValidateAcceptsTextAtMaxLength(t *testing.T) {
	req := EditRequest{
		Format: FormatShort9x16, KillEffect: KillEffectPunchIn, Transition: TransitionFlash,
		IntroText: strings.Repeat("a", 80),
		OutroText: strings.Repeat("b", 80),
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil at the 80-char limit", err)
	}
}

func TestNormalizeEditRequestTrimsBookendTextWithoutEnablingBookends(t *testing.T) {
	got := NormalizeEditRequest(EditRequest{IntroText: "  Watch this ace  ", OutroText: "  follow for more  "})
	if got.IntroText != "Watch this ace" || got.OutroText != "follow for more" {
		t.Fatalf("bookend text = %q / %q, want trimmed", got.IntroText, got.OutroText)
	}
	if got.Intro || got.Outro {
		t.Fatalf("bookend bools = %v / %v, want false: setting text must not enable the bookend", got.Intro, got.Outro)
	}
}
