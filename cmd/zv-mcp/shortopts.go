package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// maxSongAudioBytes caps get_song_audio: the base64 payload travels in one MCP message.
const maxSongAudioBytes = 8 << 20

const (
	coverGenerated = "generated-gameplay"
	coverNone      = "no-cover"
)

type shortOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Default     bool   `json:"default,omitempty"`
}

// The ids below are the constants of internal/renderplan (EditRequest); the
// defaults are DefaultEditRequest and the Studio UI's DEFAULT_EDIT_CONFIG.
var (
	formatOptions = []shortOption{
		{ID: "short-9x16", Label: "Vertical", Description: "Vertical para Shorts, TikTok y Reels", Default: true},
		{ID: "landscape-16x9", Label: "Horizontal", Description: "Apaisado para YouTube y pantallas grandes"},
	}
	killEffectOptions = []shortOption{
		{ID: "punch-in", Label: "Impacto", Description: "Zoom rápido de impacto en cada baja", Default: true},
		{ID: "clean", Label: "Limpio", Description: "Sin efectos, solo la jugada tal cual"},
		{ID: "velocity", Label: "Velocidad", Description: "Cambios de velocidad alrededor de cada baja"},
		{ID: "freeze-flash", Label: "Congelado", Description: "Imagen congelada con destello en la baja"},
		{ID: "shake", Label: "Terremoto", Description: "Sacudida de cámara en cada baja"},
		{ID: "glitch", Label: "Glitch", Description: "Distorsión digital en el momento de la baja"},
	}
	transitionOptions = []shortOption{
		{ID: "flash", Label: "Destello", Description: "Destello blanco entre una jugada y otra", Default: true},
		{ID: "cut", Label: "Corte", Description: "Corte directo sin efecto entre jugadas"},
		{ID: "whip", Label: "Barrido", Description: "Barrido rápido de cámara entre jugadas"},
		{ID: "dip", Label: "Fundido", Description: "Fundido suave entre una jugada y otra"},
		{ID: "glitch", Label: "Glitch", Description: "Distorsión digital entre una jugada y otra"},
		{ID: "zoom-whip", Label: "Zoom-whip", Description: "Zoom con barrido entre una jugada y otra"},
	}
)

type shortToggle struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Default     bool   `json:"default"`
}

var toggleOptions = []shortToggle{
	{ID: "intro", Label: "Apertura", Description: "Tarjeta de apertura con titular generado"},
	{ID: "outro", Label: "Cierre", Description: "Tarjeta de cierre al final del vídeo"},
	{ID: "hook_text", Label: "Título automático", Description: "Texto gancho al inicio del vídeo"},
	{ID: "kill_counter", Label: "Contador de kills", Description: "Contador de bajas visible durante la jugada"},
	{ID: "voice_comms", Label: "Comms del equipo", Description: "Mezcla las voces del equipo en el audio"},
	{ID: "native_hud", Label: "HUD nativo", Description: "Graba con el HUD original del juego"},
	{ID: "cover", Label: "Portada", Description: "Genera candidatos de portada desde el gameplay", Default: true},
}

type musicOption struct {
	ID              string `json:"id"`
	Title           string `json:"title" jsonschema:"untrusted track title"`
	Artist          string `json:"artist,omitempty"`
	Genre           string `json:"genre,omitempty"`
	DurationSeconds int    `json:"duration_seconds,omitempty"`
}

type shortOptionsOut struct {
	Format     []shortOption `json:"format"`
	KillEffect []shortOption `json:"kill_effect"`
	Transition []shortOption `json:"transition"`
	Music      []musicOption `json:"music" jsonschema:"track ids for the music field; leaving music unset renders without music"`
	Toggles    []shortToggle `json:"toggles" jsonschema:"on/off extras; unset keeps each default"`
}

// shortChoices are the optional edit fields of generate_short. Unset fields keep the product defaults.
type shortChoices struct {
	Format      string `json:"format,omitempty" jsonschema:"output format id from list_short_options; default short-9x16"`
	KillEffect  string `json:"kill_effect,omitempty" jsonschema:"kill effect id from list_short_options; default punch-in"`
	Transition  string `json:"transition,omitempty" jsonschema:"transition id from list_short_options; default flash"`
	Music       string `json:"music,omitempty" jsonschema:"music track id from list_short_options; default no music"`
	Intro       *bool  `json:"intro,omitempty" jsonschema:"opening card; default false"`
	Outro       *bool  `json:"outro,omitempty" jsonschema:"closing card; default false"`
	HookText    *bool  `json:"hook_text,omitempty" jsonschema:"automatic hook title; default false"`
	KillCounter *bool  `json:"kill_counter,omitempty" jsonschema:"kill counter overlay; default false"`
	VoiceComms  *bool  `json:"voice_comms,omitempty" jsonschema:"mix team voice comms; default false"`
	NativeHUD   *bool  `json:"native_hud,omitempty" jsonschema:"record with the native game HUD; default false"`
	Cover       *bool  `json:"cover,omitempty" jsonschema:"generate cover candidates; default true"`
}

type songIn struct {
	SongID string `json:"song_id" jsonschema:"track id from list_short_options"`
}

func registerShortOptions(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "list_short_options", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "List the choices generate_short accepts besides the plays: format, kill effect, transition, music tracks and on/off extras, with the Studio defaults. " +
			"Labels and descriptions are in Spanish; ask the user which they want instead of picking for them."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, shortOptionsOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, shortOptionsOut{}, toolError(err)
			}
			songs, err := c.ListSongs(ctx)
			if err != nil {
				return nil, shortOptionsOut{}, toolError(err)
			}
			return nil, shortOptionsOut{Format: formatOptions, KillEffect: killEffectOptions, Transition: transitionOptions,
				Music: musicOptions(songs), Toggles: toggleOptions}, nil
		})

	// Only the Short picker app plays previews; MCP Apps hosts hide app-only tools from the model.
	mcp.AddTool(server, &mcp.Tool{Name: "get_song_audio", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Meta:        mcp.Meta{"ui": map[string]any{"visibility": []string{"app"}}},
		Description: "Return one music track as audio, so the user can preview it. Tracks above 8 MiB are refused."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in songIn) (*mcp.CallToolResult, any, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, nil, toolError(err)
			}
			songs, err := c.ListSongs(ctx)
			if err != nil {
				return nil, nil, toolError(err)
			}
			if err := checkMusic(in.SongID, songs); err != nil {
				return nil, nil, err
			}
			data, mime, err := c.SongAudio(ctx, in.SongID, maxSongAudioBytes)
			if errors.Is(err, tuiclient.ErrAudioTooLarge) {
				return nil, nil, fmt.Errorf("track %q is larger than %d MiB and cannot be returned as audio", in.SongID, maxSongAudioBytes>>20)
			}
			if err != nil {
				return nil, nil, toolError(err)
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.AudioContent{Data: data, MIMEType: mime}}}, nil, nil
		})
}

func musicOptions(songs []tuiclient.Song) []musicOption {
	out := make([]musicOption, 0, len(songs))
	for _, s := range songs {
		out = append(out, musicOption{ID: s.ID, Title: cleanName(s.Title), Artist: cleanName(s.Artist), Genre: cleanName(s.Genre), DurationSeconds: s.DurationSec})
	}
	return out
}

func optionIDs(options []shortOption) []string {
	ids := make([]string, 0, len(options))
	for _, o := range options {
		ids = append(ids, o.ID)
	}
	return ids
}

// pickOption returns the chosen id, or the default one when unset.
func pickOption(field, chosen string, options []shortOption) (string, error) {
	if chosen == "" {
		for _, o := range options {
			if o.Default {
				return o.ID, nil
			}
		}
		return "", fmt.Errorf("%s has no default", field)
	}
	ids := optionIDs(options)
	if !slices.Contains(ids, chosen) {
		return "", fmt.Errorf("unknown %s %q; valid ids: %s", field, cleanName(chosen), strings.Join(ids, ", "))
	}
	return chosen, nil
}

func checkMusic(id string, songs []tuiclient.Song) error {
	ids := make([]string, 0, len(songs))
	for _, s := range songs {
		if s.ID == id {
			return nil
		}
		ids = append(ids, s.ID)
	}
	if len(ids) == 0 {
		return fmt.Errorf("unknown music %q; Studio has no music tracks", cleanName(id))
	}
	return fmt.Errorf("unknown music %q; valid ids: %s", cleanName(id), strings.Join(ids, ", "))
}

func orDefault(chosen *bool, def bool) bool {
	if chosen == nil {
		return def
	}
	return *chosen
}

// resolveEdit validates the choices and fills every unset field with the product default.
func resolveEdit(c shortChoices) (tuiclient.ShortEdit, error) {
	var edit tuiclient.ShortEdit
	var err error
	if edit.Format, err = pickOption("format", c.Format, formatOptions); err != nil {
		return edit, err
	}
	if edit.KillEffect, err = pickOption("kill_effect", c.KillEffect, killEffectOptions); err != nil {
		return edit, err
	}
	if edit.Transition, err = pickOption("transition", c.Transition, transitionOptions); err != nil {
		return edit, err
	}
	edit.Intro, edit.Outro = orDefault(c.Intro, false), orDefault(c.Outro, false)
	edit.HookText, edit.KillCounter = orDefault(c.HookText, false), orDefault(c.KillCounter, false)
	edit.VoiceComms, edit.NativeHUD = orDefault(c.VoiceComms, false), orDefault(c.NativeHUD, false)
	edit.CoverStrategy = coverNone
	if orDefault(c.Cover, true) {
		edit.CoverStrategy = coverGenerated
	}
	return edit, nil
}

// describeShortRequest is the part of the approval summary the user agrees to.
func describeShortRequest(req tuiclient.GenerateRequest) string {
	e := req.Edit
	music := req.Music
	if music == "" {
		music = "none"
	}
	var extras []string
	for _, x := range []struct {
		name string
		on   bool
	}{{"intro", e.Intro}, {"outro", e.Outro}, {"hook_text", e.HookText}, {"kill_counter", e.KillCounter},
		{"voice_comms", e.VoiceComms}, {"native_hud", e.NativeHUD}, {"cover", e.CoverStrategy == coverGenerated}} {
		if x.on {
			extras = append(extras, x.name)
		}
	}
	if len(extras) == 0 {
		extras = []string{"none"}
	}
	return fmt.Sprintf("format %s, kill effect %s, transition %s, music %s, extras: %s",
		e.Format, e.KillEffect, e.Transition, music, strings.Join(extras, ", "))
}
