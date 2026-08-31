package demooverlay

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FetchAvatar loads one HTTPS avatar URL. Empty or rejected URLs return (nil, nil).
type FetchAvatar func(url string) ([]byte, error)

// MaterializeAvatars writes AvatarURL bytes into dir and sets AvatarFile.
func MaterializeAvatars(doc *Document, dir string, fetch FetchAvatar) error {
	if doc == nil || fetch == nil {
		return nil
	}
	if err := materializeCardAvatars(doc.Intro.Left, dir, fetch); err != nil {
		return err
	}
	return materializeCardAvatars(doc.Intro.Right, dir, fetch)
}

func materializeCardAvatars(cards []PlayerCard, dir string, fetch FetchAvatar) error {
	for i := range cards {
		url := strings.TrimSpace(cards[i].AvatarURL)
		if url == "" || !strings.HasPrefix(url, "https://") {
			continue
		}
		body, err := fetch(url)
		if err != nil || len(body) == 0 {
			continue
		}
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create overlay avatar dir: %w", err)
		}
		path := filepath.Join(dir, cards[i].SteamID64+".img")
		if err := os.WriteFile(path, body, 0o600); err != nil {
			return fmt.Errorf("write overlay avatar %s: %w", cards[i].SteamID64, err)
		}
		cards[i].AvatarFile = path
	}
	return nil
}
