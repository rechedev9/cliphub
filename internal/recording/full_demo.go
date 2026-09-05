package recording

import (
	"fmt"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"slices"
)

func fullDemoCrosshairHash(d recapplan.Document) (string, error) {
	return recapplan.HashValue(d.Crosshairs)
}

func (p RecordingPlan) validateFullDemo() error {
	if p.FullDemo == nil {
		if len(p.FullDemoSources) > 0 {
			return fmt.Errorf("capture origins require a Full Demo document")
		}
		if p.Stream.FullDemoProfile != "" || p.Stream.FullDemoCapture != (recapplan.CaptureOptions{}) {
			return fmt.Errorf("full demo capture requires its document")
		}
		for _, s := range p.Segments {
			if s.ExactWindow {
				return fmt.Errorf("exact editorial windows require Full Demo evidence")
			}
		}
		return nil
	}
	d := p.FullDemo
	if err := d.Validate(); err != nil {
		return err
	}
	if len(d.Blockers) > 0 {
		return &recapplan.Error{Code: d.Blockers[0].Code, Detail: d.Blockers[0].Message}
	}
	if len(p.FullDemoSources) > 1000 {
		return fmt.Errorf("full demo capture origin limit exceeded")
	}
	sources := []recapplan.Document{*d}
	for _, source := range p.FullDemoSources {
		if err := source.Validate(); err != nil {
			return err
		}
		if len(source.Blockers) > 0 || source.Input.DemoSHA256 != d.Input.DemoSHA256 || source.Input.TargetSteamID64 != d.Input.TargetSteamID64 || source.Clock != d.Clock || source.Options.Capture != d.Options.Capture || !slices.Equal(source.Crosshairs, d.Crosshairs) {
			return fmt.Errorf("full demo capture origin has an incompatible profile")
		}
		sources = append(sources, source)
	}
	if p.Stream.FullDemoProfile != recapplan.ProfileChill || p.Stream.FullDemoCapture != d.Options.Capture ||
		p.DemoSHA256 != d.Input.DemoSHA256 || p.TargetSteamID64 != d.Input.TargetSteamID64 || p.Tickrate != d.Clock.TickRate ||
		p.Stream.FPS != 60 || p.Stream.Width != 1920 || p.Stream.Height != 1080 || p.Stream.HUDMode != HUDModeGameplay || p.Stream.PortraitSafeKillfeed {
		return &recapplan.Error{Code: recapplan.ErrPOVContract, Detail: "Capture profile contradicts the approved Full Demo document"}
	}
	for _, s := range p.Segments {
		found := false
		for _, source := range sources {
			for _, r := range source.Rounds {
				if s.ID == r.ID && s.TickStart == r.CaptureStartTick && s.TickEnd == r.CaptureEndTick && s.LiveEndTick == r.LiveEndTick && s.ExactWindow {
					found = true
					break
				}
			}
		}
		if !found {
			return &recapplan.Error{Code: recapplan.ErrPOVContract, Detail: "Capture window differs from the Full Demo document: " + s.ID}
		}
	}
	return nil
}
