package voiceplugin

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Dir is Studio's side of the plugin, under the profile's data folder: the discovery file,
// the CS2 token and the inbox the plugin writes recordings into.
type Dir string

// Inbox is where each recording gets a folder.
func (d Dir) Inbox() string { return filepath.Join(string(d), "inbox") }

// Recording is a recording's folder.
func (d Dir) Recording(id string) (string, error) {
	if !ValidID(id) {
		return "", fmt.Errorf("voiceplugin: %q is not a recording id", id)
	}
	return filepath.Join(d.Inbox(), id), nil
}

// Found is a recording folder the plugin finished.
type Found struct {
	ID       string
	Dir      string
	Manifest Manifest
}

// Finished lists the inbox's complete recordings, oldest first; a folder still recording or unreadable is left.
func (d Dir) Finished() ([]Found, []error) {
	entries, err := os.ReadDir(d.Inbox())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, []error{err}
	}
	var out []Found
	var errs []error
	for _, e := range entries {
		if !e.IsDir() || !ValidID(e.Name()) {
			continue
		}
		dir := filepath.Join(d.Inbox(), e.Name())
		m, err := ReadManifest(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Name(), err))
			continue
		}
		if m.State == StateComplete {
			out = append(out, Found{ID: e.Name(), Dir: dir, Manifest: m})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, errs
}

// Publish writes studio.ini, which tells the plugin where Studio's web server listens.
func (d Dir) Publish(webOrigin string) error {
	if err := os.MkdirAll(string(d), 0o750); err != nil {
		return err
	}
	body := "# ClipHub Studio writes this file at start for its TeamSpeak plugin.\nweb_origin=" + strings.TrimSpace(webOrigin) + "\n"
	path := filepath.Join(string(d), "studio.ini")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Token is the secret CS2 sends back with every post, made once per profile.
func (d Dir) Token() (string, error) {
	path := filepath.Join(string(d), "gsi-token")
	if raw, err := os.ReadFile(path); err == nil {
		if tok := strings.TrimSpace(string(raw)); len(tok) == 32 {
			return tok, nil
		}
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(buf)
	if err := os.MkdirAll(string(d), 0o750); err != nil {
		return "", err
	}
	return tok, os.WriteFile(path, []byte(tok+"\n"), 0o600)
}
