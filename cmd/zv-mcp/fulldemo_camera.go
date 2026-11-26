package main

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

type secondCameraIn struct {
	SecondCamera       *bool    `json:"second_camera,omitempty" jsonschema:"add a small window over the gameplay with the view of an enemy who stood almost still holding an angle in the 5 s before the player killed them, left side with the enemy's name under it, from about 5 s before the kill to 1 s after, no audio; at most one per round; needs an extra capture pass in CS2 (about 11 s per window, the recorded rounds are reused); a match analysed before this option is read once more for it, which takes a few seconds; default off"`
	SecondCameraMax    *int     `json:"second_camera_max,omitempty" jsonschema:"most windows in the video, 1 to 10; default 5"`
	SecondCameraRemove []string `json:"second_camera_remove,omitempty" jsonschema:"moment ids from a previous reply's second_camera_moments to leave out; a removed moment does not bring in another one; pass them again on every plan, since each plan starts from the defaults"`
}

type secondCameraMomentOut struct {
	ID        string `json:"id" jsonschema:"pass to second_camera_remove"`
	Round     int    `json:"round"`
	Enemy     string `json:"enemy"`
	RoundTime string `json:"round_time" jsonschema:"m:ss from the start of the live round to the kill"`
}

func applySecondCamera(options *recapplan.Options, in secondCameraIn) error {
	if in.SecondCamera == nil {
		if in.SecondCameraMax != nil || len(in.SecondCameraRemove) > 0 {
			return errors.New("second_camera_max and second_camera_remove need second_camera true")
		}
		return nil
	}
	options.SecondCamera = nil
	if !*in.SecondCamera {
		return nil
	}
	camera := &recapplan.SecondCameraOptions{Enabled: true}
	if in.SecondCameraMax != nil {
		if *in.SecondCameraMax < 1 || *in.SecondCameraMax > recapplan.SecondCameraMaxWindows {
			return fmt.Errorf("second_camera_max must be between 1 and %d", recapplan.SecondCameraMaxWindows)
		}
		camera.MaxWindows = *in.SecondCameraMax
	}
	if len(in.SecondCameraRemove) > recapplan.SecondCameraMaxRemoved {
		return fmt.Errorf("second_camera_remove takes at most %d ids", recapplan.SecondCameraMaxRemoved)
	}
	camera.Removed = slices.Clone(in.SecondCameraRemove)
	slices.Sort(camera.Removed)
	camera.Removed = slices.Compact(camera.Removed)
	options.SecondCamera = camera
	return nil
}

func describeSecondCamera(doc recapplan.Document) (bool, []secondCameraMomentOut, []string, string) {
	requested := doc.Options.SecondCamera != nil && doc.Options.SecondCamera.Enabled
	if !requested {
		return false, nil, nil, ""
	}
	removed := slices.Clone(doc.Options.SecondCamera.Removed)
	if doc.SecondCamera == nil {
		return true, nil, removed, ""
	}
	liveStart := map[string]int{}
	for _, round := range doc.Rounds {
		liveStart[round.ID] = round.LiveStartTick
	}
	moments := make([]secondCameraMomentOut, 0, len(doc.SecondCamera.Windows))
	rounds := make([]string, 0, len(doc.SecondCamera.Windows))
	for _, w := range doc.SecondCamera.Windows {
		seconds := 0
		if doc.Clock.TickRate > 0 {
			seconds = max(0, int(math.Round(float64(w.KillTick-liveStart[w.RoundID])/float64(doc.Clock.TickRate))))
		}
		moments = append(moments, secondCameraMomentOut{
			ID: w.ID, Round: w.Round, Enemy: cleanName(w.EnemyName), RoundTime: fmt.Sprintf("%d:%02d", seconds/60, seconds%60),
		})
		rounds = append(rounds, fmt.Sprint(w.Round))
	}
	text := fmt.Sprintf(" Con segunda cámara en %s (rondas %s).", plural(len(moments), "jugada", "jugadas"), strings.Join(rounds, ", "))
	return true, moments, removed, text
}

func describeCaptureWork(capture fullDemoCapture) string {
	rounds := plural(len(capture.MissingRounds), "ronda", "rondas")
	cameras := plural(len(capture.MissingCameras), "segunda cámara", "segundas cámaras")
	switch {
	case len(capture.MissingCameras) == 0:
		return rounds
	case len(capture.MissingRounds) == 0:
		return "solo " + cameras + ", porque las rondas ya están capturadas"
	default:
		return rounds + " y " + cameras
	}
}

func describeSecondCameraCapture(out *fullDemoPlanOut, capture *fullDemoCapture) {
	if capture == nil || !out.SecondCamera {
		return
	}
	out.MissingSecondCamera, out.OmittedSecondCamera = capture.MissingCameras, capture.OmittedCameras
	switch {
	case len(capture.MissingCameras) == 0:
	case len(capture.MissingRounds) == 0:
		out.Summary += " Graba solo las segundas cámaras; las rondas ya están capturadas."
	default:
		out.Summary += fmt.Sprintf(" También graba %s.", plural(len(capture.MissingCameras), "segunda cámara", "segundas cámaras"))
	}
	if len(capture.OmittedCameras) > 0 {
		out.Summary += fmt.Sprintf(" %s no se pudo grabar y el vídeo sale sin ella.", plural(len(capture.OmittedCameras), "segunda cámara", "segundas cámaras"))
	}
}
