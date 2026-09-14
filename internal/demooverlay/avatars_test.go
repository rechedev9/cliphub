package demooverlay

import "testing"

func TestApplyAvatarURLsOnlyChangesPortraitField(t *testing.T) {
	t.Parallel()
	elo := 2500
	doc := Document{Intro: Intro{Left: []PlayerCard{{
		SteamID64: "76561198000000001", Name: "demo-name", Kills: 21, ELO: &elo,
		Last20: &Last20{},
	}}}}
	ApplyAvatarURLs(&doc, map[string]string{
		"76561198000000001": "https://avatars.akamai.steamstatic.com/demo.jpg",
	})
	got := doc.Intro.Left[0]
	if got.AvatarURL != "https://avatars.akamai.steamstatic.com/demo.jpg" {
		t.Fatalf("avatar URL = %q", got.AvatarURL)
	}
	if got.Name != "demo-name" || got.Kills != 21 || got.ELO == nil || *got.ELO != 2500 || got.Last20 == nil {
		t.Fatalf("avatar snapshot changed demo or FACEIT fields: %+v", got)
	}
}
