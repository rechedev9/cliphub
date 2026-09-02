package demooverlay

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/rechedev9/cliphub/internal/parser"
)

func Load(path string) (Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return Document{}, fmt.Errorf("open full-demo overlay: %w", err)
	}
	defer f.Close()
	return Decode(f)
}

func Decode(r io.Reader) (Document, error) {
	var doc Document
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("decode full-demo overlay: %w", err)
	}
	if doc.SchemaVersion != "" && doc.SchemaVersion != SchemaVersion {
		return Document{}, fmt.Errorf("full-demo overlay schema %q, want %q", doc.SchemaVersion, SchemaVersion)
	}
	if doc.SchemaVersion == "" {
		doc.SchemaVersion = SchemaVersion
	}
	return doc, nil
}

func Write(path string, doc Document) error {
	doc.SchemaVersion = SchemaVersion
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return fmt.Errorf("write full-demo overlay: %w", err)
	}
	return nil
}

func FromRosterScan(result parser.RosterResult, targetSteamID string) Roster {
	players := make([]RosterPlayer, 0, len(result.Players))
	for _, p := range result.Players {
		players = append(players, RosterPlayer{
			SteamID64: p.SteamID64,
			Name:      p.Name,
			Team:      p.Team,
			Kills:     p.Kills,
			Deaths:    p.Deaths,
			Assists:   p.Assists,
			Headshots: p.Headshots,
			MVPs:      p.MVPs,
			Rounds:    p.Rounds,
			ADR:       p.ADR,
			HSPct:     p.HSPct,
			Rating:    p.Rating,
			Rounds2K:  p.Rounds2K,
			Rounds3K:  p.Rounds3K,
			Rounds4K:  p.Rounds4K,
			Rounds5K:  p.Rounds5K,
		})
	}
	return Roster{
		TargetSteamID64: targetSteamID,
		Players:         players,
		Map:             result.Match.Map,
		ScoreCT:         result.Match.ScoreCT,
		ScoreT:          result.Match.ScoreT,
		Rounds:          result.Match.Rounds,
		ClanNameCT:      result.Match.ClanNameCT,
		ClanNameT:       result.Match.ClanNameT,
	}
}

func NormalizeSource(source string) string {
	switch s := strings.ToLower(strings.TrimSpace(source)); s {
	case SourcePremier, SourceProfessional, SourceFACEIT:
		return s
	default:
		return ""
	}
}

func UsesFACEITEnrichment(source string) bool {
	return NormalizeSource(source) == SourceFACEIT
}

// Build keeps the historical FACEIT-when-enrichment-present behavior for
// existing tests. Full Demo renders must call BuildForSource so a Premier or
// professional job cannot inherit FACEIT fields from a leftover map.
func Build(roster Roster, faceit map[string]Enrichment) Document {
	source := ""
	if len(faceit) > 0 {
		source = SourceFACEIT
	}
	return BuildForSource(roster, source, faceit)
}

func BuildForSource(roster Roster, source string, faceit map[string]Enrichment) Document {
	source = NormalizeSource(source)
	if !UsesFACEITEnrichment(source) {
		faceit = nil
	}
	if faceit == nil {
		faceit = map[string]Enrichment{}
	}
	cards := make([]PlayerCard, 0, len(roster.Players))
	for _, p := range roster.Players {
		en := faceit[p.SteamID64]
		cards = append(cards, cardFromRoster(p, en))
	}
	target := targetCard(cards, roster.TargetSteamID64)
	povTeam := target.Team
	left, right := splitSides(cards, povTeam)
	allIntro := append(append([]PlayerCard{}, left...), right...)
	introCols := introColumns(source, allIntro)
	outroTeams := outroTeamsFromRoster(cards, roster)
	leftTeam, leftSub := introPanelMeta(source, povTeam, clanNameForSide(roster, povTeam), roster.Map, left)
	rightTeam, rightSub := introPanelMeta(source, oppositeSide(povTeam), clanNameForSide(roster, oppositeSide(povTeam)), roster.Map, right)
	doc := Document{
		SchemaVersion:   SchemaVersion,
		Source:          source,
		TargetSteamID64: roster.TargetSteamID64,
		TargetName:      firstNonEmpty(target.Name, roster.TargetSteamID64),
		TargetKills:     target.Kills,
		TargetDeaths:    target.Deaths,
		TargetELO:       target.ELO,
		Map:             roster.Map,
		ScoreCT:         roster.ScoreCT,
		ScoreT:          roster.ScoreT,
		Intro: Intro{
			Left:          left,
			Right:         right,
			LeftTeamName:  leftTeam,
			LeftSubtitle:  leftSub,
			RightTeamName: rightTeam,
			RightSubtitle: rightSub,
			Columns:       introCols,
		},
		Outro: Scoreboard{
			Teams:   outroTeams,
			Columns: outroColumns(source, outroTeams),
		},
	}
	return doc
}

// IsPOV reports whether card is the Full Demo POV target.
func (doc Document) IsPOV(card PlayerCard) bool {
	return doc.TargetSteamID64 != "" && card.SteamID64 == doc.TargetSteamID64
}

func cardFromRoster(p RosterPlayer, en Enrichment) PlayerCard {
	card := PlayerCard{
		SteamID64: p.SteamID64,
		Name:      firstNonEmpty(p.Name, p.SteamID64),
		Team:      p.Team,
		Kills:     p.Kills,
		Deaths:    p.Deaths,
		Assists:   p.Assists,
		Headshots: p.Headshots,
		MVPs:      p.MVPs,
		Rounds:    p.Rounds,
		Rounds2K:  p.Rounds2K,
		Rounds3K:  p.Rounds3K,
		Rounds4K:  p.Rounds4K,
		Rounds5K:  p.Rounds5K,
	}
	if p.ADR > 0 || p.Rounds > 0 {
		card.ADR = p.ADR
		card.HasADR = p.ADR > 0 || p.Rounds > 0
	}
	if p.Kills > 0 {
		card.HSPct = p.HSPct
		card.HasHSPct = true
	}
	if p.Rating > 0 {
		card.Rating = p.Rating
		card.HasRating = true
	}
	if strings.TrimSpace(en.Nickname) != "" {
		card.Name = strings.TrimSpace(en.Nickname)
	}
	card.Country = strings.TrimSpace(en.Country)
	card.ELO = intPtr(en.ELO)
	card.SkillLevel = intPtr(en.SkillLevel)
	card.Ranking = en.Ranking
	card.AvatarURL = strings.TrimSpace(en.AvatarURL)
	if last20HasAny(en.Last20) {
		card.Last20 = en.Last20
	}
	return card
}

func last20HasAny(l *Last20) bool {
	if l == nil {
		return false
	}
	return l.Matches != nil || l.WinPct != nil || l.Rating != nil || l.Swing != nil ||
		l.Kills != nil || l.Deaths != nil || l.Assists != nil ||
		l.KD != nil || l.KR != nil || l.ADR != nil
}

func targetCard(cards []PlayerCard, steamID string) PlayerCard {
	for _, card := range cards {
		if card.SteamID64 == steamID {
			return card
		}
	}
	if steamID != "" {
		return PlayerCard{SteamID64: steamID, Name: steamID}
	}
	if len(cards) > 0 {
		return cards[0]
	}
	return PlayerCard{}
}

func splitSides(cards []PlayerCard, povTeam string) (left, right []PlayerCard) {
	for _, card := range cards {
		if povTeam != "" && card.Team == povTeam {
			left = append(left, card)
			continue
		}
		right = append(right, card)
	}
	if povTeam == "" && len(cards) > 0 {
		half := (len(cards) + 1) / 2
		left, right = cards[:half], cards[half:]
	}
	sortCards(left)
	sortCards(right)
	return left, right
}

func sortCards(cards []PlayerCard) {
	sort.SliceStable(cards, func(i, j int) bool {
		if cards[i].Kills != cards[j].Kills {
			return cards[i].Kills > cards[j].Kills
		}
		return cards[i].Name < cards[j].Name
	})
}

func outroTeamsFromRoster(cards []PlayerCard, roster Roster) []TeamBoard {
	bySide := map[string][]PlayerCard{}
	for _, card := range cards {
		side := card.Team
		if side != "CT" && side != "T" {
			side = "other"
		}
		bySide[side] = append(bySide[side], card)
	}
	order := []string{"CT", "T"}
	if roster.ScoreT > roster.ScoreCT {
		order = []string{"T", "CT"}
	}
	var teams []TeamBoard
	for _, side := range order {
		players := bySide[side]
		if len(players) == 0 {
			continue
		}
		sortCards(players)
		score := roster.ScoreCT
		if side == "T" {
			score = roster.ScoreT
		}
		teams = append(teams, TeamBoard{
			Name:       teamDisplayName(side, clanNameForSide(roster, side), players),
			Side:       side,
			Score:      score,
			AverageELO: averageELO(players),
			Players:    players,
		})
	}
	if others := bySide["other"]; len(others) > 0 {
		sortCards(others)
		teams = append(teams, TeamBoard{
			Name:    teamDisplayName("", "", others),
			Players: others,
		})
	}
	return teams
}

func clanNameForSide(roster Roster, side string) string {
	switch side {
	case "CT":
		return strings.TrimSpace(roster.ClanNameCT)
	case "T":
		return strings.TrimSpace(roster.ClanNameT)
	default:
		return ""
	}
}

func oppositeSide(side string) string {
	switch side {
	case "CT":
		return "T"
	case "T":
		return "CT"
	default:
		return ""
	}
}

func sideDisplayName(side string) string {
	switch side {
	case "CT":
		return "Counter-Terrorists"
	case "T":
		return "Terrorists"
	default:
		return ""
	}
}

func teamDisplayName(side, clanName string, players []PlayerCard) string {
	if name := strings.TrimSpace(clanName); name != "" {
		return name
	}
	if label := sideDisplayName(side); label != "" {
		return label
	}
	return teamName(players, side)
}

func introPanelMeta(source, side, clanName, mapName string, players []PlayerCard) (teamName, subtitle string) {
	displayTeam := teamDisplayName(side, clanName, players)
	switch NormalizeSource(source) {
	case SourceProfessional:
		return displayTeam, displayMapName(mapName)
	case SourcePremier:
		subtitle = "CS2 PREMIER"
		if mapLabel := displayMapName(mapName); mapLabel != "" {
			subtitle += " · " + mapLabel
		}
		return displayTeam, subtitle
	case SourceFACEIT:
		if avg := averageELO(players); avg != nil {
			return displayTeam, fmt.Sprintf("%d avg ELO", *avg)
		}
		return displayTeam, ""
	default:
		return "", ""
	}
}

func teamName(players []PlayerCard, side string) string {
	if len(players) > 0 && players[0].Name != "" {
		return "team_" + players[0].Name
	}
	if side != "" {
		return "team_" + side
	}
	return "team"
}

func averageELO(players []PlayerCard) *int {
	if len(players) == 0 {
		return nil
	}
	sum := 0
	for _, p := range players {
		if p.ELO == nil {
			return nil
		}
		sum += *p.ELO
	}
	avg := sum / len(players)
	return &avg
}

func introColumns(source string, cards []PlayerCard) []string {
	switch NormalizeSource(source) {
	case SourceProfessional, SourcePremier:
		return []string{ColName}
	case SourceFACEIT:
		return introFACEITColumns(cards)
	default:
		return introLegacyColumns(cards)
	}
}

func introFACEITColumns(cards []PlayerCard) []string {
	cols := []string{ColName}
	if anyCard(cards, func(p PlayerCard) bool { return p.Country != "" }) {
		cols = append(cols, ColCountry)
	}
	if anyCard(cards, func(p PlayerCard) bool { return p.ELO != nil }) {
		cols = append(cols, ColELO)
	}
	if anyCard(cards, func(p PlayerCard) bool { return p.SkillLevel != nil }) {
		cols = append(cols, ColLevel)
	}
	if anyLast20(cards, func(l Last20) bool { return l.Matches != nil }) {
		cols = append(cols, ColMatches)
	}
	if anyLast20(cards, func(l Last20) bool { return l.WinPct != nil }) {
		cols = append(cols, ColWinPct)
	}
	if anyLast20(cards, func(l Last20) bool { return l.Rating != nil }) {
		cols = append(cols, ColRating)
	}
	if anyLast20(cards, func(l Last20) bool { return l.Swing != nil }) {
		cols = append(cols, ColSwing)
	}
	if anyLast20(cards, func(l Last20) bool { return l.KD != nil }) {
		cols = append(cols, ColKD)
	}
	if anyLast20(cards, func(l Last20) bool { return l.KR != nil }) {
		cols = append(cols, ColKR)
	}
	if anyLast20(cards, func(l Last20) bool { return l.ADR != nil }) {
		cols = append(cols, ColADR)
	}
	return cols
}

func introLegacyColumns(cards []PlayerCard) []string {
	cols := []string{ColName}
	if anyCard(cards, func(p PlayerCard) bool { return p.Country != "" }) {
		cols = append(cols, ColCountry)
	}
	if anyCard(cards, func(p PlayerCard) bool { return p.ELO != nil }) {
		cols = append(cols, ColELO)
	}
	if anyCard(cards, func(p PlayerCard) bool { return p.SkillLevel != nil }) {
		cols = append(cols, ColLevel)
	}
	if anyLast20(cards, func(l Last20) bool { return l.Matches != nil }) {
		cols = append(cols, ColMatches)
	}
	if anyLast20(cards, func(l Last20) bool { return l.WinPct != nil }) {
		cols = append(cols, ColWinPct)
	}
	if anyLast20(cards, func(l Last20) bool { return l.Rating != nil }) {
		cols = append(cols, ColRating)
	}
	if anyLast20(cards, func(l Last20) bool { return l.Swing != nil }) {
		cols = append(cols, ColSwing)
	}
	cols = append(cols, ColKDA)
	if anyLast20(cards, func(l Last20) bool { return l.KD != nil }) {
		cols = append(cols, ColKD)
	}
	if anyLast20(cards, func(l Last20) bool { return l.KR != nil }) {
		cols = append(cols, ColKR)
	}
	if anyLast20(cards, func(l Last20) bool { return l.ADR != nil }) || anyCard(cards, func(p PlayerCard) bool { return p.HasADR }) {
		if !containsCol(cols, ColADR) {
			cols = append(cols, ColADR)
		}
	}
	if anyCard(cards, func(p PlayerCard) bool { return p.HasRating }) && !containsCol(cols, ColRating) {
		cols = append(cols, ColRating)
	}
	return cols
}

func containsCol(cols []string, col string) bool {
	return slices.Contains(cols, col)
}

func outroColumns(source string, teams []TeamBoard) []string {
	var cards []PlayerCard
	for _, team := range teams {
		cards = append(cards, team.Players...)
	}
	if NormalizeSource(source) == SourceFACEIT {
		return []string{
			ColName, ColLevel, ColELO, ColRating, ColKDA, ColADR, ColKR,
			ColHSPct, Col5K, Col4K, Col3K, Col2K, ColMVP,
		}
	}
	cols := []string{ColName, ColKDA}
	if anyCard(cards, func(p PlayerCard) bool { return p.ELO != nil }) {
		cols = append(cols, ColELO)
	}
	if anyCard(cards, func(p PlayerCard) bool { return p.SkillLevel != nil }) {
		cols = append(cols, ColLevel)
	}
	if anyCard(cards, func(p PlayerCard) bool { return p.HasRating }) {
		cols = append(cols, ColRating)
	}
	if anyLast20(cards, func(l Last20) bool { return l.Swing != nil }) {
		cols = append(cols, ColSwing)
	}
	if anyCard(cards, func(p PlayerCard) bool { return p.HasADR }) {
		cols = append(cols, ColADR)
	}
	if anyCard(cards, func(p PlayerCard) bool { return p.HasHSPct || p.Headshots > 0 }) {
		cols = append(cols, ColHS, ColHSPct)
	}
	if anyCard(cards, func(p PlayerCard) bool {
		return p.Rounds2K > 0 || p.Rounds3K > 0 || p.Rounds4K > 0 || p.Rounds5K > 0
	}) {
		cols = append(cols, ColMulti)
	}
	if anyCard(cards, func(p PlayerCard) bool { return p.MVPs > 0 }) {
		cols = append(cols, ColMVP)
	}
	return cols
}

func anyCard(cards []PlayerCard, fn func(PlayerCard) bool) bool {
	for _, card := range cards {
		if fn(card) {
			return true
		}
	}
	return false
}

func anyLast20(cards []PlayerCard, fn func(Last20) bool) bool {
	for _, card := range cards {
		if card.Last20 != nil && fn(*card.Last20) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
