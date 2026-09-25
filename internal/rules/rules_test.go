package rules

import (
	"reflect"
	"strings"
	"testing"
)

func TestDefaultRules(t *testing.T) {
	r := Default()

	if r.WindowSeconds != 8 {
		t.Errorf("WindowSeconds default = %d, want 8", r.WindowSeconds)
	}
	if r.PreRollSeconds != 3 {
		t.Errorf("PreRollSeconds default = %d, want 3", r.PreRollSeconds)
	}
	if r.PostRollSeconds != 5 {
		t.Errorf("PostRollSeconds default = %d, want 5", r.PostRollSeconds)
	}
	if r.MinKillsInWindow != 1 {
		t.Errorf("MinKillsInWindow default = %d, want 1", r.MinKillsInWindow)
	}
	if !r.ExcludeTeamKills {
		t.Errorf("ExcludeTeamKills default = false, want true")
	}
	if r.IncludeHeadshotOnly {
		t.Errorf("IncludeHeadshotOnly default = true, want false")
	}
	if r.MinRound != 1 {
		t.Errorf("MinRound default = %d, want 1", r.MinRound)
	}
	if r.MaxRound != 0 {
		t.Errorf("MaxRound default = %d, want 0 (no max)", r.MaxRound)
	}
	wantWeapons := []string{AllWeapons}
	if len(r.Weapons) != len(wantWeapons) {
		t.Fatalf("Weapons default length = %d, want %d", len(r.Weapons), len(wantWeapons))
	}
	for i, w := range wantWeapons {
		if r.Weapons[i] != w {
			t.Errorf("Weapons[%d] = %q, want %q", i, r.Weapons[i], w)
		}
	}
}

func TestLoadEmptyDocumentYieldsDefaults(t *testing.T) {
	for name, body := range map[string]string{
		"empty object":    `{}`,
		"empty reader":    "",
		"whitespace only": "   \n\t  ",
	} {
		t.Run(name, func(t *testing.T) {
			r, err := Load(strings.NewReader(body))
			if err != nil {
				t.Fatalf("Load(%q) error = %v, want nil", body, err)
			}
			if !reflect.DeepEqual(r, Default()) {
				t.Fatalf("Load(%q) = %#v, want Default()", body, r)
			}
		})
	}
}

func TestLoadRejectsInvalidDocuments(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "second document", body: `{"window_seconds": 5} {"window_seconds": 10}`, wantErr: "unexpected content after rules document"},
		{name: "stray closing brace", body: `{"window_seconds": 5}}`, wantErr: "unexpected content after rules document"},
		{name: "stray closing bracket", body: `{"window_seconds": 5}]`, wantErr: "unexpected content after rules document"},
		{name: "trailing garbage", body: `{"window_seconds": 5} garbage`, wantErr: "unexpected content after rules document"},
		{name: "invalid json", body: `{not-json}`, wantErr: "decoding rules"},
		{name: "empty weapons", body: `{"weapons": []}`, wantErr: "weapons must contain at least one entry"},
		{name: "negative window", body: `{"window_seconds": -1}`, wantErr: "window_seconds must be >= 0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(strings.NewReader(tt.body))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load(%q) error = %v, want %q", tt.body, err, tt.wantErr)
			}
		})
	}
}

func TestLoadPartialJSONMergesWithDefaults(t *testing.T) {
	r, err := Load(strings.NewReader(`{"window_seconds": 15, "pre_roll_seconds": 2}`))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if r.WindowSeconds != 15 {
		t.Errorf("WindowSeconds = %d, want 15 (overridden)", r.WindowSeconds)
	}
	if r.PreRollSeconds != 2 {
		t.Errorf("PreRollSeconds = %d, want 2 (overridden)", r.PreRollSeconds)
	}
	if r.PostRollSeconds != 5 {
		t.Errorf("PostRollSeconds = %d, want 5 (default kept)", r.PostRollSeconds)
	}
	if len(r.Weapons) == 0 {
		t.Errorf("Weapons empty, expected defaults to be kept")
	}
}

func TestLoadCustomWeaponsOverridesDefaults(t *testing.T) {
	r, err := Load(strings.NewReader(`{"weapons": ["awp", "scout"]}`))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(r.Weapons) != 2 {
		t.Fatalf("Weapons length = %d, want 2", len(r.Weapons))
	}
	if r.Weapons[0] != "awp" || r.Weapons[1] != "scout" {
		t.Errorf("Weapons = %v, want [awp scout]", r.Weapons)
	}
}

func TestLoadMaxRoundSetsValue(t *testing.T) {
	r, err := Load(strings.NewReader(`{"max_round": 12}`))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if r.MaxRound != 12 {
		t.Errorf("MaxRound = %d, want 12", r.MaxRound)
	}
}

func TestAllowsWeapon(t *testing.T) {
	tests := []struct {
		name   string
		rules  Rules
		weapon string
		want   bool
	}{
		{name: "default rifle", rules: Default(), weapon: "awp", want: true},
		{name: "default p90", rules: Default(), weapon: "p90", want: true},
		{name: "default knife", rules: Default(), weapon: "knife", want: true},
		{name: "default future weapon", rules: Default(), weapon: "future_cs2_weapon", want: true},
		{name: "default empty weapon", rules: Default(), weapon: "", want: false},
		{name: "custom allowed", rules: Rules{Weapons: []string{"awp"}}, weapon: "awp", want: true},
		{name: "custom filtered", rules: Rules{Weapons: []string{"awp"}}, weapon: "p90", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rules.AllowsWeapon(tt.weapon); got != tt.want {
				t.Fatalf("AllowsWeapon(%q) = %v, want %v", tt.weapon, got, tt.want)
			}
		})
	}
}

func TestAllowsRound(t *testing.T) {
	r := Rules{MinRound: 3, MaxRound: 10}
	if r.AllowsRound(2) {
		t.Errorf("AllowsRound(2) = true with MinRound=3, want false")
	}
	if !r.AllowsRound(3) {
		t.Errorf("AllowsRound(3) = false with MinRound=3, want true")
	}
	if !r.AllowsRound(10) {
		t.Errorf("AllowsRound(10) = false with MaxRound=10, want true")
	}
	if r.AllowsRound(11) {
		t.Errorf("AllowsRound(11) = true with MaxRound=10, want false")
	}

	rNoMax := Rules{MinRound: 1, MaxRound: 0}
	if !rNoMax.AllowsRound(999) {
		t.Errorf("AllowsRound(999) = false with MaxRound=0, want true (no cap)")
	}
}
