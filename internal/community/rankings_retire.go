package community

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
)

// rankingsFile is where Studio kept the retired rankings opt-in.
const rankingsFile = "settings.json"

// RetireRankings asks the server to forget an install that shared rankings,
// then deletes the opt-in file. A failed delete keeps the file for the next start.
func RetireRankings(ctx context.Context, dir string, client *Client) {
	path := filepath.Join(dir, rankingsFile)
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	var s struct {
		RankingsEnabled bool   `json:"rankings_enabled"`
		InstallID       string `json:"install_id"`
	}
	if err == nil && json.Unmarshal(b, &s) == nil && s.RankingsEnabled && InstallIDPattern.MatchString(s.InstallID) {
		if client == nil {
			return
		}
		if err := client.Delete(ctx, s.InstallID); err != nil {
			log.Printf("community: could not delete the retired rankings on the server: %v", err)
			return
		}
		log.Printf("community: the server forgot this install's retired rankings")
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("community: remove %s: %v", rankingsFile, err)
	}
}
