package recording

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

func assignSecondCameras(segments []RecordingSegment, camera *recapplan.SecondCamera) {
	if camera == nil {
		return
	}
	for _, w := range camera.Windows {
		i := slices.IndexFunc(segments, func(s RecordingSegment) bool { return s.ID == w.RoundID })
		if i >= 0 {
			window := w
			segments[i].SecondCamera = &window
		}
	}
}

func secondCameraCapture(segment RecordingSegment) (RecordingSegment, bool) {
	w := segment.SecondCamera
	if w == nil {
		return RecordingSegment{}, false
	}
	window := *w
	return RecordingSegment{
		ExactWindow:  true,
		ID:           w.ID,
		Round:        segment.Round,
		TickStart:    w.StartTick,
		TickEnd:      w.EndTick,
		LiveEndTick:  w.KillTick,
		Kills:        cameraKills(segment, window),
		POVSteamID64: w.EnemySteamID64,
		SecondCamera: &window,
	}, true
}

func cameraKills(segment RecordingSegment, w recapplan.CameraWindow) []killplan.Kill {
	var kills []killplan.Kill
	for _, kill := range segment.Kills {
		if kill.Tick == w.KillTick && kill.Victim.SteamID64 == w.EnemySteamID64 {
			kills = append(kills, kill)
		}
	}
	return kills
}

func rewindIndex(plan RecordingPlan, captures []RecordingSegment) int {
	if plan.FullDemo == nil {
		return slices.IndexFunc(captures, func(s RecordingSegment) bool { return s.POVSteamID64 != "" })
	}
	first := slices.IndexFunc(captures, func(s RecordingSegment) bool { return !isCameraCapture(s) })
	if first <= 0 || EffectiveRecordStartTick(captures[first], plan.Tickrate) > EffectiveRecordEndTick(captures[first-1], plan)+plan.Tickrate {
		return -1
	}
	return first
}

func isCameraCapture(s RecordingSegment) bool {
	return s.SecondCamera != nil && s.POVSteamID64 != ""
}

func (p RecordingPlan) validateSecondCameras(ids map[string]bool) error {
	cameras := map[string]bool{}
	for _, segment := range p.Segments {
		w := segment.SecondCamera
		if w == nil {
			continue
		}
		if p.FullDemo == nil {
			return fmt.Errorf("segment %s second_camera requires a Full Demo plan", segment.ID)
		}
		if err := artifacts.ValidateArtifactToken("second camera id", w.ID); err != nil {
			return err
		}
		if w.RoundID != segment.ID || w.ID != recapplan.CameraWindowID(w.RoundID, w.KillTick) || ids[w.ID] || cameras[w.ID] {
			return fmt.Errorf("segment %s second camera id %q is not its own window", segment.ID, w.ID)
		}
		cameras[w.ID] = true
		if w.StartTick <= 0 || w.StartTick < segment.TickStart || w.EndTick > segment.TickEnd || w.KillTick <= w.StartTick || w.KillTick >= w.EndTick {
			return fmt.Errorf("segment %s second camera window %d-%d around kill %d is outside the segment", segment.ID, w.StartTick, w.EndTick, w.KillTick)
		}
		if _, err := AccountIDFromSteamID64(w.EnemySteamID64); err != nil || w.EnemySteamID64 == p.TargetSteamID64 {
			return fmt.Errorf("segment %s second camera steamid64 %q is not another player", segment.ID, w.EnemySteamID64)
		}
		if camera, _ := secondCameraCapture(segment); reachesDemoEnd(camera, p) {
			return fmt.Errorf("segment %s second camera ends too close to the demo end to rewind after it", segment.ID)
		}
		if len(cameraKills(segment, *w)) == 0 {
			return fmt.Errorf("segment %s second camera names no kill of the segment", segment.ID)
		}
	}
	return nil
}

func fullDemoCameraApproved(s RecordingSegment, sources []recapplan.Document) bool {
	if s.SecondCamera == nil {
		return true
	}
	return slices.ContainsFunc(sources, func(d recapplan.Document) bool {
		return d.SecondCamera != nil && slices.ContainsFunc(d.SecondCamera.Windows, func(w recapplan.CameraWindow) bool {
			return w.RoundID == s.ID && reflect.DeepEqual(w, *s.SecondCamera)
		})
	})
}

func withoutCaptureExtras(s RecordingSegment) RecordingSegment {
	s.SecondCamera = nil
	s.ReplayOnly = false
	return s
}

func (r RecordingResult) captureSegment(id string) (RecordingSegment, bool) {
	if i := slices.IndexFunc(r.Plan.Segments, func(s RecordingSegment) bool { return s.ID == id }); i >= 0 {
		return r.Plan.Segments[i], true
	}
	for _, segment := range r.Plan.Segments {
		if camera, ok := secondCameraCapture(segment); ok && camera.ID == id {
			return camera, true
		}
	}
	return RecordingSegment{}, false
}

func (r RecordingResult) validateFullDemoCameraRuns(runs map[string]FullDemoCaptureRun) (int, error) {
	clips := 0
	for _, segment := range r.Plan.Segments {
		w := segment.SecondCamera
		if w == nil {
			continue
		}
		i := slices.IndexFunc(r.Artifacts, func(a RecordingArtifact) bool { return isUsableSegmentClip(a) && a.SegmentID == w.ID })
		if i < 0 {
			continue
		}
		clips++
		run, ok := runs[r.Artifacts[i].CaptureRevision]
		if !ok {
			return 0, fmt.Errorf("full demo camera clip %s lacks its original attestation", w.ID)
		}
		matched := slices.ContainsFunc(run.Plan.Segments, func(original RecordingSegment) bool {
			return original.ID == w.RoundID && original.SecondCamera != nil && reflect.DeepEqual(*original.SecondCamera, *w)
		})
		end, certified := r.FullDemoEvidence.CertifiedEnds[w.ID]
		if !matched || !certified || run.Evidence.CertifiedEnds[w.ID] != end {
			return 0, fmt.Errorf("full demo aggregate camera differs from its original run: %s", w.ID)
		}
	}
	return clips, nil
}

func SameCameraWindow(a, b recapplan.CameraWindow) bool {
	return a.ID == b.ID && a.RoundID == b.RoundID && a.Round == b.Round && a.KillTick == b.KillTick &&
		a.StartTick == b.StartTick && a.EndTick == b.EndTick && a.EnemySteamID64 == b.EnemySteamID64
}

func (r RecordingResult) storedCamera(w recapplan.CameraWindow) bool {
	return slices.ContainsFunc(r.Plan.Segments, func(s RecordingSegment) bool {
		return s.ID == w.RoundID && s.SecondCamera != nil && SameCameraWindow(*s.SecondCamera, w)
	})
}

func (r RecordingResult) UsableFullDemoCameras(d recapplan.Document) map[string]bool {
	usable := map[string]bool{}
	if d.SecondCamera == nil || r.FullDemoEvidence == nil {
		return usable
	}
	for _, w := range d.SecondCamera.Windows {
		end, certified := r.FullDemoEvidence.CertifiedEnds[w.ID]
		usable[w.ID] = r.storedCamera(w) && certified && end == w.EndTick &&
			r.ValidateFullDemoRoundFrames(w.ID, w.StartTick, w.EndTick) == nil
	}
	return usable
}
