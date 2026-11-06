package mapgeo

import (
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestLightFromEntityBarnFrustum(t *testing.T) {
	l, ok := lightFromEntity("light_barn", map[string]any{
		"origin": "0 0 100", "angles": "90 0 0", "size_params": "1 1 0.5", "range": "200",
		"brightness": "0", "color": "255 255 255", "bakedshadowindex": "1",
	})
	if !ok {
		t.Fatal("a stationary barn light was skipped")
	}
	// Near is 1/size.z; the eye sits that far behind the luminaire, which looks straight down.
	if !near(l.Near, 2) || !near(l.Far, 202) || !near(l.Eye[2], 102) || !near(l.Forward[2], -1) {
		t.Fatalf("frustum: near %v far %v eye %v forward %v", l.Near, l.Far, l.Eye, l.Forward)
	}
	if !near(l.Falloff, 4) || !near(l.Color[0], 102*102/4.0) || l.Shadow != 1 {
		t.Fatalf("falloff %v colour %v shadow %d", l.Falloff, l.Color, l.Shadow)
	}
	if !near(l.Skirt[0], 12/10.1) || !near(l.Min[2], -100) {
		t.Fatalf("skirt %v min %v", l.Skirt, l.Min)
	}
}

func TestLightFromEntitySkipsBakedAndDisabled(t *testing.T) {
	base := map[string]any{"origin": "0 0 0", "range": "100"}
	for name, extra := range map[string]map[string]any{
		"baked":    {"directlight": "1"},
		"disabled": {"StartDisabled": "1"},
		"off":      {"enabled": false},
	} {
		vals := map[string]any{}
		for k, v := range base {
			vals[k] = v
		}
		for k, v := range extra {
			vals[k] = v
		}
		if _, ok := lightFromEntity("light_omni2", vals); ok {
			t.Errorf("%s omni light was kept", name)
		}
	}
	if _, ok := lightFromEntity("light_omni2", base); !ok {
		t.Error("a stationary omni light was skipped")
	}
}

func TestDistanceFadeEndsAtRange(t *testing.T) {
	f := distanceFade(100, 0.25)
	if !near(f[0]+f[1]*75, 1) || !near(f[0]+f[1]*100, 0) {
		t.Fatalf("fade %v does not run from 75 to 100", f)
	}
}

func TestNormalizationSHTurnsTheLinearBand(t *testing.T) {
	radiance := make([]float32, 27)
	for c := 0; c < 3; c++ {
		radiance[c*9] = 1
		radiance[c*9+3] = 0.5
	}
	sh := normalizationSH(radiance, 0, angleAffine([3]float64{}, [3]float64{}, [3]float64{1, 1, 1}))
	if !near(sh[3], 0.282095) || sh[0] >= 0 || !near(sh[1], 0) || !near(sh[2], 0) {
		t.Fatalf("harmonic %v", sh)
	}
	if got := normalizationSH(radiance, 1, angleAffine([3]float64{}, [3]float64{}, [3]float64{1, 1, 1})); got != [4]float64{} {
		t.Fatalf("a probe past the radiance should have none, got %v", got)
	}
}
