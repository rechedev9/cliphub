package renderplan

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rechedev9/cliphub/internal/brandkit"
	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/keydropbanner"
	"github.com/rechedev9/cliphub/internal/overlaylang"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

// maxBookendTextLength caps the intro/outro custom text length so an overlay
// card never has to reflow or run off the safe frame width.
const maxBookendTextLength = 80

const (
	FormatShort9x16          = "short-9x16"
	FormatLandscape16x9      = "landscape-16x9"
	KillEffectClean          = "clean"
	KillEffectPunchIn        = "punch-in"
	KillEffectVelocity       = "velocity"
	KillEffectFreezeFlash    = "freeze-flash"
	KillEffectShake          = "shake"
	KillEffectGlitch         = "glitch"
	TransitionCut            = "cut"
	TransitionFlash          = "flash"
	TransitionWhip           = "whip"
	TransitionDip            = "dip"
	TransitionGlitch         = "glitch"
	TransitionZoomWhip       = "zoom-whip"
	CoverStrategyGenerated   = "generated-gameplay"
	CoverStrategyNone        = "no-cover"
	DemoSourcePremier        = "premier"
	DemoSourceProfessional   = "professional"
	DemoSourceFACEIT         = "faceit"
	OverlayThemeFaceitOrange = "faceit-orange"
	OverlayThemeNeonViolet   = "neon-violet"
	OverlayThemeHUD          = recapplan.OverlayThemeHUD
	// DefaultRecapVoiceVolume is the team-comms gain of the legacy recap chain,
	// which goes through the Shorts editor and is capped at 1.0.
	DefaultRecapVoiceVolume = 0.85
	// DefaultVictimReplaySpeed is the victim replay speed when none is chosen.
	DefaultVictimReplaySpeed = 0.5
)

// VictimReplaySpeeds are the speeds a victim replay may play at.
var VictimReplaySpeeds = []float64{0.25, DefaultVictimReplaySpeed, 1}

// EditRequest is the user-selected editing contract captured from the UI for
// one render. Workers snapshot it into the edit document so a render is
// reproducible without knowing which screen launched it.
type EditRequest struct {
	FullDemo    *recapplan.Snapshot `json:"full_demo,omitempty"`
	Format      string              `json:"format"`
	KillEffect  string              `json:"killEffect"`
	Transition  string              `json:"transition"`
	Intro       bool                `json:"intro"`
	Outro       bool                `json:"outro"`
	HookText    bool                `json:"hook_text"`
	KillCounter bool                `json:"kill_counter"`
	MatchRecap  bool                `json:"match_recap"`
	VoiceComms  bool                `json:"voice_comms"`
	VoiceVolume *float64            `json:"voice_volume,omitempty"`
	NativeHUD   bool                `json:"native_hud"`
	// VictimReplay replays each play's last kill from the victim, at VictimReplaySpeed.
	VictimReplay bool `json:"victim_replay,omitempty"`
	// VictimReplaySpeed is 0.25, 0.5 or 1; 0 keeps DefaultVictimReplaySpeed.
	VictimReplaySpeed float64 `json:"victim_replay_speed,omitempty"`
	// PlayBadges names each clutch, ace and multikill on screen.
	PlayBadges bool `json:"play_badges,omitempty"`
	// WinProbability shows each side's post-plant round-win chance from the kill plan.
	WinProbability bool `json:"win_probability,omitempty"`
	// PlayerLabels draws teammates' and seen enemies' names and health over the gameplay, and
	// GrenadeTrails the paths of the POV team's grenades; the render reads both from the demo.
	PlayerLabels  bool `json:"player_labels,omitempty"`
	GrenadeTrails bool `json:"grenade_trails,omitempty"`
	// StatsCard ends a 9:16 Short with the player's match stats and, when known, FACEIT data.
	StatsCard bool `json:"stats_card,omitempty"`
	// OpeningCard, EndCard and CoverCard are playercard template ids for a one-player 9:16 Short; empty is no card.
	// The end card replaces the stats card and the cover card lies over the cover frame.
	OpeningCard      string `json:"opening_card,omitempty"`
	EndCard          string `json:"end_card,omitempty"`
	CoverCard        string `json:"cover_card,omitempty"`
	CenteredKillfeed bool   `json:"centered_killfeed,omitempty"`
	// VerticalHUD draws a 9:16 scoreboard and alive counter read from the demo over the Short, with the
	// centered kill feed under it, in the VerticalHUDTheme catalog design (empty is the first one).
	VerticalHUD      bool   `json:"vertical_hud,omitempty"`
	VerticalHUDTheme string `json:"vertical_hud_theme,omitempty"`
	// Radar draws ClipHub's radar in a corner, read from the demo: RadarPosition (empty is the format's
	// corner), RadarSize (empty is medium) and RadarOpacity (0 is radaroverlay.DefaultOpacity).
	Radar               bool     `json:"radar,omitempty"`
	RadarPosition       string   `json:"radar_position,omitempty"`
	RadarSize           string   `json:"radar_size,omitempty"`
	RadarOpacity        float64  `json:"radar_opacity,omitempty"`
	CoverStrategy       string   `json:"cover_strategy"`
	CoverFirstFrame     bool     `json:"cover_first_frame"`
	IntroText           string   `json:"intro_text,omitempty"`
	OutroText           string   `json:"outro_text,omitempty"`
	KeyDropFamily       string   `json:"keydrop_family,omitempty"`
	KeyDropStyle        string   `json:"keydrop_style,omitempty"`
	KeyDropCode         string   `json:"keydrop_code,omitempty"`
	KeyDropPositionY    *float64 `json:"keydrop_position_y,omitempty"`
	KeyDropStartSeconds *float64 `json:"keydrop_start_seconds,omitempty"`
	KeyDropEndSeconds   *float64 `json:"keydrop_end_seconds,omitempty"`
	DemoSource          string   `json:"demo_source,omitempty"`
	OverlayTheme        string   `json:"overlay_theme,omitempty"`
	// DeathReel records the clips of the death plan instead of the kill plan.
	DeathReel bool `json:"death_reel,omitempty"`
	// SeparateShorts renders each chosen play as its own Short instead of one compilation (Auto-Shorts).
	SeparateShorts bool `json:"separate_shorts,omitempty"`
	// PreRollSeconds starts each kill play that long before its first kill (PreRollChoices); 0 keeps the analysed windows.
	PreRollSeconds int `json:"pre_roll_seconds,omitempty"`
	// CaptureHUD records a 16:9 video of plays in one of CaptureHUDs instead of its preset's HUD; empty keeps the preset's.
	CaptureHUD string `json:"capture_hud,omitempty"`
	// PortraitSafeKillfeed records a 16:9 video's kill feed where the 9:16 crop of the Short it comes from keeps it, so the Short's clips are reused.
	PortraitSafeKillfeed bool `json:"portrait_safe_killfeed,omitempty"`
	// BrandWatermark and BrandBanner are copied whole from the brand kit, so a later kit edit never changes this render.
	BrandWatermark  *brandkit.Watermark `json:"brand_watermark,omitempty"`
	BrandBanner     *brandkit.Banner    `json:"brand_banner,omitempty"`
	SubscribeBanner *brandkit.Subscribe `json:"subscribe_banner,omitempty"`
	// Subtitles burns the team comms transcript in the spoken language; SubtitlesHidden are phrases this video leaves out.
	Subtitles       bool     `json:"subtitles,omitempty"`
	SubtitlesHidden []string `json:"subtitles_hidden,omitempty"`

	OverlayLanguage overlaylang.Language `json:"overlay_language,omitempty"`
}

func DefaultEditRequest() EditRequest {
	return EditRequest{
		Format:        FormatShort9x16,
		KillEffect:    KillEffectPunchIn,
		Transition:    TransitionFlash,
		CoverStrategy: CoverStrategyGenerated,
	}
}

// RecapEditRequestWithSource is the Studio Full Demo chain: the locked 16:9
// recap treatment (native HUD, team comms, no Shorts garnish) plus the
// user-selected overlay source (premier / professional / faceit). Empty
// source keeps demo-facts-only overlays.
func RecapEditRequestWithSource(source string) EditRequest {
	voice := DefaultRecapVoiceVolume
	return NormalizeEditRequest(EditRequest{
		Format:        FormatLandscape16x9,
		KillEffect:    KillEffectClean,
		Transition:    TransitionCut,
		MatchRecap:    true,
		VoiceComms:    true,
		VoiceVolume:   &voice,
		NativeHUD:     true,
		CoverStrategy: CoverStrategyGenerated,
		DemoSource:    source,
	})
}

func NormalizeEditRequest(req EditRequest) EditRequest {
	def := DefaultEditRequest()
	if req.Format == "" {
		req.Format = def.Format
	}
	if req.KillEffect == "" {
		req.KillEffect = def.KillEffect
	}
	if req.Transition == "" {
		req.Transition = def.Transition
	}
	if req.CoverStrategy == "" {
		req.CoverStrategy = def.CoverStrategy
	}
	req.IntroText = strings.TrimSpace(req.IntroText)
	req.OutroText = strings.TrimSpace(req.OutroText)
	req.KeyDropFamily = keydropbanner.NormalizeFamily(req.KeyDropFamily)
	req.KeyDropStyle = strings.ToLower(strings.TrimSpace(req.KeyDropStyle))
	req.KeyDropCode = strings.ToUpper(strings.TrimSpace(req.KeyDropCode))
	req.DemoSource = strings.ToLower(strings.TrimSpace(req.DemoSource))
	req.OverlayTheme = strings.ToLower(strings.TrimSpace(req.OverlayTheme))
	if req.KeyDropStyle != "" && req.KeyDropFamily == "" {
		req.KeyDropFamily = keydropbanner.FamilyKeyDrop
	}
	if req.DeathReel {
		// The hook, the counter, the badges, the win chance, the victim replay and the centered kill feed need kill segments.
		req.HookText = false
		req.KillCounter = false
		req.PlayBadges = false
		req.WinProbability = false
		req.VictimReplay = false
		req.CenteredKillfeed = false
		req.VerticalHUD = false
	}
	// The vertical HUD includes the centered kill feed, which keeps CS2's notices out of the 9:16 frame.
	req.VerticalHUDTheme = strings.ToLower(strings.TrimSpace(req.VerticalHUDTheme))
	if req.VerticalHUD {
		req.CenteredKillfeed = true
	} else {
		req.VerticalHUDTheme = ""
	}
	// The default speed is stored as unset so equal choices give equal requests.
	if !req.VictimReplay || req.VictimReplaySpeed == DefaultVictimReplaySpeed {
		req.VictimReplaySpeed = 0
	}
	return normalizeSubtitles(normalizePlayerCards(normalizePortraitSafeKillfeed(normalizeRadar(normalizeCaptureHUD(normalizePreRoll(req))))))
}

func (r EditRequest) Validate() error {
	if r.FullDemo != nil {
		if err := r.FullDemo.Validate(); err != nil {
			return err
		}
	}
	return r.validateFields()
}

// ValidateRendered checks the edit document of an admitted render (queued,
// running or finished). Its Full Demo snapshot is history: the planner of its
// day admitted it and it still decodes under the strict wire contract, so a
// later planner change (a retired option, a new timeline rule) must not make
// the render unreadable. Its record and approval still hold; only admission
// re-checks plan freshness.
func (r EditRequest) ValidateRendered() error {
	if r.FullDemo != nil {
		if err := r.FullDemo.ValidateHistory(); err != nil {
			return err
		}
	}
	return r.validateFields()
}

func (r EditRequest) validateFields() error {
	if r.FullDemo != nil {
		o := r.FullDemo.Document.Options
		if r.Format != FormatLandscape16x9 || r.KillEffect != KillEffectClean || r.Transition != TransitionCut ||
			!r.MatchRecap || !r.NativeHUD || r.VictimReplay || r.VictimReplaySpeed != 0 || r.PlayBadges || r.WinProbability || r.PlayerLabels || r.GrenadeTrails || r.StatsCard || r.Intro || r.Outro || r.HookText || r.KillCounter || r.CenteredKillfeed || r.VerticalHUD || r.VerticalHUDTheme != "" || r.Radar || r.RadarPosition != "" || r.RadarSize != "" || r.RadarOpacity != 0 || r.CoverFirstFrame || r.OverlayLanguage != overlaylang.Spanish ||
			r.IntroText != "" || r.OutroText != "" || r.KeyDropFamily != "" || r.KeyDropStyle != "" || r.KeyDropCode != "" ||
			r.KeyDropPositionY != nil || r.KeyDropStartSeconds != nil || r.KeyDropEndSeconds != nil ||
			r.VoiceComms != o.Audio.Voice.Enabled || r.VoiceVolume == nil || *r.VoiceVolume != o.Audio.Voice.Gain ||
			r.CoverStrategy != o.Outputs.CoverPolicy || r.DemoSource != o.OverlaySource() || r.OverlayTheme != o.Overlays.Theme {
			return fmt.Errorf("full demo edit fields contradict the approved document")
		}
	}
	if err := r.validateBrand(); err != nil {
		return err
	}
	if _, err := overlaylang.Parse(string(r.OverlayLanguage)); err != nil {
		return err
	}
	if r.DeathReel && (r.MatchRecap || r.FullDemo != nil) {
		return fmt.Errorf("a death reel is not a match recap")
	}
	if r.SeparateShorts && (r.DeathReel || r.MatchRecap || r.FullDemo != nil) {
		return fmt.Errorf("separate shorts are only for a reel of plays")
	}
	if (r.PlayerLabels || r.GrenadeTrails) && r.MatchRecap && r.FullDemo == nil {
		return fmt.Errorf("player labels and grenade trails are for plays, not a match recap")
	}
	switch r.Format {
	case FormatShort9x16, FormatLandscape16x9:
	default:
		return fmt.Errorf("unknown render format %q", r.Format)
	}
	if err := r.validateVerticalHUD(); err != nil {
		return err
	}
	if err := r.validateRadar(); err != nil {
		return err
	}
	if err := r.validateSubtitles(); err != nil {
		return err
	}
	if err := r.validatePlayerCards(); err != nil {
		return err
	}
	if r.CenteredKillfeed && r.Format != FormatShort9x16 {
		return fmt.Errorf("centered killfeed needs the %s format", FormatShort9x16)
	}
	if r.StatsCard && r.Format != FormatShort9x16 {
		return fmt.Errorf("the stats card needs the %s format", FormatShort9x16)
	}
	if err := ValidatePreRoll(r.PreRollSeconds); err != nil {
		return err
	}
	if err := r.validateCaptureHUD(); err != nil {
		return err
	}
	if err := r.validatePortraitSafeKillfeed(); err != nil {
		return err
	}
	if r.VictimReplaySpeed != 0 && !slices.Contains(VictimReplaySpeeds, r.VictimReplaySpeed) {
		return fmt.Errorf("victim replay speed must be 0.25, 0.5 or 1")
	}
	switch r.KillEffect {
	case KillEffectClean, KillEffectPunchIn, KillEffectVelocity, KillEffectFreezeFlash, KillEffectShake, KillEffectGlitch:
	default:
		return fmt.Errorf("unknown kill effect %q", r.KillEffect)
	}
	switch r.Transition {
	case TransitionCut, TransitionFlash, TransitionWhip, TransitionDip, TransitionGlitch, TransitionZoomWhip:
	default:
		return fmt.Errorf("unknown transition %q", r.Transition)
	}
	switch r.CoverStrategy {
	case "", CoverStrategyGenerated, CoverStrategyNone:
	default:
		return fmt.Errorf("unknown cover strategy %q", r.CoverStrategy)
	}
	if len(strings.TrimSpace(r.IntroText)) > maxBookendTextLength {
		return fmt.Errorf("intro text exceeds %d characters", maxBookendTextLength)
	}
	if len(strings.TrimSpace(r.OutroText)) > maxBookendTextLength {
		return fmt.Errorf("outro text exceeds %d characters", maxBookendTextLength)
	}
	if err := keydropbanner.ValidateFamily(r.KeyDropFamily); err != nil {
		return err
	}
	if err := keydropbanner.ValidateStyle(r.KeyDropFamily, r.KeyDropStyle); err != nil {
		return err
	}
	code := strings.ToUpper(strings.TrimSpace(r.KeyDropCode))
	if code != "" {
		if len([]rune(code)) > 16 {
			return fmt.Errorf("keydrop code must be at most 16 characters")
		}
		for _, r := range code {
			if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
				continue
			}
			return fmt.Errorf("keydrop code must use letters, numbers, underscores, or hyphens")
		}
		if code[0] == '_' || code[0] == '-' {
			return fmt.Errorf("keydrop code must start with a letter or number")
		}
	}
	if y := r.KeyDropPositionY; y != nil {
		if *y < 0.025 || *y > 0.975 {
			return fmt.Errorf("keydrop position_y must be between 0.025 and 0.975")
		}
	}
	if s := r.KeyDropStartSeconds; s != nil && *s < 0 {
		return fmt.Errorf("keydrop start_seconds must be >= 0")
	}
	if e := r.KeyDropEndSeconds; e != nil && *e <= 0 {
		return fmt.Errorf("keydrop end_seconds must be > 0")
	}
	if r.KeyDropStartSeconds != nil && r.KeyDropEndSeconds != nil && *r.KeyDropEndSeconds <= *r.KeyDropStartSeconds {
		return fmt.Errorf("keydrop end_seconds must be greater than start_seconds")
	}
	voiceMax := 1.0
	if r.FullDemo != nil {
		voiceMax = 2
	}
	if v := r.VoiceVolume; v != nil && (*v < 0 || *v > voiceMax) {
		return fmt.Errorf("voice volume must be between 0 and %g", voiceMax)
	}
	switch r.DemoSource {
	case "", DemoSourcePremier, DemoSourceProfessional, DemoSourceFACEIT:
	default:
		return fmt.Errorf("unknown demo source %q", r.DemoSource)
	}
	switch r.OverlayTheme {
	case "", OverlayThemeFaceitOrange, OverlayThemeNeonViolet:
	case OverlayThemeHUD:
		if r.FullDemo == nil {
			return fmt.Errorf("the HUD overlay design is only for a Full Demo")
		}
	default:
		return fmt.Errorf("unknown overlay theme %q", r.OverlayTheme)
	}
	return nil
}

// FullDemoEditRequest projects the approved document onto the existing render
// contract. The document remains canonical for capture and editorial choices.
func FullDemoEditRequest(snapshot recapplan.Snapshot) EditRequest {
	o := snapshot.Document.Options
	voice := o.Audio.Voice.Gain
	return EditRequest{
		FullDemo: &snapshot, Format: FormatLandscape16x9, KillEffect: KillEffectClean,
		Transition: TransitionCut, MatchRecap: true, NativeHUD: true,
		VoiceComms: o.Audio.Voice.Enabled, VoiceVolume: &voice,
		CoverStrategy: o.Outputs.CoverPolicy, DemoSource: o.OverlaySource(), OverlayTheme: o.Overlays.Theme,
	}
}

func (r EditRequest) UsesFACEITOverlay() bool {
	if r.FullDemo != nil && r.FullDemo.Document.Options.Overlays.Mode == "screenshots" {
		return false
	}
	return r.DemoSource == DemoSourceFACEIT
}

// validateBrand keeps the kit off a Full Demo, whose approved plan carries it, and off a Short with the affiliate plate in the same place.
func (r EditRequest) validateBrand() error {
	if r.BrandWatermark == nil && r.BrandBanner == nil && r.SubscribeBanner == nil {
		return nil
	}
	if r.FullDemo != nil || r.MatchRecap {
		return fmt.Errorf("a Full Demo takes the brand kit from its approved plan")
	}
	if r.BrandWatermark != nil {
		if err := r.BrandWatermark.Validate(); err != nil {
			return err
		}
	}
	if r.BrandBanner != nil {
		if err := r.BrandBanner.Validate(); err != nil {
			return err
		}
		if r.KeyDropStyle != "" {
			return fmt.Errorf("the sponsor banner and the affiliate banner cannot share a Short")
		}
	}
	if r.SubscribeBanner != nil {
		return r.SubscribeBanner.Validate()
	}
	return nil
}

// validateVerticalHUD keeps the 9:16 HUD on a Short of plays whose capture has no CS2 HUD.
func (r EditRequest) validateVerticalHUD() error {
	if !r.VerticalHUD {
		if r.VerticalHUDTheme != "" {
			return fmt.Errorf("vertical_hud_theme needs vertical_hud")
		}
		return nil
	}
	switch {
	case r.Format != FormatShort9x16:
		return fmt.Errorf("the vertical HUD needs the %s format", FormatShort9x16)
	case r.MatchRecap || r.FullDemo != nil || r.DeathReel:
		return fmt.Errorf("the vertical HUD is for a Short of plays")
	case r.NativeHUD:
		return fmt.Errorf("the vertical HUD replaces the CS2 HUD; turn native_hud off")
	case !r.CenteredKillfeed:
		return fmt.Errorf("the vertical HUD draws the centered kill feed")
	}
	if r.VerticalHUDTheme != "" {
		return customhud.ValidateTheme(r.VerticalHUDTheme)
	}
	return nil
}
