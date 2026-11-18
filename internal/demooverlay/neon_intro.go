package demooverlay

import (
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/htmlstill"
)

// The templates use browser layout, CSS perspective and SVG decoration. All
// resources are embedded data URLs, so Chromium never needs network access.
// Roboto and flag-icons licenses and provenance are bundled in assets/.
//
//go:embed neon-intro.html neon-outro.html hud-intro.html hud-outro.html intro-body.html scoreboard-body.html screenshots.html assets/fonts/*.ttf assets/flags/*.svg
var neonAssets embed.FS

type neonCard struct {
	Name, SteamID, Avatar, Flag, ELO, Rank, RankClass, Level string
	Verified, Premium, FACEIT, POV                           bool
	Matches, RecentLabel, Win, ADR, HS, KD, KR, KDA          string
}

type neonTemplateData struct {
	pageFonts
	PreviewGrey bool
	Panels      [][]neonCard
	// Sides holds the CT or T side of each panel, for the designs that colour them.
	Sides   []string
	Palette *Palette
}

// rosterHTML produces the same self-contained HTML used by the production
// Chromium renderer, allowing previews to inspect real DOM layout and styles.
func rosterHTML(doc Document, previewGrey bool) ([]byte, error) {
	fonts, err := loadPageFonts()
	if err != nil {
		return nil, err
	}
	data := neonTemplateData{pageFonts: fonts, PreviewGrey: previewGrey, Palette: doc.Palette}
	for side, cards := range [][]PlayerCard{doc.Intro.Left, doc.Intro.Right} {
		panel := []neonCard{}
		for i, card := range neonRosterOrder(cards, doc.TargetSteamID64) {
			if i >= 5 {
				break
			}
			view := neonCardView(card, NormalizeSource(doc.Source) == SourceFACEIT)
			view.Avatar = string(neonAvatarURL(card.AvatarFile))
			view.POV = doc.IsPOV(card)
			code := strings.ToLower(strings.TrimSpace(card.Country))
			if len(code) == 2 && code[0] >= 'a' && code[0] <= 'z' && code[1] >= 'a' && code[1] <= 'z' {
				if flag, err := neonAssetURL("assets/flags/"+code+".svg", "image/svg+xml"); err == nil {
					view.Flag = string(flag)
				}
			}
			panel = append(panel, view)
		}
		data.Panels = append(data.Panels, panel)
		data.Sides = append(data.Sides, panelSide(cards, []string{"CT", "T"}[side]))
	}
	return executePage(designFile(doc, "intro"), data)
}

func neonAssetURL(name, mime string) (template.URL, error) {
	data, err := neonAssets.ReadFile(name)
	if err != nil {
		return "", err
	}
	return template.URL("data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)), nil // #nosec G203 -- bundled bytes.
}

func neonAvatarURL(file string) template.URL {
	if file == "" {
		return ""
	}
	stat, err := os.Stat(file)
	if err != nil || stat.Size() > 8<<20 {
		return ""
	}
	avatar, err := os.ReadFile(file)
	if err != nil || len(avatar) == 0 {
		return ""
	}
	mime := http.DetectContentType(avatar)
	if mime != "image/png" && mime != "image/jpeg" && mime != "image/webp" && mime != "image/gif" {
		return ""
	}
	return template.URL("data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(avatar)) // #nosec G203 -- image bytes.
}

func neonCardView(card PlayerCard, faceitSource bool) neonCard {
	v := neonCard{Name: card.Name, SteamID: card.SteamID64, Verified: card.Verified, Premium: card.Premium, FACEIT: faceitSource, Matches: "—", RecentLabel: "Recent matches", Win: "—", ADR: "—", HS: "—", KD: "—", KR: "—", KDA: fmt.Sprintf("%d/%d/%d", card.Kills, card.Deaths, card.Assists)}
	if card.ELO != nil {
		v.ELO = strconv.Itoa(*card.ELO)
	}
	if card.SkillLevel != nil {
		v.Level = strconv.Itoa(*card.SkillLevel)
	}
	if card.Ranking != nil && *card.Ranking > 0 && *card.Ranking <= 1000 {
		v.Rank = "#" + strconv.Itoa(*card.Ranking)
		v.RankClass = "challenger"
		switch *card.Ranking {
		case 1:
			v.RankClass = "gold"
		case 2:
			v.RankClass = "silver"
		case 3:
			v.RankClass = "bronze"
		}
	}
	if !faceitSource {
		if card.HasADR {
			v.ADR = fmt.Sprintf("%.1f", card.ADR)
		}
		if card.HasHSPct {
			v.HS = fmt.Sprintf("%.0f%%", card.HSPct)
		}
		return v
	}
	if card.LifetimeMatches != nil {
		v.Matches = strings.ReplaceAll(formatThousands(*card.LifetimeMatches), ".", ",")
	}
	if l := card.Last20; l != nil {
		if l.Matches != nil {
			v.RecentLabel = fmt.Sprintf("Last %d Matches", *l.Matches)
		}
		if l.WinPct != nil {
			v.Win = fmt.Sprintf("%.0f%%", *l.WinPct)
		}
		if l.ADR != nil {
			v.ADR = fmt.Sprintf("%.1f", *l.ADR)
		}
		if l.HSPct != nil {
			v.HS = fmt.Sprintf("%.0f%%", *l.HSPct)
		}
		if l.KD != nil {
			v.KD = fmt.Sprintf("%.2f", *l.KD)
		}
		if l.KR != nil {
			v.KR = fmt.Sprintf("%.2f", *l.KR)
		}
	}
	return v
}

func neonRosterOrder(cards []PlayerCard, target string) []PlayerCard {
	ordered := append([]PlayerCard(nil), cards...)
	for i, card := range ordered {
		if card.SteamID64 == target && i > 0 {
			copy(ordered[1:i+1], ordered[:i])
			ordered[0] = card
			break
		}
	}
	return ordered
}

func renderHTMLStill(html []byte, outPath string) error {
	return htmlstill.Render(html, outPath, htmlstill.Landscape)
}
