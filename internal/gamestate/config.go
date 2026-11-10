package gamestate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ConfigName is the file CS2 reads from its cfg folder at start.
const ConfigName = "gamestate_integration_cliphub.cfg"

// ErrNoCS2 is a PC where Studio did not find CS2's game folder.
var ErrNoCS2 = errors.New("gamestate: CS2 was not found on this PC")

// ConfigDir is CS2's cfg folder, from the cs2.exe path (game\bin\win64\cs2.exe to game\csgo\cfg).
func ConfigDir(cs2Exe string) (string, error) {
	if cs2Exe == "" {
		return "", ErrNoCS2
	}
	game := filepath.Dir(filepath.Dir(filepath.Dir(cs2Exe)))
	csgo := filepath.Join(game, "csgo")
	if info, err := os.Stat(csgo); err != nil || !info.IsDir() {
		return "", ErrNoCS2
	}
	return filepath.Join(csgo, "cfg"), nil
}

// ConfigOptions is where CS2 posts and the token it sends back.
type ConfigOptions struct {
	URL   string
	Token string
}

// Config is the cfg text: CS2 posts the provider, map and round on every change and every 10 seconds.
func Config(o ConfigOptions) string {
	quote := func(s string) string { return strings.NewReplacer(`"`, "", "\n", "", "\r", "").Replace(s) }
	return fmt.Sprintf(`"ClipHub Studio"
{
	"uri" "%s"
	"timeout" "5.0"
	"buffer" "0.1"
	"throttle" "0.5"
	"heartbeat" "10.0"
	"auth"
	{
		"token" "%s"
	}
	"data"
	{
		"provider" "1"
		"map" "1"
		"round" "1"
	}
}
`, quote(o.URL), quote(o.Token))
}

// Install writes the cfg into CS2's cfg folder and returns its path; CS2 reads it at its next start.
func Install(cs2Exe string, o ConfigOptions) (string, error) {
	dir, err := ConfigDir(cs2Exe)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, ConfigName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(Config(o)), 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return path, nil
}

// ConfigState is where the cfg goes and whether it is there as Studio would write it now.
type ConfigState struct {
	Path    string
	Present bool
	Current bool
}

// Check reads the installed cfg against the one Studio would write.
func Check(cs2Exe string, o ConfigOptions) (ConfigState, error) {
	dir, err := ConfigDir(cs2Exe)
	if err != nil {
		return ConfigState{}, err
	}
	path := filepath.Join(dir, ConfigName)
	body, err := os.ReadFile(path)
	return ConfigState{Path: path, Present: err == nil, Current: err == nil && string(body) == Config(o)}, nil
}
