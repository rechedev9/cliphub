package store

import (
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// libraryNow is the timestamp every library write records. SQLite stores
// milliseconds, so the memory repository truncates the same way and both
// read back identical values.
func libraryNow() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}

func libraryTime(t time.Time) time.Time {
	return t.UTC().Truncate(time.Millisecond)
}

// libraryAlias is one alias with the key it is matched by.
type libraryAlias struct {
	key, alias string
}

// normalizeLibraryAliases cleans aliases, drops empty and duplicate keys (the
// first spelling wins) and sorts them by key, the order both repositories
// return aliases in.
func normalizeLibraryAliases(aliases []string) []libraryAlias {
	seen := map[string]bool{}
	out := make([]libraryAlias, 0, len(aliases))
	for _, raw := range aliases {
		alias := library.CleanName(raw)
		key := library.NormalizeName(alias)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, libraryAlias{key: key, alias: alias})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key < out[j].key })
	return out
}

func libraryAliasNames(aliases []libraryAlias) []string {
	out := make([]string, len(aliases))
	for i, a := range aliases {
		out[i] = a.alias
	}
	return out
}

func libraryTagKey(tag string) string {
	return strings.ToLower(tag)
}

// libraryPage applies a list offset and the clamped limit to an ordered slice.
func libraryPage[T any](items []T, limit, offset int) []T {
	limit = library.ClampLimit(limit)
	offset = max(offset, 0)
	if offset >= len(items) {
		return []T{}
	}
	end := min(offset+limit, len(items))
	return append([]T{}, items[offset:end]...)
}

func uuidPtr(id uuid.UUID) *uuid.UUID {
	return &id
}

func sameUUID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// prepareNewDemo fills the defaults both repositories apply to a new demo.
func prepareNewDemo(d *library.Demo) {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	now := libraryNow()
	if d.ImportedAt.IsZero() {
		d.ImportedAt = now
	}
	d.ImportedAt = libraryTime(d.ImportedAt)
	d.UpdatedAt = now
	if d.State == "" {
		d.State = library.ParseQueued
	}
	if !library.ValidSource(d.Source) {
		d.Source = library.SourceUnknown
	}
	if d.PlayedAt != nil {
		t := libraryTime(*d.PlayedAt)
		d.PlayedAt = &t
	}
	d.Tags = library.NormalizeTags(d.Tags)
	d.MatchID = nil
	d.JobIDs = []uuid.UUID{}
}

// uniqueMatchIDs drops repeated ids and keeps the first order, which is the
// order a report lists its matches in.
func uniqueMatchIDs(ids []uuid.UUID) []uuid.UUID {
	seen := map[uuid.UUID]bool{}
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
