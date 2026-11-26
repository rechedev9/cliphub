package editor

import (
	"fmt"
	"math"
	"slices"

	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

const (
	fullDemoSecondCameraMinSeconds = 3
	fullDemoCameraCovered          = "covered_by_overlay"
	fullDemoCameraOmitted          = "capture_omitted"
)

var fullDemoCameraBusySources = []string{"full-demo-intro", "full-demo-outro", fullDemoHalftimeSource}

type FullDemoCameraEvidence struct {
	ID      string `json:"id"`
	RoundID string `json:"round_id"`
	Drawn   bool   `json:"drawn"`
	Reason  string `json:"reason,omitempty"`
}

type fullDemoCameraShot struct {
	window     recapplan.CameraWindow
	clip       string
	f0, v0, v1 int64
}

type fullDemoFrameSpan struct {
	start, end int64
}

func attachFullDemoSecondCamera(short *ShortEdit, planned *recapplan.SecondCamera, baseDir string) error {
	if planned == nil {
		return nil
	}
	d := short.FullDemo.Effective
	shots := map[int64]fullDemoCameraShot{}
	drawn := map[string]bool{}
	if d.SecondCamera != nil {
		busy := fullDemoCameraBusy(short.Effects)
		for _, w := range d.SecondCamera.Windows {
			clip, ok := recording.ResolveFullDemoCameraClip(short.fullDemo.recording, w.ID)
			if !ok {
				return fmt.Errorf("full_demo_capture_incomplete: second camera %s has no clip", w.ID)
			}
			for _, item := range d.Timeline {
				if item.Role != "round" || item.IsHook() || item.SourceRef != w.RoundID {
					continue
				}
				shot, ok, err := fullDemoCameraShotFor(item, w, busy, d.Clock.TickRate)
				if err != nil {
					return err
				}
				if ok {
					shot.clip = resolvePath(baseDir, clip.Path)
					shots[item.StartFrame] = shot
					drawn[w.ID] = true
				}
			}
		}
	}
	short.fullDemo.cameras = shots
	for _, w := range planned.Windows {
		evidence := FullDemoCameraEvidence{ID: w.ID, RoundID: w.RoundID, Drawn: drawn[w.ID]}
		omitted := slices.ContainsFunc(d.Warnings, func(n recapplan.Notice) bool {
			return n.Code == recapplan.WarnSecondCameraOmitted && n.RoundID == w.RoundID
		})
		switch {
		case omitted:
			evidence.Reason = fullDemoCameraOmitted
		case !evidence.Drawn:
			evidence.Reason = fullDemoCameraCovered
		}
		short.FullDemo.SecondCamera = append(short.FullDemo.SecondCamera, evidence)
	}
	return nil
}

func fullDemoCameraBusy(effects []Effect) []fullDemoFrameSpan {
	var busy []fullDemoFrameSpan
	for _, effect := range effects {
		if effect.Type != EffectImage || !slices.Contains(fullDemoCameraBusySources, effect.Source) || effect.EndSeconds <= effect.StartSeconds {
			continue
		}
		busy = append(busy, fullDemoFrameSpan{
			start: int64(math.Floor(effect.StartSeconds * recapplan.OutputFPS)),
			end:   int64(math.Ceil(effect.EndSeconds*recapplan.OutputFPS)) + 1,
		})
	}
	return busy
}

func fullDemoCameraShotFor(item recapplan.TimelineItem, w recapplan.CameraWindow, busy []fullDemoFrameSpan, rate int) (fullDemoCameraShot, bool, error) {
	start, err := recapplan.TickFrames(w.StartTick-item.SourceStartTick, rate)
	if err != nil {
		return fullDemoCameraShot{}, false, err
	}
	end, err := recapplan.TickFrames(w.EndTick-item.SourceStartTick, rate)
	if err != nil {
		return fullDemoCameraShot{}, false, err
	}
	f0 := start - item.SourceOffsetFrames
	length := item.EndFrame - item.StartFrame
	free := []fullDemoFrameSpan{{min(length, max(0, f0)), min(length, max(0, end-item.SourceOffsetFrames))}}
	for _, taken := range busy {
		from, to := taken.start-item.StartFrame, taken.end-item.StartFrame
		var next []fullDemoFrameSpan
		for _, span := range free {
			if to <= span.start || from >= span.end {
				next = append(next, span)
				continue
			}
			if from > span.start {
				next = append(next, fullDemoFrameSpan{span.start, from})
			}
			if to < span.end {
				next = append(next, fullDemoFrameSpan{to, span.end})
			}
		}
		free = next
	}
	best := fullDemoFrameSpan{}
	for _, span := range free {
		if span.end-span.start > best.end-best.start {
			best = span
		}
	}
	if best.end-best.start < fullDemoSecondCameraMinSeconds*recapplan.OutputFPS {
		return fullDemoCameraShot{}, false, nil
	}
	return fullDemoCameraShot{window: w, f0: f0, v0: best.start, v1: best.end}, true, nil
}

func fullDemoSecondCameraY(short ShortEdit) (int, error) {
	if !short.FullDemo.Effective.Options.Capture.ClipHubRadar {
		return demooverlay.SecondCameraY, nil
	}
	box, err := fullDemoRadarBox(short)
	if err != nil {
		return 0, err
	}
	return box.Y + box.H + demooverlay.SecondCameraRadarGap, nil
}

func fullDemoSecondCameraOverlay(s fullDemoRadarStep) ([]string, string, error) {
	if s.item.Role != "round" || s.item.IsHook() || s.short.fullDemo == nil {
		return s.command, s.video, nil
	}
	shot, ok := s.short.fullDemo.cameras[s.item.StartFrame]
	if !ok || shot.window.RoundID != s.item.SourceRef {
		return s.command, s.video, nil
	}
	y, err := fullDemoSecondCameraY(s.short)
	if err != nil {
		return nil, "", err
	}
	mask, label := s.output+".camera-mask.png", s.output+".camera-label.png"
	if err := demooverlay.RenderSecondCameraMaskPNG(mask); err != nil {
		return nil, "", fmt.Errorf("second camera mask: %w", err)
	}
	if err := demooverlay.RenderSecondCameraLabelPNG(shot.window.EnemyName, demooverlay.DesignPalette(s.short.FullDemo.Effective.Options.Overlays), label); err != nil {
		return nil, "", fmt.Errorf("second camera label: %w", err)
	}
	frames := shot.v1 - shot.v0
	seek, trim := fullDemoCaptureSeek(shot.v0 - shot.f0)
	index := fullDemoInputCount(s.command)
	command := s.command
	if seek > 0 {
		command = append(command, "-ss", decimal(float64(seek)/recapplan.OutputFPS))
	}
	command = append(command, "-i", shot.clip, "-i", mask, "-i", label)
	fade := fullDemoCameraFade(shot.v0, shot.v1)
	enable := fmt.Sprintf("enable='between(n,%d,%d)'", shot.v0, shot.v1-1)
	w, h, x := demooverlay.SecondCameraWidth, demooverlay.SecondCameraHeight, demooverlay.SecondCameraX
	video := s.video + fmt.Sprintf("[cambase];"+
		"[%d:v]fps=60,trim=start_frame=%d:end_frame=%d,setpts=PTS-STARTPTS,%s,scale=%d:%d,setsar=1,format=yuva420p[camraw];"+
		"[%d:v]format=gray,setsar=1[cammask];"+
		"[camraw][cammask]alphamerge,settb=expr=1/60,setpts=N+%d,%s[cam];"+
		"[%d:v]format=rgba,setsar=1,loop=loop=%d:size=1,settb=expr=1/60,setpts=N+%d,%s[camlabel];"+
		"[cambase][cam]overlay=x=%d:y=%d:eof_action=pass:format=auto:%s[camwin];"+
		"[camwin][camlabel]overlay=x=%d:y=%d:eof_action=pass:format=auto:%s,format=yuv420p",
		index, trim, trim+frames, fullDemoFrameFilter(s.short.FullDemo.Effective.Options.Capture), w, h,
		index+1,
		shot.v0, fade,
		index+2, frames-1, shot.v0, fade,
		x, y, enable,
		x, y+h, enable)
	return command, video, nil
}

func fullDemoCameraFade(v0, v1 int64) string {
	d := demooverlay.SecondCameraFadeSeconds
	return fmt.Sprintf("fade=t=in:st=%s:d=%s:alpha=1,fade=t=out:st=%s:d=%s:alpha=1",
		decimal(float64(v0)/recapplan.OutputFPS), decimal(d), decimal(float64(v1)/recapplan.OutputFPS-d), decimal(d))
}
