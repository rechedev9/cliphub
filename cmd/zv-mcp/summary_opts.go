package main

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// preRollOptions are renderplan.PreRollChoices; the id is the pre_roll_seconds value.
var preRollOptions = []shortOption{
	{ID: "3", Label: "3 s", Description: "Lo justo antes de la primera kill, como siempre en un Short", Default: true},
	{ID: "5", Label: "5 s", Description: "Más contexto antes de cada kill; lo que usa el resumen de un jugador"},
	{ID: "8", Label: "8 s", Description: "Toda la preparación de la jugada"},
}

// captureHUDOptions are renderplan.CaptureHUDs: the Short capture profiles a 16:9 video records in.
var captureHUDOptions = []shortOption{
	{ID: "gameplay", Label: "HUD completo", Description: "Con todo el HUD de CS2: vida, munición, radar y kill feed", Default: true},
	{ID: "deathnotices", Label: "Killfeed", Description: "Sin HUD, solo con la kill feed de CS2"},
	{ID: "clean", Label: "POV limpio", Description: "Sin HUD ni kill feed"},
}

// resolveSummaryChoices sets the pre-roll, with 3 s unset as Studio sends it, and the capture style and kill feed framing of a 16:9 video.
func resolveSummaryChoices(edit *tuiclient.ShortEdit, c shortChoices) error {
	if c.PreRollSeconds != nil {
		id, err := pickOption("pre_roll_seconds", strconv.Itoa(*c.PreRollSeconds), preRollOptions)
		if err != nil {
			return err
		}
		if id != "3" {
			edit.PreRollSeconds = *c.PreRollSeconds
		}
	}
	if orDefault(c.PortraitSafeKillfeed, false) {
		if edit.Format != "landscape-16x9" || c.CaptureHUD == "clean" {
			return errors.New("portrait_safe_killfeed needs format landscape-16x9 and a capture with the kill feed; a Short frames it from its preset")
		}
		edit.PortraitSafeKillfeed = true
	}
	if c.CaptureHUD == "" {
		return nil
	}
	hud, err := pickOption("capture_hud", c.CaptureHUD, captureHUDOptions)
	switch {
	case err != nil:
		return err
	case edit.Format != "landscape-16x9":
		return errors.New("capture_hud needs format landscape-16x9; a Short records the HUD of its preset")
	case edit.NativeHUD:
		return errors.New("capture_hud and native_hud both choose the capture HUD; set only one")
	}
	edit.CaptureHUD = hud
	return nil
}

func describeSummaryChoices(e tuiclient.ShortEdit) string {
	out := ""
	if e.PreRollSeconds > 0 {
		out += fmt.Sprintf(", %d s antes de cada kill", e.PreRollSeconds)
	}
	for _, o := range captureHUDOptions {
		if o.ID == e.CaptureHUD {
			out += ", captura " + o.Label
		}
	}
	if e.PortraitSafeKillfeed {
		out += ", kill feed donde la deja el Short"
	}
	return out
}
