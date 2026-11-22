package vocab

import (
	"strings"
	"unicode"
)

type Word struct {
	Text    string
	StartMS int64
	EndMS   int64
}

type Phrase struct {
	Text  string
	Words []Word
}

type Term struct {
	Text     string
	KeepCase bool
}

type Vocabulary struct {
	Terms   []Term
	Learned Learned
}

type Result struct {
	Text  string
	Words []Word
}

const maxRun = 3

type term struct {
	Term
	words     []string
	norm      string
	key       string
	asWritten bool
}

type token struct {
	lead, core, trail string
	norm              string
}

func Correct(p Phrase, v Vocabulary) Result {
	unchanged := Result{Text: p.Text, Words: p.Words}
	fields := strings.Fields(p.Text)
	if len(fields) == 0 || !latinOnly(p.Text) || (len(p.Words) > 0 && len(p.Words) != len(fields)) {
		return unchanged
	}
	m := prepare(v)
	if len(m.terms) == 0 && len(m.rules) == 0 {
		return unchanged
	}
	tokens := make([]token, len(fields))
	for i, f := range fields {
		tokens[i] = split(f)
	}
	out, words, changed := []string{}, []Word{}, false
	for i := 0; i < len(tokens); {
		n, t, ok := m.learned(tokens[i:])
		if !ok {
			n, t, ok = m.best(tokens[i:])
		}
		if !ok {
			out = append(out, fields[i])
			if len(p.Words) > 0 {
				words = append(words, p.Words[i])
			}
			i++
			continue
		}
		written := strings.Fields(cased(t, tokens[i]))
		written[0] = tokens[i].lead + written[0]
		written[len(written)-1] += tokens[i+n-1].trail
		out = append(out, written...)
		if len(p.Words) > 0 {
			words = append(words, spread(p.Words[i:i+n], written)...)
		}
		changed = true
		i += n
	}
	if !changed {
		return unchanged
	}
	if len(p.Words) == 0 {
		words = nil
	}
	return Result{Text: strings.Join(out, " "), Words: words}
}

type matcher struct {
	terms   []term
	known   map[string]bool
	rules   map[string]term
	blocked map[[2]string]bool
}

func prepare(v Vocabulary) matcher {
	terms, known := prepareTerms(v)
	m := matcher{terms: terms, known: known, rules: map[string]term{}, blocked: map[[2]string]bool{}}
	for _, r := range v.Learned.Rules {
		ws := strings.Fields(r.Written)
		if len(ws) == 0 || !latinOnly(r.Written) {
			continue
		}
		for _, w := range ws {
			m.known[normalize(w)] = true
		}
		m.rules[normWords(r.Heard)] = term{Term: Term{Text: r.Written}, words: ws, asWritten: true}
	}
	for _, b := range v.Learned.Blocked {
		m.blocked[[2]string{normalize(b.Heard), normalize(b.Written)}] = true
	}
	return m
}

func (m matcher) learned(tokens []token) (int, term, bool) {
	for n := min(maxRun, len(tokens)); n >= 1; n-- {
		parts := make([]string, 0, n)
		for i, t := range tokens[:n] {
			if t.norm == "" || (i < n-1 && t.trail != "") || (i > 0 && t.lead != "") {
				break
			}
			parts = append(parts, t.norm)
		}
		if len(parts) < n {
			continue
		}
		if t, ok := m.rules[strings.Join(parts, " ")]; ok && len(t.words) <= n {
			return n, t, true
		}
	}
	return 0, term{}, false
}

func (m matcher) best(tokens []token) (int, term, bool) {
	bestN, bestCost, found := 0, 0, term{}
	for n := 1; n <= min(maxRun, len(tokens)); n++ {
		norm, ok := candidate(tokens[:n], m.known)
		if !ok {
			continue
		}
		t, cost, ok := closest(norm, n, m.terms)
		if ok && !m.blocked[[2]string{norm, t.norm}] && (bestN == 0 || cost < bestCost) {
			bestN, bestCost, found = n, cost, t
		}
	}
	return bestN, found, bestN > 0
}

func prepareTerms(v Vocabulary) ([]term, map[string]bool) {
	known := map[string]bool{}
	byNorm := map[string]term{}
	order := []string{}
	for _, t := range v.Terms {
		ws := strings.Fields(t.Text)
		norm := normalize(strings.Join(ws, ""))
		if len([]rune(norm)) < 3 || !latinOnly(t.Text) {
			continue
		}
		for _, w := range ws {
			known[normalize(w)] = true
		}
		known[norm] = true
		if _, seen := byNorm[norm]; seen {
			continue
		}
		byNorm[norm] = term{Term: t, words: ws, norm: norm, key: phoneticKey(norm)}
		order = append(order, norm)
	}
	out := make([]term, 0, len(order))
	for _, n := range order {
		out = append(out, byNorm[n])
	}
	return out, known
}

func candidate(run []token, known map[string]bool) (string, bool) {
	parts := make([]string, 0, len(run))
	for i, t := range run {
		if t.norm == "" || known[t.norm] {
			return "", false
		}
		if (i == 0 || i == len(run)-1) && common[t.norm] {
			return "", false
		}
		if i < len(run)-1 && t.trail != "" {
			return "", false
		}
		if i > 0 && t.lead != "" {
			return "", false
		}
		parts = append(parts, t.norm)
	}
	norm := strings.Join(parts, "")
	if len([]rune(norm)) < 3 || known[norm] || common[norm] {
		return "", false
	}
	return norm, true
}

func closest(norm string, n int, terms []term) (term, int, bool) {
	key := phoneticKey(norm)
	bestCost, tied, found := -1, false, term{}
	for _, t := range terms {
		if len(t.words) > n || !soundsAlike(key, t.key) {
			continue
		}
		cost := distance(norm, t.norm)
		if cost == 0 || cost > bound(t.norm) {
			continue
		}
		switch {
		case bestCost < 0 || cost < bestCost:
			bestCost, tied, found = cost, false, t
		case cost == bestCost:
			tied = true
		}
	}
	return found, bestCost, bestCost >= 0 && !tied
}

func soundsAlike(a, b string) bool {
	if a == b {
		return a != ""
	}
	return len(a) >= 4 && len(b) >= 4 && distance(a, b) <= 1
}

func bound(norm string) int {
	switch n := len([]rune(norm)); {
	case n <= 4:
		return 1
	case n <= 8:
		return 2
	default:
		return 3
	}
}

func cased(t term, first token) string {
	words := make([]string, len(t.words))
	for i, w := range t.words {
		words[i] = w
		if !t.KeepCase && !t.asWritten && !ownCase(w) {
			words[i] = strings.ToLower(w)
		}
	}
	out := strings.Join(words, " ")
	if t.KeepCase {
		return out
	}
	if r := []rune(first.core); len(r) > 0 && unicode.IsUpper(r[0]) {
		rs := []rune(out)
		rs[0] = unicode.ToUpper(rs[0])
		return string(rs)
	}
	return out
}

func ownCase(w string) bool {
	rs := []rune(w)
	if len(rs) == 1 {
		return true
	}
	for i, r := range rs {
		if unicode.IsDigit(r) || (i > 0 && unicode.IsUpper(r)) {
			return true
		}
	}
	return false
}

func spread(heard []Word, written []string) []Word {
	out := make([]Word, len(written))
	n, k := len(heard), len(written)
	for j := range written {
		from, to := j*n/k, (j+1)*n/k-1
		out[j] = Word{Text: written[j], StartMS: heard[from].StartMS, EndMS: heard[to].EndMS}
	}
	return out
}

func split(field string) token {
	rs := []rune(field)
	start, end := 0, len(rs)
	for start < end && !isWordRune(rs[start]) {
		start++
	}
	for end > start && !isWordRune(rs[end-1]) {
		end--
	}
	core := string(rs[start:end])
	return token{lead: string(rs[:start]), core: core, trail: string(rs[end:]), norm: normalize(core)}
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if isWordRune(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func latinOnly(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) && !unicode.Is(unicode.Latin, r) {
			return false
		}
	}
	return true
}

func distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}
