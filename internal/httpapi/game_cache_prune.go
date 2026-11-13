package httpapi

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	mapdata "github.com/rechedev9/cliphub/data/maps"
)

// PruneBundledMapPictures deletes the radars and map pictures that older
// versions extracted from CS2 into the game cache under root, for the maps
// ClipHub now ships: nothing reads them, and no old map stays on disk.
func PruneBundledMapPictures(root string) (int, error) {
	var stale []string
	radars, err := filepath.Glob(filepath.Join(root, "gameassets", "*", "*", "radar", "*.png"))
	if err != nil {
		return 0, err
	}
	for _, path := range radars {
		name := strings.TrimSuffix(filepath.Base(path), ".png")
		for _, layer := range []string{"upper", "lower"} {
			if mapName, ok := strings.CutSuffix(name, "_"+layer); ok && mapdata.Radar(mapName, layer) != nil {
				stale = append(stale, path)
			}
		}
	}
	shots, err := filepath.Glob(filepath.Join(root, "gameassets", "*", "*", "shot", "*.png"))
	if err != nil {
		return 0, err
	}
	for _, path := range shots {
		if mapdata.Shot(strings.TrimSuffix(filepath.Base(path), ".png")) != nil {
			stale = append(stale, path)
		}
	}
	var errs []error
	removed := 0
	for _, path := range stale {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
			continue
		}
		removed++
	}
	return removed, errors.Join(errs...)
}
