package editor

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/rechedev9/cliphub/internal/killplan"
)

const (
	playBadgeSeconds = 1.8
	// The stats card holds the last statsCardSeconds of the Short, never less than statsCardMinSeconds.
	statsCardSeconds    = 3.0
	statsCardMinSeconds = 1.5
	statsCardAfterKill  = 0.6
)

// ShortOverlays are the optional Short graphics drawn from the kill plan and the match stats.
type ShortOverlays struct {
	// PlayBadges names each play's clutch, ace or multikill right after its last kill.
	PlayBadges bool `json:"play_badges,omitempty"`
	// WinProbability holds each segment's post-plant win chance on its own clock.
	WinProbability map[string][]WinProbCue `json:"win_probability,omitempty"`
	// StatsCard closes a 9:16 Short with the player's match stats.
	StatsCard *StatsCard `json:"stats_card,omitempty"`
	// World holds each segment clip's player labels and grenade trails script.
	World *WorldOverlay `json:"world,omitempty"`
	// Watermark and Banner are the creator's brand kit, prepared by prepareBrandOverlays.
	Watermark *BrandWatermark `json:"watermark,omitempty"`
	Banner    *SponsorBanner  `json:"sponsor_banner,omitempty"`
}

// StatsCard is the player's line for the closing card. A nil field is unknown and is not drawn.
type StatsCard struct {
	Player string   `json:"player"`
	Kills  int      `json:"kills"`
	Deaths int      `json:"deaths"`
	ADR    *float64 `json:"adr,omitempty"`
	HSPct  *float64 `json:"hs_pct,omitempty"`
	// Rating is ClipHub's own rating (tacticalplan.Rating), never an HLTV rating.
	Rating *float64         `json:"rating,omitempty"`
	FACEIT *StatsCardFACEIT `json:"faceit,omitempty"`
}

// StatsCardFACEIT is the player's FACEIT data; the card leaves out what is missing.
type StatsCardFACEIT struct {
	Level   int      `json:"level,omitempty"`
	WinPct  *float64 `json:"win_pct,omitempty"`
	BestMap string   `json:"best_map,omitempty"`
}

// LoadStatsCard reads the card the render worker wrote.
func LoadStatsCard(path string) (*StatsCard, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read stats card: %w", err)
	}
	var card StatsCard
	if err := json.Unmarshal(raw, &card); err != nil {
		return nil, fmt.Errorf("decode stats card: %w", err)
	}
	return &card, nil
}

// playBadgeLabel names a play: its clutch (won when the round result is known) and its multikill.
func playBadgeLabel(kills []KillCue) string {
	vs, result := 0, ""
	for _, kill := range kills {
		if kill.ClutchVs > vs {
			vs, result = kill.ClutchVs, kill.ClutchResult
		}
	}
	var parts []string
	if vs > 0 {
		clutch := fmt.Sprintf("CLUTCH 1V%d", vs)
		if result == killplan.ClutchWon {
			clutch += " GANADO"
		}
		parts = append(parts, clutch)
	}
	if len(kills) >= 2 {
		parts = append(parts, killMilestoneLabel(len(kills)))
	}
	return strings.Join(parts, " · ")
}

type badgeWindow struct {
	kills         []KillCue
	offset, until float64
}

// badgeWindows splits the Short into its plays: one per part with kills, or the whole Short.
func badgeWindows(short ShortEdit) []badgeWindow {
	if len(short.Parts) == 0 {
		return []badgeWindow{{kills: short.Kills, until: short.DurationSeconds}}
	}
	var out []badgeWindow
	for _, part := range short.Parts {
		if len(part.Kills) > 0 {
			out = append(out, badgeWindow{kills: part.Kills, offset: part.TimelineStartSeconds, until: part.TimelineStartSeconds + part.DurationSeconds})
		}
	}
	return out
}

// generatedPlayBadgeEffects pops each play's badge under the kill counter lane, after its last kill.
func generatedPlayBadgeEffects(short ShortEdit) []Effect {
	if !short.Overlays.PlayBadges {
		return nil
	}
	width, _ := outputDimensions(short)
	var effects []Effect
	for _, window := range badgeWindows(short) {
		label := playBadgeLabel(window.kills)
		if label == "" {
			continue
		}
		last := window.kills[len(window.kills)-1]
		start := clampSeconds(window.offset+last.TimeSeconds+0.05, 0, short.DurationSeconds)
		end := min(start+playBadgeSeconds, window.until, short.DurationSeconds)
		if short.Outro {
			// The outro card shares the h*0.70 lane at the end of the Short.
			end = min(end, transitionStart(short, 1.6))
		}
		if cardStart, _, ok := statsCardWindow(short); ok {
			end = min(end, cardStart)
		}
		if end-start < 0.4 {
			continue
		}
		effects = append(effects, Effect{
			Type:               EffectText,
			Value:              label,
			X:                  "(w-text_w)/2",
			Y:                  "h*0.70",
			Size:               fitFontSize(label, 60, width-160),
			FontColor:          "black",
			BoxColor:           "0xFFC83D@0.94",
			BoxBorder:          18,
			Bold:               true,
			StartSeconds:       start,
			EndSeconds:         end,
			FadeInSeconds:      0.08,
			FadeOutSeconds:     0.25,
			Source:             "play-badge",
			SourceKillTick:     last.Tick,
			SourceKillWeapon:   last.Weapon,
			SourceKillVictim:   last.Victim,
			SourceKillHeadshot: last.Headshot,
		})
	}
	return effects
}

// statsCardWindow is the closing span of the card: after the last kill (and its badge) when the Short allows it.
func statsCardWindow(short ShortEdit) (float64, float64, bool) {
	end := short.DurationSeconds
	if short.Overlays.StatsCard == nil || short.OutputFormat != OutputFormatShort9x16 || short.MultiPOV() || end < statsCardMinSeconds {
		return 0, 0, false
	}
	afterKill := statsCardAfterKill
	if short.Overlays.PlayBadges {
		afterKill = playBadgeSeconds + 0.1
	}
	start := end - statsCardSeconds
	for _, kill := range short.Kills {
		start = max(start, kill.TimeSeconds+afterKill)
	}
	if end-start < statsCardMinSeconds {
		start = end - statsCardMinSeconds
	}
	return max(start, 0), end, true
}

// statsCardLines are the card rows in order; rows without data are left out.
func statsCardLines(card StatsCard) []string {
	kd := fmt.Sprintf("K/D %d/%d", card.Kills, card.Deaths)
	if card.Deaths > 0 {
		kd = fmt.Sprintf("K/D %.2f (%d/%d)", float64(card.Kills)/float64(card.Deaths), card.Kills, card.Deaths)
	}
	lines := []string{kd}
	var rates []string
	if card.ADR != nil {
		rates = append(rates, fmt.Sprintf("ADR %.1f", *card.ADR))
	}
	if card.HSPct != nil {
		rates = append(rates, fmt.Sprintf("HS %.0f %%", *card.HSPct))
	}
	if len(rates) > 0 {
		lines = append(lines, strings.Join(rates, " · "))
	}
	if card.Rating != nil {
		lines = append(lines, fmt.Sprintf("RATING DE CLIPHUB %.2f", *card.Rating))
	}
	if f := card.FACEIT; f != nil {
		var faceit []string
		if f.Level > 0 {
			faceit = append(faceit, fmt.Sprintf("FACEIT NIVEL %d", f.Level))
		}
		if f.WinPct != nil {
			faceit = append(faceit, fmt.Sprintf("%.0f %% DE VICTORIAS", *f.WinPct))
		}
		if len(faceit) > 0 {
			lines = append(lines, strings.Join(faceit, " · "))
		}
		if bestMap := strings.TrimSpace(f.BestMap); bestMap != "" {
			lines = append(lines, "MEJOR MAPA: "+strings.ToUpper(strings.TrimPrefix(bestMap, "de_")))
		}
	}
	return lines
}

// generatedStatsCardEffects dims the closing seconds of a 9:16 Short and draws the player's card over them.
func generatedStatsCardEffects(short ShortEdit) []Effect {
	start, end, ok := statsCardWindow(short)
	if !ok {
		return nil
	}
	card := short.Overlays.StatsCard
	width, height := outputDimensions(short)
	text := func(value string, y, size int, bold bool) Effect {
		return Effect{
			Type: EffectText, Value: value, X: "(w-text_w)/2", Y: fmt.Sprintf("%d", y),
			Size: fitFontSize(value, size, width-140), FontColor: "white@0.97", BoxColor: "none",
			ShadowColor: "black@0.85", ShadowX: 3, ShadowY: 3, Bold: bold,
			StartSeconds: start, EndSeconds: end, FadeInSeconds: 0.2, Source: "stats-card",
		}
	}
	effects := []Effect{
		{Type: EffectFlash, Color: "black", Opacity: 0.58, StartSeconds: start, EndSeconds: end, Source: "stats-card"},
		text("ESTADÍSTICAS DE LA PARTIDA", height*30/100, 36, false),
	}
	y := height * 35 / 100
	if name := strings.TrimSpace(card.Player); name != "" {
		effects = append(effects, text(name, y, 80, true))
		y += 130
	}
	for _, line := range statsCardLines(*card) {
		effects = append(effects, text(line, y, 50, true))
		y += 86
	}
	return effects
}
