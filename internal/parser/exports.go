package parser

import (
	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
)

// The demo vocabulary below is shared with other packages that read demos, so
// the product never grows a second weapon-name or side-label table. The
// unexported originals stay where they are used, and these wrappers are the
// only supported entry points for callers outside this package.

// WeaponName returns the canonical lowercase entity name for an equipment
// ("ak47", "m4a1_silencer", "usp_silencer"), matching the vocabulary the rules
// JSON and every downstream filter already use. CS2 demos leave
// Equipment.OriginalString empty, so the mapping is not optional.
func WeaponName(w *common.Equipment) string { return weaponName(w) }

// TeamLabel renders a demoinfocs team as "CT", "T", "SPEC", or "" for unknown.
func TeamLabel(t common.Team) string { return teamLabel(t) }

// PlayerPosition returns a player's world position, or the zero vector when the
// player is missing from the demo's entity state.
func PlayerPosition(p *common.Player) [3]float64 { return playerPosition(p) }

// ParseToEnd drives a parser to completion, treating a truncated demo as a
// complete parse: GOTV recordings routinely end mid-frame and the data up to
// that point is still valid.
func ParseToEnd(p demoinfocs.Parser) error { return parseToEnd(p) }
