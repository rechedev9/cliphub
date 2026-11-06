package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/prosettings"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	// noneChoice drops a portrait, a bumper or the pro settings from the plan.
	noneChoice      = "none"
	focusHUDID      = "focus"
	maxPortraitSize = 10 << 20
	// maxBumperSize leaves room for the multipart envelope under the 2 GiB upload cap, as the UI does.
	maxBumperSize = 2<<30 - 2<<20
	maxTitleBytes = 200
)

var portraitExtensions = []string{".png", ".jpg", ".jpeg"}

// fullDemoExtrasIn are the plan_full_demo fields that need a file upload or the pro catalog.
type fullDemoExtrasIn struct {
	ProSettings   string `json:"pro_settings,omitempty" jsonschema:"record with a pro's resolution and viewmodel: an id from list_pro_settings, or none for the user's own 1920x1080; empty keeps Studio's choice, which picks the pro when the player is one"`
	HUDPortrait   string `json:"hud_portrait,omitempty" jsonschema:"absolute path of a PNG or JPG up to 10 MB for the player card of the focus HUD, or none to drop it"`
	IntroBumper   string `json:"intro_bumper,omitempty" jsonschema:"absolute path of an MP4 played before the demo, or none"`
	SponsorBumper string `json:"sponsor_bumper,omitempty" jsonschema:"absolute path of an MP4 played after round 2, or none"`
	OutroBumper   string `json:"outro_bumper,omitempty" jsonschema:"absolute path of an MP4 played after the demo, or none"`
}

type proSettingsOut struct {
	ID         string  `json:"id" jsonschema:"value for plan_full_demo pro_settings"`
	Name       string  `json:"name"`
	Team       string  `json:"team"`
	Resolution string  `json:"resolution" jsonschema:"such as 1280x960 stretched"`
	FOV        float64 `json:"viewmodel_fov"`
}

type proSettingsListOut struct {
	Players []proSettingsOut `json:"players"`
	Note    string           `json:"note"`
}

func registerProSettingsTool(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{Name: "list_pro_settings", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "List the pros whose resolution and viewmodel a Full Demo can record with (plan_full_demo pro_settings), from prosettings.net."},
		func(context.Context, *mcp.CallToolRequest, empty) (*mcp.CallToolResult, proSettingsListOut, error) {
			out := proSettingsListOut{Note: "Graba con la resolución y el viewmodel del pro. La mira sale de la demo. Al terminar, CS2 vuelve a la configuración del usuario."}
			for _, p := range prosettings.Catalog() {
				out.Players = append(out.Players, proSettingsOut{ID: p.ID, Name: p.Name, Team: p.Team,
					Resolution: fmt.Sprintf("%dx%d %s", p.Width, p.Height, p.Scaling), FOV: p.Viewmodel.FOV})
			}
			return nil, out, nil
		})
}

// checkFullDemoExtras validates the extras and the other choices before anything is uploaded.
func checkFullDemoExtras(in planFullDemoIn, defaults recapplan.Options) error {
	if _, err := applyFullDemoChoices(defaults, in); err != nil {
		return err
	}
	if in.ProSettings != "" && in.ProSettings != noneChoice {
		if _, ok := prosettings.Find(in.ProSettings); !ok {
			return fmt.Errorf("unknown pro_settings %q; list_pro_settings has the ids", cleanName(in.ProSettings))
		}
	}
	if in.HUDPortrait != "" && in.HUDPortrait != noneChoice {
		hud := defaults.Overlays.HUDTheme
		if in.HUD != "" {
			hud = strings.ToLower(strings.TrimSpace(in.HUD))
		}
		if hud != focusHUDID {
			return errors.New("hud_portrait is only for the focus HUD; pass hud focus")
		}
		if err := checkUploadFile("hud_portrait", in.HUDPortrait, portraitExtensions, maxPortraitSize); err != nil {
			return err
		}
	}
	for _, b := range []struct{ field, path string }{{"intro_bumper", in.IntroBumper}, {"sponsor_bumper", in.SponsorBumper}, {"outro_bumper", in.OutroBumper}} {
		if b.path != "" && b.path != noneChoice {
			if err := checkUploadFile(b.field, b.path, []string{".mp4"}, maxBumperSize); err != nil {
				return err
			}
		}
	}
	return nil
}

// applyFullDemoExtras uploads the chosen files and puts them and the pro settings in options.
func applyFullDemoExtras(ctx context.Context, c *tuiclient.Client, options *recapplan.Options, in planFullDemoIn) error {
	switch in.ProSettings {
	case "":
	case noneChoice:
		options.Capture.ProSettings = recapplan.ProSettings{}
	default:
		resolved, err := recapplan.ResolveProSettings(in.ProSettings)
		if err != nil {
			return err
		}
		options.Capture.ProSettings = resolved
	}
	switch in.HUDPortrait {
	case "":
	case noneChoice:
		options.Overlays.HUDPortrait = nil
	default:
		asset, err := c.UploadFullDemoImage(ctx, in.HUDPortrait)
		if err != nil {
			return toolError(err)
		}
		options.Overlays.HUDPortrait = &recapplan.AssetRef{ID: asset.ID, SHA256: asset.SHA256}
	}
	return applyBumpers(ctx, c, options, in)
}

func applyBumpers(ctx context.Context, c *tuiclient.Client, options *recapplan.Options, in planFullDemoIn) error {
	if in.IntroBumper == "" && in.SponsorBumper == "" && in.OutroBumper == "" {
		return nil
	}
	bumpers := recapplan.BumperOptions{}
	if options.Bumpers != nil {
		bumpers = *options.Bumpers
	}
	slot := func(path string, current recapplan.BumperSlot) (recapplan.BumperSlot, error) {
		switch path {
		case "":
			return current, nil
		case noneChoice:
			return recapplan.BumperSlot{}, nil
		}
		ref, err := uploadBumper(ctx, c, path)
		return recapplan.BumperSlot{Enabled: true, Video: ref}, err
	}
	var err error
	if bumpers.Intro, err = slot(in.IntroBumper, bumpers.Intro); err != nil {
		return err
	}
	if bumpers.Outro, err = slot(in.OutroBumper, bumpers.Outro); err != nil {
		return err
	}
	sponsor := recapplan.BumperSlot{}
	if bumpers.Sponsor != nil {
		sponsor = *bumpers.Sponsor
	}
	if sponsor, err = slot(in.SponsorBumper, sponsor); err != nil {
		return err
	}
	bumpers.Sponsor = nil
	if sponsor.Enabled {
		bumpers.Sponsor = &sponsor
	}
	options.Bumpers = &bumpers
	return nil
}

// uploadBumper declares the clip as a local file without inventing authorship or a licence, like the UI.
func uploadBumper(ctx context.Context, c *tuiclient.Client, path string) (*recapplan.AssetRef, error) {
	title := clipUTF8(filepath.Base(path), maxTitleBytes)
	provenance, err := json.Marshal(map[string]string{
		"title": title, "creator": "No declarado", "source_url": "local:" + url.PathEscape(title),
		"permission": "Archivo local aportado para esta edición; licencia no declarada.", "attribution": "",
	})
	if err != nil {
		return nil, err
	}
	raw, err := c.UploadFullDemoAsset(ctx, path, provenance)
	if err != nil {
		return nil, toolError(err)
	}
	var asset tuiclient.FullDemoAsset
	if err := json.Unmarshal(raw, &asset); err != nil || asset.ID == "" || asset.SHA256 == "" {
		return nil, errors.New("Studio did not return a reference for the uploaded clip")
	}
	return &recapplan.AssetRef{ID: asset.ID, SHA256: asset.SHA256}, nil
}

func clipUTF8(s string, maxBytes int) string {
	for len(s) > maxBytes {
		_, size := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-size]
	}
	return s
}

func checkUploadFile(field, path string, extensions []string, maxSize int64) error {
	if !filepath.IsAbs(path) || strings.HasPrefix(path, `\\`) {
		return fmt.Errorf("%s must be an absolute local path, or none", field)
	}
	ext := strings.ToLower(filepath.Ext(path))
	if !slices.Contains(extensions, ext) {
		return fmt.Errorf("%s must be a %s file", field, strings.Join(extensions, " or "))
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("%s: no such file", field)
	}
	if info.Size() == 0 || info.Size() > maxSize {
		return fmt.Errorf("%s must be between 1 byte and %s", field, formatSize(maxSize))
	}
	return nil
}

func formatSize(n int64) string {
	if n >= 1<<30 {
		return fmt.Sprintf("%d GB", (n+(1<<29))>>30)
	}
	return fmt.Sprintf("%d MB", n>>20)
}

// describeExtras is the plan summary part for the pro settings, the portrait and the bumpers.
func describeExtras(o recapplan.Options) (pro string, portrait bool, bumpers []string) {
	pro = o.Capture.ProSettings.Name
	portrait = o.Overlays.HUDPortrait != nil
	if b := o.Bumpers; b != nil {
		if b.Intro.Enabled {
			bumpers = append(bumpers, "intro")
		}
		if b.Sponsor != nil && b.Sponsor.Enabled {
			bumpers = append(bumpers, "sponsor")
		}
		if b.Outro.Enabled {
			bumpers = append(bumpers, "outro")
		}
	}
	return pro, portrait, bumpers
}
