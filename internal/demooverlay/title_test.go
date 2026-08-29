package demooverlay

import (
	"strings"
	"testing"
)

func TestTitleUsesImfcndPatternFromDemoFactsOnly(t *testing.T) {
	tests := []struct {
		name string
		doc  Document
		want string
	}{
		{
			name: "kills deaths only",
			doc:  Document{TargetName: "donk666", TargetKills: 23, TargetDeaths: 14},
			want: "donk666 (23-14) | CS2 DEMO POV + VOICECOMMS",
		},
		{
			name: "map",
			doc:  Document{TargetName: "donk666", TargetKills: 23, TargetDeaths: 14, Map: "de_mirage"},
			want: "donk666 (23-14) Mirage | CS2 DEMO POV + VOICECOMMS",
		},
		{
			name: "faceit elo when present",
			doc:  Document{TargetName: "donk666", TargetKills: 23, TargetDeaths: 14, Map: "de_mirage", TargetELO: intPtr(4400)},
			want: "donk666 (23-14) Mirage 4400 ELO | CS2 DEMO POV + VOICECOMMS",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Title(tt.doc); got != tt.want {
				t.Fatalf("Title = %q, want %q", got, tt.want)
			}
			if strings.Contains(Title(tt.doc), "TOP #1") || strings.Contains(Title(tt.doc), "DUO") {
				t.Fatal("title invented FACEIT rank or duo copy")
			}
		})
	}
}

func TestCaptionAndHashtagsStayFactual(t *testing.T) {
	doc := Document{TargetName: "donk666", TargetKills: 23, TargetDeaths: 14, Map: "de_mirage"}
	caption := Caption(doc)
	if !strings.Contains(caption, "donk666") || !strings.Contains(caption, "23-14") || !strings.Contains(caption, "Mirage") {
		t.Fatalf("caption = %q", caption)
	}
	if strings.Contains(strings.ToLower(caption), "cheat") {
		t.Fatal("caption must stay factual")
	}
	tags := Hashtags(doc)
	if len(tags) == 0 || tags[0] != "#CS2" {
		t.Fatalf("hashtags = %#v", tags)
	}
	if len(tags) > 5 {
		t.Fatalf("hashtags len = %d", len(tags))
	}
}
