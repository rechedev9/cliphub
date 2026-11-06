package library

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Collection limits. A collection is a short list of plays picked while
// analysing, so it can go to a Short later.
const (
	MaxCollectionNameRunes = 80
	MaxCollectionNoteRunes = 280
	MaxCollectionItems     = 200
	// MaxCollectionList caps one list read.
	MaxCollectionList = 500
)

// CollectionItem is one saved moment: a tick of a round of a match, with an
// optional note. Items keep the order the user gave them.
type CollectionItem struct {
	MatchID uuid.UUID `json:"match_id"`
	Round   int       `json:"round"`
	Tick    int       `json:"tick"`
	Note    string    `json:"note"`
}

// Collection is a named list of moments from any matches of the library.
// Deleting a match leaves its items in place; reads report them as missing.
type Collection struct {
	ID        uuid.UUID        `json:"id"`
	Name      string           `json:"name"`
	Items     []CollectionItem `json:"items"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// CollectionPatch is a user edit of a collection. Nil fields are left unchanged.
type CollectionPatch struct {
	Name  *string
	Items *[]CollectionItem
}

// CleanCollection trims the name and notes, gives a nil list an empty one and
// checks every limit. The same moment saved twice is kept once.
func CleanCollection(c Collection) (Collection, error) {
	c.Name = CleanName(c.Name)
	switch {
	case c.Name == "":
		return c, errors.New("name must not be empty")
	case len([]rune(c.Name)) > MaxCollectionNameRunes:
		return c, fmt.Errorf("name must be at most %d characters", MaxCollectionNameRunes)
	case len(c.Items) > MaxCollectionItems:
		return c, fmt.Errorf("a collection has at most %d moments", MaxCollectionItems)
	}
	items := make([]CollectionItem, 0, len(c.Items))
	seen := map[CollectionItem]bool{}
	for i, item := range c.Items {
		item.Note = strings.TrimSpace(item.Note)
		if item.MatchID == uuid.Nil || item.Round < 1 || item.Tick < 0 {
			return c, fmt.Errorf("moment %d: needs a match, a positive round and a tick", i)
		}
		if len([]rune(item.Note)) > MaxCollectionNoteRunes {
			return c, fmt.Errorf("moment %d: note must be at most %d characters", i, MaxCollectionNoteRunes)
		}
		key := CollectionItem{MatchID: item.MatchID, Round: item.Round, Tick: item.Tick}
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, item)
	}
	c.Items = items
	return c, nil
}
