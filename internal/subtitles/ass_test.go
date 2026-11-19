package subtitles

import (
	"strings"
	"testing"
)

func dialogues(script string) []string {
	var out []string
	for _, line := range strings.Split(script, "\n") {
		if strings.HasPrefix(line, "Dialogue:") {
			out = append(out, line)
		}
	}
	return out
}

func TestASSPlacesEachFormatClearOfTheHUD(t *testing.T) {
	cue := []Cue{{ID: "a", Speaker: "ropz", Text: "stack B", StartSeconds: 1, EndSeconds: 2.5}}
	portrait, err := ASS(Script{Width: 1080, Height: 1920, DurationSeconds: 30, Cues: cue})
	if err != nil {
		t.Fatal(err)
	}
	landscape, err := ASS(Script{Width: 1920, Height: 1080, DurationSeconds: 30, Cues: cue})
	if err != nil {
		t.Fatal(err)
	}
	// 9:16 sits over the lower third, under the kill feed and above the radar corner; 16:9 above CS2's bottom HUD row.
	for _, want := range []string{"PlayResX: 1080", "PlayResY: 1920", ",2,86,86,600,1"} {
		if !strings.Contains(portrait, want) {
			t.Errorf("9:16 script lacks %q:\n%s", want, portrait)
		}
	}
	for _, want := range []string{"PlayResX: 1920", "PlayResY: 1080", ",2,230,230,150,1"} {
		if !strings.Contains(landscape, want) {
			t.Errorf("16:9 script lacks %q:\n%s", want, landscape)
		}
	}
	got := dialogues(portrait)
	want := "Dialogue: 0,0:00:01.00,0:00:02.50,Subtitle,,0,0,0,,{\\c&H4FD7FF&}ropz:{\\c&HFFFFFF&} stack B"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("dialogues = %q, want %q", got, want)
	}
}

func TestASSCapsHowLongAPhraseStaysAndDropsHiddenOnes(t *testing.T) {
	script, err := ASS(Script{Width: 1080, Height: 1920, DurationSeconds: 20, Hidden: []string{"secret"}, Cues: []Cue{
		{ID: "long", Speaker: "a", Text: "una frase muy larga", StartSeconds: 0, EndSeconds: 15},
		{ID: "blip", Speaker: "b", Text: "ok", StartSeconds: 9, EndSeconds: 9.2},
		{ID: "secret", Speaker: "c", Text: "no se ve", StartSeconds: 10, EndSeconds: 11},
		{ID: "late", Speaker: "d", Text: "tarde", StartSeconds: 19.5, EndSeconds: 23},
		{ID: "gone", Speaker: "e", Text: "fuera", StartSeconds: 21, EndSeconds: 22},
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := dialogues(script)
	want := []string{
		"Dialogue: 0,0:00:00.00,0:00:06.00,Subtitle,,0,0,0,,{\\c&H4FD7FF&}a:{\\c&HFFFFFF&} una frase muy larga",
		"Dialogue: 0,0:00:09.00,0:00:10.20,Subtitle,,0,0,0,,{\\c&H4FD7FF&}b:{\\c&HFFFFFF&} ok",
		"Dialogue: 0,0:00:19.50,0:00:20.00,Subtitle,,0,0,0,,{\\c&H4FD7FF&}d:{\\c&HFFFFFF&} tarde",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("dialogues:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestASSKeepsOneSpeakerOnePhraseAtATimeAndEscapesTags(t *testing.T) {
	script, err := ASS(Script{Width: 1920, Height: 1080, DurationSeconds: 20, Cues: []Cue{
		{ID: "1", Speaker: "m0NESY", Text: "uno", StartSeconds: 2, EndSeconds: 4},
		{ID: "2", Speaker: "m0NESY", Text: "{\\b1}dos", StartSeconds: 3, EndSeconds: 5},
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := dialogues(script)
	if len(got) != 2 || !strings.Contains(got[0], ",0:00:02.00,0:00:03.00,") || !strings.HasSuffix(got[1], "｛＼b1｝dos") {
		t.Fatalf("dialogues = %q; want the first phrase cut where the next one starts and braces kept literal", got)
	}
}

func TestASSRefusesAFrameItCannotPlace(t *testing.T) {
	if _, err := ASS(Script{Width: 0, Height: 1080, DurationSeconds: 5}); err == nil {
		t.Fatal("a zero-width frame was accepted")
	}
}
