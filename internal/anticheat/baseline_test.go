package anticheat

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

// calibrationReports returns n reports in which every player sits exactly at
// value for every metric, so a calibrated mean is predictable.
func calibrationReports(playerCount int, value float64) []Report {
	metrics := make([]MetricScore, 0, len(metricDefs))
	for _, def := range metricDefs {
		metrics = append(metrics, MetricScore{ID: def.id, Value: value, Applied: true})
	}
	players := make([]PlayerReport, 0, playerCount)
	for i := 0; i < playerCount; i++ {
		// Spread the values a little so the standard deviation is non-zero.
		scaled := make([]MetricScore, len(metrics))
		copy(scaled, metrics)
		for j := range scaled {
			scaled[j].Value = value + float64(i%5)
		}
		players = append(players, PlayerReport{SteamID64: "x", Metrics: scaled})
	}
	return []Report{{Players: players}}
}

func TestDefaultBaselineIsMeasuredAndOwnsItsMap(t *testing.T) {
	b := DefaultBaseline()
	if !b.header().Measured {
		t.Fatal("the shipped baseline must carry a sample count for every metric")
	}
	delete(b.Metrics, MetricWallTracking)
	if _, ok := DefaultBaseline().Metrics[MetricWallTracking]; !ok {
		t.Fatal("mutating one DefaultBaseline() must not corrupt the next")
	}
}

func TestBaselineRoundTripsThroughJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := DefaultBaseline().Encode(&buf); err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	got, err := LoadBaseline(&buf)
	if err != nil {
		t.Fatalf("LoadBaseline() = %v", err)
	}
	if got.ID != DefaultBaseline().ID {
		t.Fatalf("id = %q, want %q", got.ID, DefaultBaseline().ID)
	}
	if got.Metrics[MetricWallTracking] != DefaultBaseline().Metrics[MetricWallTracking] {
		t.Fatalf("wall tracking baseline = %+v, want the default", got.Metrics[MetricWallTracking])
	}
}

func TestLoadBaselineRejectsAnIncompleteDocument(t *testing.T) {
	_, err := LoadBaseline(strings.NewReader(`{"id":"partial","metrics":{"headshot_pct":{"mean":50,"stddev":8}}}`))
	if err == nil {
		t.Fatal("LoadBaseline() = nil error for a baseline missing most metrics")
	}
}

func TestLoadBaselineRejectsUnknownFields(t *testing.T) {
	_, err := LoadBaseline(strings.NewReader(`{"id":"x","metrics":{},"surprise":1}`))
	if err == nil {
		t.Fatal("LoadBaseline() = nil error for an unknown field")
	}
}

func TestCalibrateMeasuresEveryMetricWithEnoughSamples(t *testing.T) {
	b, _, err := Calibrate("pro-2026", "demos profesionales locales", calibrationReports(minCalibrationSamples, 10))
	if err != nil {
		t.Fatalf("Calibrate() = %v", err)
	}
	if err := b.Validate(); err != nil {
		t.Fatalf("calibrated baseline is invalid: %v", err)
	}
	if !b.header().Measured {
		t.Fatal("a fully measured baseline must report itself as measured")
	}
	got := b.Metrics[MetricHeadshot]
	if got.Samples != minCalibrationSamples {
		t.Fatalf("samples = %d, want %d", got.Samples, minCalibrationSamples)
	}
	if math.Abs(got.Mean-12) > 0.01 {
		t.Fatalf("center = %g, want the median 12 (values 10..14 spread evenly)", got.Mean)
	}
}

func TestCalibrateKeepsTheEditorialEstimateForThinMetrics(t *testing.T) {
	b, _, err := Calibrate("pro-thin", "muestra corta", calibrationReports(minCalibrationSamples-1, 10))
	if err != nil {
		t.Fatalf("Calibrate() = %v", err)
	}
	got, want := b.Metrics[MetricHeadshot], DefaultBaseline().Metrics[MetricHeadshot]
	if got.Mean != want.Mean || got.StdDev != want.StdDev {
		t.Fatalf("thin metric = %+v, want the shipped distribution %+v", got, want)
	}
	if got.Samples != 0 {
		t.Fatalf("thin metric samples = %d, want 0: this corpus did not measure it", got.Samples)
	}
	if b.header().Measured {
		t.Fatal("a baseline carrying an unmeasured metric must not report itself as measured")
	}
	if !strings.Contains(b.Description, "línea base por defecto") {
		t.Fatalf("description = %q, want it to name the metrics that fell back", b.Description)
	}
}

func TestCalibrateIgnoresUnappliedMetrics(t *testing.T) {
	reports := calibrationReports(minCalibrationSamples, 10)
	for i := range reports[0].Players {
		for j := range reports[0].Players[i].Metrics {
			reports[0].Players[i].Metrics[j].Applied = false
		}
	}
	b, _, err := Calibrate("pro-empty", "sin métricas aplicables", reports)
	if err != nil {
		t.Fatalf("Calibrate() = %v", err)
	}
	if got := b.Metrics[MetricHeadshot]; got.Samples != 0 || got.Mean != DefaultBaseline().Metrics[MetricHeadshot].Mean {
		t.Fatalf("headshot baseline = %+v, want the shipped distribution with no sample count", got)
	}
}

func TestCalibrateRejectsEmptyInput(t *testing.T) {
	if _, _, err := Calibrate("x", "", nil); err == nil {
		t.Fatal("Calibrate() = nil error with no reports")
	}
	if _, _, err := Calibrate("", "", calibrationReports(minCalibrationSamples, 10)); err == nil {
		t.Fatal("Calibrate() = nil error with no id")
	}
}

func TestRobustCenterScaleUsesTheMedian(t *testing.T) {
	center, scale := robustCenterScale([]float64{1, 2, 3, 4, 5})
	if math.Abs(center-3) > 1e-9 {
		t.Fatalf("center = %g, want the median 3", center)
	}
	// Deviations are 2,1,0,1,2; their median is 1.
	if math.Abs(scale-madToStdDev) > 1e-9 {
		t.Fatalf("scale = %g, want %g", scale, madToStdDev)
	}
}

func TestRobustCenterScaleIgnoresOutliers(t *testing.T) {
	clean := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9}
	// Two cheaters in the corpus would drag a mean and a standard deviation.
	contaminated := append(append([]float64(nil), clean...), 900, 1000)

	cleanCenter, cleanScale := robustCenterScale(clean)
	center, scale := robustCenterScale(contaminated)
	if math.Abs(center-cleanCenter) > 1 {
		t.Fatalf("center moved from %g to %g under contamination", cleanCenter, center)
	}
	if scale > 2*cleanScale {
		t.Fatalf("scale widened from %g to %g under contamination", cleanScale, scale)
	}
}

func TestCalibrateCountsEachDemoOnce(t *testing.T) {
	one := calibrationReports(minCalibrationSamples, 10)[0]
	one.Source.SHA256 = "aaa"
	copyOfOne := one
	other := calibrationReports(minCalibrationSamples, 40)[0]
	other.Source.SHA256 = "bbb"

	b, distinct, err := Calibrate("dedup", "corpus con duplicados", []Report{one, copyOfOne, other})
	if err != nil {
		t.Fatalf("Calibrate() = %v", err)
	}
	if distinct != 2 {
		t.Fatalf("distinct = %d, want 2: the repeated demo must count once", distinct)
	}
	// Two demos of 20 players each, deduplicated, leave 40 samples — not 60.
	if got := b.Metrics[MetricHeadshot].Samples; got != 2*minCalibrationSamples {
		t.Fatalf("samples = %d, want %d", got, 2*minCalibrationSamples)
	}
}

func TestCalibrateKeepsReportsWithNoHash(t *testing.T) {
	// A report with no content hash cannot be deduplicated, and dropping it
	// would silently shrink a corpus the caller believes it supplied.
	reports := []Report{calibrationReports(minCalibrationSamples, 10)[0], calibrationReports(minCalibrationSamples, 40)[0]}
	_, distinct, err := Calibrate("nohash", "corpus sin hashes", reports)
	if err != nil {
		t.Fatalf("Calibrate() = %v", err)
	}
	if distinct != 2 {
		t.Fatalf("distinct = %d, want 2", distinct)
	}
}
