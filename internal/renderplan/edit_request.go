package renderplan

import (
	"fmt"
	"strings"

	"github.com/rechedev9/cliphub/internal/keydropbanner"
)

// maxBookendTextLength caps the intro/outro custom text length so an overlay
// card never has to reflow or run off the safe frame width.
const maxBookendTextLength = 80

const (
	FormatShort9x16        = "short-9x16"
	FormatLandscape16x9    = "landscape-16x9"
	KillEffectClean        = "clean"
	KillEffectPunchIn      = "punch-in"
	KillEffectVelocity     = "velocity"
	KillEffectFreezeFlash  = "freeze-flash"
	KillEffectShake        = "shake"
	KillEffectGlitch       = "glitch"
	TransitionCut          = "cut"
	TransitionFlash        = "flash"
	TransitionWhip         = "whip"
	TransitionDip          = "dip"
	TransitionGlitch       = "glitch"
	TransitionZoomWhip     = "zoom-whip"
	CoverStrategyGenerated = "generated-gameplay"
	CoverStrategyNone      = "no-cover"
	DemoSourcePremier      = "premier"
	DemoSourceProfessional = "professional"
	DemoSourceFACEIT       = "faceit"
	OverlayThemeFaceitOrange = "faceit-orange"
	OverlayThemeNeonViolet   = "neon-violet"
	// DefaultRecapVoiceVolume is the locked Full Demo team-comms gain.
	DefaultRecapVoiceVolume = 0.85
)

// EditRequest is the user-selected editing contract captured from the UI for
// one render. Workers snapshot it into the edit document so a render is
// reproducible without knowing which screen launched it.
type EditRequest struct {
	Format              string   `json:"format"`
	KillEffect          string   `json:"killEffect"`
	Transition          string   `json:"transition"`
	Intro               bool     `json:"intro"`
	Outro               bool     `json:"outro"`
	HookText            bool     `json:"hook_text"`
	KillCounter         bool     `json:"kill_counter"`
	MatchRecap          bool     `json:"match_recap"`
	VoiceComms          bool     `json:"voice_comms"`
	VoiceVolume         *float64 `json:"voice_volume,omitempty"`
	NativeHUD           bool     `json:"native_hud"`
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
}

func DefaultEditRequest() EditRequest {
	return EditRequest{
		Format:        FormatShort9x16,
		KillEffect:    KillEffectPunchIn,
		Transition:    TransitionFlash,
		CoverStrategy: CoverStrategyGenerated,
	}
}

// RecapEditRequest is the locked 16:9 Full Demo treatment: native HUD, team
// comms, no Shorts garnish. Studio /record retries do not carry generate
// intent, so the record worker uses this to chain the recap render.
func RecapEditRequest() EditRequest {
	return RecapEditRequestWithSource("")
}

// RecapEditRequestWithSource is the Studio Full Demo chain: locked recap
// treatment plus the user-selected overlay source (premier / professional /
// faceit). Empty source keeps demo-facts-only overlays.
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
	return req
}

func (r EditRequest) Validate() error {
	switch r.Format {
	case FormatShort9x16, FormatLandscape16x9:
	default:
		return fmt.Errorf("unknown render format %q", r.Format)
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
	if v := r.VoiceVolume; v != nil && (*v < 0 || *v > 1) {
		return fmt.Errorf("voice volume must be between 0 and 1")
	}
	switch r.DemoSource {
	case "", DemoSourcePremier, DemoSourceProfessional, DemoSourceFACEIT:
	default:
		return fmt.Errorf("unknown demo source %q", r.DemoSource)
	}
	switch r.OverlayTheme {
	case "", OverlayThemeFaceitOrange, OverlayThemeNeonViolet:
	default:
		return fmt.Errorf("unknown overlay theme %q", r.OverlayTheme)
	}
	return nil
}

func (r EditRequest) UsesFACEITOverlay() bool {
	return r.DemoSource == DemoSourceFACEIT
}
