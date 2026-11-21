package subtitles

import (
	"strings"
	"testing"
)

func TestSlotIsTheStableOrderOfAPlayerInsideTheTeam(t *testing.T) {
	team := []string{"76561198000000030", "76561198000000010", "76561198000000050", "76561198000000020", "76561198000000040"}
	shuffled := []string{team[4], team[2], team[0], team[3], team[1]}
	want := map[string]int{team[1]: 0, team[3]: 1, team[0]: 2, team[4]: 3, team[2]: 4}
	for id, slot := range want {
		if got := Slot(team, id); got != slot {
			t.Errorf("Slot(%s) = %d, want %d", id, got, slot)
		}
		if got := Slot(shuffled, id); got != slot {
			t.Errorf("Slot(%s) = %d in another roster order, want %d", id, got, slot)
		}
	}
	six := append([]string{"76561198000000060"}, team...)
	if got := Slot(six, "76561198000000060"); got != 0 {
		t.Errorf("a sixth player got slot %d, want the palette to wrap to 0", got)
	}
	if got := Slot(team, "76561198000000099"); got != 0 {
		t.Errorf("a player outside the roster got slot %d, want 0", got)
	}
}

func colourOfLine(t *testing.T, line string) string {
	t.Helper()
	_, rest, ok := strings.Cut(line, `{\c&H`)
	if !ok {
		t.Fatalf("no colour in %q", line)
	}
	colour, _, _ := strings.Cut(rest, "&}")
	return colour
}

func TestASSGivesEachPlayerItsColourAndThePOVWhite(t *testing.T) {
	var cues []Cue
	for slot := range 5 {
		cues = append(cues, Cue{ID: "p", Speaker: string(rune('a' + slot)), Slot: slot, Lines: []string{"hola"}, StartSeconds: float64(3 * slot), EndSeconds: float64(3*slot + 2)})
	}
	cues = append(cues, Cue{ID: "pov", Speaker: "yo", Slot: 2, POV: true, Lines: []string{"hola"}, StartSeconds: 15, EndSeconds: 17})
	script, err := ASS(Script{Width: 1080, Height: 1920, DurationSeconds: 30, Cues: cues})
	if err != nil {
		t.Fatal(err)
	}
	got := dialogues(script)
	if len(got) != 6 {
		t.Fatalf("%d dialogues, want 6", len(got))
	}
	want := []string{"4FD7FF", "FFC85C", "87E76E", "4A9FFF", "D08FFF", "FFFFFF"}
	for i, line := range got {
		if colour := colourOfLine(t, line); colour != want[i] {
			t.Errorf("dialogue %d is drawn in %s, want %s", i, colour, want[i])
		}
		if strings.Count(line, `\c&H`) != 1 {
			t.Errorf("dialogue %d changes colour inside the subtítulo: %q", i, line)
		}
	}
	if !strings.HasSuffix(got[5], `{\c&HFFFFFF&}yo: hola`) {
		t.Errorf("the POV subtítulo is %q, want the name before the text in white", got[5])
	}
}
