// zv-hud-designs regenerates the picker artwork from the same vector scenes
// that are used in exported videos. It can also extract a real demo for QA.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/mediafont"
)

func main() {
	out := flag.String("out", "web/public/hud", "Output directory")
	demo := flag.String("demo", "", "Optional demo to extract instead of generating example previews")
	target := flag.String("target", "", "Observed SteamID64 for demo extraction")
	rate := flag.Int("tick-rate", 64, "Source demo tick rate")
	telemetry := flag.String("telemetry", "", "Previously extracted telemetry for an ASS window")
	theme := flag.String("theme", "arena", "Theme for a real telemetry window")
	start := flag.Int("start-tick", 0, "Source tick at the start of the window")
	frames := flag.Int64("frames", 600, "Window duration in 60 fps frames")
	flag.Parse()
	if err := run(*out, *demo, *target, *rate, *telemetry, *theme, *start, *frames); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(out, demo, target string, rate int, telemetry, theme string, start int, frames int64) error {
	if err := os.MkdirAll(out, 0750); err != nil {
		return err
	}
	if demo != "" {
		file, err := os.Open(demo)
		if err != nil {
			return err
		}
		defer file.Close()
		hash := sha256.New()
		if _, err := io.Copy(hash, file); err != nil {
			return err
		}
		if _, err := file.Seek(0, 0); err != nil {
			return err
		}
		d, err := customhud.Extract(context.Background(), file, hex.EncodeToString(hash.Sum(nil)), target, rate)
		if err != nil {
			return err
		}
		body, err := json.Marshal(d)
		if err != nil {
			return err
		}
		if len(body) > customhud.MaxTelemetryBytes {
			return fmt.Errorf("telemetry exceeds byte limit")
		}
		if err := os.WriteFile(filepath.Join(out, "telemetry.json"), body, 0600); err != nil {
			return err
		}
		fmt.Printf("Extracted %d HUD states through tick %d (%s)\n", len(d.Snapshots), d.EndTick, d.Map)
		return nil
	}
	if telemetry != "" {
		d, err := customhud.Load(telemetry)
		if err != nil {
			return err
		}
		r, err := customhud.NewRenderer(theme)
		if err != nil {
			return err
		}
		ass, err := r.ASS(d, customhud.Window{StartTick: start, Frames: frames})
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(out, theme+".ass"), []byte(ass), 0600); err != nil {
			return err
		}
		s, _ := d.At(start)
		return os.WriteFile(filepath.Join(out, theme+".svg"), []byte(r.SVG(s, d.TargetSteamID)), 0644)
	}
	for _, theme := range customhud.Themes() {
		r, err := customhud.NewRenderer(theme.ID)
		if err != nil {
			return err
		}
		if err := preview(r, out); err != nil {
			return err
		}
	}
	// Keep the picker self-contained inside Next's app root. Its production
	// packaging and Turbopack deliberately do not trace parent directories.
	catalog, err := json.MarshalIndent(customhud.Themes(), "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "catalog.json"), append(catalog, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("Generated %d HUD previews in %s\n", len(customhud.Themes()), out)
	return nil
}

func preview(r *customhud.Renderer, out string) error {
	dir, err := os.MkdirTemp("", "cliphub-hud-preview-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	state := customhud.Example()
	d := customhud.Timeline{Version: customhud.Version, DemoSHA256: strings.Repeat("0", 64), TargetSteamID: customhud.ExampleTarget, TickRate: 64, EndTick: 64, Snapshots: []customhud.Snapshot{state}}
	ass, err := r.ASS(d, customhud.Window{Frames: 1})
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "preview.ass")
	if err := os.WriteFile(path, []byte(ass), 0600); err != nil {
		return err
	}
	fontPath, err := mediafont.Materialize()
	if err != nil {
		return err
	}
	cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "color=c=black@0:s=1920x1080:r=60,format=rgba", "-vf", customhud.ASSFilter(path, filepath.Dir(fontPath), true), "-frames:v", "1", "-c:v", "libwebp", "-lossless", "1", filepath.Join(out, r.Theme.ID+".webp"))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("render HUD preview: %w: %s", err, output)
	}
	return nil
}
