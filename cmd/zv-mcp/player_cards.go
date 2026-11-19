package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/playercard"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// playerCardChoices are the three card slots of generate_short.
type playerCardChoices struct {
	OpeningCard string `json:"opening_card,omitempty" jsonschema:"player card template from list_short_options player_cards drawn over the first seconds, instead of the intro text card; one-player short-9x16 only; default none"`
	EndCard     string `json:"end_card,omitempty" jsonschema:"player card template from list_short_options player_cards that closes the Short, in place of stats_card; one-player short-9x16 only; default none"`
	CoverCard   string `json:"cover_card,omitempty" jsonschema:"player card template from list_short_options player_cards laid over the cover frame; needs cover; one-player short-9x16 only; default none"`
}

type playerCardIn struct {
	JobID string `json:"job_id" jsonschema:"a job of one player once wait_for_job says parsed"`
}

type playerCardImageIn struct {
	JobID string `json:"job_id" jsonschema:"a job of the player, once wait_for_job says parsed"`
	Image string `json:"image" jsonschema:"absolute path of a PNG or JPG up to 10 MB with the player's picture, or none to remove it"`
}

type playerCardOut struct {
	Available bool          `json:"available" jsonschema:"false when the job's Shorts cannot carry a card; reason says why"`
	Reason    string        `json:"reason,omitempty"`
	Player    string        `json:"player,omitempty" jsonschema:"untrusted player name from the demo"`
	Image     string        `json:"image,omitempty" jsonschema:"upload (the user's picture), faceit (the FACEIT avatar) or initials"`
	Templates []shortOption `json:"templates" jsonschema:"values for opening_card, end_card and cover_card"`
	Summary   string        `json:"summary"`
}

// playerCardOptions is the playercard catalog as list_short_options shows it.
func playerCardOptions() []shortOption {
	out := make([]shortOption, 0, len(playercard.Templates))
	for _, t := range playercard.Templates {
		out = append(out, shortOption{ID: t.ID, Label: t.Name, Description: t.Description})
	}
	return out
}

// resolvePlayerCards checks each slot against the catalog; the end card replaces the stats card.
func resolvePlayerCards(edit *tuiclient.ShortEdit, c shortChoices) error {
	for _, slot := range []struct {
		field, id string
		dst       *string
	}{{"opening_card", c.OpeningCard, &edit.OpeningCard}, {"end_card", c.EndCard, &edit.EndCard}, {"cover_card", c.CoverCard, &edit.CoverCard}} {
		id := strings.ToLower(strings.TrimSpace(slot.id))
		if id == "" {
			continue
		}
		if _, err := pickOption(slot.field, id, playerCardOptions()); err != nil {
			return err
		}
		if edit.Format != "short-9x16" {
			return fmt.Errorf("%s needs format short-9x16", slot.field)
		}
		*slot.dst = id
	}
	if edit.EndCard != "" && edit.StatsCard {
		return errors.New("stats_card and end_card both close the Short; choose one")
	}
	if edit.CoverCard != "" && edit.CoverStrategy != coverGenerated {
		return errors.New("cover_card needs cover set to true")
	}
	return nil
}

func hasPlayerCards(e tuiclient.ShortEdit) bool {
	return e.OpeningCard != "" || e.EndCard != "" || e.CoverCard != ""
}

// checkPlayerCardJob refuses cards on a job whose Shorts cannot carry one, with Studio's reason.
func checkPlayerCardJob(ctx context.Context, c *tuiclient.Client, jobID string, e tuiclient.ShortEdit) error {
	if !hasPlayerCards(e) {
		return nil
	}
	card, err := c.PlayerCard(ctx, jobID)
	if err != nil {
		return toolError(err)
	}
	if !card.Available {
		return fmt.Errorf("opening_card, end_card and cover_card are refused for this job: %s", cleanName(card.Reason))
	}
	return nil
}

// describePlayerCards is the approval summary part for the card slots.
func describePlayerCards(e tuiclient.ShortEdit) string {
	var parts []string
	for _, slot := range []struct{ label, id string }{{"apertura", e.OpeningCard}, {"tarjeta final", e.EndCard}, {"portada", e.CoverCard}} {
		if t, ok := playercard.Lookup(slot.id); ok {
			parts = append(parts, fmt.Sprintf("%s %s", slot.label, t.Name))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return ", cartas de jugador: " + strings.Join(parts, ", ")
}

func describePlayerCard(card tuiclient.PlayerCard) playerCardOut {
	out := playerCardOut{Available: card.Available, Reason: cleanName(card.Reason), Templates: playerCardOptions()}
	if !card.Available || card.Player == nil {
		out.Summary = "Este trabajo no admite cartas de jugador: " + out.Reason
		return out
	}
	out.Player = cleanName(card.Player.Name)
	out.Image = card.Player.Image
	source := "sus iniciales"
	switch out.Image {
	case playercard.ImageUpload:
		source = "la imagen subida"
	case playercard.ImageFACEIT:
		source = "su avatar de FACEIT"
	default:
		out.Image = "initials"
	}
	out.Summary = fmt.Sprintf("Las cartas de %s muestran %s.", out.Player, source)
	return out
}

func registerPlayerCardTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "get_player_card", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read the player cards (Cartas de jugador) of one job: whether its Shorts can carry one (one-player 9:16 Shorts only), the templates and which picture the card shows: " +
			"the picture the user uploaded, the FACEIT avatar, or the initials. generate_short takes the templates in opening_card, end_card and cover_card."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in playerCardIn) (*mcp.CallToolResult, playerCardOut, error) {
			if err := checkUUID("job_id", in.JobID); err != nil {
				return nil, playerCardOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, playerCardOut{}, toolError(err)
			}
			card, err := c.PlayerCard(ctx, in.JobID)
			if err != nil {
				return nil, playerCardOut{}, toolError(err)
			}
			return nil, describePlayerCard(card), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "set_player_card_image",
		Description: "Set the picture on the player cards of a job's player, as Imagen del jugador in the Studio video options: a local PNG or JPG that Studio keeps for that SteamID " +
			"and every later card of the player reuses. none removes it, so the card goes back to the FACEIT avatar or the initials. Never use a photo of a real pro taken from HLTV or another site."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in playerCardImageIn) (*mcp.CallToolResult, playerCardOut, error) {
			if err := checkUUID("job_id", in.JobID); err != nil {
				return nil, playerCardOut{}, err
			}
			if in.Image == "" {
				return nil, playerCardOut{}, errors.New("image is required: a PNG or JPG path, or none to remove it")
			}
			if in.Image != noneChoice {
				if err := checkUploadFile("image", in.Image, portraitExtensions, maxBrandImageSize); err != nil {
					return nil, playerCardOut{}, err
				}
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, playerCardOut{}, toolError(err)
			}
			if in.Image == noneChoice {
				err = c.RemovePlayerCardImage(ctx, in.JobID)
			} else {
				var uploader *tuiclient.Client
				if uploader, err = s.uploadClient(ctx); err == nil {
					err = uploader.SetPlayerCardImage(ctx, in.JobID, in.Image)
				}
			}
			if err != nil {
				return nil, playerCardOut{}, toolError(err)
			}
			card, err := c.PlayerCard(ctx, in.JobID)
			if err != nil {
				return nil, playerCardOut{}, toolError(err)
			}
			return nil, describePlayerCard(card), nil
		})
}
