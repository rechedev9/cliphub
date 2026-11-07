package community

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rechedev9/cliphub/internal/prolineups"
)

const (
	// maxLineupsBytes bounds one map's lineup index; a busy map is about 5 MB.
	maxLineupsBytes = 64 << 20
	// lineupsFresh is how long a fetched lineup payload is used before asking again.
	lineupsFresh = 6 * time.Hour
	lineupsIndex = "index"
)

// ErrNoLineups means the server has no lineup index for the map.
var ErrNoLineups = errors.New("community: no pro lineups for this map")

// LineupMaps is the list of maps with a pro lineup index.
type LineupMaps struct {
	Version int              `json:"version"`
	BuiltAt time.Time        `json:"built_at"`
	Maps    []LineupsMapInfo `json:"maps"`
}

// LineupsMapInfo is one map of LineupMaps.
type LineupsMapInfo struct {
	Map     string `json:"map"`
	Lineups int    `json:"lineups"`
	Combos  int    `json:"combos"`
	Maps    int    `json:"maps"`
	Recent  int    `json:"recent_maps"`
}

// ProLineups downloads the maps list (mapName "") or one map's lineup index.
func (c *Client) ProLineups(ctx context.Context, mapName, etag string) ([]byte, string, error) {
	path := "/v1/pro-lineups"
	if mapName != "" {
		if !MapPattern.MatchString(mapName) || mapName == AllMaps {
			return nil, "", fmt.Errorf("community: invalid map %q", mapName)
		}
		path += "?map=" + mapName
	}
	req, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, "", err
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("community: pro lineups: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNotModified:
		return nil, etag, ErrNotModified
	case http.StatusNotFound:
		return nil, "", ErrNoLineups
	}
	body, err := readBody(resp, maxLineupsBytes)
	if err != nil {
		return nil, "", err
	}
	return body, resp.Header.Get("ETag"), nil
}

type lineupsEntry struct {
	body      []byte
	etag      string
	fetchedAt time.Time
}

// LineupsCache keeps the pro lineup payloads on disk and asks the server again
// once they are older than lineupsFresh, so an offline Studio keeps the last ones.
type LineupsCache struct {
	dir    string
	client *Client
	// mu guards mem and keys; a key's own lock is held across its download, so
	// one slow map does not hold up the others.
	mu   sync.Mutex
	mem  map[string]lineupsEntry
	keys map[string]*sync.Mutex
}

// OpenLineupsCache stores payloads under dir; a nil client only reads the disk.
func OpenLineupsCache(dir string, client *Client) *LineupsCache {
	return &LineupsCache{dir: filepath.Join(dir, "pro-lineups"), client: client, mem: map[string]lineupsEntry{}, keys: map[string]*sync.Mutex{}}
}

// Maps returns the maps with a lineup index.
func (c *LineupsCache) Maps(ctx context.Context) (LineupMaps, bool, error) {
	body, fresh, err := c.payload(ctx, "")
	if err != nil {
		return LineupMaps{}, false, err
	}
	var out LineupMaps
	if err := json.Unmarshal(body, &out); err != nil {
		return LineupMaps{}, false, fmt.Errorf("decode lineup maps: %w", err)
	}
	return out, fresh, nil
}

// Map returns one map's lineup index; fresh is false when it is a stale copy.
func (c *LineupsCache) Map(ctx context.Context, mapName string) (prolineups.MapIndex, bool, error) {
	if !MapPattern.MatchString(mapName) || mapName == AllMaps {
		return prolineups.MapIndex{}, false, fmt.Errorf("invalid map %q", mapName)
	}
	body, fresh, err := c.payload(ctx, mapName)
	if err != nil {
		return prolineups.MapIndex{}, false, err
	}
	var out prolineups.MapIndex
	if err := json.Unmarshal(body, &out); err != nil {
		return prolineups.MapIndex{}, false, fmt.Errorf("decode lineups: %w", err)
	}
	return out, fresh, nil
}

// payload serves memory, then disk, refreshing from the server when stale.
// A failed refresh falls back to the copy it has.
func (c *LineupsCache) payload(ctx context.Context, mapName string) ([]byte, bool, error) {
	key := mapName
	if key == "" {
		key = lineupsIndex
	}
	keyLock := c.keyLock(key)
	keyLock.Lock()
	defer keyLock.Unlock()
	c.mu.Lock()
	entry, ok := c.mem[key]
	c.mu.Unlock()
	if !ok {
		entry, ok = c.readDisk(key)
	}
	if ok && time.Since(entry.fetchedAt) < lineupsFresh {
		c.remember(key, entry)
		return entry.body, true, nil
	}
	if c.client == nil {
		if ok {
			return entry.body, false, nil
		}
		return nil, false, errors.New("community server disabled")
	}
	body, etag, err := c.client.ProLineups(ctx, mapName, entry.etag)
	switch {
	case errors.Is(err, ErrNotModified) && ok:
		entry.fetchedAt = time.Now()
	case err != nil:
		if ok {
			log.Printf("community: pro lineups %s unavailable (%v); using the copy from %s", key, err, entry.fetchedAt.Format(time.RFC3339))
			return entry.body, false, nil
		}
		return nil, false, err
	default:
		entry = lineupsEntry{body: body, etag: etag, fetchedAt: time.Now()}
	}
	c.remember(key, entry)
	c.writeDisk(key, entry)
	return entry.body, true, nil
}

func (c *LineupsCache) keyLock(key string) *sync.Mutex {
	c.mu.Lock()
	defer c.mu.Unlock()
	l := c.keys[key]
	if l == nil {
		l = &sync.Mutex{}
		c.keys[key] = l
	}
	return l
}

func (c *LineupsCache) remember(key string, e lineupsEntry) {
	c.mu.Lock()
	c.mem[key] = e
	c.mu.Unlock()
}

func (c *LineupsCache) readDisk(key string) (lineupsEntry, bool) {
	name := filepath.Join(c.dir, key+".json")
	body, err := os.ReadFile(name)
	if err != nil {
		return lineupsEntry{}, false
	}
	info, err := os.Stat(name)
	if err != nil {
		return lineupsEntry{}, false
	}
	etag, _ := os.ReadFile(filepath.Join(c.dir, key+".etag"))
	return lineupsEntry{body: body, etag: string(etag), fetchedAt: info.ModTime()}, true
}

func (c *LineupsCache) writeDisk(key string, e lineupsEntry) {
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		log.Printf("community: cannot cache pro lineups: %v", err)
		return
	}
	if err := writeFileAtomic(filepath.Join(c.dir, key+".json"), e.body); err != nil {
		log.Printf("community: cannot cache pro lineups %s: %v", key, err)
		return
	}
	now := time.Now()
	_ = os.Chtimes(filepath.Join(c.dir, key+".json"), now, now)
	if err := writeFileAtomic(filepath.Join(c.dir, key+".etag"), []byte(e.etag)); err != nil {
		log.Printf("community: cannot cache pro lineups etag %s: %v", key, err)
	}
}
