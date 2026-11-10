package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/rechedev9/cliphub/internal/community"
)

// mergeSidecarSeries writes the format and veto of a sidecar into the header of an
// analysis that lacks them. It reports whether the file changed, which changes its
// hash and stamp, so pro-matches reads it again and Studio takes the newer analysis.
func mergeSidecarSeries(name string, side community.ProSidecar) (bool, error) {
	if side.Format == "" && len(side.Veto) == 0 {
		return false, nil
	}
	h, blob, err := readAnalysis(name)
	if err != nil {
		return false, err
	}
	if h.Match.MatchID != side.MatchID || len(h.Match.Veto) > len(side.Veto) ||
		(h.Match.Format == side.Format && slices.Equal(h.Match.Veto, side.Veto)) {
		return false, nil
	}
	h.Match.Format, h.Match.Veto = side.Format, side.Veto
	if err := writeAnalysis(name, h, blob); err != nil {
		return false, err
	}
	return true, nil
}

// backfillSeries merges every "<match id>.json" sidecar of sidecarsDir into the
// analyses of that match in analysesDir. It returns how many analyses changed.
func backfillSeries(analysesDir, sidecarsDir string) (int, error) {
	files, err := filepath.Glob(filepath.Join(analysesDir, "*"+analysisSuffix))
	if err != nil {
		return 0, err
	}
	sidecars := map[string]community.ProSidecar{}
	changed := 0
	for _, name := range files {
		match, _, ok := analysisName(filepath.Base(name))
		if !ok {
			continue
		}
		side, found := sidecars[match]
		if !found {
			side, err = readSidecar(filepath.Join(sidecarsDir, match+".json"))
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return changed, err
			}
			sidecars[match] = side
		}
		ok, err := mergeSidecarSeries(name, side)
		if err != nil {
			return changed, fmt.Errorf("%s: %w", filepath.Base(name), err)
		}
		if ok {
			changed++
		}
	}
	return changed, nil
}

func readSidecar(name string) (community.ProSidecar, error) {
	raw, err := os.ReadFile(name)
	if err != nil {
		return community.ProSidecar{}, err
	}
	var side community.ProSidecar
	if err := json.Unmarshal(raw, &side); err != nil || side.Source != hltvDataset || side.MatchID <= 0 {
		return community.ProSidecar{}, fmt.Errorf("bad sidecar %s", filepath.Base(name))
	}
	if strconv.Itoa(side.MatchID)+".json" != filepath.Base(name) {
		return community.ProSidecar{}, fmt.Errorf("sidecar %s names match %d", filepath.Base(name), side.MatchID)
	}
	return side, nil
}
