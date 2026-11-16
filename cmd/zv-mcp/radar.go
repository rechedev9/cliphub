package main

import (
	"errors"
	"fmt"

	"github.com/rechedev9/cliphub/internal/radaroverlay"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// radarAuto is the corner the format picks: bottom left in 9:16, top left in 16:9. It goes unset on the wire.
const radarAuto = "auto"

var radarPositionOptions = []shortOption{
	{ID: radarAuto, Label: "Automática", Description: "Abajo a la izquierda en 9:16, sobre la franja de subtítulos; arriba a la izquierda en 16:9", Default: true},
	{ID: radaroverlay.PositionTopLeft, Label: "Arriba a la izquierda", Description: "En 9:16, bajo la franja del título"},
	{ID: radaroverlay.PositionTopRight, Label: "Arriba a la derecha", Description: "En 9:16, bajo la franja del título"},
	{ID: radaroverlay.PositionBottomLeft, Label: "Abajo a la izquierda", Description: "Sobre la franja de subtítulos de Shorts"},
	{ID: radaroverlay.PositionBottomRight, Label: "Abajo a la derecha", Description: "Sobre la franja de subtítulos de Shorts"},
}

var radarSizeOptions = []shortOption{
	{ID: radaroverlay.SizeSmall, Label: "Pequeño", Description: "Un cuarto del lado corto del vídeo"},
	{ID: radaroverlay.SizeMedium, Label: "Mediano", Description: "Un tercio del lado corto del vídeo", Default: true},
	{ID: radaroverlay.SizeLarge, Label: "Grande", Description: "Dos quintos del lado corto del vídeo"},
}

// resolveRadar checks the radar choices and stores the corner, size and opacity defaults as unset,
// as the server does. The server also refuses it with a preset that records the CS2 HUD.
func resolveRadar(edit *tuiclient.ShortEdit, c shortChoices) error {
	if !orDefault(c.Radar, false) {
		if c.RadarPosition != "" || c.RadarSize != "" || c.RadarOpacity != nil {
			return errors.New("radar_position, radar_size and radar_opacity need radar set to true")
		}
		return nil
	}
	if edit.NativeHUD {
		return errors.New("radar draws over a capture without the CS2 HUD, whose own radar would show twice; leave native_hud off")
	}
	position, err := pickOption("radar_position", c.RadarPosition, radarPositionOptions)
	if err != nil {
		return err
	}
	size, err := pickOption("radar_size", c.RadarSize, radarSizeOptions)
	if err != nil {
		return err
	}
	opacity := 0.0
	if c.RadarOpacity != nil {
		percent := *c.RadarOpacity
		if percent < int(radaroverlay.MinOpacity*100) || percent > 100 {
			return fmt.Errorf("radar_opacity must be from %d to 100 percent", int(radaroverlay.MinOpacity*100))
		}
		if float64(percent)/100 != radaroverlay.DefaultOpacity {
			opacity = float64(percent) / 100
		}
	}
	if position == radarAuto {
		position = ""
	}
	if size == radaroverlay.SizeMedium {
		size = ""
	}
	edit.Radar, edit.RadarPosition, edit.RadarSize, edit.RadarOpacity = true, position, size, opacity
	return nil
}

// describeRadar is the radar's part of the request summary, empty without it.
func describeRadar(e tuiclient.ShortEdit) string {
	if !e.Radar {
		return ""
	}
	position, size := radarAuto, radaroverlay.SizeMedium
	if e.RadarPosition != "" {
		position = e.RadarPosition
	}
	if e.RadarSize != "" {
		size = e.RadarSize
	}
	return fmt.Sprintf(", radar %s %s al %.0f %%", position, size, radaroverlay.EffectiveOpacity(e.RadarOpacity)*100)
}
