package community

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

const (
	referenceFile = "pro-reference.json"
	etagFile      = "pro-reference.etag"
	// refreshEvery is how often a running Studio asks for a newer reference.
	refreshEvery = 12 * time.Hour
)

// ProReferenceCache holds the server pro reference, kept on disk so an
// offline Studio still uses the last one it saw.
type ProReferenceCache struct {
	dir    string
	client *Client
	mu     sync.RWMutex
	ref    *tacticalplan.ProReference
	etag   string
}

// OpenProReferenceCache loads the copy on disk; a nil client never fetches.
func OpenProReferenceCache(dir string, client *Client) *ProReferenceCache {
	c := &ProReferenceCache{dir: dir, client: client}
	b, err := os.ReadFile(filepath.Join(dir, referenceFile))
	if err != nil {
		return c
	}
	ref, err := tacticalplan.DecodeProReference(b)
	if err != nil {
		log.Printf("community: ignoring cached pro reference: %v", err)
		return c
	}
	c.ref = &ref
	if etag, err := os.ReadFile(filepath.Join(dir, etagFile)); err == nil {
		c.etag = string(etag)
	}
	return c
}

// Current is the server reference when one was ever fetched, else the bundled one.
func (c *ProReferenceCache) Current() (tacticalplan.ProReference, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.ref != nil {
		return *c.ref, true
	}
	return tacticalplan.DefaultProReference(), false
}

// Refresh asks the server for a newer reference and stores it.
func (c *ProReferenceCache) Refresh(ctx context.Context) error {
	if c.client == nil {
		return errors.New("community server disabled")
	}
	c.mu.RLock()
	etag := c.etag
	c.mu.RUnlock()
	body, next, err := c.client.ProReference(ctx, etag)
	if errors.Is(err, ErrNotModified) {
		return nil
	}
	if err != nil {
		return err
	}
	ref, err := tacticalplan.DecodeProReference(body)
	if err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(c.dir, referenceFile), body); err != nil {
		log.Printf("community: cannot cache the pro reference: %v", err)
	} else if err := writeFileAtomic(filepath.Join(c.dir, etagFile), []byte(next)); err != nil {
		log.Printf("community: cannot cache the pro reference etag: %v", err)
	}
	c.mu.Lock()
	c.ref, c.etag = &ref, next
	c.mu.Unlock()
	return nil
}

// Run refreshes now and every refreshEvery until ctx ends, logging what Studio compares against.
func (c *ProReferenceCache) Run(ctx context.Context) {
	if c.client == nil {
		log.Printf("community: server disabled; the pro comparison uses the %s", c.describe())
		return
	}
	for {
		if err := c.Refresh(ctx); err != nil {
			log.Printf("community: pro reference from %s unavailable (%v); using the %s", c.client.Host(), err, c.describe())
		} else {
			log.Printf("community: pro reference from %s: %s", c.client.Host(), c.describe())
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(refreshEvery):
		}
	}
}

func (c *ProReferenceCache) describe() string {
	ref, server := c.Current()
	if !server {
		return "bundled reference (" + ref.ID + ")"
	}
	return "server reference " + ref.ID + " updated " + ref.UpdatedAt
}

func writeFileAtomic(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
