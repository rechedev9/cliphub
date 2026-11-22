package vocab

import "strings"

func isVowel(c byte) bool {
	return strings.IndexByte("aeiou", c) >= 0
}

func phoneticKey(norm string) string {
	s := asciiLetters(norm)
	for _, p := range []string{"kn", "gn", "pn", "wr"} {
		if strings.HasPrefix(s, p) {
			s = s[1:]
			break
		}
	}
	if strings.HasPrefix(s, "x") {
		s = "s" + s[1:]
	}
	var b strings.Builder
	at := func(i int) byte {
		if i < 0 || i >= len(s) {
			return 0
		}
		return s[i]
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == at(i-1) && c != 'c' {
			continue
		}
		next := at(i + 1)
		switch c {
		case 'a', 'e', 'i', 'o', 'u':
			if i == 0 {
				b.WriteByte('A')
			}
		case 'b':
			if !(i == len(s)-1 && at(i-1) == 'm') {
				b.WriteByte('B')
			}
		case 'c':
			switch {
			case next == 'h' || (next == 'i' && at(i+2) == 'a'):
				b.WriteByte('X')
			case next == 'e' || next == 'i' || next == 'y':
				b.WriteByte('S')
			default:
				b.WriteByte('K')
			}
		case 'd':
			if next == 'g' && (at(i+2) == 'e' || at(i+2) == 'i' || at(i+2) == 'y') {
				b.WriteByte('J')
			} else {
				b.WriteByte('T')
			}
		case 'g':
			switch {
			case next == 'h' && !isVowel(at(i+2)):
			case next == 'e' || next == 'i' || next == 'y':
				b.WriteByte('J')
			default:
				b.WriteByte('K')
			}
		case 'h':
			if isVowel(next) && strings.IndexByte("cgpst", at(i-1)) < 0 {
				b.WriteByte('H')
			}
		case 'k':
			if at(i-1) != 'c' {
				b.WriteByte('K')
			}
		case 'p':
			if next == 'h' {
				b.WriteByte('F')
			} else {
				b.WriteByte('P')
			}
		case 'q':
			b.WriteByte('K')
		case 's':
			if next == 'h' || (next == 'i' && (at(i+2) == 'o' || at(i+2) == 'a')) {
				b.WriteByte('X')
			} else {
				b.WriteByte('S')
			}
		case 't':
			switch {
			case next == 'h':
				b.WriteByte('0')
			case next == 'i' && (at(i+2) == 'o' || at(i+2) == 'a'):
				b.WriteByte('X')
			default:
				b.WriteByte('T')
			}
		case 'v':
			b.WriteByte('F')
		case 'w', 'y':
			if isVowel(next) {
				b.WriteByte(c - 'a' + 'A')
			}
		case 'x':
			b.WriteString("KS")
		case 'z':
			b.WriteByte('S')
		default:
			if c >= '0' && c <= '9' {
				b.WriteByte(c)
			} else {
				b.WriteByte(c - 'a' + 'A')
			}
		}
	}
	return collapse(b.String())
}

func asciiLetters(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if c := s[i]; (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteByte(c)
		}
	}
	return b.String()
}

func collapse(key string) string {
	var b strings.Builder
	for i := 0; i < len(key); i++ {
		if i == 0 || key[i] != key[i-1] {
			b.WriteByte(key[i])
		}
	}
	return b.String()
}
