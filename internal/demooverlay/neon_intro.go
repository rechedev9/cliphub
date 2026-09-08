package demooverlay

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"image/png"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The template uses browser layout, CSS perspective and SVG decoration. All
// resources are embedded data URLs, so Chromium never needs network access.
// Roboto and flag-icons licenses and provenance are bundled in assets/.
//
//go:embed neon-intro.html neon-outro.html screenshots.html assets/fonts/*.ttf assets/flags/*.svg
var neonAssets embed.FS

type neonCard struct {
	Name, SteamID, Avatar, Flag, ELO, Rank, RankClass, Level string
	Verified, Premium, FACEIT                                bool
	Matches, RecentLabel, Win, ADR, HS, KD, KR, KDA          string
}

type neonTemplateData struct {
	RegularFont, MediumFont, BoldFont template.URL
	PreviewGrey                       bool
	Panels                            [][]neonCard
}

// NeonIntroHTML produces the same self-contained HTML used by the production
// Chromium renderer, allowing previews to inspect real DOM layout and styles.
func NeonIntroHTML(doc Document, previewGrey bool) ([]byte, error) {
	data := neonTemplateData{PreviewGrey: previewGrey}
	var err error
	data.RegularFont, err = neonAssetURL("assets/fonts/Roboto-Regular.ttf", "font/ttf")
	if err != nil {
		return nil, err
	}
	data.MediumFont, err = neonAssetURL("assets/fonts/Roboto-Medium.ttf", "font/ttf")
	if err != nil {
		return nil, err
	}
	data.BoldFont, err = neonAssetURL("assets/fonts/Roboto-Bold.ttf", "font/ttf")
	if err != nil {
		return nil, err
	}
	for _, cards := range [][]PlayerCard{doc.Intro.Left, doc.Intro.Right} {
		panel := []neonCard{}
		for i, card := range neonRosterOrder(cards, doc.TargetSteamID64) {
			if i >= 5 {
				break
			}
			view := neonCardView(card, NormalizeSource(doc.Source) == SourceFACEIT)
			view.Avatar = string(neonAvatarURL(card.AvatarFile))
			code := strings.ToLower(strings.TrimSpace(card.Country))
			if len(code) == 2 && code[0] >= 'a' && code[0] <= 'z' && code[1] >= 'a' && code[1] <= 'z' {
				if flag, err := neonAssetURL("assets/flags/"+code+".svg", "image/svg+xml"); err == nil {
					view.Flag = string(flag)
				}
			}
			panel = append(panel, view)
		}
		data.Panels = append(data.Panels, panel)
	}
	raw, err := neonAssets.ReadFile("neon-intro.html")
	if err != nil {
		return nil, err
	}
	t, err := template.New("neon-intro").Funcs(template.FuncMap{"asset": func(v string) template.URL {
		// These values are constructed from local bytes above, never roster strings.
		if strings.HasPrefix(v, "data:") {
			return template.URL(v)
		} // #nosec G203 -- generated data URL.
		return ""
	}}).Parse(string(raw))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := t.Execute(&out, data); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
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

func renderNeonIntroStill(doc Document, outPath string, previewGrey bool) error {
	html, err := NeonIntroHTML(doc, previewGrey)
	if err != nil {
		return err
	}
	return renderHTMLStill(html, outPath)
}

func renderHTMLStill(html []byte, outPath string) error {
	renderer := strings.TrimSpace(os.Getenv("ZV_OVERLAY_RENDERER_PATH"))
	if renderer == "" {
		return fmt.Errorf("neon overlay requires the Studio Chromium renderer (ZV_OVERLAY_RENDERER_PATH)")
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0750); err != nil {
		return err
	}
	dir, err := os.MkdirTemp(filepath.Dir(outPath), ".neon-overlay-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	htmlPath, err := filepath.Abs(filepath.Join(dir, "overlay.html"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(htmlPath, html, 0600); err != nil {
		return err
	}
	outputPath, err := filepath.Abs(outPath)
	if err != nil {
		return err
	}
	requestPath, err := filepath.Abs(filepath.Join(dir, "request.json"))
	if err != nil {
		return err
	}
	body, err := json.Marshal(struct {
		HTMLPath   string `json:"html_path"`
		OutputPath string `json:"output_path"`
		Width      int    `json:"width"`
		Height     int    `json:"height"`
	}{htmlPath, outputPath, FrameWidth, FrameHeight})
	if err != nil {
		return err
	}
	if err := os.WriteFile(requestPath, body, 0600); err != nil {
		return err
	}
	args := []string{}
	if app := strings.TrimSpace(os.Getenv("ZV_OVERLAY_RENDERER_APP")); app != "" {
		args = append(args, app)
	}
	args = append(args, "--cliphub-render-overlay", requestPath)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, renderer, args...)
	for _, env := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(env), "ELECTRON_RUN_AS_NODE=") {
			cmd.Env = append(cmd.Env, env)
		}
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Chromium overlay render: %w: %s", err, strings.TrimSpace(string(out)))
	}
	f, err := os.Open(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()
	config, err := png.DecodeConfig(f)
	if err != nil {
		return fmt.Errorf("Chromium overlay is not a PNG: %w", err)
	}
	if config.Width != FrameWidth || config.Height != FrameHeight {
		return fmt.Errorf("Chromium overlay is %dx%d, want 1920x1080", config.Width, config.Height)
	}
	return nil
}
