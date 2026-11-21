package subtitles

import (
	"cmp"
	"slices"
)

const (
	portraitAtOnce  = 2
	landscapeAtOnce = 3
)

func capped(cues []Cue, limit int) []Cue {
	var edges []float64
	since := map[string]float64{}
	for _, c := range cues {
		edges = append(edges, c.StartSeconds, c.EndSeconds)
		if first, ok := since[c.ID]; !ok || c.StartSeconds < first {
			since[c.ID] = c.StartSeconds
		}
	}
	slices.Sort(edges)
	edges = slices.Compact(edges)
	order := make([]int, len(cues))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		return cmp.Or(povFirst(cues[a], cues[b]), cmp.Compare(since[cues[a].ID], since[cues[b].ID]))
	})
	showing := make([]int, len(edges))
	var out []Cue
	for _, i := range order {
		c := cues[i]
		first, _ := slices.BinarySearch(edges, c.StartSeconds)
		last, _ := slices.BinarySearch(edges, c.EndSeconds)
		for from := first; from < last; from++ {
			if showing[from] >= limit {
				continue
			}
			to := from
			for to < last && showing[to] < limit {
				to++
			}
			if from == first && to == last || edges[to]-edges[from] >= MinSeconds {
				for e := from; e < to; e++ {
					showing[e]++
				}
				shown := c
				shown.StartSeconds, shown.EndSeconds = edges[from], edges[to]
				out = append(out, shown)
			}
			from = to
		}
	}
	slices.SortStableFunc(out, func(a, b Cue) int {
		return cmp.Or(cmp.Compare(a.StartSeconds, b.StartSeconds), povFirst(a, b))
	})
	return out
}

func povFirst(a, b Cue) int {
	switch {
	case a.POV == b.POV:
		return 0
	case a.POV:
		return -1
	}
	return 1
}
