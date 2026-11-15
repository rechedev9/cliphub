package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/hudthemes"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// hudLookIn are the plan_full_demo fields that only change how a custom HUD is drawn.
type hudLookIn struct {
	CompactHeader  *bool   `json:"compact_header,omitempty" jsonschema:"custom HUD only: hide the ten player cards at the top and keep the scoreboard; default off"`
	HideInventory  *bool   `json:"hide_inventory,omitempty" jsonschema:"custom HUD only: hide the observed player's weapon and ammo; default off"`
	RoundWinBanner *bool   `json:"round_win_banner,omitempty" jsonschema:"custom HUD only: show which team won as each round ends; default off"`
	TeamCTStart    *string `json:"team_ct_start,omitempty" jsonschema:"custom HUD only: name on the scoreboard of the team that started on CT; empty clears it; left out keeps the name from the library match"`
	TeamTStart     *string `json:"team_t_start,omitempty" jsonschema:"custom HUD only: name on the scoreboard of the team that started on T; empty clears it; left out keeps the name from the library match"`
}

// hudLookChoices mirrors the composition switches of the web HUD selector.
var hudLookChoices = []string{
	"compact_header: cabecera compacta, sin las fichas de los diez jugadores",
	"hide_inventory: ocultar el arma y la munición del jugador",
	"round_win_banner: mensaje del equipo que gana cada ronda",
	"team_ct_start y team_t_start: nombres de los equipos en el marcador",
}

type userHUDThemeOut struct {
	ID         string `json:"id" jsonschema:"value for plan_full_demo hud, or for save_hud_theme and delete_hud_theme"`
	Base       string `json:"base" jsonschema:"the catalog design it copies"`
	Name       string `json:"name"`
	CT         string `json:"ct" jsonschema:"hex colour of the CT side"`
	T          string `json:"t" jsonschema:"hex colour of the T side"`
	Background string `json:"background" jsonschema:"hex colour of the plates"`
	Radius     *int   `json:"radius,omitempty" jsonschema:"corner radius in px; absent keeps the base design's shape"`
}

type saveHUDThemeIn struct {
	ID         string `json:"id,omitempty" jsonschema:"a theme id from list_hud_themes to edit it; empty creates a new theme"`
	Base       string `json:"base,omitempty" jsonschema:"for a new theme: the catalog design id it copies (list_hud_themes)"`
	Name       string `json:"name" jsonschema:"1 to 40 characters"`
	CT         string `json:"ct" jsonschema:"hex colour of the CT side, such as 42C9E8"`
	T          string `json:"t" jsonschema:"hex colour of the T side, such as FFB32C"`
	Background string `json:"background" jsonschema:"hex colour of the plates, such as 101927"`
	Radius     *int   `json:"radius,omitempty" jsonschema:"corner radius 0 to 15 px; left out keeps the base design's shape"`
}

type deleteHUDThemeIn struct {
	ID      string `json:"id" jsonschema:"a theme id from list_hud_themes"`
	Approve string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

func userHUDTheme(t hudthemes.Theme) userHUDThemeOut {
	return userHUDThemeOut{ID: t.ID.String(), Base: t.Base, Name: cleanName(t.Name), CT: t.CT, T: t.T, Background: t.Background, Radius: t.Radius}
}

func registerHUDThemeTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "save_hud_theme",
		Description: fmt.Sprintf("Create or edit one of the user's own Full Demo HUD themes: a catalog design copied with another name, CT and T colours, plate colour and corner radius. "+
			"Studio keeps up to %d. Without id it duplicates base; with id it edits that theme. The theme only changes the render, never the capture. "+
			"Use its id as plan_full_demo hud.", hudthemes.MaxThemes)},
		func(ctx context.Context, _ *mcp.CallToolRequest, in saveHUDThemeIn) (*mcp.CallToolResult, userHUDThemeOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, userHUDThemeOut{}, toolError(err)
			}
			style := hudthemes.Style{Name: in.Name, CT: in.CT, T: in.T, Background: in.Background, Radius: in.Radius}
			var theme hudthemes.Theme
			if in.ID == "" {
				if _, ok := customhud.Lookup(in.Base); !ok {
					return nil, userHUDThemeOut{}, fmt.Errorf("base must be a catalog HUD design id from list_hud_themes, got %q", cleanName(in.Base))
				}
				theme, err = c.CreateHUDTheme(ctx, in.Base, style)
			} else {
				if err := checkUUID("id", in.ID); err != nil {
					return nil, userHUDThemeOut{}, err
				}
				theme, err = c.UpdateHUDTheme(ctx, in.ID, style)
			}
			if err != nil {
				return nil, userHUDThemeOut{}, toolError(err)
			}
			return nil, userHUDTheme(theme), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "delete_hud_theme", Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true)},
		Description: "Delete one of the user's own HUD themes. Plans already made keep their copy of it. It cannot be undone, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in deleteHUDThemeIn) (*mcp.CallToolResult, deleteOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, deleteOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			list, err := c.ListHUDThemes(ctx)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			var theme *hudthemes.Theme
			for i := range list.Themes {
				if list.Themes[i].ID.String() == in.ID {
					theme = &list.Themes[i]
				}
			}
			if theme == nil {
				return nil, deleteOut{}, fmt.Errorf("no HUD theme with id %s; list_hud_themes shows the saved ones", in.ID)
			}
			summary := fmt.Sprintf("Borrar el tema de HUD %s. No se puede deshacer.", quotedName(theme.Name, "sin nombre"))
			return approveAndDelete(req, approvalSpec{Tool: "delete_hud_theme", JobID: in.ID, Request: theme.UpdatedAt, Summary: summary, Given: in.Approve, Question: "¿Borrar ahora?"},
				func() error { return c.DeleteHUDTheme(ctx, in.ID) })
		})
}

// resolveUserHUD turns a user theme id in hud into its catalog base and style.
func resolveUserHUD(ctx context.Context, c *tuiclient.Client, in *planFullDemoIn) error {
	id, err := uuid.Parse(strings.TrimSpace(in.HUD))
	if err != nil {
		return nil
	}
	list, err := c.ListHUDThemes(ctx)
	if err != nil {
		return toolError(err)
	}
	for _, t := range list.Themes {
		if t.ID == id {
			style := t.Style
			in.HUD, in.custom = t.Base, &style
			return nil
		}
	}
	return fmt.Errorf("no HUD theme with id %s; list_hud_themes shows the saved ones", id)
}

// applyHUDLook sets the composition switches and team names a custom HUD draws.
func applyHUDLook(options *recapplan.Options, in hudLookIn) error {
	if in == (hudLookIn{}) {
		return nil
	}
	if options.Overlays.HUDTheme == "" {
		return fmt.Errorf("compact_header, hide_inventory, round_win_banner and team names need a custom HUD, not the native CS2 HUD")
	}
	look := &options.Overlays.HUDOptions
	for _, choice := range []struct {
		value  *bool
		target *bool
	}{{in.CompactHeader, &look.CompactHeader}, {in.HideInventory, &look.HideLoadout}, {in.RoundWinBanner, &look.RoundWinBanner}} {
		if choice.value != nil {
			*choice.target = *choice.value
		}
	}
	for _, name := range []struct {
		value  *string
		target *string
	}{{in.TeamCTStart, &options.Overlays.TeamNames.CTStart}, {in.TeamTStart, &options.Overlays.TeamNames.TStart}} {
		if name.value != nil {
			*name.target = strings.TrimSpace(*name.value)
		}
	}
	return nil
}

// fetchUserHUDThemes lists the saved themes; a failure only leaves them out.
func fetchUserHUDThemes(ctx context.Context, s *studio) ([]hudthemes.Theme, string) {
	c, err := s.client(ctx)
	if err != nil {
		return nil, "Your own HUD themes could not be read: " + err.Error()
	}
	list, err := c.ListHUDThemes(ctx)
	if err != nil {
		return nil, "Your own HUD themes could not be read: " + err.Error()
	}
	return list.Themes, ""
}

// describeHUDLook is the plan summary line for a user theme and the switches.
func describeHUDLook(o recapplan.OverlayOptions) string {
	var parts []string
	look := o.HUDOptions
	if look.CompactHeader {
		parts = append(parts, "cabecera compacta")
	}
	if look.HideLoadout {
		parts = append(parts, "sin inventario")
	}
	if look.RoundWinBanner {
		parts = append(parts, "mensaje de ronda ganada")
	}
	if names := strings.TrimSpace(o.TeamNames.CTStart + " " + o.TeamNames.TStart); names != "" {
		parts = append(parts, fmt.Sprintf("equipos %s y %s en el marcador", cleanName(fallback(o.TeamNames.CTStart, "CT")), cleanName(fallback(o.TeamNames.TStart, "T"))))
	}
	if len(parts) == 0 {
		return ""
	}
	return " HUD con " + strings.Join(parts, ", ") + "."
}

func fallback(value, other string) string {
	if value == "" {
		return other
	}
	return value
}
