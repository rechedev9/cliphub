package keydropbanner

import (
	"fmt"
	"strings"
)

const (
	// FamilyKeyDrop is the original KeyDrop sponsor catalog.
	FamilyKeyDrop = "KEYDROP"
	// FamilyCSGOSkins is the CSGOSkins affiliate catalog. Its plates are
	// distinct files; they must never resolve to a KeyDrop PNG.
	FamilyCSGOSkins = "CSGOSKINS"
)

// NormalizeFamily trims and uppercases; empty stays empty (legacy plans).
func NormalizeFamily(family string) string {
	return strings.ToUpper(strings.TrimSpace(family))
}

// EffectiveFamily returns the family a plan actually uses. An empty family
// with a style still means KEYDROP so persisted pre-family plans keep working.
func EffectiveFamily(family, styleID string) string {
	family = NormalizeFamily(family)
	if family != "" {
		return family
	}
	if NormalizeStyle(styleID) != "" {
		return FamilyKeyDrop
	}
	return ""
}

// ValidateFamily reports whether family is empty or a shipped catalog id.
func ValidateFamily(family string) error {
	family = NormalizeFamily(family)
	if family == "" {
		return nil
	}
	if !knownFamily(family) {
		return fmt.Errorf("unknown affiliate family %q", family)
	}
	return nil
}

// FamilyLabel is the brief/UI name. Empty family has no label.
func FamilyLabel(family string) string {
	switch NormalizeFamily(family) {
	case FamilyCSGOSkins:
		return "CSGOSkins"
	case FamilyKeyDrop:
		return "KeyDrop"
	default:
		return ""
	}
}

// KnownFamilies is the shipped affiliate set, stable for Studio chips.
func KnownFamilies() []string {
	return []string{FamilyKeyDrop, FamilyCSGOSkins}
}

func knownFamily(family string) bool {
	switch NormalizeFamily(family) {
	case FamilyKeyDrop, FamilyCSGOSkins:
		return true
	default:
		return false
	}
}

func catalogKey(family, styleID string) string {
	return NormalizeFamily(family) + "/" + NormalizeStyle(styleID)
}
