package anticheat

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
)

// minCalibrationSamples is how many player-matches a metric needs before a
// calibrated baseline will publish a distribution for it. Below this the
// spread is noise and would make the z-scores meaningless. Fifteen is low
// because the estimator is a median and a MAD rather than a mean and a
// standard deviation, and because every consumer of a baseline sees the
// per-metric sample count and can judge it.
const minCalibrationSamples = 15

// defaultBaselineJSON is the reference distribution FragForge ships with. It
// lives as data rather than as literals so recalibrating is a file swap and
// its provenance travels with the numbers.
//
//go:embed baseline_default.json
var defaultBaselineJSON []byte

// loadDefaultBaseline parses the embedded baseline once. A malformed embedded
// file is a build-time mistake, not a runtime condition, so it panics rather
// than silently degrading every score in the product.
var loadDefaultBaseline = sync.OnceValue(func() Baseline {
	b, err := LoadBaseline(strings.NewReader(string(defaultBaselineJSON)))
	if err != nil {
		panic("anticheat: embedded default baseline is invalid: " + err.Error())
	}
	return b
})

// DefaultBaseline returns the reference distribution FragForge ships with: the
// observed CS2 player population, measured with a median and a MAD-derived
// spread so cheaters inside the corpus cannot widen it.
//
// It is a population baseline, not professional play. That is the stricter
// choice for the signals that carry the composite — pros do not track enemies
// through walls any more than anyone else — but a report always names the
// baseline it used, and `zv demo anticheat calibrate` replaces it with one
// measured over demos of your choosing.
//
// The returned Baseline owns its map, so a caller can adjust a copy without
// changing what the next caller sees.
func DefaultBaseline() Baseline {
	b := loadDefaultBaseline()
	metrics := make(map[MetricID]MetricBaseline, len(b.Metrics))
	for id, m := range b.Metrics {
		metrics[id] = m
	}
	b.Metrics = metrics
	return b
}

// LoadBaseline decodes a baseline document and rejects one that cannot score
// every registered metric, so a truncated file fails at load instead of
// silently disabling half the detector.
func LoadBaseline(r io.Reader) (Baseline, error) {
	var b Baseline
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return Baseline{}, fmt.Errorf("decode baseline: %w", err)
	}
	if err := b.Validate(); err != nil {
		return Baseline{}, err
	}
	return b, nil
}

// Encode writes the baseline as indented JSON.
func (b Baseline) Encode(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(b); err != nil {
		return fmt.Errorf("encode baseline: %w", err)
	}
	return nil
}

// Calibrate builds a measured baseline from analyses of a reference corpus,
// typically demos that are known to contain professional play.
//
// It reads the per-player metric values out of the supplied reports — the
// values themselves do not depend on any baseline, only their scoring does —
// and keeps a metric only when enough player-matches contributed a usable
// sample. Metrics that fall short keep the shipped default distribution, and
// the resulting Samples count records which is which.
//
// Reports of the same demo (identical Source.SHA256) count once. The returned
// distinct count is how many demos actually shaped the baseline, which is the
// number worth putting in front of whoever has to trust it.
func Calibrate(id, description string, reports []Report) (Baseline, int, error) {
	if id == "" {
		return Baseline{}, 0, fmt.Errorf("baseline id is required")
	}
	if len(reports) == 0 {
		return Baseline{}, 0, fmt.Errorf("no reports to calibrate from")
	}

	observed := map[MetricID][]float64{}
	// The same match reaching the corpus twice — a re-upload, a copied file, a
	// series part stored under two job ids — would weigh its players double and
	// quietly bend the baseline toward one lobby. Reports carry the demo's
	// content hash, so the second copy is simply dropped.
	seen := map[string]bool{}
	distinct := 0
	for _, report := range reports {
		if sum := report.Source.SHA256; sum != "" {
			if seen[sum] {
				continue
			}
			seen[sum] = true
		}
		distinct++
		for _, player := range report.Players {
			for _, metric := range player.Metrics {
				if !metric.Applied {
					continue
				}
				observed[metric.ID] = append(observed[metric.ID], metric.Value)
			}
		}
	}

	fallback := DefaultBaseline()
	out := Baseline{
		ID:          id,
		Source:      "calibrated-from-demos",
		Description: description,
		Metrics:     map[MetricID]MetricBaseline{},
	}
	var thin []string
	for _, def := range metricDefs {
		values := observed[def.id]
		center, scale := robustCenterScale(values)
		if len(values) < minCalibrationSamples || scale <= 0 {
			// Keep the shipped distribution but drop its sample count: this
			// corpus did not measure the metric, and claiming otherwise would
			// let the report call itself fully measured when it is not.
			kept := fallback.Metrics[def.id]
			kept.Samples = 0
			out.Metrics[def.id] = kept
			thin = append(thin, string(def.id))
			continue
		}
		out.Metrics[def.id] = MetricBaseline{
			Mean:    round2(center),
			StdDev:  round2(scale),
			Samples: len(values),
		}
	}
	out.Description = fmt.Sprintf("%s (%d demos distintas)", description, distinct)
	if len(thin) > 0 {
		sort.Strings(thin)
		out.Description = fmt.Sprintf("%s (métricas sin muestra suficiente, mantienen la línea base por defecto: %v)",
			out.Description, thin)
	}
	if err := out.Validate(); err != nil {
		return Baseline{}, 0, err
	}
	return out, distinct, nil
}

// madToStdDev converts a median absolute deviation into the standard deviation
// of a normal distribution with the same spread.
const madToStdDev = 1.4826

// robustCenterScale returns the median and a MAD-derived standard deviation.
//
// A calibration corpus is never guaranteed clean — a public matchmaking demo
// can contain the exact behaviour the detector is looking for — and the mean
// and standard deviation would both absorb those outliers, widening the
// baseline until nothing ever looks unusual. The median and the median
// absolute deviation ignore up to half the sample, so a handful of cheaters in
// the corpus cannot quietly disarm the detector.
//
// It assumes len(xs) >= 2, which Calibrate enforces through
// minCalibrationSamples.
func robustCenterScale(xs []float64) (center, scale float64) {
	center = percentile(xs, 0.5)
	deviations := make([]float64, len(xs))
	for i, x := range xs {
		deviations[i] = math.Abs(x - center)
	}
	return center, madToStdDev * percentile(deviations, 0.5)
}
