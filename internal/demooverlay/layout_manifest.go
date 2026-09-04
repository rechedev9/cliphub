package demooverlay

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
)

const layoutSchemaVersion = "cliphub.full-demo-overlay-layout/v1"
const layoutSchemaReference = "./full-demo-overlay-layout.schema.json"

//go:embed faceit-layout.json
var faceitLayoutJSON []byte

//go:embed full-demo-overlay-layout.schema.json
var layoutSchemaJSON []byte

var defaultFaceitLayout = mustDecodeLayoutSpec(faceitLayoutJSON)

type layoutSpec struct {
	SchemaURI     string               `json:"$schema"`
	SchemaVersion string               `json:"schema_version"`
	Canvas        canvasSpec           `json:"canvas"`
	Assets        assetSetSpec         `json:"assets"`
	Themes        map[string]themeSpec `json:"themes"`
	Palette       paletteSpec          `json:"palette"`
	Intro         introLayoutSpec      `json:"intro"`
	Outro         outroLayoutSpec      `json:"outro"`
}

type canvasSpec struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type assetSetSpec struct {
	IntroChrome assetSpec `json:"intro_chrome"`
	OutroChrome assetSpec `json:"outro_chrome"`
}

type assetSpec struct {
	Renderer      string `json:"renderer"`
	File          string `json:"file,omitempty"`
	Fit           string `json:"fit"`
	RequiresAlpha bool   `json:"requires_alpha"`
	ArtDirection  string `json:"art_direction"`
}

type themeSpec struct {
	Accent     string `json:"accent"`
	AccentSoft string `json:"accent_soft"`
}

type paletteSpec struct {
	Text             string `json:"text"`
	MutedText        string `json:"muted_text"`
	StatText         string `json:"stat_text"`
	StatPositive     string `json:"stat_positive"`
	StatNegative     string `json:"stat_negative"`
	TargetText       string `json:"target_text"`
	POVFill          string `json:"pov_fill"`
	POVText          string `json:"pov_text"`
	RankFill         string `json:"rank_fill"`
	RankText         string `json:"rank_text"`
	Level10Fill      string `json:"level_10_fill"`
	Level8Fill       string `json:"level_8_fill"`
	Level4Fill       string `json:"level_4_fill"`
	Level2Fill       string `json:"level_2_fill"`
	LevelDefaultFill string `json:"level_default_fill"`
	LevelRingFill    string `json:"level_ring_fill"`
	IntroPanelFill   string `json:"intro_panel_fill"`
	IntroPanelBorder string `json:"intro_panel_border"`
	IntroCardFill    string `json:"intro_card_fill"`
	IntroCardBorder  string `json:"intro_card_border"`
	IntroCardDivider string `json:"intro_card_divider"`
	IntroStatsBand   string `json:"intro_stats_band"`
	IntroTexture     string `json:"intro_texture"`
	AvatarFill       string `json:"avatar_fill"`
	OutroBackdrop    string `json:"outro_backdrop"`
	OutroBoardFill   string `json:"outro_board_fill"`
	OutroBoardBorder string `json:"outro_board_border"`
	OutroLabelFill   string `json:"outro_label_fill"`
	OutroRowEven     string `json:"outro_row_even"`
	OutroRowOdd      string `json:"outro_row_odd"`
	OutroCTName      string `json:"outro_ct_name"`
	OutroCTStripe    string `json:"outro_ct_stripe"`
	OutroTName       string `json:"outro_t_name"`
	OutroTStripe     string `json:"outro_t_stripe"`
	OutroDivider     string `json:"outro_divider"`
}

type textAnchorSpec struct {
	X        int `json:"x"`
	Y        int `json:"y"`
	FontSize int `json:"font_size"`
}

type rectSpec struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// badgeSpec is a filled rect whose text is centered inside it by the ffmpeg
// drawtext text_w/text_h expressions, so the manifest never guesses glyph
// metrics.
type badgeSpec struct {
	Rect     rectSpec `json:"rect"`
	FontSize int      `json:"font_size"`
}

type introLayoutSpec struct {
	ZOrder   []string        `json:"z_order"`
	Panel    introPanelSpec  `json:"panel"`
	Header   introHeaderSpec `json:"header"`
	Card     rectSpec        `json:"card"`
	Avatar   rectSpec        `json:"avatar"`
	Name     textAnchorSpec  `json:"name"`
	Country  badgeSpec       `json:"country"`
	ELO      rightTextSpec   `json:"elo"`
	ELOLabel rightTextSpec   `json:"elo_label"`
	POV      badgeSpec       `json:"pov"`
	Level    badgeSpec       `json:"level"`
	Rank     badgeSpec       `json:"rank"`
	Stats    introStatsSpec  `json:"stats"`
	Chrome   introChromeSpec `json:"chrome"`
}

type introPanelSpec struct {
	Width        int `json:"width"`
	Height       int `json:"height"`
	Top          int `json:"top"`
	LeftX        int `json:"left_x"`
	RightX       int `json:"right_x"`
	CenterGap    int `json:"center_gap"`
	RowHeight    int `json:"row_height"`
	MaxPlayers   int `json:"max_players"`
	HeaderHeight int `json:"header_height"`
}

type introHeaderSpec struct {
	Name     textAnchorSpec `json:"name"`
	Subtitle textAnchorSpec `json:"subtitle"`
}

type introStatsSpec struct {
	X int `json:"x"`
	// BandY is the top of the darker stats band, relative to the row origin;
	// the band runs to the card bottom.
	BandY     int       `json:"band_y"`
	TitleY    int       `json:"title_y"`
	ValueY    int       `json:"value_y"`
	Right     int       `json:"right"`
	TitleSize int       `json:"title_size"`
	ValueSize int       `json:"value_size"`
	LabelSize int       `json:"label_size"`
	LabelGap  int       `json:"label_gap"`
	Weights   []float64 `json:"weights"`
}

type introChromeSpec struct {
	PanelRadius     int `json:"panel_radius"`
	PanelBorder     int `json:"panel_border"`
	AccentWidth     int `json:"accent_width"`
	HeaderDivider   int `json:"header_divider"`
	CardRadius      int `json:"card_radius"`
	CardAccentWidth int `json:"card_accent_width"`
	AvatarRing      int `json:"avatar_ring"`
	LevelRing       int `json:"level_ring"`
	TextureSpacing  int `json:"texture_spacing"`
}

type outroLayoutSpec struct {
	ZOrder           []string          `json:"z_order"`
	Margin           int               `json:"margin"`
	HeaderY          int               `json:"header_y"`
	RowY             int               `json:"row_y"`
	RowHeight        int               `json:"row_height"`
	TeamYGap         int               `json:"team_y_gap"`
	MaxPlayers       int               `json:"max_players"`
	NameWidth        int               `json:"name_width"`
	Header           textAnchorSpec    `json:"header"`
	Score            badgeSpec         `json:"score"`
	TeamAverage      rightTextSpec     `json:"team_average"`
	TeamAverageLabel rightTextSpec     `json:"team_average_label"`
	MapLabel         centeredTextSpec  `json:"map_label"`
	ColumnLabelsY    int               `json:"column_labels_y"`
	ColumnLabelSize  int               `json:"column_label_size"`
	StatSize         int               `json:"stat_size"`
	Name             textAnchorSpec    `json:"name"`
	POVNameX         int               `json:"pov_name_x"`
	POV              badgeSpec         `json:"pov"`
	Columns          []outroColumnSpec `json:"columns"`
	Chrome           outroChromeSpec   `json:"chrome"`
}

// rightTextSpec places text so its right edge sits Right pixels from the
// right edge of its container.
type rightTextSpec struct {
	Right    int `json:"right"`
	Y        int `json:"y"`
	FontSize int `json:"font_size"`
}

// centeredTextSpec places text centered horizontally on the canvas at an
// absolute Y.
type centeredTextSpec struct {
	Y        int `json:"y"`
	FontSize int `json:"font_size"`
}

type outroColumnSpec struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	X     int    `json:"x"`
	Width int    `json:"width"`
}

type outroChromeSpec struct {
	BoardLeft       int `json:"board_left"`
	BoardTop        int `json:"board_top"`
	BoardRight      int `json:"board_right"`
	BoardBottom     int `json:"board_bottom"`
	BoardRadius     int `json:"board_radius"`
	AccentWidth     int `json:"accent_width"`
	LabelTop        int `json:"label_top"`
	LabelBottom     int `json:"label_bottom"`
	RowBottomGap    int `json:"row_bottom_gap"`
	NameRightGap    int `json:"name_right_gap"`
	SideStripeWidth int `json:"side_stripe_width"`
	DividerHeight   int `json:"divider_height"`
}

func mustDecodeLayoutSpec(raw []byte) layoutSpec {
	spec, err := decodeLayoutSpec(bytes.NewReader(raw))
	if err != nil {
		panic(fmt.Sprintf("invalid embedded full-demo overlay layout: %v", err))
	}
	return spec
}

func decodeLayoutSpec(r io.Reader) (layoutSpec, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var spec layoutSpec
	if err := dec.Decode(&spec); err != nil {
		return layoutSpec{}, fmt.Errorf("decode overlay layout: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return layoutSpec{}, fmt.Errorf("decode overlay layout: multiple JSON values")
		}
		return layoutSpec{}, fmt.Errorf("decode overlay layout trailing data: %w", err)
	}
	if err := validateLayoutSpec(spec); err != nil {
		return layoutSpec{}, err
	}
	return spec, nil
}

func validateLayoutSpec(spec layoutSpec) error {
	if spec.SchemaURI != layoutSchemaReference {
		return fmt.Errorf("overlay layout $schema = %q, want %q", spec.SchemaURI, layoutSchemaReference)
	}
	if spec.SchemaVersion != layoutSchemaVersion {
		return fmt.Errorf("overlay layout schema %q, want %q", spec.SchemaVersion, layoutSchemaVersion)
	}
	if spec.Canvas.Width != FrameWidth || spec.Canvas.Height != FrameHeight {
		return fmt.Errorf("overlay canvas = %dx%d, want %dx%d", spec.Canvas.Width, spec.Canvas.Height, FrameWidth, FrameHeight)
	}
	for name, asset := range map[string]assetSpec{"intro_chrome": spec.Assets.IntroChrome, "outro_chrome": spec.Assets.OutroChrome} {
		if err := validateAsset(name, asset); err != nil {
			return err
		}
	}
	for _, name := range []string{ThemeFaceitOrange, ThemeNeonViolet} {
		theme, ok := spec.Themes[name]
		if !ok {
			return fmt.Errorf("overlay layout theme %q is required", name)
		}
		if !validOverlayColor(theme.Accent) || !validOverlayColor(theme.AccentSoft) {
			return fmt.Errorf("overlay layout theme %q has invalid colors", name)
		}
	}
	for name, value := range spec.Palette.colors() {
		if !validOverlayColor(value) {
			return fmt.Errorf("overlay layout palette %q has invalid color %q", name, value)
		}
	}
	if err := validateIntroSpec(spec.Intro, spec.Canvas); err != nil {
		return err
	}
	if err := validateOutroSpec(spec.Outro, spec.Canvas); err != nil {
		return err
	}
	return nil
}

func validateAsset(name string, asset assetSpec) error {
	if asset.Renderer != "programmatic" && asset.Renderer != "bitmap" {
		return fmt.Errorf("overlay asset %q renderer %q is invalid", name, asset.Renderer)
	}
	if asset.Renderer == "bitmap" && strings.TrimSpace(asset.File) == "" {
		return fmt.Errorf("overlay asset %q bitmap file is required", name)
	}
	if asset.Fit != "canvas" {
		return fmt.Errorf("overlay asset %q fit %q, want canvas", name, asset.Fit)
	}
	if !asset.RequiresAlpha {
		return fmt.Errorf("overlay asset %q must require alpha", name)
	}
	if strings.TrimSpace(asset.ArtDirection) == "" {
		return fmt.Errorf("overlay asset %q art direction is required", name)
	}
	return nil
}

func validateIntroSpec(spec introLayoutSpec, canvas canvasSpec) error {
	if !slices.Equal(spec.ZOrder, []string{"chrome", "avatars", "data"}) {
		return fmt.Errorf("intro z_order = %v, want [chrome avatars data]", spec.ZOrder)
	}
	p := spec.Panel
	if p.Width <= 0 || p.Height <= 0 || p.RowHeight <= 0 || p.HeaderHeight <= 0 || p.MaxPlayers < 1 || p.MaxPlayers > 5 {
		return fmt.Errorf("intro panel dimensions are invalid")
	}
	if p.LeftX < 0 || p.RightX < 0 || p.Top < 0 || p.LeftX+p.Width > canvas.Width || p.RightX+p.Width > canvas.Width || p.Top+p.Height > canvas.Height {
		return fmt.Errorf("intro panels exceed the canvas")
	}
	gap := p.RightX - (p.LeftX + p.Width)
	if gap != p.CenterGap || gap < 700 {
		return fmt.Errorf("intro center gap = %d (declared %d), want at least 700", gap, p.CenterGap)
	}
	if p.HeaderHeight+p.MaxPlayers*p.RowHeight > p.Height {
		return fmt.Errorf("intro rows exceed panel height")
	}
	for name, rect := range map[string]rectSpec{
		"card": spec.Card, "avatar": spec.Avatar, "country": spec.Country.Rect,
		"pov": spec.POV.Rect, "level": spec.Level.Rect, "rank": spec.Rank.Rect,
	} {
		if rect.Width <= 0 || rect.Height <= 0 || rect.X < 0 || rect.X+rect.Width > p.Width || rect.Y < -p.HeaderHeight || rect.Y+rect.Height > p.RowHeight {
			return fmt.Errorf("intro %s rect %+v exceeds its slot", name, rect)
		}
	}
	for name, anchor := range map[string]textAnchorSpec{
		"header name": spec.Header.Name, "header subtitle": spec.Header.Subtitle,
		"player name": spec.Name,
	} {
		maxY := p.RowHeight
		if strings.HasPrefix(name, "header ") {
			maxY = p.HeaderHeight
		}
		if err := validateTextAnchor("intro "+name, anchor, p.Width, maxY); err != nil {
			return err
		}
	}
	for name, text := range map[string]rightTextSpec{"elo": spec.ELO, "elo label": spec.ELOLabel} {
		if err := validateRightText("intro "+name, text, p.Width, p.RowHeight); err != nil {
			return err
		}
	}
	for name, badge := range map[string]badgeSpec{
		"country": spec.Country, "pov": spec.POV, "level": spec.Level, "rank": spec.Rank,
	} {
		if err := validateBadge("intro "+name, badge); err != nil {
			return err
		}
	}
	if rectsOverlap(spec.POV.Rect, spec.Level.Rect) {
		return fmt.Errorf("intro POV and level badges overlap")
	}
	if rectsOverlap(spec.Country.Rect, spec.POV.Rect) {
		return fmt.Errorf("intro country and POV badges overlap")
	}
	if rectsOverlap(spec.Level.Rect, spec.Rank.Rect) {
		return fmt.Errorf("intro level and rank badges overlap")
	}
	if spec.Name.X < 0 || spec.Stats.X < 0 || spec.Stats.Right < 0 || spec.Stats.X+spec.Stats.Right >= p.Width {
		return fmt.Errorf("intro text anchors exceed panel width")
	}
	if len(spec.Stats.Weights) != 6 {
		return fmt.Errorf("intro stat weights = %d, want 6", len(spec.Stats.Weights))
	}
	for i, weight := range spec.Stats.Weights {
		if weight <= 0 {
			return fmt.Errorf("intro stat weight %d = %g, want positive", i, weight)
		}
	}
	cardBottom := spec.Card.Y + spec.Card.Height
	if spec.Stats.BandY < 0 || spec.Stats.BandY >= cardBottom || spec.Stats.TitleY < spec.Stats.BandY || spec.Stats.ValueY < spec.Stats.TitleY ||
		spec.Stats.ValueY+spec.Stats.ValueSize+spec.Stats.LabelGap+spec.Stats.LabelSize > cardBottom ||
		spec.Stats.TitleSize <= 0 || spec.Stats.ValueSize <= 0 || spec.Stats.LabelSize <= 0 || spec.Stats.LabelGap < 0 {
		return fmt.Errorf("intro stat typography exceeds its band")
	}
	if spec.Avatar.Y+spec.Avatar.Height+spec.Chrome.AvatarRing > spec.Stats.BandY {
		return fmt.Errorf("intro avatar ring overlaps the stats band")
	}
	c := spec.Chrome
	if c.PanelRadius < 0 || c.PanelBorder <= 0 || c.AccentWidth <= 0 || c.HeaderDivider <= 0 || c.CardRadius < 0 ||
		c.CardAccentWidth <= 0 || c.AvatarRing <= 0 || c.LevelRing <= 0 || c.TextureSpacing <= 0 {
		return fmt.Errorf("intro chrome dimensions are invalid")
	}
	return nil
}

func validateOutroSpec(spec outroLayoutSpec, canvas canvasSpec) error {
	if !slices.Equal(spec.ZOrder, []string{"backdrop", "chrome", "data"}) {
		return fmt.Errorf("outro z_order = %v, want [backdrop chrome data]", spec.ZOrder)
	}
	if spec.Margin < 0 || spec.RowHeight <= 0 || spec.TeamYGap <= 0 || spec.MaxPlayers < 1 || spec.MaxPlayers > 5 || spec.NameWidth <= 0 {
		return fmt.Errorf("outro dimensions are invalid")
	}
	if spec.RowY+spec.TeamYGap+spec.MaxPlayers*spec.RowHeight+spec.Chrome.BoardBottom > canvas.Height {
		return fmt.Errorf("outro second team exceeds canvas height")
	}
	tableWidth := canvas.Width - 2*spec.Margin
	if tableWidth <= spec.NameWidth {
		return fmt.Errorf("outro table width %d is too small", tableWidth)
	}
	if spec.POV.Rect.X < 0 || spec.POV.Rect.X+spec.POV.Rect.Width > spec.NameWidth || spec.POV.Rect.Y < 0 || spec.POV.Rect.Y+spec.POV.Rect.Height > spec.RowHeight {
		return fmt.Errorf("outro POV badge exceeds name cell")
	}
	if err := validateBadge("outro pov", spec.POV); err != nil {
		return err
	}
	headerHeight := spec.RowY - spec.HeaderY
	if err := validateTextAnchor("outro header", spec.Header, tableWidth, headerHeight); err != nil {
		return err
	}
	if err := validateBadge("outro score", spec.Score); err != nil {
		return err
	}
	if spec.Score.Rect.X < 0 || spec.Score.Rect.Y < 0 || spec.Score.Rect.X+spec.Score.Rect.Width > spec.Header.X || spec.Score.Rect.Y+spec.Score.Rect.Height > headerHeight {
		return fmt.Errorf("outro score chip must sit left of the team name inside the header")
	}
	if err := validateTextAnchor("outro name", spec.Name, spec.NameWidth, spec.RowHeight); err != nil {
		return err
	}
	if spec.POVNameX < spec.POV.Rect.X+spec.POV.Rect.Width || spec.POVNameX >= spec.NameWidth {
		return fmt.Errorf("outro POV name x = %d must clear the POV badge", spec.POVNameX)
	}
	for name, text := range map[string]rightTextSpec{"team average": spec.TeamAverage, "team average label": spec.TeamAverageLabel} {
		if err := validateRightText("outro "+name, text, tableWidth, headerHeight); err != nil {
			return err
		}
	}
	if spec.MapLabel.Y < 0 || spec.MapLabel.Y >= canvas.Height || spec.MapLabel.FontSize <= 0 {
		return fmt.Errorf("outro map label exceeds the canvas")
	}
	if spec.ColumnLabelsY < 0 || spec.ColumnLabelsY >= headerHeight || spec.ColumnLabelSize <= 0 || spec.StatSize <= 0 {
		return fmt.Errorf("outro typography exceeds its slot")
	}
	allowed := map[string]bool{
		ColLevel: true, ColELO: true, ColRating: true, ColKDA: true, ColADR: true, ColKR: true,
		ColHSPct: true, Col5K: true, Col4K: true, Col3K: true, Col2K: true, ColMVP: true,
	}
	seen := make(map[string]bool, len(spec.Columns))
	previousEnd := spec.NameWidth
	for _, column := range spec.Columns {
		if !allowed[column.ID] || seen[column.ID] {
			return fmt.Errorf("outro column %q is invalid or duplicated", column.ID)
		}
		if strings.TrimSpace(column.Label) == "" || column.Width <= 0 || column.X < previousEnd || column.X+column.Width > tableWidth {
			return fmt.Errorf("outro column %q exceeds or overlaps the table", column.ID)
		}
		seen[column.ID] = true
		previousEnd = column.X + column.Width
	}
	if len(seen) != len(allowed) {
		return fmt.Errorf("outro columns = %d, want %d", len(seen), len(allowed))
	}
	c := spec.Chrome
	if c.AccentWidth <= 0 || c.BoardRadius < 0 || c.RowBottomGap < 0 || c.NameRightGap < 0 || c.SideStripeWidth <= 0 || c.DividerHeight <= 0 ||
		spec.Margin+c.BoardLeft < 0 || canvas.Width-spec.Margin+c.BoardRight > canvas.Width ||
		spec.HeaderY+c.BoardTop < 0 || spec.RowY+spec.TeamYGap+spec.MaxPlayers*spec.RowHeight+c.BoardBottom > canvas.Height {
		return fmt.Errorf("outro chrome exceeds the canvas")
	}
	return nil
}

func validateTextAnchor(name string, anchor textAnchorSpec, maxX, maxY int) error {
	if anchor.X < 0 || anchor.X >= maxX || anchor.Y < 0 || anchor.Y >= maxY || anchor.FontSize <= 0 {
		return fmt.Errorf("%s anchor exceeds its slot", name)
	}
	return nil
}

func validateBadge(name string, badge badgeSpec) error {
	if badge.FontSize <= 0 || badge.FontSize > badge.Rect.Height {
		return fmt.Errorf("%s text exceeds its badge", name)
	}
	return nil
}

func validateRightText(name string, text rightTextSpec, maxX, maxY int) error {
	if text.Right < 0 || text.Right >= maxX || text.Y < 0 || text.Y >= maxY || text.FontSize <= 0 {
		return fmt.Errorf("%s anchor exceeds its slot", name)
	}
	return nil
}

func rectsOverlap(a, b rectSpec) bool {
	return a.X < b.X+b.Width && a.X+a.Width > b.X && a.Y < b.Y+b.Height && a.Y+a.Height > b.Y
}

func validOverlayColor(value string) bool {
	parts := strings.SplitN(value, "@", 2)
	hex := strings.TrimPrefix(parts[0], "0x")
	if len(hex) != 6 {
		return false
	}
	if _, err := strconv.ParseUint(hex, 16, 24); err != nil {
		return false
	}
	if len(parts) == 1 {
		return true
	}
	alpha, err := strconv.ParseFloat(parts[1], 64)
	return err == nil && alpha >= 0 && alpha <= 1
}

func (p paletteSpec) colors() map[string]string {
	return map[string]string{
		"text": p.Text, "muted_text": p.MutedText, "stat_text": p.StatText,
		"stat_positive": p.StatPositive, "stat_negative": p.StatNegative, "target_text": p.TargetText,
		"pov_fill": p.POVFill, "pov_text": p.POVText, "rank_fill": p.RankFill, "rank_text": p.RankText,
		"level_10_fill": p.Level10Fill, "level_8_fill": p.Level8Fill, "level_4_fill": p.Level4Fill,
		"level_2_fill": p.Level2Fill, "level_default_fill": p.LevelDefaultFill, "level_ring_fill": p.LevelRingFill,
		"intro_panel_fill": p.IntroPanelFill, "intro_panel_border": p.IntroPanelBorder,
		"intro_card_fill": p.IntroCardFill, "intro_card_border": p.IntroCardBorder,
		"intro_card_divider": p.IntroCardDivider, "intro_stats_band": p.IntroStatsBand,
		"intro_texture": p.IntroTexture, "avatar_fill": p.AvatarFill,
		"outro_backdrop": p.OutroBackdrop, "outro_board_fill": p.OutroBoardFill,
		"outro_board_border": p.OutroBoardBorder, "outro_label_fill": p.OutroLabelFill,
		"outro_row_even": p.OutroRowEven, "outro_row_odd": p.OutroRowOdd,
		"outro_ct_name": p.OutroCTName, "outro_ct_stripe": p.OutroCTStripe,
		"outro_t_name": p.OutroTName, "outro_t_stripe": p.OutroTStripe,
		"outro_divider": p.OutroDivider,
	}
}
