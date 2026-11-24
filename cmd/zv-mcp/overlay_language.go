package main

import (
	"strings"

	"github.com/rechedev9/cliphub/internal/overlaylang"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

var overlayLanguageOptions = []shortOption{
	{ID: overlaylang.Spanish.String(), Label: "Español", Description: "Titular, insignias, tarjetas, probabilidad de victoria y HUD vertical en español", Default: true},
	{ID: overlaylang.English.String(), Label: "Inglés", Description: "Los mismos textos del vídeo en inglés, para un canal en inglés"},
}

func resolveOverlayLanguage(edit *tuiclient.ShortEdit, code string) error {
	id, err := pickOption("overlay_language", strings.ToLower(strings.TrimSpace(code)), overlayLanguageOptions)
	if err != nil {
		return err
	}
	edit.OverlayLanguage, err = overlaylang.Parse(id)
	return err
}

func describeOverlayLanguage(e tuiclient.ShortEdit) string {
	if e.OverlayLanguage == overlaylang.Spanish {
		return ""
	}
	return ", textos del vídeo en inglés"
}
