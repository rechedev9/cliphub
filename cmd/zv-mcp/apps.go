package main

import (
	"context"
	"embed"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/customhud"
)

// MCP Apps (ext-apps, spec 2026-01-26): a tool names a ui:// resource in
// _meta.ui.resourceUri and the host renders it in a sandboxed iframe.
const (
	appMIMEType        = "text/html;profile=mcp-app"
	shortOptionsAppURI = "ui://cliphub/short-options"
	hudGalleryAppURI   = "ui://cliphub/hud-gallery"
	jobProgressAppURI  = "ui://cliphub/job-progress"
)

//go:embed apps/*.html apps/*.js apps/*.css
var appFiles embed.FS

type appPage struct {
	uri, name, title, description, file string
	// bundled pages come whole from mcp-apps/scripts/build.mjs and skip composeAppPage.
	bundled bool
}

var appPages = []appPage{
	{shortOptionsAppURI, "short-options", "Opciones del Short", "Picker for the format, effects, transition, music and extras of a Short.", "short-options.html", false},
	{hudGalleryAppURI, "hud-gallery", "HUD de la Demo completa", "Gallery of the HUD designs a Full Demo can use.", "hud-gallery.html", false},
	{jobProgressAppURI, "job-progress", "Progreso de ClipHub Studio", "Live progress bar of a demo import, a parse, a capture or a render.", "job-progress.html", true},
}

type chooseShortIn struct {
	JobID      string   `json:"job_id" jsonschema:"a parsed job from create_job"`
	SegmentIDs []string `json:"segment_ids" jsonschema:"segment ids from list_moments"`
}

type chooseShortOut struct {
	JobID      string   `json:"job_id"`
	SegmentIDs []string `json:"segment_ids"`
	shortOptionsOut
}

type chooseHUDIn struct {
	JobID string `json:"job_id" jsonschema:"a parsed job from create_job"`
}

type hudOption struct {
	ID          string `json:"id" jsonschema:"value for plan_full_demo hud"`
	Name        string `json:"name"`
	Description string `json:"description" jsonschema:"one line, in Spanish"`
}

type chooseHUDOut struct {
	JobID  string      `json:"job_id"`
	Themes []hudOption `json:"themes"`
}

func registerApps(server *mcp.Server, s *studio) {
	for _, p := range appPages {
		html, err := appPageHTML(p)
		if err != nil {
			// The files are embedded, so this is a build defect, not a runtime state.
			panic(err)
		}
		server.AddResource(&mcp.Resource{URI: p.uri, Name: p.name, Title: p.title, Description: p.description, MIMEType: appMIMEType},
			func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
				return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: p.uri, MIMEType: appMIMEType, Text: html}}}, nil
			})
	}

	mcp.AddTool(server, &mcp.Tool{Name: "choose_short_options", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}, Meta: appToolMeta(shortOptionsAppURI),
		Description: "Let the user choose how a Short looks (format, kill effect, transition, music, extras) in an interactive picker, for the plays segment_ids of job_id. " +
			"Without a UI the result lists the choices: ask the user in chat, then call generate_short. With the picker, the user's Generar button starts generate_short itself, so do not call it again."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in chooseShortIn) (*mcp.CallToolResult, chooseShortOut, error) {
			if err := checkUUID("job_id", in.JobID); err != nil {
				return nil, chooseShortOut{}, err
			}
			if err := checkSegmentIDs(in.SegmentIDs); err != nil {
				return nil, chooseShortOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, chooseShortOut{}, toolError(err)
			}
			songs, err := c.ListSongs(ctx)
			if err != nil {
				return nil, chooseShortOut{}, toolError(err)
			}
			out := chooseShortOut{JobID: in.JobID, SegmentIDs: in.SegmentIDs, shortOptionsOut: shortOptionsOut{
				Format: formatOptions, KillEffect: killEffectOptions, Transition: transitionOptions, Music: musicOptions(songs), Toggles: toggleOptions, Banners: bannerFamilies}}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: describeShortChoices(out.shortOptionsOut)}}}, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "choose_hud", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}, Meta: appToolMeta(hudGalleryAppURI),
		Description: "Let the user pick the HUD design of a Full Demo for job_id from a gallery of previews. " +
			"Without a UI the result lists the designs: ask the user in chat, then call plan_full_demo with hud. " +
			"With the gallery, the user's confirmation runs plan_full_demo itself and reports the plan_hash back; then follow with render_full_demo."},
		func(_ context.Context, _ *mcp.CallToolRequest, in chooseHUDIn) (*mcp.CallToolResult, chooseHUDOut, error) {
			if err := checkUUID("job_id", in.JobID); err != nil {
				return nil, chooseHUDOut{}, err
			}
			out := chooseHUDOut{JobID: in.JobID, Themes: hudOptions()}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: describeHUDChoices(out.Themes)}}}, out, nil
		})
}

// appToolMeta links a tool to its UI. The flat "ui/resourceUri" key is deprecated
// in the spec, but the reference SDK still emits it beside _meta.ui for older hosts.
func appToolMeta(uri string) mcp.Meta {
	return mcp.Meta{"ui": map[string]any{"resourceUri": uri}, "ui/resourceUri": uri}
}

func appPageHTML(p appPage) (string, error) {
	if !p.bundled {
		return composeAppPage(p.file)
	}
	page, err := appFiles.ReadFile("apps/" + p.file)
	return string(page), err
}

// composeAppPage inlines the shared stylesheet and bridge into a page, so each
// page stays one self-contained document under the default CSP.
func composeAppPage(file string) (string, error) {
	page, err := appFiles.ReadFile("apps/" + file)
	if err != nil {
		return "", err
	}
	css, err := appFiles.ReadFile("apps/base.css")
	if err != nil {
		return "", err
	}
	js, err := appFiles.ReadFile("apps/bridge.js")
	if err != nil {
		return "", err
	}
	html := string(page)
	for _, slot := range []struct{ marker, body string }{{"/*BASE_CSS*/", string(css)}, {"/*BRIDGE_JS*/", string(js)}} {
		if !strings.Contains(html, slot.marker) {
			return "", fmt.Errorf("apps/%s lacks the %s marker", file, slot.marker)
		}
		html = strings.Replace(html, slot.marker, slot.body, 1)
	}
	return html, nil
}

func hudOptions() []hudOption {
	themes := customhud.Themes()
	out := make([]hudOption, 0, len(themes)+1)
	for _, t := range themes {
		out = append(out, hudOption{ID: t.ID, Name: t.Name, Description: t.Description})
	}
	return append(out, hudOption{ID: nativeHUDID, Name: nativeHUDName, Description: nativeHUDText})
}

func describeHUDChoices(themes []hudOption) string {
	var b strings.Builder
	b.WriteString("HUD designs for the Full Demo (pass the id as hud to plan_full_demo):\n")
	for _, t := range themes {
		fmt.Fprintf(&b, "- %s (%s): %s\n", t.ID, t.Name, t.Description)
	}
	return strings.TrimSpace(b.String())
}

func describeShortChoices(o shortOptionsOut) string {
	var b strings.Builder
	b.WriteString("Short options. Ask the user, then call generate_short with the chosen ids; omitted fields keep the default (marked *).\n")
	list := func(title, field string, options []shortOption) {
		fmt.Fprintf(&b, "\n%s (%s):\n", title, field)
		for _, o := range options {
			fmt.Fprintf(&b, "- %s%s (%s): %s\n", o.ID, defaultMark(o.Default), o.Label, o.Description)
		}
	}
	list("Format", "format", o.Format)
	list("Kill effect", "kill_effect", o.KillEffect)
	list("Transition", "transition", o.Transition)
	b.WriteString("\nMusic (music; default none):\n")
	if len(o.Music) == 0 {
		b.WriteString("- Studio has no music tracks\n")
	}
	for _, m := range o.Music {
		fmt.Fprintf(&b, "- %s: %s\n", m.ID, strings.Join(nonEmpty(m.Title, m.Artist, m.Genre), ", "))
	}
	b.WriteString("\nExtras (true/false):\n")
	for _, t := range o.Toggles {
		fmt.Fprintf(&b, "- %s%s (%s): %s\n", t.ID, defaultMark(t.Default), t.Label, t.Description)
	}
	b.WriteString("\nWith music: music_volume 5 to 100 (default 100), game_volume 0 to 100 (default 70).\n")
	b.WriteString("With intro or outro: intro_text, outro_text (empty keeps the generated text).\n")
	b.WriteString("\nAffiliate banner (banner_family and banner_style; none by default; banner_code, banner_start_seconds 0, banner_end_seconds 4):\n")
	for _, f := range o.Banners {
		for _, st := range f.Styles {
			fmt.Fprintf(&b, "- %s %s (%s %s): %s\n", f.ID, st.ID, f.Label, st.Label, st.Description)
		}
	}
	return strings.TrimSpace(b.String())
}

func defaultMark(isDefault bool) string {
	if isDefault {
		return "*"
	}
	return ""
}

func nonEmpty(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
