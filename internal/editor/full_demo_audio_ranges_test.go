package editor

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

// Ranges FFmpeg 8.1 accepts for the filter options this package computes, as
// printed by `ffmpeg -h filter=<name>`. They are written out rather than taken
// from the clamp constants so the test checks the clamps against FFmpeg itself.
var computedFilterRanges = map[string]map[string][2]float64{
	"loudnorm": {"I": {-70, -5}, "TP": {-9, 0}, "LRA": {1, 50}},
	"alimiter": {"limit": {0.0625, 1}},
}

// assertFilterRanges fails when a filter of computedFilterRanges appears in
// chain without all its computed options or with one out of range, and returns
// the names of the filters it checked so callers can rule out a vacuous pass.
func assertFilterRanges(t *testing.T, chain string) map[string]bool {
	t.Helper()
	checked := map[string]bool{}
	for _, stage := range strings.Split(chain, ",") {
		name, args, _ := strings.Cut(stage, "=")
		ranges, ok := computedFilterRanges[name]
		if !ok {
			continue
		}
		seen := 0
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
			seen++
		}
		if seen != len(ranges) {
			t.Fatalf("%s is missing a computed option in %q", name, chain)
		}
		checked[name] = true
	}
	return checked
}

// The Studio 3.0.0 render aborted because a retarget handed loudnorm TP=-10.18.
// Whatever target or ceiling a retarget path produces, the filter strings
// FFmpeg receives must stay in range: the clamp lives where they are built.
func TestFullDemoFilterBuildersClampOutOfRangeTargets(t *testing.T) {
	lra, threshold, offset, lufs, peak := 7.0, -30.0, 0.0, -20.0, 0.0
	measured := LoudnessMeasurement{Status: "measured", IntegratedLUFS: &lufs, TruePeakDBTP: &peak, LRA: &lra, Threshold: &threshold, Offset: &offset}
	for _, target := range []recapplan.LoudnessOptions{
		{TargetILUFS: -80, TargetTPDBTP: -10.18, TargetLRA: 0},
		{TargetILUFS: -14, TargetTPDBTP: -21.5, TargetLRA: 11},
		{TargetILUFS: 3, TargetTPDBTP: 2, TargetLRA: 70},
	} {
		filter, err := measuredLoudnessFilter(target, measured)
		if err != nil {
			t.Fatal(err)
		}
		if !assertFilterRanges(t, filter)["loudnorm"] {
			t.Fatalf("no loudnorm stage in %q", filter)
		}
	}
	for _, ceiling := range []float64{-40, -24, 0, 6} {
		chain := ProgramAACFallbackMaster{Encoder: "aac_mf", CeilingDBFS: ceiling}.filter("anull")
		if !assertFilterRanges(t, chain)["alimiter"] {
			t.Fatalf("no alimiter stage in %q", chain)
		}
	}
}

// Generalizes TestFullDemoMasterRetargetStaysWithinLoudnormRange from the
// incident's single measurement to the plausible measurement space, starting
// from the production initial targets. The retarget must stay in range without
// relying on the builder clamp and must exhaust its headroom, which is what
// lets masterFullDemoMeasuredProgram hand over to AAC recovery early.
func TestFullDemoRetargetStaysInRangeAndExhaustsForAnyMeasurement(t *testing.T) {
	target := recapplan.DefaultOptions().Audio.Loudness
	for _, lufs := range []float64{-80, -40, -23, -14, -8, -5, 0} {
		for _, peak := range []float64{-40, -9, -1.8, -1.5, 0, 3, 12, 40} {
			decoded := LoudnessMeasurement{Status: "measured", IntegratedLUFS: &lufs, TruePeakDBTP: &peak}
			inRange := func(stage string, got recapplan.LoudnessOptions) {
				if clampLoudnormTarget(got) != got {
					t.Fatalf("%s left loudnorm's range for %.1f LUFS / %.1f dBTP: %+v", stage, lufs, peak, got)
				}
			}

			current := aacHeadroomTarget(target)
			inRange("initial native target", current)
			exhausted := false
			// Far more steps than the three native masters, so the bound holds for
			// any attempt count.
			for range 100 {
				var changed bool
				current, changed = nextMasterTarget(current, target, decoded)
				inRange("native retarget", current)
				if !changed {
					exhausted = true
					break
				}
			}
			if !exhausted {
				t.Fatalf("native retarget never exhausted its headroom for %.1f LUFS / %.1f dBTP: %+v", lufs, peak, current)
			}

			inRange("recovery target", aacHeadroomTarget(target))
			master := initialAACRecoveryMaster(target)
			bounds := computedFilterRanges["alimiter"]["limit"]
			for range 100 {
				if limit := math.Pow(10, master.CeilingDBFS/20); limit < bounds[0] || limit > bounds[1] {
					t.Fatalf("recovery ceiling %.2f dBFS left alimiter's range for %.1f LUFS / %.1f dBTP", master.CeilingDBFS, lufs, peak)
				}
				master = master.corrected(decoded, target)
			}
		}
	}
}
