package tacticalplan

import "sort"

// Norms are the library's team rates: every team of every match read, both
// sides pooled, so a team's numbers compare with the library instead of with
// its own mean. Each rate carries its count and denominator.
type Norms struct {
	Matches int `json:"matches"`
	// Bombsites uses the definitions of Bombsites; its examples are left empty.
	Bombsites []BombsiteMap `json:"bombsites"`
	// Matchups uses the definitions of the prep's buy matchups.
	Matchups []NormMatchup `json:"matchups"`
	// Playbook is per side, with the playbook's categories; empty unless the
	// builder read positions, which the categories need.
	Playbook []NormPlaybook `json:"playbook"`
}

// NormMatchup is a side's buy against the rival's buy and how often it won.
type NormMatchup struct {
	Side        Side    `json:"side"`
	Buy         BuyType `json:"buy"`
	OpponentBuy BuyType `json:"opponent_buy"`
	WinRate     Rate    `json:"win_rate"`
}

// NormPlaybook is one side's category use and success over the library.
type NormPlaybook struct {
	Side       Side           `json:"side"`
	RoundCount int            `json:"round_count"`
	WinRate    Rate           `json:"win_rate"`
	Categories []NormCategory `json:"categories"`
}

// NormCategory is one playbook category: Share is over the side's rounds,
// WinRate over the category's.
type NormCategory struct {
	Category string `json:"category"`
	Share    Rate   `json:"share"`
	WinRate  Rate   `json:"win_rate"`
}

type winCount struct{ won, rounds int }

func (c *winCount) add(won bool) {
	c.rounds++
	if won {
		c.won++
	}
}

type normSide struct {
	all        winCount
	categories map[string]*winCount
}

// NormsBuilder reads one match at a time, so a library-wide pass keeps a
// single document in memory.
type NormsBuilder struct {
	playbook  bool
	matches   int
	bombsites map[string]*bombsiteMapAcc
	matchups  map[matchupKey]*winCount
	sides     map[Side]*normSide
}

// NewNormsBuilder starts an empty pass; with playbook it also classifies each
// round into a playbook category, which decodes the match's positions.
func NewNormsBuilder(playbook bool) *NormsBuilder {
	b := &NormsBuilder{playbook: playbook, bombsites: map[string]*bombsiteMapAcc{}, matchups: map[matchupKey]*winCount{}, sides: map[Side]*normSide{}}
	for _, side := range []Side{SideT, SideCT} {
		b.sides[side] = &normSide{categories: map[string]*winCount{}}
	}
	return b
}

// Add counts every round of the match once per team that played it.
func (b *NormsBuilder) Add(m FramedMatch) {
	doc := m.Document
	b.matches++
	acc, ok := b.bombsites[doc.Demo.Map]
	if !ok {
		acc = newBombsiteMapAcc()
		b.bombsites[doc.Demo.Map] = acc
	}
	acc.matches[m.MatchID.String()] = true
	var src frameSource
	if b.playbook {
		src = openFrames(m)
	}
	for _, r := range doc.Rounds {
		var frames []Frame
		if b.playbook {
			frames = src.round(r.Number)
		}
		for _, team := range doc.Teams {
			if !(Filter{TeamKey: team.Key}).Match(doc, r) {
				continue
			}
			side := doc.TeamSide(team.Key, r)
			if side == "" {
				continue
			}
			b.addRound(m.Document, r, side, frames)
			if r.Winner != "" {
				sm := m.ScoutMatch
				sm.TeamKey = team.Key
				acc.add(sm, r, side)
			}
		}
	}
}

func (b *NormsBuilder) addRound(doc Document, r Round, side Side, frames []Frame) {
	won := r.Winner == side
	mk := matchupKey{side: side, buy: r.Economy.Buy(side), opponentBuy: r.Economy.Buy(side.Opponent())}
	c, ok := b.matchups[mk]
	if !ok {
		c = &winCount{}
		b.matchups[mk] = c
	}
	c.add(won)
	if !b.playbook {
		return
	}
	read, _ := readPlaybookRound(doc, r, side, frames)
	category, _ := read.category()
	s := b.sides[side]
	s.all.add(won)
	cat, ok := s.categories[category]
	if !ok {
		cat = &winCount{}
		s.categories[category] = cat
	}
	cat.add(won)
}

// Norms returns the pooled rates read so far.
func (b *NormsBuilder) Norms() Norms {
	out := Norms{Matches: b.matches, Bombsites: []BombsiteMap{}, Matchups: []NormMatchup{}, Playbook: []NormPlaybook{}}
	for name, acc := range b.bombsites {
		m := acc.finish(name)
		for i := range m.Sites {
			m.Sites[i].CT.Examples = []Evidence{}
			m.Sites[i].T.Examples = []Evidence{}
		}
		out.Bombsites = append(out.Bombsites, m)
	}
	sort.Slice(out.Bombsites, func(i, j int) bool { return out.Bombsites[i].Map < out.Bombsites[j].Map })
	for k, c := range b.matchups {
		out.Matchups = append(out.Matchups, NormMatchup{Side: k.side, Buy: k.buy, OpponentBuy: k.opponentBuy, WinRate: newRate(c.won, c.rounds)})
	}
	sort.Slice(out.Matchups, func(i, j int) bool {
		a, c := out.Matchups[i], out.Matchups[j]
		if a.Side != c.Side {
			return a.Side == SideT
		}
		if a.Buy != c.Buy {
			return buyOrder[a.Buy] < buyOrder[c.Buy]
		}
		return buyOrder[a.OpponentBuy] < buyOrder[c.OpponentBuy]
	})
	if !b.playbook {
		return out
	}
	for _, side := range []Side{SideT, SideCT} {
		s := b.sides[side]
		p := NormPlaybook{Side: side, RoundCount: s.all.rounds, WinRate: newRate(s.all.won, s.all.rounds), Categories: []NormCategory{}}
		for _, name := range playbookCategories[side] {
			c := s.categories[name]
			if c == nil {
				c = &winCount{}
			}
			p.Categories = append(p.Categories, NormCategory{Category: name, Share: newRate(c.rounds, s.all.rounds), WinRate: newRate(c.won, c.rounds)})
		}
		out.Playbook = append(out.Playbook, p)
	}
	return out
}
