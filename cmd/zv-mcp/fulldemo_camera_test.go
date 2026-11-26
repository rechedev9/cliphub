package main

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

func TestApplyFullDemoChoicesSecondCamera(t *testing.T) {
	on, off := true, false
	max, tooMany := 3, 11
	plain, err := applyFullDemoChoices(recapplan.DefaultOptions(), planFullDemoIn{})
	if err != nil || plain.SecondCamera != nil {
		t.Fatalf("the second camera must stay off unless asked: %+v %v", plain.SecondCamera, err)
	}
	chosen, err := applyFullDemoChoices(recapplan.DefaultOptions(), planFullDemoIn{secondCameraIn: secondCameraIn{
		SecondCamera: &on, SecondCameraMax: &max, SecondCameraRemove: []string{"round-002-camera-900", "round-001-camera-500", "round-002-camera-900"},
	}})
	if err != nil || chosen.SecondCamera == nil || !chosen.SecondCamera.Enabled || chosen.SecondCamera.MaxWindows != 3 {
		t.Fatalf("second camera was dropped: %+v %v", chosen.SecondCamera, err)
	}
	if got := strings.Join(chosen.SecondCamera.Removed, ","); got != "round-001-camera-500,round-002-camera-900" {
		t.Fatalf("removed ids = %q", got)
	}
	if canonical, err := recapplan.CanonicalNewOptions(chosen); err != nil || canonical.SecondCamera == nil || canonical.SecondCamera.MaxWindows != 3 {
		t.Fatalf("the planner dropped the option: %+v %v", canonical.SecondCamera, err)
	}
	defaulted, err := applyFullDemoChoices(recapplan.DefaultOptions(), planFullDemoIn{secondCameraIn: secondCameraIn{SecondCamera: &on}})
	if err != nil || defaulted.SecondCamera == nil {
		t.Fatalf("on without a cap: %+v %v", defaulted.SecondCamera, err)
	}
	if canonical, err := recapplan.CanonicalNewOptions(defaulted); err != nil || canonical.SecondCamera.MaxWindows != recapplan.SecondCameraDefaultMax {
		t.Fatalf("default cap = %+v %v", canonical.SecondCamera, err)
	}
	removed, err := applyFullDemoChoices(chosen, planFullDemoIn{secondCameraIn: secondCameraIn{SecondCamera: &off}})
	if err != nil || removed.SecondCamera != nil {
		t.Fatalf("false must remove the option: %+v %v", removed.SecondCamera, err)
	}
	if _, err := applyFullDemoChoices(recapplan.DefaultOptions(), planFullDemoIn{secondCameraIn: secondCameraIn{SecondCamera: &on, SecondCameraMax: &tooMany}}); err == nil {
		t.Fatal("a cap above the limit must be refused")
	}
	if _, err := applyFullDemoChoices(recapplan.DefaultOptions(), planFullDemoIn{secondCameraIn: secondCameraIn{SecondCameraMax: &max}}); err == nil {
		t.Fatal("a cap without the switch must be refused")
	}
}

func TestDescribeFullDemoPlanSecondCamera(t *testing.T) {
	doc := recapplan.Document{
		Options: recapplan.DefaultOptions(), Clock: recapplan.Clock{FPS: 60, TickRate: 64},
		Rounds: []recapplan.Round{{ID: "round-003", Number: 3, LiveStartTick: 1000}, {ID: "round-009", Number: 9, LiveStartTick: 5000}},
	}
	if out := describeFullDemoPlan("job-1", doc); out.SecondCamera || out.SecondCameraMoments != nil || strings.Contains(out.Summary, "segunda cámara") {
		t.Fatalf("a default plan reports a second camera: %+v", out)
	}
	doc.Options.SecondCamera = &recapplan.SecondCameraOptions{Enabled: true, MaxWindows: 5, Removed: []string{"round-005-camera-1"}}
	out := describeFullDemoPlan("job-1", doc)
	if !out.SecondCamera || len(out.SecondCameraMoments) != 0 || strings.Contains(out.Summary, "segunda cámara") || len(out.SecondCameraRemoved) != 1 {
		t.Fatalf("a skipped second camera must be reported as asked for and empty: %+v", out)
	}
	doc.SecondCamera = &recapplan.SecondCamera{Windows: []recapplan.CameraWindow{
		{ID: "round-003-camera-5800", RoundID: "round-003", Round: 3, KillTick: 1000 + 64*75, EnemyName: "Zed"},
		{ID: "round-009-camera-5960", RoundID: "round-009", Round: 9, KillTick: 5000 + 4198, EnemyName: "Kai"},
	}}
	out = describeFullDemoPlan("job-1", doc)
	if len(out.SecondCameraMoments) != 2 || out.SecondCameraMoments[0].RoundTime != "1:15" || out.SecondCameraMoments[0].Enemy != "Zed" || out.SecondCameraMoments[1].RoundTime != "1:06" {
		t.Fatalf("moments = %+v", out.SecondCameraMoments)
	}
	if !strings.Contains(out.Summary, "Con segunda cámara en 2 jugadas (rondas 3, 9).") {
		t.Fatalf("summary = %s", out.Summary)
	}
	describeFullDemoCapture(&out, &fullDemoCapture{Needed: true, MissingRounds: nil, MissingCameras: []string{"round-003-camera-5800"}, OmittedCameras: []string{"round-009-camera-5960"}})
	if len(out.MissingSecondCamera) != 1 || len(out.OmittedSecondCamera) != 1 || !strings.Contains(out.Summary, "Graba solo las segundas cámaras") || strings.Contains(out.Summary, "Graba de nuevo") {
		t.Fatalf("capture = %+v", out)
	}
	both := describeFullDemoPlan("job-1", doc)
	describeFullDemoCapture(&both, &fullDemoCapture{Needed: true, MissingRounds: []int{3}, MissingCameras: []string{"round-003-camera-5800", "round-009-camera-5960"}})
	if !strings.Contains(both.Summary, "También graba 2 segundas cámaras.") {
		t.Fatalf("rounds and cameras = %s", both.Summary)
	}
	for _, want := range []string{"plan_full_demo second_camera", "second_camera_moments", "second_camera_remove", "extra capture pass"} {
		if !strings.Contains(serverInstructions, want) {
			t.Fatalf("the server instructions lack %q", want)
		}
	}
}
