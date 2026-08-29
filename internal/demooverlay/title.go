package demooverlay

import (
	"fmt"
	"strings"
	"unicode"
)

const voiceCommsSuffix = "CS2 DEMO POV + VOICECOMMS"

func Title(doc Document) string {
	player := firstNonEmpty(doc.TargetName, "CS2")
	parts := []string{fmt.Sprintf("%s (%d-%d)", player, doc.TargetKills, doc.TargetDeaths)}
	if mapName := displayMapName(doc.Map); mapName != "" {
		parts = append(parts, mapName)
	}
	if doc.TargetELO != nil {
		parts = append(parts, fmt.Sprintf("%d ELO", *doc.TargetELO))
	}
	return strings.Join(parts, " ") + " | " + voiceCommsSuffix
}

func Caption(doc Document) string {
	player := firstNonEmpty(doc.TargetName, "This player")
	mapName := displayMapName(doc.Map)
	mapPhrase := "in CS2"
	if mapName != "" {
		mapPhrase = "on " + mapName
	}
	return fmt.Sprintf("%s POV %s (%d-%d). CS2 demo with team voice comms.", player, mapPhrase, doc.TargetKills, doc.TargetDeaths)
}

func Hashtags(doc Document) []string {
	raw := []string{"CS2", "CS2Demo", "POV", "VoiceComms", socialTag(displayMapName(doc.Map))}
	out := make([]string, 0, 5)
	seen := map[string]bool{}
	for _, value := range raw {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		tag := "#" + value
		if seen[strings.ToLower(tag)] {
			continue
		}
		seen[strings.ToLower(tag)] = true
		out = append(out, tag)
		if len(out) >= 5 {
			break
		}
	}
	return out
}

func displayMapName(raw string) string {
	raw = strings.TrimSpace(raw)
	switch strings.ToLower(raw) {
	case "":
		return ""
	case "de_inferno":
		return "Inferno"
	case "de_dust2":
		return "Dust2"
	case "de_anubis":
		return "Anubis"
	case "de_ancient":
		return "Ancient"
	case "de_mirage":
		return "Mirage"
	case "de_nuke":
		return "Nuke"
	case "de_overpass":
		return "Overpass"
	case "de_train":
		return "Train"
	case "de_vertigo":
		return "Vertigo"
	default:
		raw = strings.TrimPrefix(raw, "de_")
		if raw == "" {
			return ""
		}
		return strings.ToUpper(raw[:1]) + strings.ToLower(raw[1:])
	}
}

func socialTag(raw string) string {
	var sb strings.Builder
	upperNext := true
	for _, r := range raw {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			if upperNext && r >= 'a' && r <= 'z' {
				r = r - 'a' + 'A'
			}
			sb.WriteRune(r)
			upperNext = false
			continue
		}
		if unicode.IsSpace(r) {
			upperNext = true
		}
	}
	return sb.String()
}
