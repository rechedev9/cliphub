package editor

import (
	"strconv"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

// Ranges FFmpeg 8.1 accepts for the filter options this package computes.
// ffmpeg aborts the whole render when one is exceeded (Studio 3.0.0 incident).
var computedFilterRanges = map[string]map[string][2]float64{
	"loudnorm": {"I": {-70, -5}, "TP": {-9, 0}, "LRA": {1, 50}},
	"alimiter": {"limit": {0.0625, 1}},
}

// assertFilterRanges fails when a computed option in chain is outside its range
// and returns how many options it checked, so callers can rule out a vacuous pass.
func assertFilterRanges(t *testing.T, chain string) int {
	t.Helper()
	checked := 0
	for _, stage := range strings.Split(chain, ",") {
		name, args, _ := strings.Cut(stage, "=")
		ranges, ok := computedFilterRanges[name]
		if !ok {
			continue
		}
		for _, arg := range strings.Split(args, ":") {
			key, raw, _ := strings.Cut(arg, "=")
			bounds, ok := ranges[key]
			if !ok {
				continue
			}
			value, err := strconv.ParseFloat(raw, 64)
			if err != nil || value < bounds[0] || value > bounds[1] {
				t.Fatalf("%s %s=%s is outside [%g, %g] in %q", name, key, raw, bounds[0], bounds[1], chain)
			}
			checked++
		}
	}
	return checked
}

// Generalizes TestFullDemoMasterRetargetStaysWithinLoudnormRange from the
// incident's single measurement to the whole plausible measurement space: the
// native retarget and the Media Foundation recovery must keep every computed
// filter option in range and reach their exit however far the AAC misses.
func TestFullDemoMasteringFiltersStayInRangeForAnyMeasurement(t *testing.T) {
	target := recapplan.DefaultOptions().Audio.Loudness
	lra, threshold, offset := 7.0, -30.0, 0.0
	for _, lufs := range []float64{-80, -40, -23, -14, -8, -5, 0} {
		for _, peak := range []float64{-40, -9, -1.8, -1.5, 0, 3, 12, 40} {
			decoded := LoudnessMeasurement{Status: "measured", IntegratedLUFS: &lufs, TruePeakDBTP: &peak, LRA: &lra, Threshold: &threshold, Offset: &offset}

			current := target
			current.TargetTPDBTP -= 0.3
			exhausted := false
			for range 100 {
				filter, err := measuredLoudnessFilter(current, decoded)
				if err != nil {
					t.Fatal(err)
				}
				if assertFilterRanges(t, filter) != 3 {
					t.Fatalf("native master filter lost a loudnorm target: %q", filter)
				}
				var changed bool
				if current, changed = nextMasterTarget(current, target, decoded); !changed {
					exhausted = true
					break
				}
			}
			if !exhausted {
				t.Fatalf("native retarget never handed over to recovery for %.1f LUFS / %.1f dBTP: %+v", lufs, peak, current)
			}

			base, err := measuredLoudnessFilter(fullDemoAACRecoveryTarget(target), decoded)
			if err != nil {
				t.Fatal(err)
			}
			master := ProgramAACFallbackMaster{Encoder: "aac_mf", GainDB: 3, CeilingDBFS: target.TargetTPDBTP - 3.5}
			for range 100 {
				if assertFilterRanges(t, master.filter(base)) != 4 {
					t.Fatalf("recovery filter lost a loudnorm target or its limiter: %q", master.filter(base))
				}
				master = master.corrected(decoded, target)
			}
		}
	}
}
