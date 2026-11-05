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
	// assScale converts a \fs size to the em size libass renders: it sizes a
	// face so usWinAscent+usWinDescent equals \fs, not the em square.
	assScale float64
}

func assEmScale(data []byte) float64 {
	u16 := func(at int) int {
		if at < 0 || at+2 > len(data) {
			return 0
		}
		return int(data[at])<<8 | int(data[at+1])
	}
	tables := map[string]int{}
	for i := 0; i < u16(4); i++ {
		rec := 12 + 16*i
		if rec+16 > len(data) {
			break
		}
		tables[string(data[rec:rec+4])] = u16(rec+8)<<16 | u16(rec+10)
	}
	head, okHead := tables["head"]
	os2, okOS2 := tables["OS/2"]
	if !okHead || !okOS2 {
		return 1
	}
	upm, win := u16(head+18), u16(os2+74)+u16(os2+76)
	if upm == 0 || win == 0 {
		return 1
	}
	return float64(upm) / float64(win)
}

var fontOnce sync.Once
var hudFaces []measuredFace
var hudFontError error

func NewRenderer(id string) (*Renderer, error) {
	theme, ok := Lookup(id)
	if !ok {
		return nil, fmt.Errorf("unknown custom HUD %q", id)
	}
	if err := loadAssets(); err != nil {
		return nil, err
	}
	return &Renderer{Theme: theme}, nil
}

// loadAssets parses the embedded faces and icons once; text fitting and icon
// paths depend on both.
func loadAssets() error {
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
			hudFaces = append(hudFaces, measuredFace{face, parsed, assEmScale(face.Data)})
		}
	})
	if hudFontError != nil {
		return hudFontError
	}
	return loadIcons()
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
