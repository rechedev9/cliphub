package library

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// Tag limits keep a demo's labels short enough for a list cell.
const (
	maxTags     = 32
	maxTagRunes = 40
)

// NormalizeName is the key a team name or alias is matched by: case folded,
// whitespace collapsed and a leading "team " word dropped, so "Team  Vitality"
// and "vitality" resolve to the same team.
func NormalizeName(name string) string {
	key := strings.ToLower(strings.Join(strings.Fields(name), " "))
	if rest, ok := strings.CutPrefix(key, "team "); ok {
		return rest
	}
	return key
}

// CleanName trims a display name and collapses its inner whitespace.
func CleanName(name string) string {
	return strings.Join(strings.Fields(name), " ")
}

// NormalizeTags cleans user tags: whitespace collapsed, empty and duplicate
// tags dropped (case-insensitively, first spelling wins), long tags cut, and
// the result sorted so two equal sets always read back the same.
func NormalizeTags(tags []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(tags))
	for _, raw := range tags {
		tag := CleanName(raw)
		if runes := []rune(tag); len(runes) > maxTagRunes {
			tag = strings.TrimSpace(string(runes[:maxTagRunes]))
		}
		key := strings.ToLower(tag)
		if tag == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, tag)
		if len(out) == maxTags {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i]), strings.ToLower(out[j])
		if a != b {
			return a < b
		}
		return out[i] < out[j]
	})
	return out
}

// Tournament organizers whose GOTV server names mark an official match demo,
// the kind HLTV publishes. Matched as whole words of the server name.
var tournamentWords = map[string]bool{
	"esl": true, "iem": true, "blast": true, "pgl": true, "starladder": true,
	"cct": true, "thunderpick": true, "fissure": true, "hltv": true, "major": true,
	"esea": true, "perfectworld": true,
}

var (
	faceitFileName = regexp.MustCompile(`^1-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	valveFileName  = regexp.MustCompile(`^match730_`)
	hltvVersus     = regexp.MustCompile(`(^|[-_ ])vs([-_ ]|$)`)
	hltvMapNumber  = regexp.MustCompile(`(^|[-_ ])m[1-5]([-_ .]|$)`)
)

// DetectSource proposes where a demo came from, from the server name in its
// header and its file name. The server name wins: it is written by the server
// that recorded the demo, while a file can be renamed.
func DetectSource(serverName, fileName string) Source {
	server := strings.ToLower(serverName)
	switch {
	case strings.Contains(server, "faceit"):
		return SourceFaceit
	case strings.Contains(server, "valve"):
		return SourceValve
	}
	words := strings.FieldsFunc(server, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	for i, word := range words {
		if tournamentWords[word] || (word == "perfect" && i+1 < len(words) && words[i+1] == "world") {
			return SourceHLTV
		}
	}
	name := strings.ToLower(fileName)
	switch {
	case faceitFileName.MatchString(name):
		return SourceFaceit
	case valveFileName.MatchString(name):
		return SourceValve
	case hltvVersus.MatchString(name) && hltvMapNumber.MatchString(name):
		return SourceHLTV
	}
	return SourceUnknown
}

// AnalysisFromDocument summarizes a tactical document into what the library
// stores about a match. It is a pure function of the document: scores are the
// rounds each team won, and every player line is the sum of that player's
// rounds, so each number can be recomputed from the rounds it came from.
func AnalysisFromDocument(demoID uuid.UUID, doc tacticalplan.Document) Analysis {
	analysis := Analysis{
		DemoID:        demoID,
		Map:           doc.Demo.Map,
		Tickrate:      doc.Demo.Tickrate,
		DurationTicks: doc.Demo.DurationTicks,
		RoundCount:    len(doc.Rounds),
		Teams:         make([]AnalysisTeam, 0, len(doc.Teams)),
		Players:       make([]MatchPlayer, 0, len(doc.Players)),
	}
	for _, team := range doc.Teams {
		clan := CleanName(team.Name)
		if clan == tacticalplan.FallbackTeamName(team.StartSide) {
			clan = ""
		}
		score := 0
		for _, round := range doc.Rounds {
			if round.Winner != "" && doc.TeamSide(team.Key, round) == round.Winner {
				score++
			}
		}
		analysis.Teams = append(analysis.Teams, AnalysisTeam{
			Key:       team.Key,
			StartSide: string(team.StartSide),
			ClanName:  clan,
			Score:     score,
		})
	}
	sort.Slice(analysis.Teams, func(i, j int) bool { return analysis.Teams[i].Key < analysis.Teams[j].Key })

	lines := make(map[uint8]*MatchPlayer, len(doc.Players))
	for _, player := range doc.Players {
		if player.SteamID64 == "" || player.SteamID64 == "0" {
			continue
		}
		lines[player.Slot] = &MatchPlayer{
			SteamID64: player.SteamID64,
			Name:      CleanName(player.Name),
			TeamKey:   player.TeamKey,
			Slot:      player.Slot,
		}
	}
	for _, round := range doc.Rounds {
		for _, pr := range round.Players {
			line, ok := lines[pr.Slot]
			if !ok {
				continue
			}
			line.Rounds++
			line.Kills += pr.Kills
			line.Deaths += pr.Deaths
			line.Assists += pr.Assists
			line.Damage += pr.Damage
		}
	}
	for _, player := range doc.Players {
		if line, ok := lines[player.Slot]; ok {
			analysis.Players = append(analysis.Players, *line)
		}
	}
	sort.Slice(analysis.Players, func(i, j int) bool { return analysis.Players[i].Slot < analysis.Players[j].Slot })
	return analysis
}
