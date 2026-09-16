package httpapi

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/rechedev9/cliphub/internal/editor"
)

// Publication uses the completed render's effective evidence, never a newer
// editable plan. Legacy long videos get generic POV copy without source/comms.
func addLongVideoPublishFacts(facts *publishAssistantFacts, short editor.ShortResult, item editor.PublishItem) {
	evidence := short.FullDemo
	if evidence == nil {
		evidence = item.FullDemo
	}
	facts.LongVideo = evidence != nil || (item.SegmentID == "demo-compilation" && item.Preset == "gameplay-pov-60")
	if evidence == nil {
		return
	}
	doc := evidence.Effective
	facts.SourceKind = doc.Options.SourceKind
	voice := doc.Options.Audio.Voice
	if voice.Enabled && voice.Gain > 0 && doc.Voice.Availability == "available" && doc.Voice.SelectedPackets > 0 {
		for _, track := range evidence.TrackLevels {
			if track.Role == "team-voice" && track.Measurement.Status == "measured" {
				facts.Comms = true
				break
			}
		}
	}
	// Pick the most frequently encountered enemy, using IDs and teams rather
	// than guessing whether a nickname belongs to a professional player.
	counts := map[string]int{}
	best := 0
	for _, round := range doc.Rounds {
		for _, kill := range round.Kills {
			if doc.Input.TargetSteamID64 == "" || kill.Killer.SteamID64 != doc.Input.TargetSteamID64 ||
				kill.Victim.SteamID64 == "" || kill.Victim.SteamID64 == kill.Killer.SteamID64 ||
				kill.Killer.TeamAtKill == "" || kill.Victim.TeamAtKill == "" || kill.Victim.TeamAtKill == kill.Killer.TeamAtKill ||
				strings.TrimSpace(kill.Victim.NameInDemo) == "" {
				continue
			}
			counts[kill.Victim.SteamID64]++
			if counts[kill.Victim.SteamID64] > best {
				best = counts[kill.Victim.SteamID64]
				facts.Opponent = kill.Victim.NameInDemo
			}
		}
	}
}

func longVideoPublishRecommendations(facts publishAssistantFacts) []publishRecommendation {
	player := publishName(facts.Player, 24)
	mapName := publishName(strings.TrimPrefix(facts.Map, "de_"), 20)
	knownMaps := map[string]string{"dust2": "Dust 2", "mirage": "Mirage", "anubis": "Anubis", "inferno": "Inferno", "nuke": "Nuke", "ancient": "Ancient", "vertigo": "Vertigo", "overpass": "Overpass", "cache": "Cache", "train": "Train"}
	if name, ok := knownMaps[strings.ToLower(mapName)]; ok {
		mapName = name
	}
	source := "CS2"
	switch facts.SourceKind {
	case "faceit":
		source = "FACEIT"
	case "premier":
		source = "Premier"
	}
	pov := "POV"
	if facts.Comms {
		pov += " with COMMS"
	}
	tags := []string{"CS2", "Counter-Strike 2", player, mapName, "POV", "CS2 gameplay"}
	if source != "CS2" {
		tags = append(tags, source)
	}
	if facts.Comms {
		tags = append(tags, "team comms")
	}
	description := fmt.Sprintf("%s %s on %s (%s).\n\nPlayer: %s\nMap: %s\nKills in this video: %d\n\nRound-by-round gameplay from a Counter-Strike 2 demo.\n\n#CS2 #CounterStrike2 #POV", player, pov, mapName, source, player, mapName, facts.KillCount)
	result := make([]publishRecommendation, 0, 5)
	add := func(label, title, rationale string) {
		result = append(result, publishRecommendation{
			Template: label, Title: publishName(title, 100), Description: description,
			Keywords: []string{player, mapName, source, "POV"}, Tags: append([]string(nil), tags...),
			Rationale: rationale,
		})
	}
	if facts.KillCount > 0 {
		add("Bajas destacadas", fmt.Sprintf("%s Drops %d KILLS on %s! %s (%s)", player, facts.KillCount, source, pov, mapName), "Bajas incluidas en el vídeo; no se presentan como un ace o una sola jugada.")
	}
	add("POV clásico", fmt.Sprintf("%s %s on %s (%s)", player, pov, source, mapName), "Jugador, origen y mapa. COMMS solo aparece cuando el render contiene voces verificadas.")
	add("Sesión de juego", fmt.Sprintf("%s Plays %s! %s (%s)", player, source, pov, mapName), "Estilo sesión de YouTube sin atribuir rango, ELO o nivel profesional.")
	add("Mapa protagonista", fmt.Sprintf("%s %s — %s %s | CS2", mapName, source, player, pov), "Destaca el mapa y la perspectiva del jugador sin prometer una partida sin cortes.")
	if facts.Opponent != "" && len(result) < 5 {
		opponent := publishName(facts.Opponent, 24)
		add("Duelo de la demo", fmt.Sprintf("%s vs %s on %s! %s (%s)", player, opponent, source, pov, mapName), "Rival identificado por SteamID y equipos en las bajas de la demo; no presupone que sea un profesional.")
	}
	return result
}

// Bound each variable before interpolation; discard line breaks, controls and
// angle brackets so demo nicknames remain plain, single-line publication text.
func publishName(value string, limit int) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if unicode.IsControl(r) || r == '<' || r == '>' {
			return -1
		}
		return r
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit-1]) + "…"
	}
	return value
}
