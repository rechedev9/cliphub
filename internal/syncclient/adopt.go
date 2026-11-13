package syncclient

import (
	"context"
	"log"
	"path"

	"github.com/rechedev9/cliphub/internal/store"
)

// applyPulled applies the hub's changes. A demo imported here and on another
// device under another id takes the hub's id; its files follow it.
func (c *Client) applyPulled(ctx context.Context, changes []store.SyncChange) error {
	var adopted []store.SyncAdoption
	err := c.syncLog.ApplyAdopting(ctx, changes, func(a store.SyncAdoption) error {
		adopted = append(adopted, a)
		files, from, to := c.adoptedFolders(a)
		if files == nil {
			return nil
		}
		return linkFolder(files, from, to)
	})
	if err != nil {
		return err
	}
	// The old folders go only once the new ids are committed.
	for _, a := range adopted {
		log.Printf("sync: %s %s takes the workspace id %s", a.Table, a.From, a.To)
		if files, from, _ := c.adoptedFolders(a); files != nil {
			if err := files.DeleteTree(from); err != nil {
				log.Printf("sync: remove %s: %v", from, err)
			}
		}
	}
	return nil
}

// adoptedFolders names the storage folder an adopted id keeps its files in.
func (c *Client) adoptedFolders(a store.SyncAdoption) (Files, string, string) {
	switch a.Table {
	case "library_demos":
		return c.cfg.Files, path.Join("library", a.From), path.Join("library", a.To)
	case "library_matches":
		if c.cfg.VoiceFiles != nil {
			return c.cfg.VoiceFiles, a.From, a.To
		}
	}
	return nil, "", ""
}

func linkFolder(files Files, from, to string) error {
	names, err := files.List(from)
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := files.Link(path.Join(from, name), path.Join(to, name)); err != nil {
			return err
		}
	}
	return nil
}
