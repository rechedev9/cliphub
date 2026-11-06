package library

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestCleanCollection(t *testing.T) {
	match := uuid.MustParse("aaaaaaaa-0000-4000-8000-000000000001")
	moment := CollectionItem{MatchID: match, Round: 4, Tick: 9000}
	cases := []struct {
		name    string
		in      Collection
		wantErr string
	}{
		{name: "empty list", in: Collection{Name: "  Clutches  de  ropz "}},
		{name: "no name", in: Collection{Name: "   "}, wantErr: "name"},
		{name: "long name", in: Collection{Name: strings.Repeat("a", MaxCollectionNameRunes+1)}, wantErr: "name"},
		{name: "no match", in: Collection{Name: "x", Items: []CollectionItem{{Round: 1}}}, wantErr: "match"},
		{name: "round zero", in: Collection{Name: "x", Items: []CollectionItem{{MatchID: match, Round: 0}}}, wantErr: "round"},
		{name: "negative tick", in: Collection{Name: "x", Items: []CollectionItem{{MatchID: match, Round: 1, Tick: -1}}}, wantErr: "tick"},
		{name: "long note", in: Collection{Name: "x", Items: []CollectionItem{{MatchID: match, Round: 1, Note: strings.Repeat("n", MaxCollectionNoteRunes+1)}}}, wantErr: "note"},
		{name: "too many", in: Collection{Name: "x", Items: make([]CollectionItem, MaxCollectionItems+1)}, wantErr: "at most"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CleanCollection(tc.in)
			if tc.wantErr == "" && err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("error = %v, want one about %q", err, tc.wantErr)
			}
		})
	}

	got, err := CleanCollection(Collection{Name: "  Clutches  de  ropz ", Items: []CollectionItem{
		{MatchID: match, Round: 4, Tick: 9000, Note: "  1v3  "}, moment, {MatchID: match, Round: 2, Tick: 100},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Clutches de ropz" || len(got.Items) != 2 || got.Items[0].Note != "1v3" || got.Items[1].Round != 2 {
		t.Fatalf("clean = %+v, want the name tidied, the note trimmed and the repeated moment dropped in order", got)
	}
}
