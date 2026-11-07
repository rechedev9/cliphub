package tacticalplan

import "sort"

// GameRound is a round's number in the game, from the score before it, which
// keeps counting across the demos a server restart splits a map into.
func GameRound(r Round) int {
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
		switch {
		case p.FirstRound > 0 && last.LastRound > 0 && p.FirstRound > last.LastRound:
			kept = append(kept, p)
		case p.FirstRound > 0 && p.FirstRound == last.LastRound && len(last.LastRoundPlayers) == len(last.Players) && last.Match.Rounds > 1:
			// The restart replays the round the server went down in; the later copy counts.
			// A final round that misses players was cut short, so the longer part wins instead.
			*last = withoutLastRound(*last)
			kept = append(kept, p)
		case p.Match.Rounds > last.Match.Rounds:
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

// withoutLastRound subtracts the part's final round from its totals.
func withoutLastRound(d ProDemoSummary) ProDemoSummary {
	index := map[string]int{}
	players := make([]ProDemoLine, len(d.Players))
	for i, line := range d.Players {
		line.RoundSums = copySums(line.RoundSums)
		players[i] = line
		index[line.SteamID64] = i
	}
	for _, last := range d.LastRoundPlayers {
		if at, ok := index[last.SteamID64]; ok {
			players[at].add(last.negated())
		}
	}
	d.Players = players
	d.Match.Rounds--
	d.LastRound--
	d.LastRoundPlayers = nil
	return d
}

func (l ProDemoLine) negated() ProDemoLine {
	out := ProDemoLine{
		Rounds: -l.Rounds, Kills: -l.Kills, Deaths: -l.Deaths, TradedDeaths: -l.TradedDeaths, FlashAssists: -l.FlashAssists,
		UtilityDamage: -l.UtilityDamage, WPA: -l.WPA, RoundSums: map[string]RoundSums{},
		Box: BoxScore{
			Damage: -l.Box.Damage, Assists: -l.Box.Assists, KASTRounds: -l.Box.KASTRounds, OpeningKills: -l.Box.OpeningKills,
			OpeningDeaths: -l.Box.OpeningDeaths, UtilityRounds: -l.Box.UtilityRounds,
		},
	}
	for metric, s := range l.RoundSums {
		out.RoundSums[metric] = RoundSums{N: -s.N, Sum: -s.Sum, SumSq: -s.SumSq}
	}
	return out
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
