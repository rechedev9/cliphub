package renderplan

import (
	"fmt"
	"strings"
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
	TransitionCut          = "cut"
	TransitionFlash        = "flash"
	TransitionWhip         = "whip"
	TransitionDip          = "dip"
	CoverStrategyGenerated = "generated-gameplay"
	CoverStrategyNone      = "no-cover"
)

// EditRequest is the user-selected editing contract captured from the UI for
// one render. Workers snapshot it into the edit document so a render is
// reproducible without knowing which screen launched it.
type EditRequest struct {
	Format          string `json:"format"`
	KillEffect      string `json:"killEffect"`
	Transition      string `json:"transition"`
	Intro           bool   `json:"intro"`
	Outro           bool   `json:"outro"`
	HookText        bool   `json:"hook_text"`
	KillCounter     bool   `json:"kill_counter"`
	CoverStrategy   string `json:"cover_strategy"`
	CoverFirstFrame bool   `json:"cover_first_frame"`
	// IntroText and OutroText customize the intro/outro overlay card text.
	// Setting either does not enable its bookend; Intro/Outro remain the
	// switch, so a render can carry custom text while the bookend stays off.
	IntroText string `json:"intro_text,omitempty"`
	OutroText string `json:"outro_text,omitempty"`
	// KeyDropStyle enables the sponsor plate when set to "operator" or
	// "classic". Empty leaves the reel without a KeyDrop banner.
	KeyDropStyle string `json:"keydrop_style,omitempty"`
	// KeyDropCode is the sponsor code drawn on the plate; empty defaults to
	// ZACKCSGO when KeyDropStyle is set.
	KeyDropCode string `json:"keydrop_code,omitempty"`
	// KeyDropPositionY is the plate center as a fraction of output height.
	// Nil uses the shared default (bottom-safe ~0.86).
	KeyDropPositionY *float64 `json:"keydrop_position_y,omitempty"`
}

func DefaultEditRequest() EditRequest {
	return EditRequest{
		Format:        FormatShort9x16,
		KillEffect:    KillEffectPunchIn,
		Transition:    TransitionFlash,
		CoverStrategy: CoverStrategyGenerated,
	}
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
	req.KeyDropStyle = strings.ToLower(strings.TrimSpace(req.KeyDropStyle))
	req.KeyDropCode = strings.ToUpper(strings.TrimSpace(req.KeyDropCode))
	return req
}

func (r EditRequest) Validate() error {
	switch r.Format {
	case FormatShort9x16, FormatLandscape16x9:
	default:
		return fmt.Errorf("unknown render format %q", r.Format)
	}
	switch r.KillEffect {
	case KillEffectClean, KillEffectPunchIn, KillEffectVelocity, KillEffectFreezeFlash:
	default:
		return fmt.Errorf("unknown kill effect %q", r.KillEffect)
	}
	switch r.Transition {
	case TransitionCut, TransitionFlash, TransitionWhip, TransitionDip:
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
	style := strings.ToLower(strings.TrimSpace(r.KeyDropStyle))
	if style != "" && style != "operator" && style != "classic" {
		return fmt.Errorf("unknown keydrop style %q", r.KeyDropStyle)
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
	return nil
}
