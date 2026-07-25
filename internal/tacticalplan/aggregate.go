package tacticalplan

import (
	"math"
	"sort"
)

// MinReliableSample is the round count below which a rate is reported but
// flagged: a 1-of-1 site take is not a tendency, and a scouting report that
// hides the denominator is how analysts get misled.
const MinReliableSample = 4

// timingBucketSeconds is the width of the first-contact and plant histograms.
const timingBucketSeconds = 5

// Tendencies is the aggregate answer to "what does this team do", computed over
// the rounds a filter selected. Every rate ships with the counts it came from.
type Tendencies struct {
	Filter     Filter `json:"filter"`
	RoundCount int    `json:"round_count"`
	// Perspective is the side the rates are computed from when the filter fixes
	// one, either directly or through a team key.
	Perspective Side `json:"perspective,omitempty"`
	Wins        int  `json:"wins"`

	Buys       []BuyBucket     `json:"buys"`
	Matchups   []MatchupBucket `json:"matchups"`
	Sites      []SiteBucket    `json:"sites"`
	BuySites   []BuySiteBucket `json:"buy_sites"`
	TPatterns  []PatternBucket `json:"t_patterns"`
	CTPatterns []PatternBucket `json:"ct_patterns"`
	Openings   OpeningSummary  `json:"openings"`
	Timings    TimingSummary   `json:"timings"`
	Players    []PlayerSummary `json:"players"`
}

// Rate is a count over a denominator. It carries both numbers so a caller
// cannot render a percentage without the sample it rests on.
type Rate struct {
	Count int     `json:"count"`
	Total int     `json:"total"`
	Pct   float64 `json:"pct"`
	// Reliable is false when Total is below MinReliableSample.
	Reliable bool `json:"reliable"`
}

func newRate(count, total int) Rate {
	r := Rate{Count: count, Total: total, Reliable: total >= MinReliableSample}
	if total > 0 {
		r.Pct = math.Round(float64(count)/float64(total)*1000) / 10
	}
	return r
}

// BuyBucket is one economic state and how the perspective fared in it.
type BuyBucket struct {
	Buy           BuyType `json:"buy"`
	Rounds        int     `json:"rounds"`
	Share         Rate    `json:"share"`
	WinRate       Rate    `json:"win_rate"`
	PlantOrDefuse Rate    `json:"conversion"`
}

// MatchupBucket crosses the perspective's buy with the opponent's, which is the
// only correct way to express "anti-eco": it is a matchup, not a buy type.
type MatchupBucket struct {
	Buy         BuyType `json:"buy"`
	OpponentBuy BuyType `json:"opponent_buy"`
	Rounds      int     `json:"rounds"`
	WinRate     Rate    `json:"win_rate"`
}

// SiteBucket is where the round was decided and how it went.
type SiteBucket struct {
	Site    Site `json:"site"`
	Rounds  int  `json:"rounds"`
	Share   Rate `json:"share"`
	WinRate Rate `json:"win_rate"`
}

// BuySiteBucket answers the scouting question directly: on this economy, where
// do they go, and does it work.
type BuySiteBucket struct {
	Buy     BuyType `json:"buy"`
	Site    Site    `json:"site"`
	Rounds  int     `json:"rounds"`
	Share   Rate    `json:"share"`
	WinRate Rate    `json:"win_rate"`
}

// PatternBucket is one round shape's frequency and success.
type PatternBucket struct {
	Pattern string `json:"pattern"`
	Rounds  int    `json:"rounds"`
	Share   Rate   `json:"share"`
	WinRate Rate   `json:"win_rate"`
}

// OpeningSummary describes the opening duel, the single most predictive event
// in a round. Traded is reported separately because being traded after losing
// an entry is the job working, not the job failing.
type OpeningSummary struct {
	Rounds        int  `json:"rounds"`
	Won           Rate `json:"won"`
	TradedOnLoss  Rate `json:"traded_on_loss"`
	RoundWinAfter Rate `json:"round_win_after_opening_win"`
	RoundWinLost  Rate `json:"round_win_after_opening_loss"`
}

// TimingSummary reports when things happened, in seconds from freeze-time end.
type TimingSummary struct {
	FirstContact  Histogram `json:"first_contact"`
	Plant         Histogram `json:"plant"`
	RoundDuration Histogram `json:"round_duration"`
}

// Histogram is a bucketed distribution with the median kept alongside, since a
// median survives the long tail that a mean does not.
type Histogram struct {
	BucketSeconds int               `json:"bucket_seconds"`
	Samples       int               `json:"samples"`
	Median        float64           `json:"median"`
	Buckets       []HistogramBucket `json:"buckets"`
}

// HistogramBucket counts samples in [FromSeconds, FromSeconds+BucketSeconds).
type HistogramBucket struct {
	FromSeconds int `json:"from_seconds"`
	Count       int `json:"count"`
}

// PlayerSummary is one player's contribution across the filtered rounds.
type PlayerSummary struct {
	Slot          uint8   `json:"slot"`
	Name          string  `json:"name"`
	Rounds        int     `json:"rounds"`
	Kills         int     `json:"kills"`
	Deaths        int     `json:"deaths"`
	Assists       int     `json:"assists"`
	Damage        int     `json:"damage"`
	ADR           float64 `json:"adr"`
	OpeningKills  int     `json:"opening_kills"`
	OpeningDeaths int     `json:"opening_deaths"`
	TradeKills    int     `json:"trade_kills"`
	SurvivalRate  Rate    `json:"survival_rate"`
}

// Aggregate computes tendencies over the rounds the filter selects.
func Aggregate(d Document, f Filter) Tendencies {
	rounds := f.Apply(d)
	t := Tendencies{
		Filter:     f,
		RoundCount: len(rounds),
		Buys:       []BuyBucket{},
		Matchups:   []MatchupBucket{},
		Sites:      []SiteBucket{},
		BuySites:   []BuySiteBucket{},
		TPatterns:  []PatternBucket{},
		CTPatterns: []PatternBucket{},
		Players:    []PlayerSummary{},
	}
	if len(rounds) == 0 {
		t.Timings = TimingSummary{
			FirstContact:  newHistogram(nil),
			Plant:         newHistogram(nil),
			RoundDuration: newHistogram(nil),
		}
		return t
	}
	if len(rounds) > 0 {
		t.Perspective = f.Perspective(d, rounds[0])
	}

	buyRounds := map[BuyType]int{}
	buyWins := map[BuyType]int{}
	buyConversion := map[BuyType]int{}
	matchupRounds := map[[2]BuyType]int{}
	matchupWins := map[[2]BuyType]int{}
	siteRounds := map[Site]int{}
	siteWins := map[Site]int{}
	buySiteRounds := map[BuySiteKey]int{}
	buySiteWins := map[BuySiteKey]int{}
	tRounds := map[TPattern]int{}
	tWins := map[TPattern]int{}
	ctRounds := map[CTPattern]int{}
	ctWins := map[CTPattern]int{}
	tSideRounds := 0
	ctSideRounds := 0

	openings := 0
	openingWon := 0
	openingLost := 0
	openingLostTraded := 0
	roundWinAfterWin := 0
	roundWinAfterLoss := 0

	var firstContact, plant, duration []float64
	players := map[uint8]*PlayerSummary{}
	playerSurvived := map[uint8]int{}

	for _, r := range rounds {
		side := f.Perspective(d, r)
		won := side != "" && r.Winner == side

		// A round has two economies, so a buy type only means something once a
		// perspective picks one of them. Without a perspective these rollups
		// are omitted rather than filled with an "unknown" bucket that would
		// read as a finding and carry a 0% win rate by construction.
		if side != "" {
			buy := r.Economy.Buy(side)
			opponentBuy := r.Economy.Buy(side.Opponent())
			buyRounds[buy]++
			if won {
				buyWins[buy]++
			}
			// Conversion is what the economy bought: a plant for the attackers,
			// a defuse or a stopped plant for the defenders.
			if converted(r, side) {
				buyConversion[buy]++
			}
			key := [2]BuyType{buy, opponentBuy}
			matchupRounds[key]++
			if won {
				matchupWins[key]++
			}
			bs := BuySiteKey{Buy: buy, Site: r.Class.Site}
			buySiteRounds[bs]++
			if won {
				buySiteWins[bs]++
			}
		}

		// Where a round was played is a fact about the round, so it is reported
		// with or without a perspective; only its win rate needs one.
		siteRounds[r.Class.Site]++
		if won {
			siteWins[r.Class.Site]++
		}

		// A round shape only belongs to the perspective when the perspective
		// was playing that side. Counting a team's attacking patterns in the
		// rounds it spent defending would divide real wins by a denominator
		// that includes rounds the pattern was never theirs.
		if side == "" || side == SideT {
			tSideRounds++
			tRounds[r.Class.TSide]++
			if won {
				tWins[r.Class.TSide]++
			}
		}
		if side == "" || side == SideCT {
			ctSideRounds++
			ctRounds[r.Class.CTSide]++
			if won {
				ctWins[r.Class.CTSide]++
			}
		}

		if r.Class.OpeningSide != "" {
			openings++
			switch {
			case side == "":
				// No perspective: count the duel but not its ownership.
			case r.Class.OpeningSide == side:
				openingWon++
				if won {
					roundWinAfterWin++
				}
			default:
				openingLost++
				if r.Class.OpeningTraded {
					openingLostTraded++
				}
				if won {
					roundWinAfterLoss++
				}
			}
		}

		if r.Class.FirstContactTick > 0 && r.TickFreezeEnd > 0 && d.Demo.Tickrate > 0 {
			firstContact = append(firstContact, float64(r.Class.FirstContactTick-r.TickFreezeEnd)/d.Demo.Tickrate)
		}
		if r.Bomb != nil && r.Bomb.PlantTick > 0 && r.TickFreezeEnd > 0 && d.Demo.Tickrate > 0 {
			plant = append(plant, float64(r.Bomb.PlantTick-r.TickFreezeEnd)/d.Demo.Tickrate)
		}
		if secs := r.Duration(d.Demo.Tickrate); secs > 0 {
			duration = append(duration, secs)
		}

		for _, pr := range r.Players {
			if side != "" && pr.Side != side {
				continue
			}
			acc, ok := players[pr.Slot]
			if !ok {
				name := ""
				if p, found := d.PlayerBySlot(pr.Slot); found {
					name = p.Name
				}
				acc = &PlayerSummary{Slot: pr.Slot, Name: name}
				players[pr.Slot] = acc
			}
			acc.Rounds++
			acc.Kills += pr.Kills
			acc.Deaths += pr.Deaths
			acc.Assists += pr.Assists
			acc.Damage += pr.Damage
			acc.TradeKills += pr.TradeKills
			if pr.OpeningKill {
				acc.OpeningKills++
			}
			if pr.OpeningDeath {
				acc.OpeningDeaths++
			}
			if pr.Survived {
				playerSurvived[pr.Slot]++
			}
		}
	}

	total := len(rounds)
	for _, r := range rounds {
		side := f.Perspective(d, r)
		if side != "" && r.Winner == side {
			t.Wins++
		}
	}

	for _, buy := range BuyTypes {
		n := buyRounds[buy]
		if n == 0 {
			continue
		}
		t.Buys = append(t.Buys, BuyBucket{
			Buy:           buy,
			Rounds:        n,
			Share:         newRate(n, total),
			WinRate:       newRate(buyWins[buy], n),
			PlantOrDefuse: newRate(buyConversion[buy], n),
		})
	}

	for key, n := range matchupRounds {
		t.Matchups = append(t.Matchups, MatchupBucket{
			Buy: key[0], OpponentBuy: key[1], Rounds: n, WinRate: newRate(matchupWins[key], n),
		})
	}
	sort.Slice(t.Matchups, func(i, j int) bool {
		if t.Matchups[i].Rounds != t.Matchups[j].Rounds {
			return t.Matchups[i].Rounds > t.Matchups[j].Rounds
		}
		if t.Matchups[i].Buy != t.Matchups[j].Buy {
			return t.Matchups[i].Buy < t.Matchups[j].Buy
		}
		return t.Matchups[i].OpponentBuy < t.Matchups[j].OpponentBuy
	})

	for _, site := range []Site{SiteA, SiteB, SiteMid, SiteNone} {
		n := siteRounds[site]
		if n == 0 {
			continue
		}
		t.Sites = append(t.Sites, SiteBucket{
			Site: site, Rounds: n, Share: newRate(n, total), WinRate: newRate(siteWins[site], n),
		})
	}

	for key, n := range buySiteRounds {
		t.BuySites = append(t.BuySites, BuySiteBucket{
			Buy:     key.Buy,
			Site:    key.Site,
			Rounds:  n,
			Share:   newRate(n, buyRounds[key.Buy]),
			WinRate: newRate(buySiteWins[key], n),
		})
	}
	sort.Slice(t.BuySites, func(i, j int) bool {
		if t.BuySites[i].Buy != t.BuySites[j].Buy {
			return t.BuySites[i].Buy < t.BuySites[j].Buy
		}
		return t.BuySites[i].Site < t.BuySites[j].Site
	})

	for _, pattern := range []TPattern{TExecute, TDefault, TSplit, TFast, TEcoRush, TSave, TUnknown} {
		n := tRounds[pattern]
		if n == 0 {
			continue
		}
		t.TPatterns = append(t.TPatterns, PatternBucket{
			Pattern: string(pattern), Rounds: n, Share: newRate(n, tSideRounds), WinRate: newRate(tWins[pattern], n),
		})
	}
	for _, pattern := range []CTPattern{CTHold, CTRetake, CTAggression, CTStack, CTSave, CTUnknown} {
		n := ctRounds[pattern]
		if n == 0 {
			continue
		}
		t.CTPatterns = append(t.CTPatterns, PatternBucket{
			Pattern: string(pattern), Rounds: n, Share: newRate(n, ctSideRounds), WinRate: newRate(ctWins[pattern], n),
		})
	}

	t.Openings = OpeningSummary{
		Rounds:        openings,
		Won:           newRate(openingWon, openings),
		TradedOnLoss:  newRate(openingLostTraded, openingLost),
		RoundWinAfter: newRate(roundWinAfterWin, openingWon),
		RoundWinLost:  newRate(roundWinAfterLoss, openingLost),
	}
	t.Timings = TimingSummary{
		FirstContact:  newHistogram(firstContact),
		Plant:         newHistogram(plant),
		RoundDuration: newHistogram(duration),
	}

	for slot, acc := range players {
		if acc.Rounds > 0 {
			acc.ADR = math.Round(float64(acc.Damage)/float64(acc.Rounds)*10) / 10
		}
		acc.SurvivalRate = newRate(playerSurvived[slot], acc.Rounds)
		t.Players = append(t.Players, *acc)
	}
	sort.Slice(t.Players, func(i, j int) bool {
		if t.Players[i].Kills != t.Players[j].Kills {
			return t.Players[i].Kills > t.Players[j].Kills
		}
		return t.Players[i].Slot < t.Players[j].Slot
	})
	return t
}

// BuySiteKey identifies one economy-and-site cell.
type BuySiteKey struct {
	Buy  BuyType
	Site Site
}

// converted reports whether the round's economy turned into the objective
// progress that side was playing for.
func converted(r Round, side Side) bool {
	if r.Bomb == nil {
		return false
	}
	switch side {
	case SideT:
		return r.Bomb.PlantTick > 0
	case SideCT:
		return r.Bomb.DefuseTick > 0
	default:
		return r.Bomb.PlantTick > 0
	}
}

func newHistogram(samples []float64) Histogram {
	h := Histogram{BucketSeconds: timingBucketSeconds, Samples: len(samples), Buckets: []HistogramBucket{}}
	if len(samples) == 0 {
		return h
	}
	sorted := append([]float64(nil), samples...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		h.Median = sorted[mid]
	} else {
		h.Median = (sorted[mid-1] + sorted[mid]) / 2
	}
	h.Median = math.Round(h.Median*10) / 10

	counts := map[int]int{}
	maxBucket := 0
	for _, v := range sorted {
		if v < 0 {
			v = 0
		}
		bucket := int(v) / timingBucketSeconds * timingBucketSeconds
		counts[bucket]++
		if bucket > maxBucket {
			maxBucket = bucket
		}
	}
	for from := 0; from <= maxBucket; from += timingBucketSeconds {
		h.Buckets = append(h.Buckets, HistogramBucket{FromSeconds: from, Count: counts[from]})
	}
	return h
}
