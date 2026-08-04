// Package keydropbanner ships the KeyDrop sponsor banner plates and the
// shared validation/materialize helpers used by stream clips and demo reels.
package keydropbanner

import (
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

const (
	// StyleOperator is the tactical KeyDrop plate (operator + logo mark).
	StyleOperator = "operator"
	// StyleClassic is the promo plate with the gift mark and character art.
	StyleClassic = "classic"

	DefaultStyle = StyleOperator
	DefaultCode  = "ZACKCSGO"

	// Version bumps force re-materialization when embedded plates change.
	Version = "v4"

	maxCodeRunes = 16
)

//go:embed style-operator.png
var styleOperatorPNG []byte

//go:embed style-classic.png
var styleClassicPNG []byte

// codePattern accepts the short sponsor codes streamers type by hand.
// Letters, digits, and a few separators; no spaces or control chars.
var codePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,15}$`)

// Style describes one selectable plate and the relative cover/text geometry
// used to replace the baked-in code at render time. Coordinates are fractions
// of the plate's own width/height so they survive any output scale.
type Style struct {
	ID           string
	FileName     string
	SHA256       string
	Data         []byte
	Width        int
	Height       int
	CoverX       float64 // left of baked-code cover, 0..1 of plate width
	CoverY       float64
	CoverW       float64
	CoverH       float64
	CoverColor   string // 0xRRGGBB for FFmpeg drawbox
	TextCenterY  float64
	FontSizeFrac float64 // fontsize ≈ plate_height * FontSizeFrac after scale
}

var styles = map[string]Style{
	StyleOperator: {
		ID:           StyleOperator,
		FileName:     "style-operator.png",
		SHA256:       "d9b53431aaf6019e9b65a588704d99becc0442dff9bda074df42df0dcfd26452",
		Data:         styleOperatorPNG,
		Width:        1080,
		Height:       722,
		CoverX:       0.28,
		CoverY:       0.442,
		CoverW:       0.62,
		CoverH:       0.148,
		CoverColor:   "0x0c0c0e",
		TextCenterY:  0.516,
		FontSizeFrac: 0.095,
	},
	StyleClassic: {
		ID:           StyleClassic,
		FileName:     "style-classic.png",
		SHA256:       "4e95761419634e4ff90ef1f738f6029a02e75264c61f9a2228b94c67c5d0224e",
		Data:         styleClassicPNG,
		Width:        1080,
		Height:       475,
		CoverX:       0.10,
		CoverY:       0.50,
		CoverW:       0.80,
		CoverH:       0.30,
		CoverColor:   "0x1c120c",
		TextCenterY:  0.65,
		FontSizeFrac: 0.12,
	},
}

// Styles returns the selectable styles in stable UI order.
func Styles() []Style {
	return []Style{styles[StyleOperator], styles[StyleClassic]}
}

// Lookup returns a style by id.
func Lookup(id string) (Style, bool) {
	s, ok := styles[NormalizeStyle(id)]
	return s, ok
}

// NormalizeStyle trims and lowercases; empty stays empty (banner off).
func NormalizeStyle(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

// NormalizeCode trims and uppercases the sponsor code.
func NormalizeCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// ValidateStyle reports whether style is empty (off) or a known plate.
func ValidateStyle(id string) error {
	id = NormalizeStyle(id)
	if id == "" {
		return nil
	}
	if _, ok := styles[id]; !ok {
		return fmt.Errorf("unknown keydrop banner style %q", id)
	}
	return nil
}

// ValidateCode reports whether code is empty (default at render) or a valid
// sponsor code.
func ValidateCode(code string) error {
	code = NormalizeCode(code)
	if code == "" {
		return nil
	}
	if len([]rune(code)) > maxCodeRunes {
		return fmt.Errorf("keydrop code must be at most %d characters", maxCodeRunes)
	}
	if !codePattern.MatchString(code) {
		return fmt.Errorf("keydrop code must use 1-16 letters, numbers, underscores, or hyphens")
	}
	return nil
}

// EffectiveCode returns the code burned into the render, applying the default
// when the plan leaves it blank.
func EffectiveCode(code string) string {
	code = NormalizeCode(code)
	if code == "" {
		return DefaultCode
	}
	return code
}

// DisplayLabel is the full lower-third string drawn on the plate.
func DisplayLabel(code string) string {
	return "CODE: " + EffectiveCode(code)
}

var materializeMu sync.Mutex

// Materialize writes the embedded plate for style into the user cache and
// returns its absolute path. Empty style is an error.
func Materialize(styleID string) (string, error) {
	style, ok := Lookup(styleID)
	if !ok {
		return "", fmt.Errorf("unknown keydrop banner style %q", styleID)
	}
	root, err := os.UserCacheDir()
	if err != nil || root == "" {
		root = os.TempDir()
	}
	if root == "" {
		return "", fmt.Errorf("materialize keydrop banner: no user cache or temp directory available")
	}
	dir := filepath.Join(root, "TickCut", "keydrop-banner", Version)
	return materializeAt(dir, style)
}

func materializeAt(dir string, style Style) (string, error) {
	materializeMu.Lock()
	defer materializeMu.Unlock()

	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("materialize keydrop banner: create cache directory: %w", err)
	}
	target := filepath.Join(dir, style.FileName)
	match, err := fileMatchesSHA(target, style.SHA256)
	if err == nil && match {
		return target, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("materialize keydrop banner: inspect cached plate: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".keydrop-*.png")
	if err != nil {
		return "", fmt.Errorf("materialize keydrop banner: create temporary plate: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o644); err != nil {
		return "", fmt.Errorf("materialize keydrop banner: set plate permissions: %w", errors.Join(err, tmp.Close()))
	}
	if _, err := tmp.Write(style.Data); err != nil {
		return "", fmt.Errorf("materialize keydrop banner: write plate: %w", errors.Join(err, tmp.Close()))
	}
	if err := tmp.Sync(); err != nil {
		return "", fmt.Errorf("materialize keydrop banner: sync plate: %w", errors.Join(err, tmp.Close()))
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("materialize keydrop banner: close plate: %w", err)
	}
	match, err = fileMatchesSHA(target, style.SHA256)
	if err == nil && match {
		return target, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("materialize keydrop banner: recheck cached plate: %w", err)
	}
	if err := os.Rename(tmpPath, target); err != nil {
		match, verifyErr := fileMatchesSHA(target, style.SHA256)
		if verifyErr == nil && match {
			return target, nil
		}
		return "", fmt.Errorf("materialize keydrop banner: install cached plate: %w", err)
	}
	return target, nil
}

func fileMatchesSHA(path, wantSHA string) (bool, error) {
	// #nosec G304 -- path is under TickCut's fixed cache root and a bundled filename.
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	return fmt.Sprintf("%x", h.Sum(nil)) == wantSHA, nil
}

// TargetWidth picks the plate width on the output canvas so the banner reads
// as a compact lower-third without burying gameplay.
func TargetWidth(outputWidth int) int {
	if outputWidth <= 0 {
		return 594
	}
	// ~55% of frame width: full character art stays readable while leaving most
	// of the 9:16 gameplay clear (the previous ~86% plate dominated the frame).
	w := int(float64(outputWidth) * 0.55)
	if w < 280 {
		w = 280
	}
	if w > 720 {
		w = 720
	}
	return w
}

// DefaultPositionY is the vertical center for the KeyDrop plate when the plan
// does not pin one. Bottom-safe so it sits clear of typical HUD.
const DefaultPositionY = 0.86
