package anticheat

import (
	"fmt"
	"math"
	"sort"
)

// MetricID names one scored signal. The identifiers are stable: they end up in
// stored artifacts and in the web UI.
type MetricID string

const (
	// MetricWallTracking is the share of a player's living time spent with the
	// crosshair on an enemy they cannot see. The single strongest wallhack
	// signal available from a demo alone.
	MetricWallTracking MetricID = "wall_tracking_pct"
	// MetricPreaimKills is the share of gun kills that came out of a sustained
	// crosshair lock on the victim through cover.
	MetricPreaimKills MetricID = "unspotted_preaim_kill_pct"
	// MetricSettle is the median time between the fastest pre-shot angle
	// change and the kill. Assisted aim does not need correction time.
	MetricSettle MetricID = "preshot_settle_ms"
	// MetricFlickSpeed is the 90th percentile of peak pre-shot angular speed.
	MetricFlickSpeed MetricID = "flick_speed_p90"
	// MetricReaction is the median time between an enemy becoming visible and
	// the kill that followed.
	MetricReaction MetricID = "reaction_ms"
	// MetricJitter is the share of pre-shot angle deltas that reversed
	// direction: machine micro-correction reverses far more than a hand.
	MetricJitter MetricID = "aim_jitter"
	// MetricUnspottedKills is the share of gun kills on a victim who was never
	// visible to the killer at any point in the round, excluding smoke and
	// wallbang kills where an unseen victim is expected.
	MetricUnspottedKills MetricID = "unspotted_kill_pct"
	// MetricHeadshot is the headshot share of gun kills.
	MetricHeadshot MetricID = "headshot_pct"
	// MetricKillsPerRound is raw output, included as context rather than as a
	// mechanical signal: a strong legitimate player scores high here too.
	MetricKillsPerRound MetricID = "kills_per_round"
)

// Direction says which tail of the baseline is the suspicious one.
type Direction string

const (
	// DirectionHigh means values above the professional mean are suspicious.
	DirectionHigh Direction = "high"
	// DirectionLow means values below the professional mean are suspicious.
	DirectionLow Direction = "low"
)

// cluster groups metrics by the kind of advantage they measure. Cheating comes
// in kinds: a wall-only user is extreme on information and ordinary on aim, and
// an aim-assisted one is the reverse. Scoring each kind separately is what lets
// either be caught (see composite).
type cluster string

const (
	// clusterInformation covers knowing where enemies are without seeing them.
	clusterInformation cluster = "information"
	// clusterAim covers how the crosshair reaches and settles on a target.
	clusterAim cluster = "aim"
	// clusterOutput covers raw results. It is context, never evidence: a strong
	// legitimate player scores as high here as a cheater, so it can inform the
	// composite but never drive it.
	clusterOutput cluster = "output"
)

// metricDef is the fixed description of one metric: what it means, which tail
// matters, which kind of advantage it belongs to, and how much of the composite
// it carries.
type metricDef struct {
	id          MetricID
	label       string
	unit        string
	direction   Direction
	cluster     cluster
	weight      float64
	minSamples  int
	description string
}

// metricDefs is the authoritative metric registry. Weights sum to 1: the
// information-advantage and timing signals carry the composite, while raw
// output (headshots, kills per round) is context a good player also produces.
var metricDefs = []metricDef{
	{
		id: MetricWallTracking, label: "Seguimiento a través de muros", unit: "%",
		direction: DirectionHigh, cluster: clusterInformation, weight: 0.22, minSamples: 2000,
		description: "Porcentaje del tiempo con vida apuntando a un enemigo que el jugador no puede ver.",
	},
	{
		id: MetricPreaimKills, label: "Bajas con preapuntado", unit: "%",
		direction: DirectionHigh, cluster: clusterInformation, weight: 0.18, minSamples: 8,
		description: "Bajas precedidas de un bloqueo sostenido de mira sobre la víctima a través de cobertura.",
	},
	{
		id: MetricSettle, label: "Estabilización antes del disparo", unit: "ms",
		direction: DirectionLow, cluster: clusterAim, weight: 0.15, minSamples: 8,
		description: "Tiempo mediano entre el pico de giro de la mira y la baja, solo en bajas con un giro real previo. Una mira asistida no necesita corregir.",
	},
	{
		id: MetricFlickSpeed, label: "Velocidad de flick (p90)", unit: "°/s",
		direction: DirectionHigh, cluster: clusterAim, weight: 0.12, minSamples: 8,
		description: "Percentil 90 de la velocidad angular máxima en el medio segundo previo a cada baja.",
	},
	{
		id: MetricReaction, label: "Tiempo de reacción", unit: "ms",
		direction: DirectionLow, cluster: clusterAim, weight: 0.12, minSamples: 6,
		description: "Tiempo mediano entre que el enemigo se hace visible y la baja.",
	},
	{
		id: MetricJitter, label: "Micro-corrección de mira", unit: "ratio",
		direction: DirectionHigh, cluster: clusterAim, weight: 0.08, minSamples: 8,
		description: "Proporción de cambios de ángulo que invierten el sentido antes del disparo.",
	},
	{
		id: MetricUnspottedKills, label: "Bajas sobre enemigo nunca visible", unit: "%",
		direction: DirectionHigh, cluster: clusterInformation, weight: 0.05, minSamples: 8,
		description: "Bajas sobre una víctima que nunca fue visible, excluyendo humo y penetraciones.",
	},
	{
		id: MetricHeadshot, label: "Porcentaje de headshots", unit: "%",
		direction: DirectionHigh, cluster: clusterOutput, weight: 0.05, minSamples: 8,
		description: "Proporción de bajas con arma de fuego que fueron a la cabeza.",
	},
	{
		id: MetricKillsPerRound, label: "Bajas por ronda", unit: "kpr",
		direction: DirectionHigh, cluster: clusterOutput, weight: 0.03, minSamples: 8,
		description: "Producción bruta. Es contexto: un jugador legítimo muy fuerte también puntúa alto aquí.",
	},
}

// MetricBaseline is the reference distribution of one metric.
type MetricBaseline struct {
	Mean   float64 `json:"mean"`
	StdDev float64 `json:"stddev"`
	// Samples is how many player-matches produced this distribution. Zero
	// marks a metric a calibration could not measure, which kept the shipped
	// distribution instead.
	Samples int `json:"samples"`
}

// Baseline is a full set of reference distributions plus its provenance.
type Baseline struct {
	ID          string                      `json:"id"`
	Source      string                      `json:"source"`
	Description string                      `json:"description"`
	Metrics     map[MetricID]MetricBaseline `json:"metrics"`
}

// BaselineHeader is the provenance recorded inside every report so a reader
// always knows what the scores were measured against.
type BaselineHeader struct {
	ID          string `json:"id"`
	Source      string `json:"source"`
	Description string `json:"description"`
	// Measured is true only when every scored metric came from a real corpus
	// of demos rather than from a hand-written estimate.
	Measured bool `json:"measured"`
}

// Validate reports whether the baseline can score every registered metric.
func (b Baseline) Validate() error {
	if b.ID == "" {
		return fmt.Errorf("baseline id is required")
	}
	for _, def := range metricDefs {
		mb, ok := b.Metrics[def.id]
		if !ok {
			return fmt.Errorf("baseline %q is missing metric %q", b.ID, def.id)
		}
		if mb.StdDev <= 0 {
			return fmt.Errorf("baseline %q metric %q: stddev must be > 0, got %g", b.ID, def.id, mb.StdDev)
		}
	}
	return nil
}

// header returns the provenance block copied into a report.
func (b Baseline) header() BaselineHeader {
	measured := true
	for _, def := range metricDefs {
		if b.Metrics[def.id].Samples <= 0 {
			measured = false
			break
		}
	}
	return BaselineHeader{ID: b.ID, Source: b.Source, Description: b.Description, Measured: measured}
}

// MetricScore is one metric measured on one player and placed against the
// baseline.
type MetricScore struct {
	ID          MetricID       `json:"id"`
	Label       string         `json:"label"`
	Unit        string         `json:"unit"`
	Description string         `json:"description"`
	Direction   Direction      `json:"direction"`
	Value       float64        `json:"value"`
	Samples     int            `json:"samples"`
	Baseline    MetricBaseline `json:"baseline"`
	// Z is how many standard deviations the value sits on the suspicious side
	// of the professional mean. Negative means safer than the baseline.
	Z float64 `json:"z"`
	// Suspicion is Z mapped to 0..100. It is only meaningful when Applied.
	Suspicion float64 `json:"suspicion"`
	Weight    float64 `json:"weight"`
	// Applied is false when the player produced too little of this metric for
	// the value to mean anything; the composite then ignores it entirely.
	Applied bool `json:"applied"`
}

// Verdict is the conservative band a composite score falls into.
type Verdict string

const (
	// VerdictInsufficient means the demo did not hold enough of this player to
	// say anything at all.
	VerdictInsufficient Verdict = "insufficient_data"
	// VerdictClean means nothing stood out against the baseline.
	VerdictClean Verdict = "clean"
	// VerdictInconclusive means some signals are elevated but not enough to
	// separate the player from a strong legitimate one.
	VerdictInconclusive Verdict = "inconclusive"
	// VerdictAnomalous means several signals sit outside professional play and
	// the flagged ticks deserve a human look.
	VerdictAnomalous Verdict = "anomalous"
	// VerdictHighlyAnomalous is the strongest band this tool will ever emit.
	// It still means "review this", not "this player cheats".
	VerdictHighlyAnomalous Verdict = "highly_anomalous"
)

// Scoring thresholds. They are deliberately hard to reach: a false positive
// here can end with a real person reported.
const (
	minGunKills           = 6
	minAppliedMetrics     = 3
	minVerdictConfidence  = 0.30
	highBandConfidence    = 0.60
	scoreInconclusiveBand = 40
	scoreAnomalousBand    = 60
	scoreHighBand         = 80
	// zMidpoint is the z-score mapped to a suspicion of 50: two standard
	// deviations beyond the professional mean.
	zMidpoint = 2.0
	// clusterWeight is how much of the composite the strongest kind of
	// advantage carries, with the rest coming from the overall mean. At 0.6 a
	// maxed-out single-kind cheat clears the anomalous band while a very strong
	// legitimate player stays well inside "clean".
	clusterWeight = 0.6
	// zClamp bounds z so one wild metric cannot dominate the composite.
	zClamp = 6.0
)

// report turns everything the collector observed into the final Report.
func (c *collector) report(baseline Baseline, opts Options) Report {
	rounds := c.rounds
	players := make([]PlayerReport, 0, len(c.tracks))
	for _, t := range c.tracks {
		players = append(players, t.playerReport(baseline, rounds, c.tickRate))
	}
	sort.Slice(players, func(i, j int) bool {
		if players[i].Score != players[j].Score {
			return players[i].Score > players[j].Score
		}
		return players[i].Name < players[j].Name
	})

	return Report{
		SchemaVersion: SchemaVersion,
		Source:        Source{DemoPath: opts.DemoPath, SHA256: opts.SHA256},
		Baseline:      baseline.header(),
		Match: MatchSummary{
			Map:          c.mapName,
			Rounds:       rounds,
			TickRate:     c.tickRate,
			SampledTicks: c.sampledTicks,
		},
		Players:     players,
		Limitations: standardLimitations(),
	}
}

// playerReport scores one player's raw observations against the baseline.
func (t *track) playerReport(baseline Baseline, rounds int, tickRate float64) PlayerReport {
	values, samples := t.metricValues(rounds)

	metrics := make([]MetricScore, 0, len(metricDefs))
	sums := map[cluster]*weightedSum{}
	applied := 0
	for _, def := range metricDefs {
		mb := baseline.Metrics[def.id]
		score := MetricScore{
			ID:          def.id,
			Label:       def.label,
			Unit:        def.unit,
			Description: def.description,
			Direction:   def.direction,
			Value:       round2(values[def.id]),
			Samples:     samples[def.id],
			Baseline:    mb,
			Weight:      def.weight,
		}
		if samples[def.id] >= def.minSamples && mb.StdDev > 0 {
			score.Z = round2(zScore(values[def.id], mb, def.direction))
			score.Suspicion = round1(suspicionFromZ(score.Z))
			score.Applied = true
			if sums[def.cluster] == nil {
				sums[def.cluster] = &weightedSum{}
			}
			sums[def.cluster].add(def.weight, score.Suspicion)
			applied++
		}
		metrics = append(metrics, score)
	}

	report := PlayerReport{
		SteamID64: fmt.Sprintf("%d", t.steamID),
		Name:      t.name,
		Team:      t.team,
		Rounds:    rounds,
		GunKills:  len(t.kills),
		Metrics:   metrics,
		Evidence:  t.evidence(),
	}
	report.Score = round1(composite(sums))
	report.Confidence = round2(confidence(len(t.kills), rounds, t.aliveTicks, tickRate))
	report.Verdict = verdict(report.Score, report.Confidence, len(t.kills), applied)
	return report
}

// metricValues reduces the raw kill observations and tick counters into one
// value plus a sample count per metric. The sample count is what decides
// whether a metric is trustworthy enough to enter the composite.
func (t *track) metricValues(rounds int) (map[MetricID]float64, map[MetricID]int) {
	values := map[MetricID]float64{}
	samples := map[MetricID]int{}

	if t.aliveTicks > 0 {
		values[MetricWallTracking] = 100 * float64(t.wallTrackTicks) / float64(t.aliveTicks)
	}
	samples[MetricWallTracking] = t.aliveTicks

	gunKills := len(t.kills)
	samples[MetricPreaimKills] = gunKills
	samples[MetricUnspottedKills] = gunKills
	samples[MetricHeadshot] = gunKills
	samples[MetricKillsPerRound] = gunKills

	var preaim, headshots, unspotted, unspottedEligible int
	settles := make([]float64, 0, gunKills)
	flicks := make([]float64, 0, gunKills)
	jitters := make([]float64, 0, gunKills)
	reactions := make([]float64, 0, gunKills)
	for _, k := range t.kills {
		if k.preaimLocked {
			preaim++
		}
		if k.headshot {
			headshots++
		}
		// A kill through smoke or through a wall is expected to land on an
		// invisible victim, so it says nothing here and is left out of the
		// denominator rather than counted as clean.
		if !k.throughSmoke && k.penetrated == 0 {
			unspottedEligible++
			if !k.victimEverSpotted {
				unspotted++
			}
		}
		if k.hasAngles {
			flicks = append(flicks, k.peakDegPerSec)
			jitters = append(jitters, k.jitter)
		}
		if k.hasSettle {
			settles = append(settles, k.settleMS)
		}
		if k.visibleForMS >= 0 && k.visibleForMS <= reactionWindowSeconds*1000 {
			reactions = append(reactions, k.visibleForMS)
		}
	}

	if gunKills > 0 {
		values[MetricPreaimKills] = 100 * float64(preaim) / float64(gunKills)
		values[MetricHeadshot] = 100 * float64(headshots) / float64(gunKills)
	}
	if unspottedEligible > 0 {
		values[MetricUnspottedKills] = 100 * float64(unspotted) / float64(unspottedEligible)
	}
	samples[MetricUnspottedKills] = unspottedEligible
	if rounds > 0 {
		values[MetricKillsPerRound] = float64(gunKills) / float64(rounds)
	} else {
		samples[MetricKillsPerRound] = 0
	}

	values[MetricSettle] = median(settles)
	samples[MetricSettle] = len(settles)
	values[MetricFlickSpeed] = percentile(flicks, 0.90)
	samples[MetricFlickSpeed] = len(flicks)
	values[MetricJitter] = median(jitters)
	samples[MetricJitter] = len(jitters)
	values[MetricReaction] = median(reactions)
	samples[MetricReaction] = len(reactions)

	return values, samples
}

// evidence returns the reviewable moments behind the metrics, worst first, so
// a human can seek straight to the ticks instead of trusting the numbers.
func (t *track) evidence() []Evidence {
	out := make([]Evidence, 0, maxEvidencePerPlayer)
	for _, k := range t.kills {
		switch {
		case k.preaimLocked:
			out = append(out, Evidence{
				Kind: EvidenceWallPreaim, Round: k.round, Tick: k.tick,
				Victim: k.victimName, Weapon: k.weapon,
				Detail: fmt.Sprintf("La mira siguió a %s durante al menos %.0f ms sin que fuera visible, hasta la baja.",
					k.victimName, preaimLockSeconds*1000),
			})
		case k.hasAngles && k.peakDegPerSec >= roboticFlickDegPerSec && k.settleMS <= roboticSettleMS:
			out = append(out, Evidence{
				Kind: EvidenceRoboticFlick, Round: k.round, Tick: k.tick,
				Victim: k.victimName, Weapon: k.weapon,
				Detail: fmt.Sprintf("Giro de %.0f °/s y disparo %.0f ms después, sin tiempo de corrección.",
					k.peakDegPerSec, k.settleMS),
			})
		case k.visibleForMS >= 0 && k.visibleForMS <= instantReactionMS:
			out = append(out, Evidence{
				Kind: EvidenceInstantReaction, Round: k.round, Tick: k.tick,
				Victim: k.victimName, Weapon: k.weapon,
				Detail: fmt.Sprintf("Baja %.0f ms después de que %s fuera visible por primera vez.",
					k.visibleForMS, k.victimName),
			})
		}
	}

	// Keep the most extreme moments rather than simply the earliest ones:
	// pre-aim outranks a robotic flick, which outranks an instant reaction.
	sort.SliceStable(out, func(i, j int) bool {
		return evidenceRank(out[i].Kind) > evidenceRank(out[j].Kind)
	})
	if len(out) > maxEvidencePerPlayer {
		out = out[:maxEvidencePerPlayer]
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Tick < out[j].Tick })
	return out
}

func evidenceRank(k EvidenceKind) int {
	switch k {
	case EvidenceWallPreaim:
		return 3
	case EvidenceRoboticFlick:
		return 2
	case EvidenceInstantReaction:
		return 1
	default:
		return 0
	}
}

// weightedSum accumulates one cluster's weighted suspicion.
type weightedSum struct {
	weighted float64
	weight   float64
}

func (w *weightedSum) add(weight, suspicion float64) {
	w.weighted += weight * suspicion
	w.weight += weight
}

// applied reports whether any metric of this cluster cleared its sample
// minimum.
func (w *weightedSum) applied() bool {
	return w != nil && w.weight > 0
}

// mean returns the cluster's weighted suspicion, or 0 when nothing applied.
func (w *weightedSum) mean() float64 {
	if !w.applied() {
		return 0
	}
	return w.weighted / w.weight
}

// composite folds the per-cluster suspicions into one 0..100 score.
//
// A plain weighted mean across every metric cannot flag a single-kind cheat: a
// wall-only user maxes out the information metrics and sits at the median on
// aim, so the mean lands halfway and reads as inconclusive. Taking the strongest
// of the information and aim clusters lets either kind carry the verdict, while
// keeping the overall mean in the blend stops one extreme metric — inside an
// otherwise ordinary cluster — from doing it alone.
//
// The output cluster (headshots, kills per round) never enters the max: a
// strong legitimate player scores just as high there as a cheater, so it may
// only nudge the mean.
func composite(sums map[cluster]*weightedSum) float64 {
	if !sums[clusterInformation].applied() && !sums[clusterAim].applied() {
		// Only raw output survived the sample minimums. That is context and
		// never evidence, so there is nothing here to score — and without it
		// the mean below would collapse onto the output metrics alone.
		return 0
	}
	strongest := math.Max(sums[clusterInformation].mean(), sums[clusterAim].mean())

	var weighted, weight float64
	for _, sum := range sums {
		weighted += sum.weighted
		weight += sum.weight
	}
	if weight <= 0 {
		return 0
	}
	return clusterWeight*strongest + (1-clusterWeight)*(weighted/weight)
}

// zScore returns how many standard deviations value sits on the suspicious
// side of the baseline mean.
func zScore(value float64, b MetricBaseline, dir Direction) float64 {
	if b.StdDev <= 0 {
		return 0
	}
	z := (value - b.Mean) / b.StdDev
	if dir == DirectionLow {
		z = -z
	}
	return math.Max(-zClamp, math.Min(zClamp, z))
}

// suspicionFromZ maps a z-score onto 0..100 with a logistic centred on
// zMidpoint, so matching the professional mean scores low and clearing it by
// two standard deviations scores 50.
func suspicionFromZ(z float64) float64 {
	return 100 / (1 + math.Exp(-(z - zMidpoint)))
}

// confidence weighs how much of this player the demo actually held: kills
// drive the aim metrics, rounds drive the outcome metrics, and living time
// drives the tracking metric.
func confidence(gunKills, rounds, aliveTicks int, tickRate float64) float64 {
	if tickRate <= 0 {
		tickRate = fallbackTickRate
	}
	aliveMinutes := float64(aliveTicks) / tickRate / 60
	return 0.5*math.Min(1, float64(gunKills)/20) +
		0.3*math.Min(1, float64(rounds)/16) +
		0.2*math.Min(1, aliveMinutes/5)
}

// verdict places a composite score into a band. Low confidence and thin
// samples can only lower the band, never raise it.
func verdict(score, conf float64, gunKills, appliedMetrics int) Verdict {
	if gunKills < minGunKills || appliedMetrics < minAppliedMetrics || conf < minVerdictConfidence {
		return VerdictInsufficient
	}
	switch {
	case score >= scoreHighBand && conf >= highBandConfidence:
		return VerdictHighlyAnomalous
	case score >= scoreAnomalousBand:
		return VerdictAnomalous
	case score >= scoreInconclusiveBand:
		return VerdictInconclusive
	default:
		return VerdictClean
	}
}

// median returns the middle value of xs, or 0 for an empty input.
func median(xs []float64) float64 {
	return percentile(xs, 0.5)
}

// percentile returns the linear-interpolated q-quantile of xs (0 <= q <= 1),
// or 0 for an empty input. xs is copied, so the caller's slice keeps its order.
func percentile(xs []float64, q float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sorted := append([]float64(nil), xs...)
	sort.Float64s(sorted)
	if len(sorted) == 1 {
		return sorted[0]
	}
	pos := q * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	return sorted[lo] + (sorted[hi]-sorted[lo])*(pos-float64(lo))
}

func round1(x float64) float64 { return math.Round(x*10) / 10 }
func round2(x float64) float64 { return math.Round(x*100) / 100 }
