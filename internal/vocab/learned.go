package vocab

import (
	"cmp"
	"slices"
	"strings"
)

type Edit struct {
	Heard   string
	Machine string
	Written string
}

type Learned struct {
	Rules   []Change
	Blocked []Change
}

const (
	minLearnedFrases  = 2
	maxLearnedChanges = 2
	minLearnedLetters = 3
)

type tally struct {
	counts  map[Change]int
	shown   map[Change]string
	blocked map[Change]bool
	terms   map[string]bool
}

func Learn(edits []Edit, terms []Term) Learned {
	_, known := prepareTerms(Vocabulary{Terms: terms})
	t := tally{counts: map[Change]int{}, shown: map[Change]string{}, blocked: map[Change]bool{}, terms: known}
	for _, e := range edits {
		t.add(e)
	}
	return Learned{Rules: t.rules(), Blocked: sortedChanges(t.blocked)}
}

func (t tally) add(e Edit) {
	machine := cmp.Or(e.Machine, e.Heard)
	if e.Written == "" || e.Written == machine {
		return
	}
	made := map[Change]bool{}
	for _, c := range Changes(e.Heard, machine) {
		made[learnKey(c)] = true
	}
	mine, touched := map[Change]string{}, map[string]bool{}
	for _, c := range Changes(e.Heard, e.Written) {
		k := learnKey(c)
		touched[k.Heard] = true
		if !made[k] && k.Heard != k.Written {
			mine[k] = cores(c.Written)
		}
	}
	for k := range made {
		if !touched[k.Heard] {
			t.blocked[k] = true
		}
	}
	if len(mine) > maxLearnedChanges {
		return
	}
	for k, written := range mine {
		if t.learnable(k) {
			t.counts[k]++
			t.shown[k] = cmp.Or(t.shown[k], written)
		}
	}
}

func (t tally) learnable(k Change) bool {
	heard, written := strings.Fields(k.Heard), strings.Fields(k.Written)
	if len(heard) == 0 || len(written) == 0 || len(heard) > maxRun || len(written) > len(heard) {
		return false
	}
	if len([]rune(strings.Join(heard, ""))) < minLearnedLetters {
		return false
	}
	return !allIn(heard, common) || allIn(written, t.terms)
}

func allIn(words []string, set map[string]bool) bool {
	return !slices.ContainsFunc(words, func(w string) bool { return !set[w] })
}

func (t tally) rules() []Change {
	best, tied := map[string]Change{}, map[string]bool{}
	for k, n := range t.counts {
		if n < minLearnedFrases || t.blocked[k] {
			continue
		}
		cur, seen := best[k.Heard]
		switch {
		case !seen || n > t.counts[cur]:
			best[k.Heard], tied[k.Heard] = k, false
		case n == t.counts[cur]:
			tied[k.Heard] = true
		}
	}
	out := []Change{}
	for heard, k := range best {
		if !tied[heard] {
			out = append(out, Change{Heard: heard, Written: t.shown[k]})
		}
	}
	slices.SortFunc(out, func(a, b Change) int { return strings.Compare(a.Heard, b.Heard) })
	return out
}

func sortedChanges(set map[Change]bool) []Change {
	out := make([]Change, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	slices.SortFunc(out, func(a, b Change) int {
		return cmp.Or(strings.Compare(a.Heard, b.Heard), strings.Compare(a.Written, b.Written))
	})
	return out
}

func learnKey(c Change) Change {
	return Change{Heard: normWords(c.Heard), Written: normWords(c.Written)}
}

func normWords(s string) string {
	out := []string{}
	for _, f := range strings.Fields(s) {
		if n := normalize(f); n != "" {
			out = append(out, n)
		}
	}
	return strings.Join(out, " ")
}

func cores(s string) string {
	out := []string{}
	for _, f := range strings.Fields(s) {
		if c := split(f).core; c != "" {
			out = append(out, c)
		}
	}
	return strings.Join(out, " ")
}
