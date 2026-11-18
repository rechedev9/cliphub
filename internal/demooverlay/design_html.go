package demooverlay

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
)

// DesignView picks one page of an HTML design: the intro roster or the final scoreboard.
type DesignView struct {
	Outro bool
	// PreviewGrey paints a mid-grey canvas instead of transparency.
	PreviewGrey bool
}

// UsesDesignHTML reports whether Chromium draws the document's design from HTML.
func UsesDesignHTML(doc Document) bool {
	theme := NormalizeTheme(doc.Theme)
	return theme == ThemeNeonViolet || theme == ThemeHUD
}

// DesignHTML is the self-contained page the Chromium renderer draws. Every
// design shares the page body, so a design changes the look, never the content.
func DesignHTML(doc Document, view DesignView) ([]byte, error) {
	if !UsesDesignHTML(doc) {
		return nil, fmt.Errorf("the %s overlay theme is not drawn from HTML", NormalizeTheme(doc.Theme))
	}
	if view.Outro {
		return scoreboardHTML(doc, view.PreviewGrey)
	}
	return rosterHTML(doc, view.PreviewGrey)
}

type designStill struct {
	path string
	DesignView
}

func renderDesignStill(doc Document, still designStill) error {
	html, err := DesignHTML(doc, still.DesignView)
	if err != nil {
		return err
	}
	return renderHTMLStill(html, still.path)
}

func designFile(doc Document, page string) string {
	if NormalizeTheme(doc.Theme) == ThemeHUD {
		return "hud-" + page + ".html"
	}
	return "neon-" + page + ".html"
}

type pageFonts struct {
	RegularFont, MediumFont, BoldFont template.URL
}

func loadPageFonts() (pageFonts, error) {
	var fonts pageFonts
	for _, font := range []struct {
		url  *template.URL
		file string
	}{{&fonts.RegularFont, "Roboto-Regular"}, {&fonts.MediumFont, "Roboto-Medium"}, {&fonts.BoldFont, "Roboto-Bold"}} {
		url, err := neonAssetURL("assets/fonts/"+font.file+".ttf", "font/ttf")
		if err != nil {
			return pageFonts{}, err
		}
		*font.url = url
	}
	return fonts, nil
}

// executePage runs a design page with the shared body its page kind defines.
func executePage(file string, data any) ([]byte, error) {
	body := "intro-body.html"
	if strings.HasSuffix(file, "-outro.html") {
		body = "scoreboard-body.html"
	}
	t, err := template.New(file).Funcs(template.FuncMap{"asset": func(v string) template.URL {
		// These values are constructed from local bytes, never roster strings.
		if strings.HasPrefix(v, "data:") {
			return template.URL(v)
		} // #nosec G203 -- generated data URL.
		return ""
	}}).ParseFS(neonAssets, file, body)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := t.Execute(&out, data); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// panelSide is the CT or T side of a roster panel, from its players.
func panelSide(cards []PlayerCard, fallback string) string {
	for _, card := range cards {
		if card.Team == "CT" || card.Team == "T" {
			return card.Team
		}
	}
	return fallback
}
