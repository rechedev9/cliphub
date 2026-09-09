package customhud

import (
	"fmt"
	"sync"

	"github.com/rechedev9/cliphub/internal/mediafont"
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

type measuredFace struct {
	mediafont.HUDFace
	font *sfnt.Font
}

var fontOnce sync.Once
var hudFaces []measuredFace
var hudFontError error

func NewRenderer(id string) (*Renderer, error) {
	theme, ok := Lookup(id)
	if !ok {
		return nil, fmt.Errorf("unknown custom HUD %q", id)
	}
	fontOnce.Do(func() {
		faces, err := mediafont.HUDFaces()
		if err != nil {
			hudFontError = err
			return
		}
		for _, face := range faces {
			parsed, err := sfnt.Parse(face.Data)
			if err != nil {
				hudFontError = err
				return
			}
			hudFaces = append(hudFaces, measuredFace{face, parsed})
		}
	})
	if hudFontError != nil {
		return nil, hudFontError
	}
	if err := loadIcons(); err != nil {
		return nil, err
	}
	return &Renderer{Theme: theme}, nil
}

func textFace(value string, weight int) measuredFace {
	face := hudFaces[1]
	for _, candidate := range hudFaces {
		if candidate.Weight == weight {
			face = candidate
			break
		}
	}
	var buf sfnt.Buffer
	for _, c := range value {
		glyph, err := face.font.GlyphIndex(&buf, c)
		if err != nil || glyph == 0 {
			return hudFaces[len(hudFaces)-1]
		}
	}
	return face
}

func measureText(value string, size int, face measuredFace) int {
	var buf sfnt.Buffer
	var advance fixed.Int26_6
	var previous sfnt.GlyphIndex
	for i, c := range value {
		glyph, err := face.font.GlyphIndex(&buf, c)
		if err != nil {
			return Width + 1
		}
		if i > 0 {
			kern, _ := face.font.Kern(&buf, previous, glyph, fixed.I(size), font.HintingNone)
			advance += kern
		}
		step, err := face.font.GlyphAdvance(&buf, glyph, fixed.I(size), font.HintingNone)
		if err != nil {
			return Width + 1
		}
		advance += step
		previous = glyph
	}
	return advance.Ceil()
}

func fitText(value string, size, width, weight int) (string, measuredFace) {
	value = cleanText(value, 100)
	face := textFace(value, weight)
	if measureText(value, size, face) <= width {
		return value, face
	}
	runes := []rune(value)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		candidate := string(runes) + "…"
		if measureText(candidate, size, face) <= width {
			return candidate, face
		}
	}
	return "", face
}
