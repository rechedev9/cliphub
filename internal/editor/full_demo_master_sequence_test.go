package editor

import (
	"reflect"
	"testing"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

// The saved replay (930.6 s program, three rejected native masters, second
// Media Foundation recovery candidate accepted) pins the retarget and recovery
// sequence to real evidence: fed the decoded AAC measurements of
// full-demo-loudness.json, the pure retarget helpers must reproduce its
// master_targets and fallback_masters exactly, in order, and its acceptance
// decisions. Any change to nextMasterTarget, fullDemoAACRecoveryTarget,
// ProgramAACFallbackMaster.corrected or fullDemoDecodedAACAccepted that alters
// the sequence on a real render fails here without FFmpeg.
func TestFullDemoMasterSequenceMatchesSavedReplay(t *testing.T) {
	target := recapplan.DefaultOptions().Audio.Loudness
	if target.TargetILUFS != -14 || target.TargetTPDBTP != -1.5 || target.PolicyVersion != "program-aac-v1" {
		t.Skipf("saved replay was mastered against -14 LUFS / -1.5 dBTP program-aac-v1, not %+v", target)
	}
	measured := func(integrated, peak float64) LoudnessMeasurement {
		return LoudnessMeasurement{Status: "measured", IntegratedLUFS: &integrated, TruePeakDBTP: &peak}
	}
	retarget := func(integrated, peak float64) recapplan.LoudnessOptions {
		next := target
		next.TargetILUFS, next.TargetTPDBTP = integrated, peak
		return next
	}
	// decoded_aac of the saved replay, in order: three native masters, then two
	// recovery candidates.
	native := []LoudnessMeasurement{measured(-15.43, 0.51), measured(-16.34, -0.5), measured(-19.35, 6.96)}
	recovered := []LoudnessMeasurement{measured(-14.86, -3.2), measured(-14.44, -3.35)}

	// Native chain: master_targets[0..2].
	attemptTarget := target
	attemptTarget.TargetTPDBTP -= 0.3
	var masterTargets []recapplan.LoudnessOptions
	for i, decoded := range native {
		masterTargets = append(masterTargets, attemptTarget)
		accepted, err := fullDemoDecodedAACAccepted(decoded, target, false)
		if err != nil || accepted {
			t.Fatalf("native master %d: accepted=%v err=%v, want rejected", i, accepted, err)
		}
		next, changed := nextMasterTarget(attemptTarget, target, decoded)
		if !changed {
			t.Fatalf("native master %d: retargeting reported exhausted headroom", i)
		}
		attemptTarget = next
	}
	wantNative := []recapplan.LoudnessOptions{retarget(-14, -1.8), retarget(-13, -4.01), retarget(-12, -5.21)}
	if !reflect.DeepEqual(masterTargets, wantNative) {
		t.Fatalf("native master targets = %+v, want %+v", masterTargets, wantNative)
	}

	// Recovery chain: master_targets[3..4] and fallback_masters[0..1].
	master := ProgramAACFallbackMaster{Encoder: "aac_mf", GainDB: 3, CeilingDBFS: target.TargetTPDBTP - 3.5}
	var fallbackMasters []ProgramAACFallbackMaster
	var acceptedAt = -1
	for i, decoded := range recovered {
		masterTargets = append(masterTargets, fullDemoAACRecoveryTarget(target))
		fallbackMasters = append(fallbackMasters, master)
		accepted, err := fullDemoDecodedAACAccepted(decoded, target, false)
		if err != nil {
			t.Fatalf("recovery candidate %d: %v", i, err)
		}
		if accepted {
			acceptedAt = i
			break
		}
		master = master.corrected(decoded, target)
	}
	if acceptedAt != 1 {
		t.Fatalf("recovery accepted candidate %d, want the second one", acceptedAt)
	}
	wantTargets := append(wantNative, retarget(-14, -1.8), retarget(-14, -1.8))
	if !reflect.DeepEqual(masterTargets, wantTargets) {
		t.Fatalf("master targets = %+v, want %+v", masterTargets, wantTargets)
	}
	wantFallback := []ProgramAACFallbackMaster{
		{Encoder: "aac_mf", GainDB: 3, CeilingDBFS: -5},
		{Encoder: "aac_mf", GainDB: 3.8599999999999994, CeilingDBFS: -5},
	}
	if !reflect.DeepEqual(fallbackMasters, wantFallback) {
		t.Fatalf("fallback masters = %+v, want %+v", fallbackMasters, wantFallback)
	}
}
