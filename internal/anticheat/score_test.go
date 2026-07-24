package anticheat

import (
	"math"
	"strings"
	"testing"
)

// gunKill returns a plain kill observation with human-looking aim, so a test
// only has to state the property it cares about.
func gunKill(tick int) killObservation {
	return killObservation{
		tick:              tick,
		round:             1,
		victimName:        "victim",
		weapon:            "AK-47",
		hasAngles:         true,
		hasSettle:         true,
		peakDegPerSec:     300,
		settleMS:          200,
		jitter:            0.25,
		visibleForMS:      400,
		victimEverSpotted: true,
	}
}

// newTrack returns a track with the given kills and a plausible amount of
// living time, so metrics that divide by aliveTicks are defined.
func newTrack(kills []killObservation, aliveTicks, wallTrackTicks int) *track {
	return &track{
		steamID:        76561198000000000,
		name:           "player",
		team:           "CT",
		kills:          kills,
		aliveTicks:     aliveTicks,
		wallTrackTicks: wallTrackTicks,
		preaimTicks:    map[uint64]int{},
		spottedSince:   map[uint64]int{},
	}
}

func metricByID(t *testing.T, report PlayerReport, id MetricID) MetricScore {
	t.Helper()
	for _, m := range report.Metrics {
		if m.ID == id {
			return m
		}
	}
	t.Fatalf("metric %q missing from report", id)
	return MetricScore{}
}

func TestMetricWeightsSumToOne(t *testing.T) {
	var total float64
	for _, def := range metricDefs {
		total += def.weight
	}
	if math.Abs(total-1) > 1e-9 {
		t.Fatalf("metric weights sum = %g, want 1", total)
	}
}

func TestDefaultBaselineCoversEveryMetric(t *testing.T) {
	if err := DefaultBaseline().Validate(); err != nil {
		t.Fatalf("DefaultBaseline().Validate() = %v, want nil", err)
	}
}

func TestBaselineValidateRejectsMissingMetric(t *testing.T) {
	b := DefaultBaseline()
	delete(b.Metrics, MetricWallTracking)
	if err := b.Validate(); err == nil {
		t.Fatal("Validate() = nil, want an error for the missing metric")
	}
}

func TestBaselineValidateRejectsZeroStdDev(t *testing.T) {
	b := DefaultBaseline()
	b.Metrics[MetricReaction] = MetricBaseline{Mean: 400, StdDev: 0}
	if err := b.Validate(); err == nil {
		t.Fatal("Validate() = nil, want an error for a zero standard deviation")
	}
}

func TestZScoreFlipsForLowDirectionMetrics(t *testing.T) {
	b := MetricBaseline{Mean: 400, StdDev: 100}
	// 200 ms is two standard deviations *faster* than the pro mean, which is
	// the suspicious side for a reaction time.
	if got := zScore(200, b, DirectionLow); math.Abs(got-2) > 1e-9 {
		t.Fatalf("zScore(200, low) = %g, want 2", got)
	}
	if got := zScore(200, b, DirectionHigh); math.Abs(got+2) > 1e-9 {
		t.Fatalf("zScore(200, high) = %g, want -2", got)
	}
}

func TestZScoreIsClamped(t *testing.T) {
	b := MetricBaseline{Mean: 0, StdDev: 1}
	if got := zScore(1000, b, DirectionHigh); got != zClamp {
		t.Fatalf("zScore(1000) = %g, want the clamp %g", got, zClamp)
	}
}

func TestSuspicionIsFiftyAtTheMidpoint(t *testing.T) {
	if got := suspicionFromZ(zMidpoint); math.Abs(got-50) > 1e-9 {
		t.Fatalf("suspicionFromZ(%g) = %g, want 50", zMidpoint, got)
	}
	if suspicionFromZ(0) >= 20 {
		t.Fatalf("matching the pro mean should score low, got %g", suspicionFromZ(0))
	}
	if suspicionFromZ(5) <= 90 {
		t.Fatalf("clearing the pro mean by 5σ should score high, got %g", suspicionFromZ(5))
	}
}

func TestPercentileInterpolates(t *testing.T) {
	xs := []float64{10, 20, 30, 40}
	if got := percentile(xs, 0.5); math.Abs(got-25) > 1e-9 {
		t.Fatalf("median = %g, want 25", got)
	}
	if got := percentile(xs, 0); got != 10 {
		t.Fatalf("p0 = %g, want 10", got)
	}
	if got := percentile(xs, 1); got != 40 {
		t.Fatalf("p100 = %g, want 40", got)
	}
	if got := percentile(nil, 0.9); got != 0 {
		t.Fatalf("percentile(nil) = %g, want 0", got)
	}
}

func TestPercentileLeavesTheCallerSliceOrdered(t *testing.T) {
	xs := []float64{30, 10, 20}
	percentile(xs, 0.5)
	if xs[0] != 30 || xs[1] != 10 || xs[2] != 20 {
		t.Fatalf("percentile reordered the caller's slice: %v", xs)
	}
}

func TestCleanPlayerScoresLow(t *testing.T) {
	kills := make([]killObservation, 0, 20)
	for i := 0; i < 20; i++ {
		kills = append(kills, gunKill(1000+i*100))
	}
	report := newTrack(kills, 40000, 700).playerReport(DefaultBaseline(), 26, 64)

	if report.Verdict != VerdictClean {
		t.Fatalf("verdict = %q, want %q (score %.1f)", report.Verdict, VerdictClean, report.Score)
	}
	if report.Score >= scoreInconclusiveBand {
		t.Fatalf("score = %.1f, want below %d", report.Score, scoreInconclusiveBand)
	}
}

func TestWallhackLikeProfileScoresHigh(t *testing.T) {
	kills := make([]killObservation, 0, 20)
	for i := 0; i < 20; i++ {
		k := gunKill(1000 + i*100)
		k.preaimLocked = true
		k.settleMS = 15
		k.peakDegPerSec = 1400
		k.jitter = 0.7
		k.headshot = i%10 != 0
		// Half the victims were killed the instant they became visible; the
		// other half were never visible to the killer at all.
		if i%2 == 0 {
			k.visibleForMS = 60
		} else {
			k.visibleForMS = -1
			k.victimEverSpotted = false
		}
		kills = append(kills, k)
	}
	report := newTrack(kills, 40000, 12000).playerReport(DefaultBaseline(), 26, 64)

	if report.Verdict != VerdictHighlyAnomalous {
		t.Fatalf("verdict = %q, want %q (score %.1f, confidence %.2f)",
			report.Verdict, VerdictHighlyAnomalous, report.Score, report.Confidence)
	}
	if wall := metricByID(t, report, MetricWallTracking); !wall.Applied || wall.Value <= 25 {
		t.Fatalf("wall tracking = %.1f%% (applied=%v), want a high applied value", wall.Value, wall.Applied)
	}
}

func TestThinSampleNeverProducesAVerdict(t *testing.T) {
	kills := []killObservation{gunKill(1000), gunKill(1100)}
	for i := range kills {
		kills[i].preaimLocked = true
		kills[i].settleMS = 0
		kills[i].visibleForMS = 10
	}
	report := newTrack(kills, 900, 890).playerReport(DefaultBaseline(), 3, 64)

	if report.Verdict != VerdictInsufficient {
		t.Fatalf("verdict = %q, want %q; two kills must never convict", report.Verdict, VerdictInsufficient)
	}
}

func TestHighBandRequiresConfidence(t *testing.T) {
	// A maximal score with middling confidence must stop one band short.
	if got := verdict(100, highBandConfidence-0.01, 30, 9); got != VerdictAnomalous {
		t.Fatalf("verdict = %q, want %q when confidence is below the high band", got, VerdictAnomalous)
	}
	if got := verdict(100, highBandConfidence, 30, 9); got != VerdictHighlyAnomalous {
		t.Fatalf("verdict = %q, want %q at the high-band confidence", got, VerdictHighlyAnomalous)
	}
}

func TestMetricIsNotAppliedBelowItsMinimumSample(t *testing.T) {
	// Six kills clear minGunKills but not the eight-kill aim metrics.
	kills := make([]killObservation, 0, 6)
	for i := 0; i < 6; i++ {
		kills = append(kills, gunKill(1000+i*100))
	}
	report := newTrack(kills, 40000, 700).playerReport(DefaultBaseline(), 20, 64)

	if m := metricByID(t, report, MetricHeadshot); m.Applied {
		t.Fatal("headshot% must not be scored on six kills")
	}
	if m := metricByID(t, report, MetricWallTracking); !m.Applied {
		t.Fatal("wall tracking has its own tick-based sample and should still apply")
	}
}

func TestSmokeAndWallbangKillsLeaveTheUnspottedDenominator(t *testing.T) {
	kills := make([]killObservation, 0, 10)
	for i := 0; i < 10; i++ {
		k := gunKill(1000 + i*100)
		k.visibleForMS = -1
		k.victimEverSpotted = false
		if i < 5 {
			k.throughSmoke = true
		}
		kills = append(kills, k)
	}
	report := newTrack(kills, 40000, 700).playerReport(DefaultBaseline(), 20, 64)

	m := metricByID(t, report, MetricUnspottedKills)
	if m.Samples != 5 {
		t.Fatalf("unspotted-kill samples = %d, want 5 (smoke kills excluded)", m.Samples)
	}
	if math.Abs(m.Value-100) > 1e-9 {
		t.Fatalf("unspotted-kill value = %g, want 100", m.Value)
	}
}

// Holding a static angle and killing whoever walks into it is the most ordinary
// kill in CS2, and it leaves no pre-shot turn to settle from. Those kills must
// stay out of the settle metric instead of entering it as 0 ms, which is the
// most suspicious value the metric has.
func TestKillsWithoutAPreShotTurnLeaveTheSettleMetric(t *testing.T) {
	kills := make([]killObservation, 0, 20)
	for i := 0; i < 20; i++ {
		k := gunKill(1000 + i*100)
		k.peakDegPerSec = 0
		k.settleMS = 0
		k.hasSettle = false
		kills = append(kills, k)
	}
	report := newTrack(kills, 40000, 700).playerReport(DefaultBaseline(), 26, 64)

	if m := metricByID(t, report, MetricSettle); m.Applied || m.Samples != 0 {
		t.Fatalf("settle metric = %+v, want no samples for kills with no pre-shot turn", m)
	}
	if m := metricByID(t, report, MetricFlickSpeed); !m.Applied {
		t.Fatal("flick speed still has a usable sample and must stay applied")
	}
	if report.Verdict != VerdictClean {
		t.Fatalf("verdict = %q, want %q for a player who simply held angles (score %.1f)",
			report.Verdict, VerdictClean, report.Score)
	}
}

// A victim the killer had already seen this round is not an unseen kill, even
// when the spotted mask happened to be off at the exact kill tick.
func TestUnspottedKillsCountOnlyVictimsNeverSeen(t *testing.T) {
	kills := make([]killObservation, 0, 10)
	for i := 0; i < 10; i++ {
		k := gunKill(1000 + i*100)
		// Visibility had broken by the kill tick for every one of them, but
		// half had been in view earlier in the round.
		k.visibleForMS = -1
		k.victimEverSpotted = i < 5
		kills = append(kills, k)
	}
	report := newTrack(kills, 40000, 700).playerReport(DefaultBaseline(), 20, 64)

	m := metricByID(t, report, MetricUnspottedKills)
	if m.Samples != 10 {
		t.Fatalf("unspotted-kill samples = %d, want all 10 kills in the denominator", m.Samples)
	}
	if math.Abs(m.Value-50) > 1e-9 {
		t.Fatalf("unspotted-kill value = %g, want 50: only the never-seen half counts", m.Value)
	}
}

func TestEvidencePrefersPreaimAndStaysCapped(t *testing.T) {
	kills := make([]killObservation, 0, 40)
	for i := 0; i < 20; i++ {
		k := gunKill(1000 + i)
		k.visibleForMS = 10 // instant reaction, the weakest evidence kind
		kills = append(kills, k)
	}
	for i := 0; i < 20; i++ {
		k := gunKill(5000 + i)
		k.preaimLocked = true
		kills = append(kills, k)
	}
	evidence := newTrack(kills, 40000, 700).evidence()

	if len(evidence) != maxEvidencePerPlayer {
		t.Fatalf("evidence count = %d, want the %d cap", len(evidence), maxEvidencePerPlayer)
	}
	for _, e := range evidence {
		if e.Kind != EvidenceWallPreaim {
			t.Fatalf("evidence kind = %q, want the pre-aim moments to win the cap", e.Kind)
		}
	}
	for i := 1; i < len(evidence); i++ {
		if evidence[i-1].Tick > evidence[i].Tick {
			t.Fatal("evidence must stay in tick order so a reviewer can walk it")
		}
	}
}

func TestReportOrdersPlayersByScore(t *testing.T) {
	clean := make([]killObservation, 0, 20)
	dirty := make([]killObservation, 0, 20)
	for i := 0; i < 20; i++ {
		clean = append(clean, gunKill(1000+i*100))
		k := gunKill(1000 + i*100)
		k.preaimLocked = true
		k.settleMS = 10
		dirty = append(dirty, k)
	}

	c := &collector{tickRate: 64, rounds: 26, mapName: "de_dust2", tracks: map[uint64]*track{
		1: newTrack(clean, 40000, 700),
		2: newTrack(dirty, 40000, 12000),
	}}
	c.tracks[1].steamID, c.tracks[1].name = 1, "clean"
	c.tracks[2].steamID, c.tracks[2].name = 2, "dirty"

	report := c.report(DefaultBaseline(), Options{DemoPath: "match.dem", SHA256: "abc"})
	if len(report.Players) != 2 {
		t.Fatalf("players = %d, want 2", len(report.Players))
	}
	if report.Players[0].Name != "dirty" {
		t.Fatalf("first player = %q, want the highest score first", report.Players[0].Name)
	}
	if report.Match.Map != "de_dust2" || report.Match.Rounds != 26 {
		t.Fatalf("match summary = %+v, want the collected map and rounds", report.Match)
	}
	if report.Source.DemoPath != "match.dem" || report.Source.SHA256 != "abc" {
		t.Fatalf("source = %+v, want the options carried through", report.Source)
	}
	if len(report.Limitations) == 0 {
		t.Fatal("every report must carry its limitations")
	}
	if report.SchemaVersion != SchemaVersion {
		t.Fatalf("schema version = %d, want %d", report.SchemaVersion, SchemaVersion)
	}
}

func TestReportPlayerLookup(t *testing.T) {
	report := Report{Players: []PlayerReport{{SteamID64: "76561198000000000", Name: "x"}}}
	if _, ok := report.Player("76561198000000000"); !ok {
		t.Fatal("Player() = false for a present steamid")
	}
	if _, ok := report.Player("nope"); ok {
		t.Fatal("Player() = true for an absent steamid")
	}
}

func TestLimitationsRefuseToClaimProof(t *testing.T) {
	joined := strings.Join(standardLimitations(), " ")
	if !strings.Contains(joined, "no una prueba") {
		t.Fatalf("limitations must state this is not proof, got: %s", joined)
	}
}

// proMedianKill returns a kill whose aim matches the shipped professional
// baseline exactly, so a test can isolate one signal from the rest.
func proMedianKill(tick int) killObservation {
	b := DefaultBaseline()
	return killObservation{
		tick: tick, round: 1, victimName: "victim", weapon: "AK-47",
		hasAngles:         true,
		hasSettle:         true,
		peakDegPerSec:     b.Metrics[MetricFlickSpeed].Mean,
		settleMS:          b.Metrics[MetricSettle].Mean,
		jitter:            b.Metrics[MetricJitter].Mean,
		visibleForMS:      b.Metrics[MetricReaction].Mean,
		victimEverSpotted: true,
	}
}

// A wall-only cheat is the common case and the hardest to see: the information
// metrics max out while the aim metrics stay ordinary. A plain weighted mean
// over every metric lands halfway and reads as inconclusive, which is what the
// cluster blend exists to prevent.
func TestWallOnlyProfileStillReachesTheReviewBand(t *testing.T) {
	kills := make([]killObservation, 0, 20)
	for i := 0; i < 20; i++ {
		k := proMedianKill(1000 + i*100)
		k.preaimLocked = true
		if i%2 == 0 {
			k.visibleForMS = -1 // killed through cover
			k.victimEverSpotted = false
		}
		kills = append(kills, k)
	}
	// 22% of living time with the crosshair on an enemy behind cover.
	report := newTrack(kills, 40000, 8800).playerReport(DefaultBaseline(), 26, 64)

	if report.Score < scoreAnomalousBand {
		t.Fatalf("score = %.1f, want at least %d for a maxed-out wallhack profile",
			report.Score, scoreAnomalousBand)
	}
	if report.Verdict != VerdictAnomalous && report.Verdict != VerdictHighlyAnomalous {
		t.Fatalf("verdict = %q, want a review band (score %.1f)", report.Verdict, report.Score)
	}
}

// The mirror case: ordinary information, machine-grade aim.
func TestAimOnlyProfileStillReachesTheReviewBand(t *testing.T) {
	kills := make([]killObservation, 0, 20)
	for i := 0; i < 20; i++ {
		k := proMedianKill(1000 + i*100)
		k.settleMS = 20
		k.peakDegPerSec = 900
		k.jitter = 0.35
		k.visibleForMS = 70
		kills = append(kills, k)
	}
	report := newTrack(kills, 40000, 2292).playerReport(DefaultBaseline(), 26, 64)

	if report.Score < scoreAnomalousBand {
		t.Fatalf("score = %.1f, want at least %d for a maxed-out aim profile",
			report.Score, scoreAnomalousBand)
	}
}

// The blend must not turn skill into an accusation: a player well above the
// professional median on every legitimate axis stays clean.
func TestVeryStrongLegitimatePlayerStaysClean(t *testing.T) {
	b := DefaultBaseline()
	kills := make([]killObservation, 0, 24)
	for i := 0; i < 24; i++ {
		k := proMedianKill(1000 + i*100)
		// Reaction and flick a step and a half beyond the professional median,
		// and headshots on nearly every kill.
		k.visibleForMS = b.Metrics[MetricReaction].Mean - 1.5*b.Metrics[MetricReaction].StdDev
		k.peakDegPerSec = b.Metrics[MetricFlickSpeed].Mean + 1.5*b.Metrics[MetricFlickSpeed].StdDev
		k.headshot = i%5 != 0
		kills = append(kills, k)
	}
	report := newTrack(kills, 40000, 2292).playerReport(b, 26, 64)

	if report.Verdict != VerdictClean {
		t.Fatalf("verdict = %q, want %q (score %.1f)", report.Verdict, VerdictClean, report.Score)
	}
}

// Raw output alone must never carry a verdict: headshot share and kills per
// round are exactly what a strong legitimate player produces.
func TestOutputMetricsAloneCannotDriveTheComposite(t *testing.T) {
	sums := map[cluster]*weightedSum{clusterOutput: {}}
	sums[clusterOutput].add(0.05, 100)
	sums[clusterOutput].add(0.03, 100)
	if got := composite(sums); got >= scoreInconclusiveBand {
		t.Fatalf("composite = %.1f from output metrics alone, want below %d", got, scoreInconclusiveBand)
	}
}

func TestCompositeIsZeroWithoutAnyAppliedMetric(t *testing.T) {
	if got := composite(map[cluster]*weightedSum{}); got != 0 {
		t.Fatalf("composite = %g, want 0", got)
	}
}
