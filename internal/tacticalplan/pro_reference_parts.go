package tacticalplan

import "sort"

// gameRound is a round's number in the game, from the score before it, which
// keeps counting across the demos a server restart splits a map into.
func gameRound(r Round) int {
	return r.ScoreCTBefore + r.ScoreTBefore + 1
}

// MergeProDemoParts joins the restart parts of one map and returns how many it
// dropped: of parts whose round ranges overlap (or lack one), only the longest counts.
func MergeProDemoParts(parts []ProDemoSummary) (ProDemoSummary, int) {
	if len(parts) == 1 {
		return parts[0], 0
	}
	sorted := append([]ProDemoSummary(nil), parts...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].FirstRound < sorted[j].FirstRound })
	kept := []ProDemoSummary{}
	for _, p := range sorted {
		if len(kept) == 0 {
			kept = append(kept, p)
			continue
		}
		last := &kept[len(kept)-1]
		if p.FirstRound > 0 && last.LastRound > 0 && p.FirstRound > last.LastRound {
			kept = append(kept, p)
		} else if p.Match.Rounds > last.Match.Rounds {
			*last = p
		}
	}
	out := kept[0]
	out.Players = append([]ProDemoLine(nil), kept[0].Players...)
	index := map[string]int{}
	for i, line := range out.Players {
		index[line.SteamID64] = i
		out.Players[i].RoundSums = copySums(line.RoundSums)
	}
	for _, p := range kept[1:] {
		out.Match.Rounds += p.Match.Rounds
		out.LastRound = p.LastRound
		for _, line := range p.Players {
			at, ok := index[line.SteamID64]
			if !ok {
				index[line.SteamID64] = len(out.Players)
				line.RoundSums = copySums(line.RoundSums)
				out.Players = append(out.Players, line)
				continue
			}
			out.Players[at].add(line)
		}
	}
	return out, len(parts) - len(kept)
}

func (l *ProDemoLine) add(o ProDemoLine) {
	l.Rounds += o.Rounds
	l.Kills += o.Kills
	l.Deaths += o.Deaths
	l.TradedDeaths += o.TradedDeaths
	l.FlashAssists += o.FlashAssists
	l.UtilityDamage += o.UtilityDamage
	l.WPA += o.WPA
	l.Box.add(o.Box)
	for metric, s := range o.RoundSums {
		merged := l.RoundSums[metric]
		merged.add(s)
		l.RoundSums[metric] = merged
	}
}

func copySums(in map[string]RoundSums) map[string]RoundSums {
	out := make(map[string]RoundSums, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
