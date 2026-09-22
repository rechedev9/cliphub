package customhud

import (
	"fmt"
	"strconv"

	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
)

// Shared with the editor's portrait composition. Contain-fit preserves PNG
// transparency and never crops the person's face or stretches their image.
const PortraitX, PortraitY, PortraitWidth, PortraitHeight = 654, 898, 116, 116

func (s *scene) focusCard(p Player) {
	t := s.r.Theme
	x, y, w, h := t.FocusX, t.FocusY, t.FocusWidth, 120
	gold := t.T
	if p.Known && !p.Alive {
		gold = t.Muted
	}
	s.path("focus/shadow", shapePath("round", x, y+4, w, h), "000000", 0)
	s.fade(.25)
	s.path("focus/border", shapePath("round", x, y, w, h), gold, 1)
	s.path("focus/plate", shapePath("round", x+2, y+2, w-4, h-4), t.Background, 2)
	s.fade(t.Opacity)
	s.rect("focus/divider", x+16, y+86, w-32, 1, t.Surface, 3)
	if !s.r.Portrait {
		s.typeText("focus/side", p.Side, x+76, y+36, 38, 100, 700, gold, "center")
		s.typeText("focus/identity-label", "PLAYER", x+76, y+64, 12, 100, 500, t.Muted, "center")
	}
	s.typeText("focus/name", p.Name, x+156, y+27, 32, 294, 700, t.Text, "left")
	kda, hp, armor, money, ammo := "K / A / D  — / — / —", "—", "—", "$ —", "—"
	if p.Known {
		kda = fmt.Sprintf("K / A / D  %d / %d / %d", p.Kills, p.Assists, p.Deaths)
		hp, armor = strconv.Itoa(p.Health), strconv.Itoa(p.Armor)
		if p.Money >= 0 {
			money = fmt.Sprintf("$%d", p.Money)
		}
		if p.Alive && p.Ammo >= 0 {
			reserve := "—"
			if p.Reserve >= 0 {
				reserve = strconv.Itoa(p.Reserve)
			}
			ammo = fmt.Sprintf("%d/%s", p.Ammo, reserve)
		}
	}
	s.typeText("focus/kd", kda, x+156, y+63, 20, 248, 600, t.Muted, "left")
	healthIcon := "status/icon_health_default"
	if p.Known && !p.Alive {
		healthIcon = "status/icon_skull_default"
	}
	s.icon("focus/health-icon", healthIcon, x+18, y+94, 20, 18, "63DC96", false)
	s.text("focus/health", hp, x+44, y+103, 20, 42, t.Text, "left")
	s.icon("focus/armor-icon", "status/icon_armor_full_default", x+88, y+94, 18, 18, "85BBDD", false)
	s.text("focus/armor", armor, x+114, y+103, 20, 42, t.Text, "left")
	s.typeText("focus/money", money, x+280, y+103, 20, 120, 700, gold, "left")
	s.rect("focus/track", x+156, y+99, 104, 5, t.Surface, 4)
	if p.Known && p.Alive && p.Health > 0 {
		s.rect("focus/bar", x+156, y+99, 104*min(100, p.Health)/100, 5, "63DC96", 5)
	}
	weapon := knownWeapon(p)
	if p.Known && !p.Alive {
		weapon = "ELIMINATED"
	}
	if !p.Known || !p.Alive || !s.icon("focus/weapon-icon", weaponIcon(p.Weapon), x+422, y+43, 126, 34, t.Text, false) {
		s.text("focus/weapon", weapon, x+485, y+60, 17, 126, t.Muted, "center")
	}
	s.typeText("focus/ammo", ammo, x+w-18, y+61, 27, 80, 700, t.Text, "right")
	s.movementKeys(p)
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
