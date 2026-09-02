package demooverlay

import (
	"strings"
	"testing"
)

func TestValidateFACEITEnrichment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		roster     Roster
		enrichment map[string]Enrichment
		wantError  string
	}{
		{
			name: "complete roster",
			roster: Roster{Players: []RosterPlayer{
				{SteamID64: "1", Name: "alpha"},
				{SteamID64: "2", Name: "bravo"},
			}},
			enrichment: map[string]Enrichment{
				"1": {Nickname: "alpha", ELO: 2500, SkillLevel: 10},
				"2": {Nickname: "bravo", ELO: 1700, SkillLevel: 8},
			},
		},
		{
			name:      "empty roster",
			wantError: "parsed roster",
		},
		{
			name:       "missing SteamID",
			roster:     Roster{Players: []RosterPlayer{{Name: "alpha"}}},
			enrichment: map[string]Enrichment{},
			wantError:  "no SteamID64",
		},
		{
			name:       "missing profile",
			roster:     Roster{Players: []RosterPlayer{{SteamID64: "1", Name: "alpha"}}},
			enrichment: map[string]Enrichment{},
			wantError:  "no FACEIT profile",
		},
		{
			name:       "incomplete profile",
			roster:     Roster{Players: []RosterPlayer{{SteamID64: "1", Name: "alpha"}}},
			enrichment: map[string]Enrichment{"1": {}},
			wantError:  "no FACEIT nickname",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateFACEITEnrichment(tt.roster, tt.enrichment)
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("ValidateFACEITEnrichment() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("ValidateFACEITEnrichment() error = %v, want %q", err, tt.wantError)
			}
		})
	}
}
