package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/keydropbanner"
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

// replaySpeedOptions are renderplan.VictimReplaySpeeds; the id is the victim_replay_speed value.
var replaySpeedOptions = []shortOption{
	{ID: "0.25", Label: "0,25x", Description: "Muy lenta, cuatro veces más larga"},
	{ID: "0.5", Label: "0,5x", Description: "Cámara lenta", Default: true},
	{ID: "1", Label: "1x", Description: "A velocidad real"},
}

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
	{ID: "play_badges", Label: "Insignias de jugada", Description: "Nombra el clutch (1vX, ganado si se sabe), el ace o la multikill de cada jugada"},
	{ID: "win_probability", Label: "Probabilidad de victoria", Description: "Tras el plantado, el % de victoria de cada equipo (con 5 s o más de bomba y 2 o más vivos por bando)"},
	{ID: "stats_card", Label: "Tarjeta de estadísticas", Description: "Cierra el Short con K/D, ADR, HS % y rating de ClipHub del jugador, y su nivel FACEIT si lo hay (solo 9:16)"},
	{ID: "centered_killfeed", Label: "Kill feed centrada arriba", Description: "Dibuja tus kills en el centro, arriba del Short (solo 9:16)"},
	{ID: "voice_comms", Label: "Comms del equipo", Description: "Mezcla las voces del equipo en el audio"},
	{ID: "native_hud", Label: "HUD nativo", Description: "Graba con el HUD original del juego"},
	{ID: "victim_replay", Label: "Repetición desde la víctima", Description: "Tras cada jugada, la última baja otra vez vista por quien muere, a la velocidad elegida"},
	{ID: "cover", Label: "Portada", Description: "Genera candidatos de portada desde el gameplay", Default: true},
	{ID: "brand_watermark", Label: "Marca de agua", Description: "Dibuja el logo del kit de marca en su esquina y con su opacidad (necesita un logo guardado)"},
	{ID: "brand_banner", Label: "Banner de sponsor", Description: "Muestra el banner del kit de marca en su ventana de tiempo (necesita un banner guardado; no va con el banner de afiliado)"},
}

// bannerFamilies mirror AFFILIATE_FAMILY_CATALOG of the Studio UI (web/lib/api/types.ts).
var bannerFamilies = []bannerFamily{
	{ID: keydropbanner.FamilyKeyDrop, Label: "KeyDrop", Styles: []shortOption{
		{ID: "operator", Label: "Operator", Description: "KeyDrop táctico"},
		{ID: "classic", Label: "Classic", Description: "Promo con regalo"},
		{ID: "tigerr", Label: "Tigerr", Description: "Tiger Tooth naranja"},
		{ID: "jcorko", Label: "Jcorko", Description: "Cebra azul"},
	}},
	{ID: keydropbanner.FamilyCSGOSkins, Label: "CSGOSkins", Styles: []shortOption{
		{ID: "classic", Label: "Classic", Description: "CSGOSkins promo"},
		{ID: "operator", Label: "Operator", Description: "CSGOSkins táctico"},
	}},
}

// Defaults of the Short editor: the banner shows from 0 to 4 s.
const (
	bannerStartDefault  = 0.0
	bannerEndDefault    = 4.0
	minMusicVolume      = 5
	maxBookendTextBytes = 80
)

type bannerFamily struct {
	ID     string        `json:"id" jsonschema:"value for banner_family"`
	Label  string        `json:"label"`
	Styles []shortOption `json:"styles" jsonschema:"values for banner_style"`
}

type musicOption struct {
	ID              string `json:"id"`
	Title           string `json:"title" jsonschema:"untrusted track title"`
	Artist          string `json:"artist,omitempty"`
	Genre           string `json:"genre,omitempty"`
	DurationSeconds int    `json:"duration_seconds,omitempty"`
}

type shortOptionsOut struct {
	Format     []shortOption  `json:"format"`
	KillEffect []shortOption  `json:"kill_effect"`
	Transition []shortOption  `json:"transition"`
	Music      []musicOption  `json:"music" jsonschema:"track ids for the music field; leaving music unset renders without music"`
	Toggles    []shortToggle  `json:"toggles" jsonschema:"on/off extras; unset keeps each default"`
	Banners    []bannerFamily `json:"banners" jsonschema:"affiliate banner (Banner afiliado) families and styles; no banner unless banner_style is set"`

	// ReplaySpeed lists the victim_replay_speed values; the id is the number to pass.
	ReplaySpeed []shortOption `json:"replay_speed" jsonschema:"values for victim_replay_speed, used only with victim_replay"`
}

// shortChoices are the optional edit fields of generate_short. Unset fields keep the product defaults.
type shortChoices struct {
	Format       string `json:"format,omitempty" jsonschema:"output format id from list_short_options; default short-9x16"`
	KillEffect   string `json:"kill_effect,omitempty" jsonschema:"kill effect id from list_short_options; default punch-in"`
	Transition   string `json:"transition,omitempty" jsonschema:"transition id from list_short_options; default flash"`
	Music        string `json:"music,omitempty" jsonschema:"music track id from list_short_options; default no music"`
	Intro        *bool  `json:"intro,omitempty" jsonschema:"opening card; default false"`
	Outro        *bool  `json:"outro,omitempty" jsonschema:"closing card; default false"`
	HookText     *bool  `json:"hook_text,omitempty" jsonschema:"automatic hook title; default false"`
	KillCounter  *bool  `json:"kill_counter,omitempty" jsonschema:"kill counter overlay; default false"`
	VoiceComms   *bool  `json:"voice_comms,omitempty" jsonschema:"mix team voice comms; default false"`
	NativeHUD    *bool  `json:"native_hud,omitempty" jsonschema:"record with the native game HUD; default false"`
	VictimReplay *bool  `json:"victim_replay,omitempty" jsonschema:"after each play, replay its last kill from the victim's view; default false"`
	Cover        *bool  `json:"cover,omitempty" jsonschema:"generate cover candidates; default true"`
	// CenteredKillfeed is rejected outside short-9x16, like the server does.
	CenteredKillfeed *bool    `json:"centered_killfeed,omitempty" jsonschema:"draw the kill feed centered near the top; short-9x16 only; default false"`
	MusicVolume      *int     `json:"music_volume,omitempty" jsonschema:"music level in percent, 5 to 100; needs music; default 100"`
	GameVolume       *int     `json:"game_volume,omitempty" jsonschema:"game sound level under the music in percent, 0 to 100; needs music; default 70"`
	IntroText        string   `json:"intro_text,omitempty" jsonschema:"opening card title, at most 80 bytes; needs intro; empty uses the generated headline"`
	OutroText        string   `json:"outro_text,omitempty" jsonschema:"closing card text such as the user handle, at most 80 bytes; needs outro; empty shows ClipHub"`
	BannerFamily     string   `json:"banner_family,omitempty" jsonschema:"affiliate banner family from list_short_options; default KEYDROP when banner_style is set"`
	BannerStyle      string   `json:"banner_style,omitempty" jsonschema:"affiliate banner style from list_short_options; empty means no banner"`
	BannerCode       string   `json:"banner_code,omitempty" jsonschema:"affiliate code on the banner: letters, digits, _ or -, at most 16; empty uses Studio's default code"`
	BannerStart      *float64 `json:"banner_start_seconds,omitempty" jsonschema:"second of the Short when the banner appears; default 0"`
	BannerEnd        *float64 `json:"banner_end_seconds,omitempty" jsonschema:"second of the Short when the banner leaves; default 4"`

	PlayBadges     *bool `json:"play_badges,omitempty" jsonschema:"badge naming each play's clutch (1vX, GANADO when the round was won), ace or multikill; default false"`
	WinProbability *bool `json:"win_probability,omitempty" jsonschema:"after the plant, each team's round-win chance (ClipHub's wp1 model) while the bomb has 5 s or more left and each side has 2 or more alive; plays from jobs parsed before this option show nothing; default false"`
	StatsCard      *bool `json:"stats_card,omitempty" jsonschema:"closing card with the player's K/D, ADR, HS% and ClipHub rating (never an HLTV rating), plus FACEIT level, win rate and best map when FACEIT has them; short-9x16 only; default false"`
	// VictimReplaySpeed is rejected without victim_replay.
	VictimReplaySpeed *float64 `json:"victim_replay_speed,omitempty" jsonschema:"victim replay speed: 0.25, 0.5 or 1; needs victim_replay; default 0.5"`
	BrandWatermark    *bool    `json:"brand_watermark,omitempty" jsonschema:"draw the channel logo of the brand kit (get_brand_kit) as a watermark in its corner and opacity; needs a saved logo; default false"`
	BrandBanner       *bool    `json:"brand_banner,omitempty" jsonschema:"show the sponsor banner of the brand kit in its time window; needs a saved banner; not with banner_style; default false"`
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
				Music: musicOptions(songs), Toggles: toggleOptions, ReplaySpeed: replaySpeedOptions, Banners: bannerFamilies}, nil
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
	edit.VictimReplay = orDefault(c.VictimReplay, false)
	edit.CenteredKillfeed = orDefault(c.CenteredKillfeed, false)
	if edit.CenteredKillfeed && edit.Format != "short-9x16" {
		return edit, fmt.Errorf("centered_killfeed needs format short-9x16")
	}
	edit.PlayBadges, edit.StatsCard = orDefault(c.PlayBadges, false), orDefault(c.StatsCard, false)
	edit.WinProbability = orDefault(c.WinProbability, false)
	if edit.StatsCard && edit.Format != "short-9x16" {
		return edit, fmt.Errorf("stats_card needs format short-9x16")
	}
	if err := resolveReplaySpeed(&edit, c.VictimReplaySpeed); err != nil {
		return edit, err
	}
	edit.CoverStrategy = coverNone
	if orDefault(c.Cover, true) {
		edit.CoverStrategy = coverGenerated
	}
	if err := resolveBookendText(&edit, c); err != nil {
		return edit, err
	}
	edit.BrandWatermark, edit.BrandBanner = orDefault(c.BrandWatermark, false), orDefault(c.BrandBanner, false)
	if edit.BrandBanner && c.BannerStyle != "" {
		return edit, errors.New("brand_banner and banner_style both put a banner in the same place; choose one")
	}
	return edit, resolveBanner(&edit, c)
}

// resolveReplaySpeed keeps the default half speed unset, as the Studio UI sends it.
func resolveReplaySpeed(edit *tuiclient.ShortEdit, speed *float64) error {
	if speed == nil {
		return nil
	}
	if !edit.VictimReplay {
		return errors.New("victim_replay_speed needs victim_replay set to true")
	}
	id := strconv.FormatFloat(*speed, 'f', -1, 64)
	if _, err := pickOption("victim_replay_speed", id, replaySpeedOptions); err != nil {
		return err
	}
	if id != "0.5" {
		edit.VictimReplaySpeed = *speed
	}
	return nil
}

func resolveBookendText(edit *tuiclient.ShortEdit, c shortChoices) error {
	for _, t := range []struct {
		field, card, text string
		on                bool
		dst               *string
	}{{"intro_text", "intro", c.IntroText, edit.Intro, &edit.IntroText}, {"outro_text", "outro", c.OutroText, edit.Outro, &edit.OutroText}} {
		text := strings.TrimSpace(t.text)
		if text == "" {
			continue
		}
		if !t.on {
			return fmt.Errorf("%s needs %s set to true", t.field, t.card)
		}
		if len(text) > maxBookendTextBytes || strings.ContainsFunc(text, unicode.IsControl) {
			return fmt.Errorf("%s must be one line of at most %d bytes", t.field, maxBookendTextBytes)
		}
		*t.dst = text
	}
	return nil
}

func resolveBanner(edit *tuiclient.ShortEdit, c shortChoices) error {
	if c.BannerStyle == "" {
		if c.BannerFamily != "" || c.BannerCode != "" || c.BannerStart != nil || c.BannerEnd != nil {
			return errors.New("the banner fields need banner_style; leave them all empty for no banner")
		}
		return nil
	}
	family := cmp.Or(keydropbanner.NormalizeFamily(c.BannerFamily), keydropbanner.FamilyKeyDrop)
	i := slices.IndexFunc(bannerFamilies, func(f bannerFamily) bool { return f.ID == family })
	if i < 0 {
		return fmt.Errorf("unknown banner_family %q; valid ids: %s, %s", cleanName(c.BannerFamily), keydropbanner.FamilyKeyDrop, keydropbanner.FamilyCSGOSkins)
	}
	style, err := pickOption("banner_style", strings.ToLower(strings.TrimSpace(c.BannerStyle)), bannerFamilies[i].Styles)
	if err != nil {
		return err
	}
	code := keydropbanner.NormalizeCode(c.BannerCode)
	if err := keydropbanner.ValidateCode(code); err != nil {
		return fmt.Errorf("banner_code: %w", err)
	}
	start, end := orDefaultFloat(c.BannerStart, bannerStartDefault), orDefaultFloat(c.BannerEnd, bannerEndDefault)
	if start < 0 || end <= start {
		return errors.New("the banner needs 0 <= banner_start_seconds < banner_end_seconds")
	}
	edit.KeyDropFamily, edit.KeyDropStyle, edit.KeyDropCode = family, style, code
	edit.KeyDropStartSeconds, edit.KeyDropEndSeconds = &start, &end
	return nil
}

func orDefaultFloat(chosen *float64, def float64) float64 {
	if chosen == nil {
		return def
	}
	return *chosen
}

// resolveMusic builds the music field: percents in the tool, gains on the wire.
func resolveMusic(c shortChoices) (*tuiclient.ShortMusic, error) {
	if c.Music == "" {
		if c.MusicVolume != nil || c.GameVolume != nil {
			return nil, errors.New("music_volume and game_volume need a music track")
		}
		return nil, nil
	}
	music := &tuiclient.ShortMusic{Key: c.Music}
	if v := c.MusicVolume; v != nil {
		if *v < minMusicVolume || *v > 100 {
			return nil, fmt.Errorf("music_volume goes from %d to 100", minMusicVolume)
		}
		// Full volume stays unset, as the Studio UI sends it.
		if *v < 100 {
			music.Volume = float64(*v) / 100
		}
	}
	if v := c.GameVolume; v != nil {
		if *v < 0 || *v > 100 {
			return nil, errors.New("game_volume goes from 0 to 100")
		}
		gain := float64(*v) / 100
		music.GameVolume = &gain
	}
	return music, nil
}

// describeShortRequest is the part of the approval summary the user agrees to.
func describeShortRequest(req tuiclient.GenerateRequest) string {
	e := req.Edit
	music := "ninguna"
	if m := req.Music; m != nil {
		music = m.Key
		if m.Volume > 0 || m.GameVolume != nil {
			game := 70.0
			if m.GameVolume != nil {
				game = *m.GameVolume * 100
			}
			music += fmt.Sprintf(" (música %.0f %%, juego %.0f %%)", cmp.Or(m.Volume, 1)*100, game)
		}
	}
	var extras []string
	for _, x := range []struct {
		name string
		on   bool
	}{{"intro", e.Intro}, {"outro", e.Outro}, {"hook_text", e.HookText}, {"kill_counter", e.KillCounter}, {"play_badges", e.PlayBadges}, {"win_probability", e.WinProbability}, {"stats_card", e.StatsCard}, {"centered_killfeed", e.CenteredKillfeed},
		{"voice_comms", e.VoiceComms}, {"native_hud", e.NativeHUD}, {"victim_replay", e.VictimReplay}, {"cover", e.CoverStrategy == coverGenerated},
		{"brand_watermark", e.BrandWatermark}, {"brand_banner", e.BrandBanner}} {
		if x.on {
			extras = append(extras, x.name)
		}
	}
	if len(extras) == 0 {
		extras = []string{"ninguno"}
	}
	out := fmt.Sprintf("formato %s, efecto de baja %s, transición %s, música %s, extras: %s",
		e.Format, e.KillEffect, e.Transition, music, strings.Join(extras, ", "))
	if e.VictimReplay {
		out += fmt.Sprintf(", repetición a %gx", cmp.Or(e.VictimReplaySpeed, 0.5))
	}
	if e.IntroText != "" {
		out += fmt.Sprintf(", apertura «%s»", e.IntroText)
	}
	if e.OutroText != "" {
		out += fmt.Sprintf(", cierre «%s»", e.OutroText)
	}
	if e.KeyDropStyle != "" && e.KeyDropStartSeconds != nil && e.KeyDropEndSeconds != nil {
		out += fmt.Sprintf(", banner %s %s «%s» de %g s a %g s", keydropbanner.FamilyLabel(e.KeyDropFamily), e.KeyDropStyle,
			keydropbanner.DisplayLabelFor(e.KeyDropFamily, e.KeyDropStyle, e.KeyDropCode), *e.KeyDropStartSeconds, *e.KeyDropEndSeconds)
	}
	return out
}
