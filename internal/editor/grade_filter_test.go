package editor

import (
	"strings"
	"testing"
)

func TestVelocityGradesOnlyAroundEachKill(t *testing.T) {
	t.Parallel()
	short := ShortEdit{DurationSeconds: 20, KillEffect: KillEffectVelocity, Kills: []KillCue{{TimeSeconds: 4}, {TimeSeconds: 12}}}
	filters := gradeFilters(generatedKillEffects(short), short.DurationSeconds)
	want := []string{`enable='between(t\,3.900\,4.340)'`, `enable='between(t\,11.900\,12.340)'`}
	if len(filters) != len(want) {
		t.Fatalf("grade filters = %q, want one per kill", filters)
	}
	for i, filter := range filters {
		if !strings.HasPrefix(filter, "eq=contrast=1.080:saturation=1.180:") || !strings.HasSuffix(filter, want[i]) {
			t.Errorf("filter %d = %q, want the velocity grade gated by %s", i, filter, want[i])
		}
	}
}

func TestWholeShortGradeIsNotGated(t *testing.T) {
	t.Parallel()
	effects := []Effect{
		{Type: EffectGrade, Contrast: 1.1, Saturation: 1.05, Gamma: 1},
		{Type: EffectGrade, Contrast: 1.1, Saturation: 1.05, Gamma: 1, EndSeconds: 20},
	}
	for i, filter := range gradeFilters(effects, 20.0004) {
		if filter != "eq=contrast=1.100:saturation=1.050:gamma=1.000" {
			t.Errorf("grade %d = %q, want one ungated eq", i, filter)
		}
	}
}
