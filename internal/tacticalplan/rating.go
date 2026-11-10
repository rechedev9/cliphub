package tacticalplan

// RatingFormula is ClipHub's own rating, written the way a person reads it.
const RatingFormula = "0.0073*KAST + 0.3591*KPR - 0.5329*DPR + 0.2372*impact + 0.0032*ADR + 0.1587, with impact = 2.13*KPR + 0.42*APR - 0.41"

// RatingLine is what the rating reads: totals over some rounds of one player.
type RatingLine struct {
	Rounds, Kills, Deaths, Assists, Damage, KASTRounds int
}

// Rating is ClipHub's rating of a line, or 0 when it holds no rounds.
func Rating(l RatingLine) float64 {
	if l.Rounds <= 0 {
		return 0
	}
	rounds := float64(l.Rounds)
	kpr, dpr, apr, adr := float64(l.Kills)/rounds, float64(l.Deaths)/rounds, float64(l.Assists)/rounds, float64(l.Damage)/rounds
	kast := 100 * float64(l.KASTRounds) / rounds
	impact := 2.13*kpr + 0.42*apr - 0.41
	return 0.0073*kast + 0.3591*kpr - 0.5329*dpr + 0.2372*impact + 0.0032*adr + 0.1587
}
