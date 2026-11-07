package tacticalplan

import (
	"math"
	"testing"
)

func TestSetangAnglesComeIntoRange(t *testing.T) {
	for _, tc := range []struct{ in, pitch, yaw float64 }{
		{in: 0, pitch: 0, yaw: 0},
		{in: 45, pitch: 45, yaw: 45},
		{in: 300, pitch: -60, yaw: -60},
		{in: 359.5, pitch: -0.5, yaw: -0.5},
		{in: -30, pitch: -30, yaw: -30},
		{in: 540, pitch: 90, yaw: 180},
	} {
		if got := SetangPitch(tc.in); math.Abs(got-tc.pitch) > 1e-9 {
			t.Errorf("SetangPitch(%v) = %v, want %v", tc.in, got, tc.pitch)
		}
		if got := SetangYaw(tc.in); math.Abs(got-tc.yaw) > 1e-9 {
			t.Errorf("SetangYaw(%v) = %v, want %v", tc.in, got, tc.yaw)
		}
	}
	if SetangPitch(math.NaN()) != 0 || SetangYaw(math.Inf(1)) != 0 {
		t.Error("non-finite angles must read as 0")
	}
}

func TestEstimateTechniqueReadsSpeedAndFlags(t *testing.T) {
	still := Sample{X: 100, Y: 100}
	for _, tc := range []struct {
		name string
		at   Sample
		want Technique
	}{
		{"standing", Sample{X: 102, Y: 100}, Technique{Movement: "standing"}},
		{"walking", Sample{X: 116, Y: 100}, Technique{Movement: "walking"}},
		{"running", Sample{X: 130, Y: 100}, Technique{Movement: "running"}},
		{"jump throw", Sample{X: 100, Y: 100, Flags: FlagAirborne}, Technique{Movement: "standing", Jump: true}},
		{"crouched", Sample{X: 110, Y: 100, Flags: FlagDucking}, Technique{Movement: "crouching"}},
	} {
		if got := EstimateTechnique(still, tc.at, 0.125); got != tc.want {
			t.Errorf("%s: EstimateTechnique = %+v, want %+v", tc.name, got, tc.want)
		}
	}
	if got := EstimateTechnique(still, Sample{X: 500, Y: 100}, 0); got.Movement != "standing" {
		t.Errorf("no time between samples = %+v, want standing", got)
	}
}
