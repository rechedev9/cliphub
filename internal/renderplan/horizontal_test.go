package renderplan

import "testing"

// Crear horizontal is the same video in 16:9, so the Short's team comms stay as the Short had them.
func TestHorizontalKeepsTheShortsTeamComms(t *testing.T) {
	capture := ShortCapture{HUDMode: "deathnotices"}
	if edit, _ := HorizontalEdit(NormalizeEditRequest(EditRequest{Format: FormatShort9x16}), capture); edit.VoiceComms {
		t.Fatal("a Short without team comms gets them in its horizontal")
	}
	level := 0.4
	edit, _ := HorizontalEdit(NormalizeEditRequest(EditRequest{Format: FormatShort9x16, VoiceComms: true, VoiceVolume: &level}), capture)
	if !edit.VoiceComms || edit.VoiceVolume == nil || *edit.VoiceVolume != level {
		t.Fatalf("horizontal comms = %v at %v, want the Short's comms at %v", edit.VoiceComms, edit.VoiceVolume, level)
	}
	if err := edit.Validate(); err != nil {
		t.Fatalf("the horizontal is refused: %v", err)
	}
}
