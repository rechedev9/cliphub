package playercard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/faceit"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

const faceitTimeout = 10 * time.Second

// Image sources a card can show; without one it draws the initials.
const (
	ImageUpload = "upload"
	ImageFACEIT = "faceit"
)

// errNoPlayer is a job whose roster does not have the player.
var errNoPlayer = errors.New("the player is not in the match roster")

// FACEITLookup asks FACEIT for players by SteamID64.
type FACEITLookup interface {
	OverlayPlayers(ctx context.Context, steamIDs []string) (map[string]faceit.OverlayPlayer, error)
}

// MatchLister lists a player's library matches.
type MatchLister interface {
	ListMatches(ctx context.Context, filter library.MatchFilter) ([]library.Match, error)
}

// Sources are where a card's facts come from; a nil one leaves its facts out.
type Sources struct {
	Files storage.Storage
	// FACEIT is asked when the job stored no FACEIT snapshot of the match.
	FACEIT FACEITLookup
	// Library holds the player's matches the role is read from.
	Library MatchLister
	// Avatar downloads the player's FACEIT avatar.
	Avatar func(ctx context.Context, url string) ([]byte, error)
}

// Facts are one player's card and image for one job.
type Facts struct {
	Card        Card
	Image       []byte
	ImageSource string
}

// Gather reads one player's facts in a job: the match roster, the job's tactical
// document when it has one, FACEIT, the library role and the player's image.
func Gather(ctx context.Context, src Sources, jobID uuid.UUID, steamID string) (Facts, error) {
	var roster parser.RosterResult
	if err := readJSON(src.Files, artifacts.RosterKey(jobID), &roster); err != nil {
		return Facts{}, fmt.Errorf("read roster: %w", err)
	}
	card, ok := RosterCard(roster, steamID)
	if !ok {
		return Facts{}, errNoPlayer
	}
	var doc tacticalplan.Document
	if err := readJSON(src.Files, artifacts.TacticalIndexKey(jobID), &doc); err == nil {
		addTactical(&card, doc)
	}
	facts := Facts{Card: card}
	avatar := ""
	if f, ok := lookupFACEIT(ctx, src, jobID, steamID); ok {
		if f.SkillLevel > 0 || f.ELO > 0 {
			facts.Card.FACEIT = &FACEIT{Level: max(f.SkillLevel, 0), ELO: max(f.ELO, 0)}
		}
		if !faceit.IsDefaultFaceitAvatar(f.AvatarURL) {
			avatar = f.AvatarURL
		}
	}
	if role := libraryRole(ctx, src, steamID); roleLabels[role] != "" {
		facts.Card.Role = role
	}
	if ref, found, err := loadImage(src.Files, steamID); err == nil && found {
		if image, err := readImage(ctx, src.Files, ref); err == nil {
			facts.Image, facts.ImageSource = image, ImageUpload
			return facts, nil
		}
	}
	if avatar != "" && src.Avatar != nil {
		if image, err := src.Avatar(ctx, avatar); err == nil && imageURL(image) != "" {
			facts.Image, facts.ImageSource = image, ImageFACEIT
		}
	}
	return facts, nil
}

func readJSON(files storage.Storage, key string, into any) error {
	r, err := files.Open(key)
	if err != nil {
		return err
	}
	defer r.Close()
	return json.NewDecoder(r).Decode(into)
}

// rosterCard is the player's line from the roster scan, with ClipHub's own rating.
func RosterCard(roster parser.RosterResult, steamID string) (Card, bool) {
	for _, p := range roster.Players {
		if p.SteamID64 != steamID || steamID == "" {
			continue
		}
		m := roster.Match
		card := Card{SteamID64: p.SteamID64, Name: p.Name, Map: m.Map, Rounds: p.Rounds, Kills: p.Kills, Deaths: p.Deaths}
		switch p.Team {
		case "CT":
			card.Team, card.Result = m.ClanNameCT, &Result{Own: m.ScoreCT, Rival: m.ScoreT}
		case "T":
			card.Team, card.Result = m.ClanNameT, &Result{Own: m.ScoreT, Rival: m.ScoreCT}
		}
		if p.Kills > 0 {
			card.HSPct = &p.HSPct
		}
		if p.Rounds > 0 {
			rounds := float64(p.Rounds)
			adr, kast := p.ADR, p.KAST
			rating := math.Round(100*tacticalplan.Rating(tacticalplan.RatingLine{
				Rounds: p.Rounds, Kills: p.Kills, Deaths: p.Deaths, Assists: p.Assists,
				Damage: int(math.Round(adr * rounds)), KASTRounds: int(math.Round(kast * rounds / 100)),
			})) / 100
			card.ADR, card.KAST, card.Rating = &adr, &kast, &rating
		}
		return card, true
	}
	return Card{}, false
}

// addTactical adds the opening kills and, when the scan measured grenades, the utility damage.
func addTactical(card *Card, doc tacticalplan.Document) {
	doc.DropRepeatedUtility()
	report := tacticalplan.Metrics([]tacticalplan.MetricsMatch{{Document: doc}}, tacticalplan.MetricsOptions{SteamIDs: []string{card.SteamID64}})
	for _, p := range report.Players {
		if p.SteamID64 != card.SteamID64 {
			continue
		}
		entries := p.Box.OpeningKills
		card.OpeningKills = &entries
		if p.Utility.Measured {
			damage := p.Utility.HE.EnemyDamage + p.Utility.Molotov.EnemyDamage
			card.UtilityDamage = &damage
		}
	}
}

// lookupFACEIT prefers the job's stored FACEIT snapshot; without one it asks FACEIT for the player alone.
func lookupFACEIT(ctx context.Context, src Sources, jobID uuid.UUID, steamID string) (demooverlay.Enrichment, bool) {
	var stored map[string]demooverlay.Enrichment
	if err := readJSON(src.Files, artifacts.FullDemoFaceitKey(jobID), &stored); err == nil {
		en, ok := stored[steamID]
		return en, ok
	}
	if src.FACEIT == nil {
		return demooverlay.Enrichment{}, false
	}
	ctx, cancel := context.WithTimeout(ctx, faceitTimeout)
	defer cancel()
	players, _ := src.FACEIT.OverlayPlayers(ctx, []string{steamID})
	player, ok := players[steamID]
	if !ok {
		return demooverlay.Enrichment{}, false
	}
	return demooverlay.Enrichment{ELO: player.ELO, SkillLevel: player.SkillLevel, AvatarURL: player.Avatar}, true
}

// libraryRole is the role the library reads from the player's recent matches, as the player page shows it.
func libraryRole(ctx context.Context, src Sources, steamID string) string {
	if src.Library == nil {
		return ""
	}
	matches, err := src.Library.ListMatches(ctx, library.MatchFilter{SteamID: steamID, Limit: library.MaxListLimit})
	if err != nil {
		return ""
	}
	docs := make([]library.RoleDocument, 0, len(matches))
	for _, m := range matches {
		var doc tacticalplan.Document
		if readJSON(src.Files, artifacts.LibraryTacticalIndexKey(m.DemoID), &doc) != nil {
			continue
		}
		doc.DropRepeatedUtility()
		docs = append(docs, library.RoleDocument{MatchID: m.ID, Document: doc})
	}
	return library.InferPlayerRole(steamID, docs).Role
}
