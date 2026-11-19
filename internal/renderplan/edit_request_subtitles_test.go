package renderplan

import (
	"strings"
	"testing"
)

func TestSubtitlesNeedTheTeamCommsAndKnownPhrases(t *testing.T) {
	short := DefaultEditRequest()
	short.VoiceComms, short.Subtitles = true, true
	phrase := "6f1c2a52-3d4b-4c8e-9a51-1f0e2b3c4d5e"
	short.SubtitlesHidden = []string{" " + strings.ToUpper(phrase), phrase}
	got := NormalizeEditRequest(short)
	if err := got.Validate(); err != nil {
		t.Fatalf("subtitles on a Short with comms: %v", err)
	}
	if len(got.SubtitlesHidden) != 1 || got.SubtitlesHidden[0] != phrase {
		t.Fatalf("hidden = %q; want one canonical phrase ID", got.SubtitlesHidden)
	}
	off := NormalizeEditRequest(EditRequest{SubtitlesEnglish: true, SubtitlesHidden: []string{phrase}})
	if off.SubtitlesEnglish || off.SubtitlesHidden != nil {
		t.Fatalf("subtitles off kept %+v", off)
	}
	for name, edit := range map[string]EditRequest{
		"no comms":       {Subtitles: true},
		"unknown phrase": {Subtitles: true, VoiceComms: true, SubtitlesHidden: []string{"not-a-phrase"}},
	} {
		edit = NormalizeEditRequest(edit)
		if err := edit.Validate(); err == nil {
			t.Errorf("%s: accepted %+v", name, edit)
		}
	}
}
