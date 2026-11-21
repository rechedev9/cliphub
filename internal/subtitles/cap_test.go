package subtitles

import (
	"strings"
	"testing"
)

func shown(t *testing.T, s Script) []string {
	t.Helper()
	script, err := ASS(s)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, line := range dialogues(script) {
		fields := strings.SplitN(line, ",", 10)
		_, text, _ := strings.Cut(fields[9], "&}")
		out = append(out, fields[1]+" "+fields[2]+" "+text)
	}
	return out
}

func assertShown(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("shown:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func talk(speaker string, start, end float64) Cue {
	return Cue{ID: speaker, Speaker: speaker, Lines: []string{"hola"}, StartSeconds: start, EndSeconds: end}
}

func TestASSShowsAtMostTwoSubtitulosAtOnceInPortraitAndThreeInLandscape(t *testing.T) {
	cues := []Cue{talk("a", 0, 4), talk("b", 1, 5), talk("c", 2, 6), talk("d", 3, 7)}
	assertShown(t, shown(t, Script{Width: 1080, Height: 1920, DurationSeconds: 30, Cues: cues}), []string{
		"0:00:00.00 0:00:04.00 a: hola",
		"0:00:01.00 0:00:05.00 b: hola",
		"0:00:04.00 0:00:06.00 c: hola",
		"0:00:05.00 0:00:07.00 d: hola",
	})
	assertShown(t, shown(t, Script{Width: 1920, Height: 1080, DurationSeconds: 30, Cues: cues}), []string{
		"0:00:00.00 0:00:04.00 a: hola",
		"0:00:01.00 0:00:05.00 b: hola",
		"0:00:02.00 0:00:06.00 c: hola",
		"0:00:04.00 0:00:07.00 d: hola",
	})
}

func TestASSPutsThePOVFirstWhenTooManyTalk(t *testing.T) {
	pov := talk("yo", 2, 6)
	pov.POV = true
	cues := []Cue{talk("a", 0, 4), talk("b", 0.5, 5.5), pov}
	assertShown(t, shown(t, Script{Width: 1080, Height: 1920, DurationSeconds: 30, Cues: cues}), []string{
		"0:00:00.00 0:00:04.00 a: hola",
		"0:00:00.50 0:00:02.00 b: hola",
		"0:00:02.00 0:00:06.00 yo: hola",
		"0:00:04.00 0:00:05.50 b: hola",
	})
}

func TestASSDropsWhatIsLeftOfABumpedSubtituloWhenTooShortToRead(t *testing.T) {
	pov := talk("yo", 2, 6)
	pov.POV = true
	cues := []Cue{talk("a", 0, 5), talk("b", 1, 5.5), pov}
	assertShown(t, shown(t, Script{Width: 1080, Height: 1920, DurationSeconds: 30, Cues: cues}), []string{
		"0:00:00.00 0:00:05.00 a: hola",
		"0:00:02.00 0:00:06.00 yo: hola",
	})
	brief := []Cue{talk("a", 0, 0.6), talk("b", 0, 0.6)}
	if got := shown(t, Script{Width: 1080, Height: 1920, DurationSeconds: 0.6, Cues: brief}); len(got) != 2 {
		t.Fatalf("shown = %q, want both subtítulos that never had to wait, however brief", got)
	}
}

func TestASSKeepsAFraseThatStartedEarlierToItsLastSubtitulo(t *testing.T) {
	first, second := talk("a", 0, 2), talk("a", 2, 4)
	second.Lines = []string{"sigo"}
	cues := []Cue{first, talk("b", 0.5, 4), talk("c", 1, 6), second}
	assertShown(t, shown(t, Script{Width: 1080, Height: 1920, DurationSeconds: 30, Cues: cues}), []string{
		"0:00:00.00 0:00:02.00 a: hola",
		"0:00:00.50 0:00:04.00 b: hola",
		"0:00:02.00 0:00:04.00 a: sigo",
		"0:00:04.00 0:00:06.00 c: hola",
	})
}

func TestASSGivesTheRoomOfADroppedSubtituloToTheNextOne(t *testing.T) {
	cues := []Cue{talk("a", 0, 6), talk("b", 0, 3), talk("c", 1, 4), talk("d", 2, 6)}
	assertShown(t, shown(t, Script{Width: 1080, Height: 1920, DurationSeconds: 30, Cues: cues}), []string{
		"0:00:00.00 0:00:06.00 a: hola",
		"0:00:00.00 0:00:03.00 b: hola",
		"0:00:03.00 0:00:06.00 d: hola",
	})
}
