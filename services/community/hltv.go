package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/nwaples/rardecode/v2"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/tactical"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// hltvDataset names the analyses prepared from HLTV demo archives on an
// operator PC (tools/hltv-harvester downloads them, prepare-hltv scans them).
const hltvDataset = "hltv"

// analysisSuffix ends every prepared analysis; community.ProAnalysisFormat is its layout.
const analysisSuffix = community.ProAnalysisSuffix

var demoEntry = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.()-]{0,200}\.dem$`)

// prepareHLTV scans every downloaded archive that has its sidecar, writes one
// analysis per demo into outDir and deletes the archive once all its demos are in.
// prepareDirs are the folders prepare-hltv reads and writes; keep is empty when no demo is kept.
type prepareDirs struct {
	downloads, out, work, keep string
}

func prepareHLTV(ctx context.Context, dirs prepareDirs) error {
	for _, dir := range []string{dirs.out, dirs.keep} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	sidecars, err := filepath.Glob(filepath.Join(dirs.downloads, "*.json"))
	if err != nil {
		return err
	}
	slices.Sort(sidecars)
	prepared, failed := 0, 0
	for _, side := range sidecars {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		archive := strings.TrimSuffix(side, ".json") + ".rar"
		if _, err := os.Stat(archive); err != nil {
			continue
		}
		n, err := prepareArchive(ctx, side, archive, dirs)
		if err != nil {
			failed++
			log.Printf("community stage=prepare class=failed archive=%s error=%v", filepath.Base(archive), err)
			continue
		}
		prepared += n
		_ = os.Remove(archive)
		_ = os.Remove(side)
	}
	log.Printf("community stage=prepare class=done analyses=%d failed_archives=%d", prepared, failed)
	return nil
}

func prepareArchive(ctx context.Context, sidecarPath, archive string, dirs prepareDirs) (int, error) {
	raw, err := os.ReadFile(sidecarPath)
	if err != nil {
		return 0, err
	}
	var meta community.ProSidecar
	if err := json.Unmarshal(raw, &meta); err != nil || meta.Source != hltvDataset || meta.MatchID <= 0 {
		return 0, fmt.Errorf("bad sidecar %s", filepath.Base(sidecarPath))
	}
	r, err := rardecode.OpenReader(archive)
	if err != nil {
		return 0, fmt.Errorf("open archive: %w", err)
	}
	defer r.Close()
	written := 0
	for {
		h, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return written, fmt.Errorf("read archive: %w", err)
		}
		entry := path.Base(filepath.ToSlash(h.Name))
		if h.IsDir || !demoEntry.MatchString(entry) {
			continue
		}
		base := fmt.Sprintf("%d__%s", meta.MatchID, strings.TrimSuffix(entry, ".dem"))
		out := filepath.Join(dirs.out, base+analysisSuffix)
		keep := ""
		if dirs.keep != "" {
			keep = filepath.Join(dirs.keep, base+".dem")
		}
		if _, err := os.Stat(out); err == nil {
			written++
			// An analysis made before its sidecar had the veto takes it now.
			if _, err := mergeSidecarSeries(out, meta); err != nil {
				return written, fmt.Errorf("%s: series: %w", entry, err)
			}
			if err := keepDemo(r, keep); err != nil {
				return written, fmt.Errorf("%s: %w", entry, err)
			}
			continue
		}
		err = prepareDemo(ctx, r, demoJob{meta: meta, entry: entry, out: out, work: dirs.work, keep: keep})
		if errors.Is(err, errNotAMatch) {
			log.Printf("community stage=prepare class=skipped match=%d entry=%s error=%v", meta.MatchID, entry, err)
			continue
		}
		if err != nil {
			return written, fmt.Errorf("%s: %w", entry, err)
		}
		written++
		log.Printf("community stage=prepare class=prepared match=%d entry=%s", meta.MatchID, entry)
	}
	if written == 0 {
		return 0, errors.New("archive held no usable demo")
	}
	return written, nil
}

// prepareDemo copies one archive entry to a temporary demo, scans it and writes the analysis.
// demoJob is one archive entry to prepare; keep is where the demo stays for lineup clips, or empty.
type demoJob struct {
	meta                   community.ProSidecar
	entry, out, work, keep string
}

// keepDemo copies an entry already prepared into the kept demos, unless it is there.
func keepDemo(src io.Reader, keep string) error {
	if keep == "" {
		return nil
	}
	if _, err := os.Stat(keep); err == nil {
		return nil
	}
	tmp := keep + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(src, maxDemoBytes+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > maxDemoBytes {
		err = fmt.Errorf("keep: demo is over %d MB", maxDemoBytes>>20)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, keep)
}

func prepareDemo(ctx context.Context, src io.Reader, job demoJob) error {
	meta, entry, out := job.meta, job.entry, job.out
	tmp, err := os.CreateTemp(job.work, "hltv-*.dem")
	if err != nil {
		return err
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()
	n, err := io.Copy(tmp, io.LimitReader(src, maxDemoBytes+1))
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}
	if n > maxDemoBytes {
		return fmt.Errorf("extract: demo is over %d MB", maxDemoBytes>>20)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	res, err := tactical.ScanFile(ctx, tmp.Name(), tactical.Options{})
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}
	if err := checkMatch(tacticalplan.SummarizeProDemo(res.Document)); err != nil {
		return err
	}
	res.Document.Demo.Path = ""
	if err := writeAnalysis(out, community.ProAnalysisHeader{Format: community.ProAnalysisFormat, Match: meta, Entry: entry, Document: res.Document, Positions: res.Positions.Descriptor}, res.Positions.Data); err != nil {
		return err
	}
	if job.keep == "" {
		return nil
	}
	// The work folder may sit on another drive, where a rename fails; a copy always works.
	if err := os.Rename(tmp.Name(), job.keep); err == nil {
		return nil
	}
	f, err := os.Open(tmp.Name())
	if err != nil {
		return err
	}
	defer f.Close()
	return keepDemo(f, job.keep)
}

func checkMatch(s tacticalplan.ProDemoSummary) error {
	if s.Match.Map == "" || s.Match.Rounds == 0 || len(s.Players) < 10 {
		return fmt.Errorf("%w: map %q, %d rounds, %d players", errNotAMatch, s.Match.Map, s.Match.Rounds, len(s.Players))
	}
	return nil
}

func writeAnalysis(out string, h community.ProAnalysisHeader, blob []byte) error {
	tmp := out + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	werr := community.WriteProAnalysis(f, h, blob)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		_ = os.Remove(tmp)
		return werr
	}
	return os.Rename(tmp, out)
}

// writeSidecars writes, for every match with an analysis, the sidecar its archive came with,
// so the harvester can download that archive again for lineup clips.
func writeSidecars(analysesDir, outDir string) (int, error) {
	files, err := filepath.Glob(filepath.Join(analysesDir, "*"+analysisSuffix))
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return 0, err
	}
	seen := map[int]bool{}
	for _, name := range files {
		h, _, err := readAnalysis(name)
		if err != nil || seen[h.Match.MatchID] || h.Match.MatchID <= 0 {
			continue
		}
		seen[h.Match.MatchID] = true
		body, err := json.Marshal(h.Match)
		if err != nil {
			return 0, err
		}
		if err := writeFile(filepath.Join(outDir, fmt.Sprintf("%d.json", h.Match.MatchID)), body); err != nil {
			return 0, err
		}
	}
	return len(seen), nil
}

// readAnalysis decodes a prepared analysis file.
func readAnalysis(name string) (community.ProAnalysisHeader, []byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return community.ProAnalysisHeader{}, nil, err
	}
	defer f.Close()
	return community.ReadProAnalysis(f)
}

// listAnalyses lists prepared analyses with their size and SHA-256. Their path
// is "<match id>/<demo entry>" so restart parts of one map join like dataset demos.
func listAnalyses(dir string) ([]hfFile, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := []hfFile{}
	for _, e := range entries {
		match, entry, ok := analysisName(e.Name())
		if !e.Type().IsRegular() || !ok {
			continue
		}
		sum, size, err := hashFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		f := hfFile{Type: "file", Path: match + "/" + entry, Size: size}
		f.LFS = &struct {
			OID string `json:"oid"`
		}{OID: sum}
		out = append(out, f)
	}
	slices.SortFunc(out, func(a, b hfFile) int { return strings.Compare(a.Path, b.Path) })
	return out, nil
}

var analysisFile = regexp.MustCompile(`^(\d{1,12})__([A-Za-z0-9][A-Za-z0-9_.()-]{0,200})` + regexp.QuoteMeta(analysisSuffix) + `$`)

func analysisName(file string) (match, entry string, ok bool) {
	m := analysisFile.FindStringSubmatch(file)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2] + ".dem", true
}

func analysisFileName(p string) string {
	match, entry, _ := strings.Cut(p, "/")
	return match + "__" + strings.TrimSuffix(entry, ".dem") + analysisSuffix
}

// indexAnalysis summarizes a prepared analysis and moves it to the archive, where
// the lineup index reads it. It returns the event named in the analysis.
func (ix *indexer) indexAnalysis(demo pendingDemo) (tacticalplan.ProDemoSummary, string, error) {
	name := filepath.Join(ix.hltvDir, analysisFileName(demo.path))
	h, _, err := readAnalysis(name)
	if err != nil {
		return tacticalplan.ProDemoSummary{}, "", err
	}
	summary := tacticalplan.SummarizeProDemo(h.Document)
	if err := checkMatch(summary); err != nil {
		return tacticalplan.ProDemoSummary{}, "", err
	}
	if ix.archiveDir != "" {
		if err := os.MkdirAll(ix.archiveDir, 0o750); err != nil {
			return tacticalplan.ProDemoSummary{}, "", err
		}
		if err := os.Rename(name, filepath.Join(ix.archiveDir, filepath.Base(name))); err != nil {
			return tacticalplan.ProDemoSummary{}, "", fmt.Errorf("archive analysis: %w", err)
		}
	}
	event := h.Match.Event
	if event == "" {
		event = otherEvents
	}
	return summary, event, nil
}
