// Package anticheat derives statistical cheat-suspicion signals from a CS2
// demo and scores them against a measured reference distribution.
//
// Everything here is demo-only and deterministic: one parser pass, no game
// launch, no rendered video, no network call. The demo remains the source of
// truth, exactly as it is for the recording pipeline.
//
// The output is an anomaly report, never a proof of guilt. A high score means
// "this player's mechanics and information usage sit far outside the reference
// distribution, and a human should look at the listed ticks". Legitimate causes for a high score exist (a smurf on a low-skill
// lobby, a POV demo with sparse angle data, a very short sample), which is why
// every player report carries a sample-size-derived confidence and why the
// verdict bands are deliberately conservative.
package anticheat

import (
	"context"
	"fmt"
	"sync"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
)

// SchemaVersion is the version of the Report JSON shape. Bump it whenever a
// field changes meaning so stored artifacts stay interpretable.
const SchemaVersion = 1

// Options configures one analysis run.
type Options struct {
	// Baseline is the reference distribution every metric is scored against.
	// The zero value means DefaultBaseline.
	Baseline Baseline

	// DemoPath and SHA256 are recorded in the report so a dossier can name the
	// exact file its findings came from. Both are optional.
	DemoPath string
	SHA256   string
}

// Source identifies the demo an analysis was derived from.
type Source struct {
	DemoPath string `json:"demo_path,omitempty"`
	SHA256   string `json:"sha256,omitempty"`
}

// MatchSummary is the match-level context a reviewer needs to judge a report.
type MatchSummary struct {
	Map      string  `json:"map"`
	Rounds   int     `json:"rounds"`
	TickRate float64 `json:"tick_rate"`
	// SampledTicks is how many distinct ticks contributed view-angle samples.
	// A POV demo or a heavily truncated recording yields far fewer, which is
	// the main reason a report can come back with low confidence.
	SampledTicks int `json:"sampled_ticks"`
}

// Report is the full result of one demo analysis.
type Report struct {
	SchemaVersion int            `json:"schema_version"`
	Source        Source         `json:"source"`
	Baseline      BaselineHeader `json:"baseline"`
	Match         MatchSummary   `json:"match"`
	Players       []PlayerReport `json:"players"`
	// Limitations spells out, in the artifact itself, what this analysis
	// cannot conclude. It travels with every dossier on purpose.
	Limitations []string `json:"limitations"`
}

// PlayerReport is one player's suspicion profile.
type PlayerReport struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name"`
	Team      string `json:"team"`

	Rounds   int `json:"rounds"`
	GunKills int `json:"gun_kills"`
	Kills    int `json:"kills"`
	Deaths   int `json:"deaths"`

	// Score is the weighted suspicion composite, 0..100.
	Score float64 `json:"score"`
	// Confidence is how much sample the score rests on, 0..1. A verdict is
	// never raised above "inconclusive" below minVerdictConfidence.
	Confidence float64 `json:"confidence"`
	Verdict    Verdict `json:"verdict"`

	Metrics  []MetricScore `json:"metrics"`
	Evidence []Evidence    `json:"evidence"`
}

// Evidence is one reviewable moment: a tick a human can seek to in the demo to
// see for themselves what the metric counted.
type Evidence struct {
	Kind   EvidenceKind `json:"kind"`
	Round  int          `json:"round"`
	Tick   int          `json:"tick"`
	Victim string       `json:"victim,omitempty"`
	Weapon string       `json:"weapon,omitempty"`
	Detail string       `json:"detail"`
}

// EvidenceKind labels why a moment was flagged.
type EvidenceKind string

const (
	// EvidenceWallPreaim is a kill whose crosshair was already tracking the
	// victim while the victim was not visible to the killer.
	EvidenceWallPreaim EvidenceKind = "wall_preaim"
	// EvidenceRoboticFlick is a kill preceded by a fast angle change that
	// settled onto the victim with no human-scale correction time.
	EvidenceRoboticFlick EvidenceKind = "robotic_flick"
	// EvidenceInstantReaction is a kill landed faster after the victim became
	// visible than an unaided human reaction allows.
	EvidenceInstantReaction EvidenceKind = "instant_reaction"
)

// maxEvidencePerPlayer caps how many reviewable moments a player report keeps.
// The report is meant to be read, and the highest-signal moments come first.
const maxEvidencePerPlayer = 12

// Analyze runs one pass over p and returns the suspicion report. The caller
// owns p and remains responsible for Close.
func Analyze(p demoinfocs.Parser, opts Options) (Report, error) {
	baseline := opts.Baseline
	if len(baseline.Metrics) == 0 {
		baseline = DefaultBaseline()
	}

	c := newCollector(p)
	c.register(p)
	if err := p.ParseToEnd(); err != nil {
		return Report{}, fmt.Errorf("parse demo: %w", err)
	}

	return c.report(baseline, opts), nil
}

// AnalyzeWithContext drives Analyze but aborts parsing when ctx is cancelled,
// returning the context error instead of a partial report. demoinfocs has no
// context-aware entry point, so a watcher goroutine calls p.Cancel() — which
// aborts ParseToEnd — when ctx is done. The watcher is joined before returning,
// so a caller's Close never races a Cancel in flight.
func AnalyzeWithContext(ctx context.Context, p demoinfocs.Parser, opts Options) (Report, error) {
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		select {
		case <-ctx.Done():
			p.Cancel()
		case <-stop:
		}
	})
	defer func() {
		close(stop)
		wg.Wait()
	}()

	report, err := Analyze(p, opts)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Report{}, fmt.Errorf("analyze demo: %w", ctxErr)
	}
	return report, err
}

// Player returns the report for one SteamID64, or false when the demo held no
// scoreable sample for that player.
func (r Report) Player(steamID64 string) (PlayerReport, bool) {
	for _, p := range r.Players {
		if p.SteamID64 == steamID64 {
			return p, true
		}
	}
	return PlayerReport{}, false
}

// standardLimitations is copied into every report. These are the conclusions
// the analysis is structurally unable to reach, and they ship with the data so
// no downstream surface can present the score as more than it is.
func standardLimitations() []string {
	return []string{
		"Este informe es un detector de anomalías estadísticas, no una prueba de trampas.",
		"La visibilidad se aproxima con la máscara de 'spotted' de la demo, que no es un trazado de línea de visión real.",
		"Una muestra corta, una demo POV o una partida con nivel muy desigual pueden elevar la puntuación de un jugador legítimo.",
		"La referencia es juego profesional medido; un jugador legítimo por debajo de ese nivel puede desviarse sin hacer trampas.",
		"Solo una revisión humana de los ticks señalados puede confirmar o descartar lo que miden las métricas.",
	}
}
