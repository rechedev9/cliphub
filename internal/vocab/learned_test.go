package vocab

import "testing"

func twice(e Edit) []Edit {
	return []Edit{e, e}
}

func correctLearned(edits []Edit, heard string) string {
	terms := Lexicon("")
	return Correct(Phrase{Text: heard}, Vocabulary{Terms: terms, Learned: Learn(edits, terms)}).Text
}

func TestAFreeRewriteTeachesNoWordPairs(t *testing.T) {
	edits := twice(Edit{Heard: "vesty kallo mingo", Written: "rest call mid"})
	if got := correctLearned(edits, "vesty kallo now"); got != "vesty kallo now" {
		t.Fatalf("after two free rewrites = %q; want no rule from the word alignment", got)
	}
}

func TestACommonOrShortHeardWordIsNotLearned(t *testing.T) {
	edits := append(twice(Edit{Heard: "the vest", Written: "a vest"}), twice(Edit{Heard: "kk go", Written: "okay go"})...)
	if got := correctLearned(edits, "the kk now"); got != "the kk now" {
		t.Fatalf("after correcting the and kk twice = %q; want a common word and a two letter word left", got)
	}
}

func TestACommonHeardWordIsLearnedWhenThePersonWritesACS2Term(t *testing.T) {
	edits := twice(Edit{Heard: "flesh out", Written: "flash out"})
	if got := correctLearned(edits, "Flesh, now"); got != "Flash, now" {
		t.Fatalf("after correcting flesh to flash twice = %q; want the grenade written", got)
	}
}

func TestALearnedCorrectionKeepsThePersonsCasing(t *testing.T) {
	edits := twice(Edit{Heard: "grinn is low", Written: "Grin is low"})
	if got := correctLearned(edits, "grinn now"); got != "Grin now" {
		t.Fatalf("after writing Grin twice = %q; want the nick as the person wrote it", got)
	}
}
