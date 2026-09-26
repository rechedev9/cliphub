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
// decisions. Any change to nextNativeMaster, nextMasterTarget, aacHeadroomTarget,
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
	attemptTarget := aacHeadroomTarget(target)
	var masterTargets []recapplan.LoudnessOptions
	for i, decoded := range native {
		masterTargets = append(masterTargets, attemptTarget)
		accepted, err := fullDemoDecodedAACAccepted(decoded, target, false)
		if err != nil || accepted {
			t.Fatalf("native master %d: accepted=%v err=%v, want rejected", i, accepted, err)
		}
		next, retry := nextNativeMaster(attemptTarget, target, native[:i+1])
		// The second master came closer (2.94 -> 2.84 dB outside the window),
		// so the third still runs; the third diverged and hands over.
		if retry != (i < len(native)-1) {
			t.Fatalf("native master %d: retry=%v", i, retry)
		}
		attemptTarget = next
	}
	wantNative := []recapplan.LoudnessOptions{retarget(-14, -1.8), retarget(-13, -4.01), retarget(-12, -5.21)}
	if !reflect.DeepEqual(masterTargets, wantNative) {
		t.Fatalf("native master targets = %+v, want %+v", masterTargets, wantNative)
	}

	// Recovery chain: master_targets[3..4] and fallback_masters[0..1].
	master := initialAACRecoveryMaster(target)
	var fallbackMasters []ProgramAACFallbackMaster
	var acceptedAt = -1
	for i, decoded := range recovered {
		masterTargets = append(masterTargets, aacHeadroomTarget(target))
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

// Lab replay of job b7c0c77e (19-round FACEIT Mirage, 18:50 program measured
// at -32.59 LUFS / -5.32 dBTP / LRA 19.2): loudnorm could only run in dynamic
// mode, so lowering the TP target tightened its limiter and the second native
// master came out further from the window (-20.54 LUFS / +3.70 dBTP) than the
// first (-15.64 / +0.75). The loop must hand over to AAC recovery right there
// instead of spending a third master at TP -9 that measured -25.36 / -2.00.
func TestFullDemoNativeMasterHandsOverWhenRetargetDiverges(t *testing.T) {
	target := recapplan.DefaultOptions().Audio.Loudness
	if target.TargetILUFS != -14 || target.TargetTPDBTP != -1.5 {
		t.Skipf("lab replay was mastered against -14 LUFS / -1.5 dBTP, not %+v", target)
	}
	measured := func(integrated, peak float64) LoudnessMeasurement {
		return LoudnessMeasurement{Status: "measured", IntegratedLUFS: &integrated, TruePeakDBTP: &peak}
	}
	decodedByTarget := map[recapplan.LoudnessOptions]LoudnessMeasurement{}
	retarget := func(integrated, peak float64) recapplan.LoudnessOptions {
		next := target
		next.TargetILUFS, next.TargetTPDBTP = integrated, peak
		return next
	}
	decodedByTarget[retarget(-14, -1.8)] = measured(-15.64, 0.75)
	decodedByTarget[retarget(-13, -4.25)] = measured(-20.54, 3.70)
	decodedByTarget[retarget(-12, -9)] = measured(-25.36, -2.00)

	// Mirrors the native loop of masterFullDemoMeasuredProgram.
	attemptTarget := aacHeadroomTarget(target)
	var masterTargets []recapplan.LoudnessOptions
	var decodedAAC []LoudnessMeasurement
	for attempt := 0; attempt < 3; attempt++ {
		if clampLoudnormTarget(attemptTarget) != attemptTarget {
			t.Fatalf("native master %d left loudnorm's range: %+v", attempt, attemptTarget)
		}
		masterTargets = append(masterTargets, attemptTarget)
		decoded, ok := decodedByTarget[attemptTarget]
		if !ok {
			t.Fatalf("native master %d used a target the lab never measured: %+v", attempt, attemptTarget)
		}
		decodedAAC = append(decodedAAC, decoded)
		if accepted, err := fullDemoDecodedAACAccepted(decoded, target, false); err != nil || accepted {
			t.Fatalf("native master %d: accepted=%v err=%v, want rejected", attempt, accepted, err)
		}
		next, retry := nextNativeMaster(attemptTarget, target, decodedAAC)
		if !retry {
			break
		}
		attemptTarget = next
	}
	want := []recapplan.LoudnessOptions{retarget(-14, -1.8), retarget(-13, -4.25)}
	if !reflect.DeepEqual(masterTargets, want) {
		t.Fatalf("native master targets = %+v, want hand-over after %+v", masterTargets, want)
	}
	// A retarget that lands exactly as far from the window made no progress.
	first := decodedByTarget[retarget(-14, -1.8)]
	if _, retry := nextNativeMaster(want[1], target, []LoudnessMeasurement{first, first}); retry {
		t.Fatal("a retarget with no progress must hand over")
	}
}
