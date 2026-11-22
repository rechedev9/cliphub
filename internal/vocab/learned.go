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

const minLearnedFrases = 2

func Learn(edits []Edit) Learned {
	counts := map[Change]int{}
	shown := map[Change]string{}
	blocked := map[Change]bool{}
	for _, e := range edits {
		machine := cmp.Or(e.Machine, e.Heard)
		if e.Written == "" || e.Written == machine {
			continue
		}
		made := map[Change]bool{}
		for _, c := range Changes(e.Heard, machine) {
			made[learnKey(c)] = true
		}
		mine, touched := map[Change]bool{}, map[string]bool{}
		for _, c := range Changes(e.Heard, e.Written) {
			k := learnKey(c)
			touched[k.Heard] = true
			if !made[k] && k.Heard != "" && k.Written != "" && k.Heard != k.Written {
				mine[k] = true
				shown[k] = cmp.Or(shown[k], cores(c.Written))
			}
		}
		for k := range mine {
			counts[k]++
		}
		for k := range made {
			if !touched[k.Heard] {
				blocked[k] = true
			}
		}
	}
	return Learned{Rules: pickRules(counts, shown, blocked), Blocked: sortedChanges(blocked)}
}

func pickRules(counts map[Change]int, shown map[Change]string, blocked map[Change]bool) []Change {
	best, tied := map[string]Change{}, map[string]bool{}
	for k, n := range counts {
		if n < minLearnedFrases || blocked[k] || len(strings.Fields(k.Heard)) > maxRun || len(strings.Fields(k.Written)) > len(strings.Fields(k.Heard)) {
			continue
		}
		cur, seen := best[k.Heard]
		switch {
		case !seen || n > counts[cur]:
			best[k.Heard], tied[k.Heard] = k, false
		case n == counts[cur]:
			tied[k.Heard] = true
		}
	}
	out := []Change{}
	for heard, k := range best {
		if !tied[heard] {
			out = append(out, Change{Heard: heard, Written: shown[k]})
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
