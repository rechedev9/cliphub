package vocab

import "strings"

type Change struct {
	Heard   string `json:"heard"`
	Written string `json:"written"`
}

func Changes(heard, written string) []Change {
	a, b := strings.Fields(heard), strings.Fields(written)
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	out := []Change{}
	i, j, fromA, fromB := 0, 0, 0, 0
	flush := func() {
		switch {
		case i-fromA > 1 && i-fromA == j-fromB:
			for k := range i - fromA {
				out = append(out, Change{Heard: a[fromA+k], Written: b[fromB+k]})
			}
		case i > fromA || j > fromB:
			out = append(out, Change{Heard: strings.Join(a[fromA:i], " "), Written: strings.Join(b[fromB:j], " ")})
		}
	}
	for i < len(a) || j < len(b) {
		switch {
		case i < len(a) && j < len(b) && a[i] == b[j]:
			flush()
			i, j = i+1, j+1
			fromA, fromB = i, j
		case j < len(b) && (i == len(a) || lcs[i][j+1] >= lcs[i+1][j]):
			j++
		default:
			i++
		}
	}
	flush()
	return out
}
