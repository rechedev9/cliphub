package customhud

import (
	"fmt"
	"strconv"

	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
)

// Shared with the editor's portrait composition. Contain-fit preserves PNG
// transparency and never crops the person's face or stretches their image.
const PortraitX, PortraitY, PortraitWidth, PortraitHeight = 654, 898, 116, 116

// CS2 records reserve ammo as whole magazines, not rounds. Beyond this many,
// a count reads better than a row of pips.
const maxMagazinePips = 6

func (s *scene) focusCard(p Player) {
	t := s.r.Theme
	x, y, w, h := t.FocusX, t.FocusY, t.FocusWidth, 120
	// Gold is the design's frame; only the side letter carries the team color.
	gold, side := t.T, t.CT
	if p.Side == "T" {
		side = t.T
	}
	if p.Known && !p.Alive {
		gold, side = t.Muted, t.Muted
	}
	s.path("focus/shadow", shapePath("round", x, y+4, w, h), "000000", 0)
	s.fade(.25)
	s.path("focus/border", shapePath("round", x, y, w, h), gold, 1)
	s.path("focus/plate", shapePath("round", x+2, y+2, w-4, h-4), t.Background, 2)
	s.fade(t.Opacity)
	s.rect("focus/divider", x+152, y+18, 1, h-40, t.Surface, 3)
	if !s.r.Portrait {
		s.typeText("focus/side", p.Side, x+76, y+56, 44, 116, 700, side, "center")
	}

	info := x + 172
	s.typeText("focus/name", p.Name, info, y+32, 30, 270, 700, t.Text, "left")
	kda, hp, armor, money := "— / — / —", "—", "—", "$ —"
	if p.Known {
		kda = fmt.Sprintf("%d / %d / %d", p.Kills, p.Assists, p.Deaths)
		hp, armor = strconv.Itoa(p.Health), strconv.Itoa(p.Armor)
		if p.Money >= 0 {
			money = fmt.Sprintf("$%d", p.Money)
		}
	}
	const kdaLabel = "K / A / D"
	s.typeText("focus/kd-label", kdaLabel, info, y+64, 13, 70, 600, t.Muted, "left")
	s.typeText("focus/kd", kda, info+measureText(kdaLabel, 13, textFace(kdaLabel, 600))+10, y+64, 20, 180, 700, t.Text, "left")
	healthIcon := "status/icon_health_default"
	if p.Known && !p.Alive {
		healthIcon = "status/icon_skull_default"
	}
	s.icon("focus/health-icon", healthIcon, info, y+84, 20, 18, "63DC96", false)
	s.typeText("focus/health", hp, info+26, y+93, 20, 46, 700, t.Text, "left")
	s.icon("focus/armor-icon", "status/icon_armor_full_default", info+82, y+84, 18, 18, "85BBDD", false)
	s.typeText("focus/armor", armor, info+106, y+93, 20, 46, 700, t.Text, "left")
	s.typeText("focus/money", money, info+168, y+93, 20, 100, 700, t.T, "left")
	s.rect("focus/track", x+18, y+h-11, w-36, 4, t.Surface, 4)
	if p.Known && p.Alive && p.Health > 0 {
		s.rect("focus/bar", x+18, y+h-11, (w-36)*min(100, p.Health)/100, 4, "63DC96", 5)
	}

	// Without a magazine the weapon centers in the space the ammo row would use.
	right, armed := x+w-18, p.Known && p.Alive && p.Ammo >= 0
	weaponMid := y + 54
	if armed {
		weaponMid = y + 37
	}
	switch {
	case p.Known && !p.Alive:
		s.typeText("focus/weapon", "ELIMINATED", right, weaponMid, 17, 170, 600, t.Muted, "right")
	case !p.Known || !s.icon("focus/weapon-icon", weaponIcon(p.Weapon), right-126, weaponMid-17, 126, 34, t.Text, false):
		s.typeText("focus/weapon", knownWeapon(p), right, weaponMid, 17, 170, 600, t.Muted, "right")
	}
	if armed {
		s.ammo(p, right, y+83)
	}
	s.movementKeys(p)
}

// Knives, grenades and the bomb have no magazine, so the block disappears
// instead of showing a placeholder that reads like missing data.
func (s *scene) ammo(p Player, right, mid int) {
	t := s.r.Theme
	clipRight := right
	switch {
	case p.Reserve > maxMagazinePips:
		s.typeText("focus/reserve", fmt.Sprintf("×%d", p.Reserve), right, mid+2, 17, 48, 600, t.Muted, "right")
		clipRight -= 52
	case p.Reserve > 0:
		for i := range p.Reserve {
			s.rect(fmt.Sprintf("focus/magazine-%d", i), right-6-i*10, mid-10, 6, 20, t.Muted, 4)
		}
		clipRight -= p.Reserve*10 + 8
	case p.Reserve == 0:
		s.rect("focus/magazine-empty", right-6, mid-10, 6, 20, t.Muted, 4)
		s.fade(.35)
		clipRight -= 18
	}
	s.typeText("focus/ammo", strconv.Itoa(p.Ammo), clipRight, mid, 34, 80, 700, t.Text, "right")
}

func (s *scene) movementKeys(p Player) {
	// Dead, disconnected and unavailable input must never look like a factual
	// all-keys-released state. The widget disappears when its source is unknown.
	if !p.Known || !p.Alive || p.Inactive || p.Movement == nil {
		return
	}
	t := s.r.Theme
	for _, key := range []struct {
		label   string
		x, y, w int
		bit     common.ButtonBitMask
	}{
		{"W", 990, 758, 46, common.ButtonForward},
		{"SHIFT", 840, 810, 90, common.ButtonSpeed},
		{"A", 938, 810, 46, common.ButtonMoveLeft},
		{"S", 990, 810, 46, common.ButtonBack},
		{"D", 1042, 810, 46, common.ButtonMoveRight},
		{"CTRL", 840, 862, 90, common.ButtonDuck},
		{"SPACE", 938, 862, 150, common.ButtonJump},
	} {
		id := "keys/" + key.label
		active := *p.Movement&uint64(key.bit) != 0
		plate, ink, edge, opacity := t.Background, t.Text, t.Muted, .5
		if active {
			plate, ink, edge, opacity = t.T, t.Background, t.T, 1
		}
		s.path(id+"/edge", shapePath("round", key.x, key.y, key.w, 44), edge, 1)
		s.fade(opacity * .65)
		s.path(id+"/plate", shapePath("round", key.x+1, key.y+1, key.w-2, 42), plate, 2)
		s.fade(opacity)
		size := 16
		if len(key.label) == 1 {
			size = 25
		}
		s.typeText(id+"/label", key.label, key.x+key.w/2, key.y+22, size, key.w-8, 700, ink, "center")
	}
}
