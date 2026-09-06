package httpapi

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const cachedJSONMaxEntries = 8

type cachedJSONEntry struct {
	etag string
	body []byte
}

// cachedJSON remembers the last encoded list bodies so an unchanged poll
// skips json.Marshal and can answer 304.
type cachedJSON struct {
	mu    sync.Mutex
	items map[string]cachedJSONEntry
}

func (c *cachedJSON) write(w http.ResponseWriter, r *http.Request, key string, body any) {
	c.mu.Lock()
	if hit, ok := c.items[key]; ok {
		etag, raw := hit.etag, hit.body
		c.mu.Unlock()
		writeConditionalJSON(w, r, etag, raw)
		return
	}
	c.mu.Unlock()

	raw, err := json.Marshal(body)
	if err != nil {
		internalError(w, "encode json", err)
		return
	}
	etag := weakETag(raw)
	c.mu.Lock()
	if c.items == nil || len(c.items) >= cachedJSONMaxEntries {
		c.items = make(map[string]cachedJSONEntry, cachedJSONMaxEntries)
	}
	c.items[key] = cachedJSONEntry{etag: etag, body: raw}
	c.mu.Unlock()
	writeConditionalJSON(w, r, etag, raw)
}

func writeConditionalJSON(w http.ResponseWriter, r *http.Request, etag string, raw []byte) {
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "private, no-cache")
	if noneMatch(r, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

func weakETag(raw []byte) string {
	sum := sha256.Sum256(raw)
	return fmt.Sprintf(`W/"%x"`, sum[:16])
}

func noneMatch(r *http.Request, etag string) bool {
	raw := r.Header.Get("If-None-Match")
	if raw == "" || etag == "" {
		return false
	}
	if raw == "*" {
		return true
	}
	want := etagToken(etag)
	for _, part := range strings.Split(raw, ",") {
		if etagToken(part) == want {
			return true
		}
	}
	return false
}

func etagToken(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "W/")
	return strings.Trim(value, `"`)
}

func jobListCacheKey(r *http.Request, items []jobListItem) string {
	parts := make([]string, len(items))
	for i := range items {
		var b strings.Builder
		writeJobListFingerprint(&b, &items[i])
		parts[i] = b.String()
	}
	sort.Strings(parts)
	return r.URL.RawQuery + "\n" + strings.Join(parts, "")
}

func writeJobListFingerprint(b *strings.Builder, item *jobListItem) {
	b.WriteString(item.ID.String())
	b.WriteByte('\n')
	b.WriteString(item.Status.String())
	b.WriteByte('\n')
	b.WriteString(strconv.FormatInt(item.UpdatedAt.UnixNano(), 10))
	b.WriteByte('\n')
	b.WriteString(item.FailureReason)
	b.WriteByte('\n')
	b.WriteString(item.FailureCode)
	b.WriteByte('\n')
	b.WriteString(item.DemoFileName)
	b.WriteByte('\n')
	b.WriteString(item.SeriesID)
	b.WriteByte('\n')
	b.WriteString(item.TargetSteamID)
	b.WriteByte('\n')
	b.WriteString(strconv.FormatInt(item.CreatedAt.UnixNano(), 10))
	b.WriteByte('\n')
	if item.Summary != nil {
		raw, err := json.Marshal(item.Summary)
		if err == nil {
			b.Write(raw)
		}
	}
	b.WriteByte('\x1e')
}

func streamListCacheKey(r *http.Request, items []streamJobListItem) string {
	parts := make([]string, len(items))
	for i := range items {
		var b strings.Builder
		writeStreamListFingerprint(&b, &items[i])
		parts[i] = b.String()
	}
	sort.Strings(parts)
	return r.URL.RawQuery + "\n" + strings.Join(parts, "")
}

func writeStreamListFingerprint(b *strings.Builder, item *streamJobListItem) {
	b.WriteString(item.ID.String())
	b.WriteByte('\n')
	b.WriteString(string(item.Status))
	b.WriteByte('\n')
	b.WriteString(strconv.FormatInt(item.UpdatedAt.UnixNano(), 10))
	b.WriteByte('\n')
	b.WriteString(item.FailureReason)
	b.WriteByte('\n')
	b.WriteString(strconv.Itoa(item.ClipCount))
	b.WriteByte('\n')
	b.WriteString(strconv.FormatBool(item.RenderedOutputsUnavailable))
	b.WriteByte('\n')
	if raw, err := json.Marshal(item.RenderedOutputs); err == nil {
		b.Write(raw)
	}
	b.WriteByte('\x1e')
}

func jobListFingerprintForTest(items []jobListItem) string {
	var b strings.Builder
	for i := range items {
		writeJobListFingerprint(&b, &items[i])
	}
	return b.String()
}
