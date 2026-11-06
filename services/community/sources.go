package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// inboxDataset names demos an operator copied by hand into the inbox folder,
// such as tier-one demos downloaded from HLTV in a browser.
const inboxDataset = "inbox"

// tierOneSuffix marks a dataset whose demos count only when both teams also
// played in a dataset listed before it without the suffix.
const tierOneSuffix = "@tier1"

type datasetSpec struct {
	id      string
	tierOne bool
}

func parseDatasetSpecs(raw string) []datasetSpec {
	out := []datasetSpec{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, tierOne := strings.CutSuffix(part, tierOneSuffix)
		out = append(out, datasetSpec{id: id, tierOne: tierOne})
	}
	return out
}

var (
	demoTail  = regexp.MustCompile(`(-p\d+|\(\d+\))?\.dem$`)
	mapSuffix = regexp.MustCompile(`(-m\d+)?-[a-z0-9]+$`)
)

// demoTeams reads the two team slugs from a "team-a-vs-team-b-m1-map.dem" file name.
func demoTeams(file string) []string {
	stem := demoTail.ReplaceAllString(strings.ToLower(path.Base(file)), "")
	a, b, ok := strings.Cut(mapSuffix.ReplaceAllString(stem, ""), "-vs-")
	if !ok || a == "" || b == "" {
		return nil
	}
	return []string{a, b}
}

// keepTierOne drops the files whose two teams are not both in known.
func keepTierOne(files []hfFile, known map[string]bool) []hfFile {
	out := []hfFile{}
	for _, f := range files {
		teams := demoTeams(f.Path)
		if len(teams) == 2 && known[teams[0]] && known[teams[1]] {
			out = append(out, f)
		}
	}
	return out
}

func addTeams(known map[string]bool, files []hfFile) {
	for _, f := range files {
		for _, t := range demoTeams(f.Path) {
			known[t] = true
		}
	}
}

// listInbox lists the .dem files in dir with their size and SHA-256, so the
// same pending and dedupe rules apply as for a dataset.
func listInbox(dir string) ([]hfFile, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := []hfFile{}
	for _, e := range entries {
		if !e.Type().IsRegular() || !strings.HasSuffix(strings.ToLower(e.Name()), ".dem") {
			continue
		}
		sum, size, err := hashFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		if size == 0 || size > maxDemoBytes {
			continue
		}
		f := hfFile{Type: "file", Path: e.Name(), Size: size}
		f.LFS = &struct {
			OID string `json:"oid"`
		}{OID: sum}
		out = append(out, f)
	}
	slices.SortFunc(out, func(a, b hfFile) int { return strings.Compare(a.Path, b.Path) })
	return out, nil
}

func hashFile(name string) (string, int64, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, fmt.Errorf("hash %s: %w", name, err)
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
