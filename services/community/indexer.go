package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rechedev9/cliphub/internal/tactical"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

const (
	hfBase = "https://huggingface.co"
	// maxDemoBytes bounds one download; tier-one CS2 demos are about 450 MB.
	maxDemoBytes   = 1500 << 20
	maxTreeBytes   = 32 << 20
	maxTreePages   = 50
	maxAttempts    = 3
	rebuildEvery   = 25
	indexUserAgent = "ClipHub-community-indexer/1 (+https://cliphub.rechedev.cloud)"
)

// errNotAMatch marks a demo with no players, such as the one-round stub before a restart; it is never retried.
var errNotAMatch = errors.New("demo holds no match")

// datasetPattern is an owner/name Hugging Face dataset id.
var datasetPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,95}/[A-Za-z0-9][A-Za-z0-9_.-]{0,95}$`)

// hfFile is one entry of the dataset tree API.
type hfFile struct {
	Type string `json:"type"`
	Path string `json:"path"`
	Size int64  `json:"size"`
	LFS  *struct {
		OID string `json:"oid"`
	} `json:"lfs"`
}

type indexer struct {
	store    *store
	http     *http.Client
	workDir  string
	inboxDir string
}

type indexOptions struct {
	datasets []datasetSpec
	inboxDir string
	maxDemos int
}

// run lists every dataset, indexes up to maxDemos new demos one at a time
// (each file is deleted right after its scan) and rebuilds the reference.
func (ix *indexer) run(ctx context.Context, opts indexOptions) error {
	todo := []pendingDemo{}
	known := map[string]bool{}
	for _, spec := range opts.datasets {
		files, err := ix.listDataset(ctx, spec.id)
		if err != nil {
			return fmt.Errorf("list %s: %w", spec.id, err)
		}
		listed := len(files)
		if spec.tierOne {
			files = keepTierOne(files, known)
		} else {
			addTeams(known, files)
		}
		pending, err := ix.store.pendingDemos(ctx, spec.id, files)
		if err != nil {
			return err
		}
		log.Printf("community stage=index class=listed dataset=%s demos=%d kept=%d pending=%d", spec.id, listed, len(files), len(pending))
		todo = append(todo, pending...)
	}
	if opts.inboxDir != "" {
		files, err := listInbox(opts.inboxDir)
		if err != nil {
			return fmt.Errorf("list inbox: %w", err)
		}
		pending, err := ix.store.pendingDemos(ctx, inboxDataset, files)
		if err != nil {
			return err
		}
		log.Printf("community stage=index class=listed dataset=%s demos=%d pending=%d", inboxDataset, len(files), len(pending))
		todo = append(todo, pending...)
	}
	done, failed := 0, 0
	for _, demo := range todo {
		if done+failed >= opts.maxDemos || ctx.Err() != nil {
			break
		}
		start := time.Now()
		summary, err := ix.indexDemo(ctx, demo)
		if err != nil {
			failed++
			log.Printf("community stage=index class=failed path=%s error=%v", demo.path, err)
			if err := ix.store.markFailed(ctx, demo, err); err != nil {
				return err
			}
			continue
		}
		done++
		log.Printf("community stage=index class=indexed path=%s map=%s rounds=%d seconds=%.0f", demo.path, summary.Match.Map, summary.Match.Rounds, time.Since(start).Seconds())
		if err := ix.store.markIndexed(ctx, demo, summary); err != nil {
			return err
		}
		// A long first run publishes as it goes instead of only at the end.
		if done%rebuildEvery == 0 {
			if _, err := rebuildReference(ctx, ix.store); err != nil {
				log.Printf("community stage=index class=rebuild_failed error=%v", err)
			}
		}
	}
	index, err := rebuildReference(ctx, ix.store)
	if err != nil {
		return fmt.Errorf("rebuild reference: %w", err)
	}
	log.Printf("community stage=index class=built indexed=%d failed=%d remaining=%d maps=%d rounds=%d players=%d", done, failed, max(len(todo)-done-failed, 0), index.Matches, index.Rounds, index.Players)
	for _, m := range index.Maps {
		log.Printf("community stage=index class=map map=%s maps=%d rounds=%d players=%d", m.Map, m.Matches, m.Rounds, m.Players)
	}
	return nil
}

// listDataset walks the tree API, following its Link pagination.
func (ix *indexer) listDataset(ctx context.Context, dataset string) ([]hfFile, error) {
	if !datasetPattern.MatchString(dataset) {
		return nil, fmt.Errorf("invalid dataset id %q", dataset)
	}
	next := hfBase + "/api/datasets/" + dataset + "/tree/main?recursive=true"
	out := []hfFile{}
	for page := 0; next != "" && page < maxTreePages; page++ {
		resp, err := ix.get(ctx, next)
		if err != nil {
			return nil, err
		}
		var files []hfFile
		err = json.NewDecoder(io.LimitReader(resp.Body, maxTreeBytes)).Decode(&files)
		link := resp.Header.Get("Link")
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("decode tree: %w", err)
		}
		for _, f := range files {
			if f.Type == "file" && strings.HasSuffix(strings.ToLower(f.Path), ".dem") && f.LFS != nil && f.Size > 0 && f.Size <= maxDemoBytes {
				out = append(out, f)
			}
		}
		next = nextLink(link)
		if next != "" && !strings.HasPrefix(next, hfBase+"/") {
			return nil, fmt.Errorf("pagination left %s", hfBase)
		}
	}
	return out, nil
}

var nextLinkPattern = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

func nextLink(header string) string {
	if m := nextLinkPattern.FindStringSubmatch(header); m != nil {
		return m[1]
	}
	return ""
}

func (ix *indexer) get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", indexUserAgent)
	resp, err := ix.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", rawURL, resp.Status)
	}
	return resp, nil
}

// indexDemo downloads one demo, checks its SHA-256 against the dataset, scans it and deletes it.
func (ix *indexer) indexDemo(ctx context.Context, demo pendingDemo) (tacticalplan.ProDemoSummary, error) {
	if demo.dataset == inboxDataset {
		return ix.indexInboxDemo(ctx, demo)
	}
	escaped := []string{}
	for _, part := range strings.Split(demo.path, "/") {
		escaped = append(escaped, url.PathEscape(part))
	}
	resp, err := ix.get(ctx, hfBase+"/datasets/"+demo.dataset+"/resolve/main/"+strings.Join(escaped, "/"))
	if err != nil {
		return tacticalplan.ProDemoSummary{}, err
	}
	defer resp.Body.Close()
	tmp, err := os.CreateTemp(ix.workDir, "proref-*.dem")
	if err != nil {
		return tacticalplan.ProDemoSummary{}, err
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(resp.Body, maxDemoBytes+1))
	if err != nil {
		return tacticalplan.ProDemoSummary{}, fmt.Errorf("download: %w", err)
	}
	if n != demo.size || hex.EncodeToString(hash.Sum(nil)) != demo.sha256 {
		return tacticalplan.ProDemoSummary{}, errors.New("download does not match the dataset size and SHA-256")
	}
	if err := tmp.Close(); err != nil {
		return tacticalplan.ProDemoSummary{}, err
	}
	return scanProDemo(ctx, tmp.Name())
}

// indexInboxDemo scans a hand-copied demo and deletes it, like a download.
func (ix *indexer) indexInboxDemo(ctx context.Context, demo pendingDemo) (tacticalplan.ProDemoSummary, error) {
	name := filepath.Join(ix.inboxDir, filepath.Base(demo.path))
	defer func() { _ = os.Remove(name) }()
	return scanProDemo(ctx, name)
}

func scanProDemo(ctx context.Context, name string) (tacticalplan.ProDemoSummary, error) {
	res, err := tactical.ScanFile(ctx, name, tactical.Options{})
	if err != nil {
		return tacticalplan.ProDemoSummary{}, fmt.Errorf("scan: %w", err)
	}
	summary := tacticalplan.SummarizeProDemo(res.Document)
	if summary.Match.Map == "" || summary.Match.Rounds == 0 || len(summary.Players) < 10 {
		return tacticalplan.ProDemoSummary{}, fmt.Errorf("%w: map %q, %d rounds, %d players", errNotAMatch, summary.Match.Map, summary.Match.Rounds, len(summary.Players))
	}
	return summary, nil
}

type pendingDemo struct {
	dataset string
	path    string
	sha256  string
	size    int64
	event   string
}

// pendingDemos are the listed files not yet indexed and not failed too often.
func (s *store) pendingDemos(ctx context.Context, dataset string, files []hfFile) ([]pendingDemo, error) {
	out := []pendingDemo{}
	for _, f := range files {
		var status, sum string
		var attempts int
		err := s.db.QueryRowContext(ctx, `SELECT status, sha256, attempts FROM pro_demos WHERE dataset = ? AND path = ?`, dataset, f.Path).Scan(&status, &sum, &attempts)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		changed := sum != f.LFS.OID
		if err == nil && !changed && (status == "ok" || attempts >= maxAttempts) {
			continue
		}
		out = append(out, pendingDemo{dataset: dataset, path: f.Path, sha256: f.LFS.OID, size: f.Size, event: eventName(f.Path)})
	}
	return out, nil
}

func (s *store) markIndexed(ctx context.Context, d pendingDemo, summary tacticalplan.ProDemoSummary) error {
	body, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO pro_demos (dataset, path, sha256, size, event, status, attempts, error, summary, indexed_at)
		VALUES (?, ?, ?, ?, ?, 'ok', 1, '', ?, ?)
		ON CONFLICT (dataset, path) DO UPDATE SET sha256 = excluded.sha256, size = excluded.size, event = excluded.event,
			status = 'ok', attempts = pro_demos.attempts + 1, error = '', summary = excluded.summary, indexed_at = excluded.indexed_at`,
		d.dataset, d.path, d.sha256, d.size, d.event, string(body), now())
	return err
}

func (s *store) markFailed(ctx context.Context, d pendingDemo, cause error) error {
	attempts := 1
	if errors.Is(cause, errNotAMatch) {
		attempts = maxAttempts
	}
	msg := cause.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO pro_demos (dataset, path, sha256, size, event, status, attempts, error, indexed_at)
		VALUES (?, ?, ?, ?, ?, 'failed', ?, ?, ?)
		ON CONFLICT (dataset, path) DO UPDATE SET sha256 = excluded.sha256, status = 'failed',
			attempts = CASE WHEN pro_demos.sha256 = excluded.sha256 THEN MAX(pro_demos.attempts + 1, excluded.attempts) ELSE excluded.attempts END, error = excluded.error, indexed_at = excluded.indexed_at`,
		d.dataset, d.path, d.sha256, d.size, d.event, attempts, msg, now())
	return err
}

// otherEvents names demos a dataset keeps at its root, with no event folder.
const otherEvents = "otros torneos"

var eventDigits = regexp.MustCompile(`([a-z])(\d{4})$`)

// eventName turns the dataset folder "iem_cologne_major_2026" into "IEM Cologne Major 2026".
func eventName(path string) string {
	folder, _, nested := strings.Cut(path, "/")
	if !nested {
		return otherEvents
	}
	folder = eventDigits.ReplaceAllString(strings.ToLower(folder), "$1 $2")
	words := strings.FieldsFunc(folder, func(r rune) bool { return r == '_' || r == '-' || r == ' ' })
	for i, w := range words {
		switch {
		case len(w) <= 4 && !strings.ContainsAny(w, "0123456789"):
			words[i] = strings.ToUpper(w)
		default:
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	if len(words) == 0 {
		return filepath.Base(path)
	}
	return strings.Join(words, " ")
}
