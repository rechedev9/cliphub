package editor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/mediafont"
)

// centeredKillfeedTop is the first row's top edge on a 1920 px tall frame:
// below the hook title (y=150) and the native CS2 scoreboard strip.
const centeredKillfeedTop = 300

// writeCenteredKillfeed writes the short's killfeed script beside its output.
// Times are on the edited timeline, which is what the post-concat chain sees.
func writeCenteredKillfeed(short *ShortEdit) error {
	short.KillfeedASS, short.KillfeedFontsDir = "", ""
	if !short.CenteredKillfeed || len(short.Kills) == 0 {
		return nil
	}
	width, height := outputDimensions(*short)
	notices := make([]customhud.KillNotice, 0, len(short.Kills))
	lastKill := 0.0
	for _, kill := range short.Kills {
		attacker := kill.Attacker
		if attacker == "" {
			attacker = short.Player
		}
		notices = append(notices, customhud.KillNotice{
			AtSeconds:    kill.TimeSeconds,
			Attacker:     attacker,
			AttackerSide: kill.AttackerSide,
			Victim:       kill.Victim,
			VictimSide:   kill.VictimSide,
			Weapon:       kill.Weapon,
			Headshot:     kill.Headshot,
		})
		lastKill = max(lastKill, kill.TimeSeconds)
	}
	duration := short.DurationSeconds
	if duration <= 0 {
		duration = lastKill + 10
	}
	script, err := customhud.KillfeedASS(notices, customhud.KillfeedLayout{
		Width:           width,
		Height:          height,
		Top:             centeredKillfeedTop * height / 1920,
		DurationSeconds: duration,
	})
	if err != nil {
		return fmt.Errorf("centered killfeed: %w", err)
	}
	path := short.Output + ".killfeed.ass"
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		return err
	}
	fonts, err := mediafont.MaterializeHUD()
	if err != nil {
		return fmt.Errorf("centered killfeed fonts: %w", err)
	}
	short.KillfeedASS, short.KillfeedFontsDir = path, fonts
	return nil
}

func appendCenteredKillfeedFilter(filters []string, short ShortEdit) []string {
	if short.KillfeedASS == "" {
		return filters
	}
	return append(filters, customhud.ASSFilter(short.KillfeedASS, short.KillfeedFontsDir, false))
}
