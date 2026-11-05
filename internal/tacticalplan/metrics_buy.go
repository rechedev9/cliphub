package tacticalplan

import "sort"

// BuyLine is a player's rounds under one buy type of their own team.
type BuyLine struct {
	Buy    BuyType `json:"buy"`
	Rounds int     `json:"rounds"`
	Won    int     `json:"won"`
	Kills  int     `json:"kills"`
	Deaths int     `json:"deaths"`
	WPA    float64 `json:"wpa"`
}

// roundStart is a tracked player's totals when a round begins, so the round's
// own kills and deaths are the difference at its end.
type roundStart struct {
	kills  int
	deaths int
}

func buyStart(tracked map[uint8]*playerAcc) map[uint8]roundStart {
	out := make(map[uint8]roundStart, len(tracked))
	for slot, a := range tracked {
		out[slot] = roundStart{kills: a.m.Kills, deaths: a.m.Deaths}
	}
	return out
}

func addBuyRound(a *playerAcc, start roundStart, r Round, side Side, wpa float64) {
	buy := r.Economy.Buy(side)
	if buy == "" {
		buy = BuyUnknown
	}
	var line *BuyLine
	for i := range a.m.Buys {
		if a.m.Buys[i].Buy == buy {
			line = &a.m.Buys[i]
		}
	}
	if line == nil {
		a.m.Buys = append(a.m.Buys, BuyLine{Buy: buy})
		line = &a.m.Buys[len(a.m.Buys)-1]
	}
	line.Rounds++
	if r.Winner != "" && r.Winner == side {
		line.Won++
	}
	line.Kills += a.m.Kills - start.kills
	line.Deaths += a.m.Deaths - start.deaths
	line.WPA += wpa
}

// finishBuys rounds the WPA and orders the lines from poorest to richest buy.
func finishBuys(lines []BuyLine) []BuyLine {
	rank := map[BuyType]int{}
	for i, b := range BuyTypes {
		rank[b] = i
	}
	for i := range lines {
		lines[i].WPA = round4(lines[i].WPA)
	}
	sort.Slice(lines, func(i, j int) bool { return rank[lines[i].Buy] < rank[lines[j].Buy] })
	return lines
}
