package gameassets

import "testing"

func TestTeamLogoCodeFoldsTheName(t *testing.T) {
	for name, want := range map[string]string{"Natus Vincere": "navi", "The MongolZ": "mngz", "Team Vitality": "vita", "3DMAX": "3dm", "Virtus.pro": "vp", "PURE": ""} {
		if got := TeamLogoCode(name); got != want {
			t.Errorf("TeamLogoCode(%q) = %q, want %q", name, got, want)
		}
	}
}
