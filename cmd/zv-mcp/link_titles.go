package main

import (
	"regexp"
	"strings"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// The tables below mirror the web's Spanish wording (web/lib/tactical-labels.ts,
// lib/analysis/playbook.ts, lib/analysis/playbook-combo.ts, lib/stratbook/plays.ts
// and lib/format.ts), so a finding gets the same Replay title from the UI and the MCP.

var (
	buyLabels      = map[string]string{"pistol": "Pistola", "eco": "Eco", "semi": "Semi", "force": "Force", "full": "Full", "unknown": "Sin dato"}
	siteWords      = map[string]string{"A": "A", "B": "B", "mid": "Medio", "none": "Sin sitio"}
	categoryLabels = map[string]string{"save": "Save", "fake": "Fake", "rush": "Rush", "execute": "Ejecución", "map_control": "Control de mapa",
		"stack": "Stack", "aggression": "Agresión", "retake": "Retoma", "default": "Default"}
	patternLabels = map[string]string{"execute": "Ejecución", "default": "Default", "split": "Split", "fast": "Rápida", "eco_rush": "Rush de eco",
		"save": "Save", "unknown": "Sin forma", "hold": "Defensa", "retake": "Retoma", "aggression": "Agresión", "stack": "Stack"}
	placeNames = map[string]string{"TopofMid": "Top of Mid", "BackofA": "Back of A", "BackofB": "Back of B", "SnipersNest": "Sniper's Nest",
		"VipRescueZone": "VIP Rescue Zone", "Control": "Control Room", "Floor50": "50th Floor", "Floor51": "51st Floor", "MotorBoats": "Motorboats",
		"BoatHouse": "Boathouse", "DriveWay": "Driveway", "PopDog": "Popdog", "LongDog": "Longdog", "Gravelpit": "Gravel Pit"}
	camelLower = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	camelAcro  = regexp.MustCompile(`([A-Z])([A-Z][a-z])`)
	letterNum  = regexp.MustCompile(`([A-Za-z])([0-9])`)
	mapPrefix  = regexp.MustCompile(`^(de|cs)_`)
)

func labelOr(table map[string]string, key string) string {
	if label, ok := table[key]; ok {
		return label
	}
	return key
}

func buyLabel(buy string) string      { return labelOr(buyLabels, buy) }
func siteLabel(site string) string    { return labelOr(siteWords, site) }
func patternLabel(kind string) string { return labelOr(patternLabels, kind) }

// placeLabel is a CS2 place token as the callout players say it.
func placeLabel(place string) string {
	if known, ok := placeNames[place]; ok {
		return known
	}
	place = camelLower.ReplaceAllString(place, "${1} ${2}")
	place = camelAcro.ReplaceAllString(place, "${1} ${2}")
	return letterNum.ReplaceAllString(place, "${1} ${2}")
}

// prettyMapName is de_nuke as Nuke.
func prettyMapName(name string) string {
	name = mapPrefix.ReplaceAllString(name, "")
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// playShapeLabel is a detected play as "Ejecución A", or the kind alone without a site.
func playShapeLabel(kind, site string) string {
	label := patternLabel(kind)
	if site == "none" || site == "" {
		return label
	}
	return label + " " + siteLabel(site)
}

// tacticLabel is one playbook tactic as a short phrase, like "Humo en Ventana".
func tacticLabel(t tuiclient.PlaybookTactic) string {
	place := t.Label
	if place == "" {
		place = t.Value
	}
	switch t.Kind {
	case "category":
		return labelOr(categoryLabels, t.Value)
	case "site":
		if _, ok := siteWords[t.Value]; !ok {
			return t.Value
		}
		if t.Value == "none" {
			return "Ningún sitio"
		}
		return "Ronda en " + siteLabel(t.Value)
	case "buy":
		if _, ok := buyLabels[t.Value]; !ok {
			return t.Value
		}
		return "Compra " + strings.ToLower(buyLabel(t.Value))
	case "opponent_buy":
		if _, ok := buyLabels[t.Value]; !ok {
			return t.Value
		}
		return "Rival en " + strings.ToLower(buyLabel(t.Value))
	case "opening":
		return "Primer duelo en " + place
	case "opening_result":
		if t.Value == "won" {
			return "Primer duelo ganado"
		}
		return "Primer duelo perdido"
	case "awp":
		return "AWP en " + place
	case "zone":
		return "Control de " + place
	case "utility":
		if t.Grenade == "molotov" {
			return "Molotov en " + place
		}
		return "Humo en " + place
	default:
		return t.Value
	}
}

// titleScope is the team, side and map a link title names; side and map may be empty.
type titleScope struct {
	team, side, mapName string
}

// of writes "Equipo · <what> en T · Nuke", the title shape of the web's playbookReplayTitle.
func (s titleScope) of(what string) string {
	if s.side != "" {
		what += " en " + s.side
	}
	return joinTitle(s.team, what, prettyMapName(s.mapName))
}
