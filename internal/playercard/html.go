package playercard

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"unicode"

	"github.com/rechedev9/cliphub/internal/mediafont"
	"github.com/rechedev9/cliphub/internal/overlaylang"
)

//go:embed templates/*.html
var templates embed.FS

// maxImageBytes bounds the player image inlined into a page; uploads are already capped at 10 MB.
const maxImageBytes = 10 << 20

type stat struct{ Label, Value string }

type fontFace struct {
	Family string
	Weight int
	URL    template.URL
}

type view struct {
	Text                       cardText
	Fonts                      []fontFace
	Name, Team, Role, Initials string
	Image                      template.URL
	Rating, Level, ELO         string
	Result, Outcome, Map       string
	Stats                      []stat
	Best                       *stat
}

type Page struct {
	Template string
	Card     Card
	Image    []byte
	Language overlaylang.Language
}

// HTML is the page the overlay renderer draws for one template: the card over a
// transparent 1080x1920 frame, with every font and the image inlined.
func HTML(page Page) ([]byte, error) {
	if _, ok := Lookup(page.Template); !ok {
		return nil, fmt.Errorf("unknown player card template %q", page.Template)
	}
	v, err := newView(page)
	if err != nil {
		return nil, err
	}
	switch page.Template {
	case "carta":
		v.Stats = v.Text.matchStats(page.Card, statKD, statADR, statKAST, statHS, statEntries, statUtility)
	case "ficha":
		v.Stats = firstStats(v.Text.matchStats(page.Card, statRating, statKD, statADR, statKAST, statHS), 3)
	case "mvp":
		v.Best = v.Text.bestStat(page.Card)
	}
	t, err := template.ParseFS(templates, "templates/page.html", "templates/"+page.Template+".html")
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := t.ExecuteTemplate(&out, "page", v); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func newView(page Page) (view, error) {
	faces, err := mediafont.HUDFaces()
	if err != nil {
		return view{}, err
	}
	card, text := page.Card, cardTexts(page.Language)
	v := view{Text: text, Name: strings.TrimSpace(card.Name), Team: strings.TrimSpace(card.Team), Role: text.roles[card.Role], Initials: initials(card.Name), Image: imageURL(page.Image)}
	for _, face := range faces {
		family := "Card Sans"
		if face.Weight >= 800 {
			family = "Card Display"
		}
		v.Fonts = append(v.Fonts, fontFace{Family: family, Weight: face.Weight, URL: template.URL("data:font/ttf;base64," + base64.StdEncoding.EncodeToString(face.Data))}) // #nosec G203 -- embedded font bytes.
	}
	if card.Rating != nil {
		v.Rating = fmt.Sprintf("%.2f", *card.Rating)
	}
	if f := card.FACEIT; f != nil {
		if f.Level > 0 {
			v.Level = fmt.Sprint(f.Level)
		}
		if f.ELO > 0 {
			v.ELO = fmt.Sprint(f.ELO)
		}
	}
	if r := card.Result; r != nil {
		v.Result, v.Outcome = fmt.Sprintf(text.tied, r.Own, r.Rival), "tied"
		if r.Own > r.Rival {
			v.Result, v.Outcome = fmt.Sprintf(text.won, r.Own, r.Rival), "won"
		} else if r.Own < r.Rival {
			v.Result, v.Outcome = fmt.Sprintf(text.lost, r.Own, r.Rival), "lost"
		}
	}
	v.Map = strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(card.Map), "de_"))
	return v, nil
}

// imageURL inlines a PNG, JPEG, WebP or GIF; anything else leaves the card on its initials.
func imageURL(image []byte) template.URL {
	if len(image) == 0 || len(image) > maxImageBytes {
		return ""
	}
	mime := http.DetectContentType(image)
	if mime != "image/png" && mime != "image/jpeg" && mime != "image/webp" && mime != "image/gif" {
		return ""
	}
	return template.URL("data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(image)) // #nosec G203 -- sniffed image bytes.
}

// initials are the first letters of the first two words of the name, or its first two letters.
func initials(name string) string {
	words := strings.FieldsFunc(name, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	var out []rune
	switch {
	case len(words) >= 2:
		out = []rune{[]rune(words[0])[0], []rune(words[1])[0]}
	case len(words) == 1:
		out = []rune(words[0])
		out = out[:min(2, len(out))]
	default:
		return "?"
	}
	return strings.ToUpper(string(out))
}

func firstStats(stats []stat, n int) []stat {
	return stats[:min(n, len(stats))]
}
