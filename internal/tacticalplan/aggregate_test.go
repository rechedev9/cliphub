package tacticalplan

import "testing"

func buyBucket(t *testing.T, tend Tendencies, buy BuyType) BuyBucket {
	t.Helper()
	for _, b := range tend.Buys {
		if b.Buy == buy {
			return b
		}
	}
	t.Fatalf("no bucket for buy %q in %+v", buy, tend.Buys)
	return BuyBucket{}
}

func TestAggregateFollowsTheTeamPerspective(t *testing.T) {
	d := testDocument()
	tend := Aggregate(d, Filter{TeamKey: "ct-start"})

	if tend.RoundCount != 4 {
		t.Fatalf("round count = %d, want 4", tend.RoundCount)
	}
	// Alpha won rounds 2 and 3.
	if tend.Wins != 2 {
		t.Fatalf("wins = %d, want 2", tend.Wins)
	}

	// Alpha's economy across the side swap: pistol, full (CT), full (T), eco (T).
	full := buyBucket(t, tend, BuyFull)
	if full.Rounds != 2 || full.WinRate.Count != 2 || full.WinRate.Pct != 100 {
		t.Fatalf("full buy bucket = %+v, want 2 rounds both won", full)
	}
	if full.WinRate.Reliable {
		t.Fatal("a 2-round sample must not be reported as reliable")
	}
	eco := buyBucket(t, tend, BuyEco)
	if eco.Rounds != 1 || eco.WinRate.Count != 0 {
		t.Fatalf("eco bucket = %+v, want 1 round lost", eco)
	}
}

func TestAggregateOpeningDuels(t *testing.T) {
	d := testDocument()
	tend := Aggregate(d, Filter{TeamKey: "ct-start"})

	o := tend.Openings
	if o.Rounds != 3 {
		t.Fatalf("opening duels = %d, want 3", o.Rounds)
	}
	if o.Won.Count != 2 || o.Won.Total != 3 {
		t.Fatalf("openings won = %d/%d, want 2/3", o.Won.Count, o.Won.Total)
	}
	if o.RoundWinAfter.Count != 2 || o.RoundWinAfter.Total != 2 {
		t.Fatalf("round wins after an opening win = %d/%d, want 2/2", o.RoundWinAfter.Count, o.RoundWinAfter.Total)
	}
	if o.RoundWinLost.Count != 0 || o.RoundWinLost.Total != 1 {
		t.Fatalf("round wins after an opening loss = %d/%d, want 0/1", o.RoundWinLost.Count, o.RoundWinLost.Total)
	}
	if o.TradedOnLoss.Count != 0 || o.TradedOnLoss.Total != 1 {
		t.Fatalf("traded on loss = %d/%d, want 0/1", o.TradedOnLoss.Count, o.TradedOnLoss.Total)
	}
}

// The scouting question this feature exists for: on this economy, where do they
// go? The answer must come with its denominator.
func TestAggregateCrossesEconomyWithSite(t *testing.T) {
	d := testDocument()
	tend := Aggregate(d, Filter{Side: SideT})

	var ecoRounds int
	for _, cell := range tend.BuySites {
		if cell.Buy != BuyEco {
			continue
		}
		ecoRounds += cell.Rounds
		if cell.Share.Total == 0 {
			t.Fatalf("cell %+v reports a share with no denominator", cell)
		}
	}
	if ecoRounds != 2 {
		t.Fatalf("T eco rounds across sites = %d, want 2", ecoRounds)
	}
}

func TestAggregateMatchupsExposeAntiEco(t *testing.T) {
	d := testDocument()
	tend := Aggregate(d, Filter{Side: SideCT})

	found := false
	for _, m := range tend.Matchups {
		if m.Buy == BuyFull && m.OpponentBuy == BuyEco {
			found = true
			if m.Rounds != 2 {
				t.Fatalf("full-vs-eco rounds = %d, want 2", m.Rounds)
			}
		}
	}
	if !found {
		t.Fatalf("no full-vs-eco matchup in %+v", tend.Matchups)
	}
}

func TestAggregateTimings(t *testing.T) {
	d := testDocument()
	tend := Aggregate(d, Filter{})

	fc := tend.Timings.FirstContact
	if fc.Samples != 3 {
		t.Fatalf("first-contact samples = %d, want 3", fc.Samples)
	}
	if fc.BucketSeconds != timingBucketSeconds {
		t.Fatalf("bucket width = %d, want %d", fc.BucketSeconds, timingBucketSeconds)
	}
	// 13.4s, 5.6s and 21.25s: the median is the middle one.
	if fc.Median < 13 || fc.Median > 14 {
		t.Fatalf("median first contact = %v, want ~13.4", fc.Median)
	}
	total := 0
	for _, b := range fc.Buckets {
		total += b.Count
	}
	if total != 3 {
		t.Fatalf("buckets hold %d samples, want 3", total)
	}

	if tend.Timings.Plant.Samples != 1 {
		t.Fatalf("plant samples = %d, want 1", tend.Timings.Plant.Samples)
	}
}

func TestAggregatePlayerLines(t *testing.T) {
	d := testDocument()
	tend := Aggregate(d, Filter{TeamKey: "ct-start"})

	if len(tend.Players) != 2 {
		t.Fatalf("player lines = %d, want the 2 team members", len(tend.Players))
	}
	top := tend.Players[0]
	if top.Slot != 0 || top.Name != "a1" {
		t.Fatalf("top player = %+v, want slot 0", top)
	}
	if top.Kills != 4 {
		t.Fatalf("slot 0 kills = %d, want 4", top.Kills)
	}
	if top.OpeningKills != 2 || top.OpeningDeaths != 1 {
		t.Fatalf("slot 0 opening record = %d/%d, want 2 kills and 1 death", top.OpeningKills, top.OpeningDeaths)
	}
	if top.Rounds != 4 || top.ADR == 0 {
		t.Fatalf("slot 0 rounds/ADR = %d/%v, want 4 rounds and a non-zero ADR", top.Rounds, top.ADR)
	}
}

func TestAggregateEmptySelection(t *testing.T) {
	d := testDocument()
	tend := Aggregate(d, Filter{Sites: []Site{SiteMid}})

	if tend.RoundCount != 0 {
		t.Fatalf("round count = %d, want 0", tend.RoundCount)
	}
	if tend.Buys == nil || tend.Players == nil || tend.Timings.FirstContact.Buckets == nil {
		t.Fatal("an empty aggregate must still marshal as empty arrays, not null")
	}
}

func TestRateCarriesItsDenominator(t *testing.T) {
	r := newRate(1, 1)
	if r.Pct != 100 {
		t.Fatalf("pct = %v, want 100", r.Pct)
	}
	if r.Reliable {
		t.Fatal("a 1-of-1 rate must never be reported as reliable")
	}
	if got := newRate(2, MinReliableSample); !got.Reliable {
		t.Fatalf("a %d-sample rate must be reliable", MinReliableSample)
	}
	if got := newRate(0, 0); got.Pct != 0 || got.Reliable {
		t.Fatalf("an empty rate = %+v, want a zero unreliable rate", got)
	}
}

// A team's attacking patterns must not be diluted by the rounds it spent
// defending: counting all 24 rounds under both vocabularies makes every win
// rate look half as good as it was.
func TestAggregatePatternsOnlyCountTheSideThePerspectivePlayed(t *testing.T) {
	d := testDocument()
	tend := Aggregate(d, Filter{TeamKey: "ct-start"})

	tRounds := 0
	for _, b := range tend.TPatterns {
		tRounds += b.Rounds
		if b.Share.Total != 2 {
			t.Fatalf("T pattern %q share denominator = %d, want the 2 rounds played as T", b.Pattern, b.Share.Total)
		}
	}
	if tRounds != 2 {
		t.Fatalf("T pattern rounds = %d, want the 2 second-half rounds", tRounds)
	}

	ctRounds := 0
	for _, b := range tend.CTPatterns {
		ctRounds += b.Rounds
		if b.Share.Total != 2 {
			t.Fatalf("CT pattern %q share denominator = %d, want the 2 rounds played as CT", b.Pattern, b.Share.Total)
		}
	}
	if ctRounds != 2 {
		t.Fatalf("CT pattern rounds = %d, want the 2 first-half rounds", ctRounds)
	}

	// Every win must be attributable to exactly one of the two vocabularies.
	wins := 0
	for _, b := range tend.TPatterns {
		wins += b.WinRate.Count
	}
	for _, b := range tend.CTPatterns {
		wins += b.WinRate.Count
	}
	if wins != tend.Wins {
		t.Fatalf("pattern wins total %d, want the %d rounds the team won", wins, tend.Wins)
	}
}

// Without a perspective there is no "our" economy: a round has two. Reporting
// an "unknown" buy bucket covering every round, with a 0% win rate that is zero
// by construction, would read as a finding rather than as a missing filter.
func TestAggregateWithoutPerspectiveOmitsTheEconomyRollups(t *testing.T) {
	d := testDocument()
	tend := Aggregate(d, Filter{})

	if tend.RoundCount != len(d.Rounds) {
		t.Fatalf("round count = %d, want every round", tend.RoundCount)
	}
	if len(tend.Buys) != 0 {
		t.Fatalf("buys = %+v, want none without a perspective", tend.Buys)
	}
	if len(tend.Matchups) != 0 {
		t.Fatalf("matchups = %+v, want none without a perspective", tend.Matchups)
	}
	if len(tend.BuySites) != 0 {
		t.Fatalf("buy x site = %+v, want none without a perspective", tend.BuySites)
	}
	// Where rounds were played is a fact about the round, so it survives.
	if len(tend.Sites) == 0 {
		t.Fatal("site distribution must not need a perspective")
	}
	if tend.Timings.FirstContact.Samples == 0 {
		t.Fatal("timings must not need a perspective")
	}

	// With one, they come back.
	withSide := Aggregate(d, Filter{Side: SideT})
	if len(withSide.Buys) == 0 || len(withSide.BuySites) == 0 {
		t.Fatal("a perspective must restore the economy rollups")
	}
}
