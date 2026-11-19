package renderplan

import (
	"encoding/json"
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
	off := NormalizeEditRequest(EditRequest{SubtitlesHidden: []string{phrase}})
	if off.SubtitlesHidden != nil {
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

// Studio 5.25.0 stored subtitles_english in edits and render documents; they still load, in the spoken language.
func TestAStoredEditWithTheRetiredEnglishSubtitlesStillLoads(t *testing.T) {
	const stored = `{"schema_version":"1.0","variant":"default","edit":{"format":"short-9x16","voice_comms":true,
		"subtitles":true,"subtitles_english":true,"subtitles_hidden":["6f1c2a52-3d4b-4c8e-9a51-1f0e2b3c4d5e"]}}`
	var doc EditDocument
	if err := json.Unmarshal([]byte(stored), &doc); err != nil {
		t.Fatalf("a 5.25.0 edit document does not decode: %v", err)
	}
	edit := NormalizeEditRequest(doc.Edit)
	if err := edit.Validate(); err != nil || !edit.Subtitles || len(edit.SubtitlesHidden) != 1 {
		t.Fatalf("edit = %+v, %v; want its subtitles and hidden phrase kept", edit, err)
	}
	again, err := json.Marshal(edit)
	if err != nil || strings.Contains(string(again), "english") {
		t.Fatalf("the edit wrote %s, %v; want the retired flag gone", again, err)
	}
}
