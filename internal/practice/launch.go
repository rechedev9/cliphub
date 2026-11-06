package practice

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rechedev9/cliphub/internal/filecommit"
	"github.com/rechedev9/cliphub/internal/winproc"
)

// ErrCS2NotFound means Studio found no CS2 install to write the cfg into.
var ErrCS2NotFound = errors.New("CS2 is not installed")

// Game is the CS2 install a practice session runs in. Tests fake it.
type Game interface {
	CfgDir() (string, error)
	Running() (bool, error)
	Launch(mapName string) error
}

// Result reports what Start did.
type Result struct {
	Launched       bool
	AlreadyRunning bool
	CfgPath        string
}

// Start writes the practice cfg, then launches CS2 unless it already runs.
// A running CS2 keeps its session: the player runs ExecCommand in its console.
func Start(game Game, plan Plan) (Result, error) {
	dir, err := game.CfgDir()
	if err != nil {
		return Result{}, err
	}
	path := filepath.Join(dir, CfgName)
	if err := writeCfg(path, RenderCfg(plan)); err != nil {
		return Result{}, fmt.Errorf("write practice cfg: %w", err)
	}
	running, err := game.Running()
	if err != nil {
		return Result{}, fmt.Errorf("check CS2 process: %w", err)
	}
	if running {
		return Result{AlreadyRunning: true, CfgPath: path}, nil
	}
	if err := game.Launch(plan.Map); err != nil {
		return Result{}, fmt.Errorf("launch CS2: %w", err)
	}
	return Result{Launched: true, CfgPath: path}, nil
}

func writeCfg(path, text string) error {
	attempt, cleanup, err := filecommit.Attempt(path)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := os.WriteFile(attempt, []byte(text), 0o600); err != nil {
		return err
	}
	return filecommit.Commit(attempt, path)
}

// SteamGame is the installed CS2 found by capture tool detection.
type SteamGame struct {
	CS2Path string
}

// CfgDir is game\csgo\cfg beside game\bin\win64\cs2.exe. It must exist, so
// a wrong ZV_CS2_PATH never creates folders somewhere else.
func (g SteamGame) CfgDir() (string, error) {
	if g.CS2Path == "" {
		return "", ErrCS2NotFound
	}
	dir := filepath.Join(filepath.Dir(g.CS2Path), "..", "..", "csgo", "cfg")
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", ErrCS2NotFound
	}
	return dir, nil
}

// Running reports whether cs2.exe is already open.
func (SteamGame) Running() (bool, error) {
	return winproc.Running("cs2.exe")
}

// Launch asks Steam to start CS2 with normal launch options plus the map and
// the practice cfg. Steam shows its own prompt for these launch arguments.
func (SteamGame) Launch(mapName string) error {
	if runtime.GOOS != "windows" {
		return errors.New("CS2 practice needs Windows")
	}
	// #nosec G204 -- fixed handler; the URL holds a validated map name only.
	cmd := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", LaunchURL(mapName))
	return cmd.Run()
}

// LaunchURL is the Steam URL that starts CS2 on mapName and execs the cfg.
func LaunchURL(mapName string) string {
	args := "+map " + mapName + " +" + ExecCommand
	return "steam://rungameid/730//" + strings.ReplaceAll(args, " ", "%20")
}
