package library

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestCleanPlaylistKeepsTheFirstOfARepeatedRoundAndDerivesTheMaps(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	p, err := CleanPlaylist(Playlist{Name: "  FUT   T ", Folder: " Rivales ", Rounds: []PlaylistRound{
		{MatchID: a, Round: 3, Map: "DE_NUKE", Comment: " primero "},
		{MatchID: b, Round: 1, Map: "de_mirage"},
		{MatchID: a, Round: 3, Map: "de_nuke", Comment: "repetida"},
		{MatchID: a, Round: 4, Map: "de_nuke", Missing: true},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "FUT T" || p.Folder != "Rivales" || len(p.Rounds) != 3 || p.Rounds[0].Comment != "primero" || p.Rounds[2].Missing {
		t.Fatalf("cleaned = %+v", p)
	}
	if len(p.Maps) != 2 || p.Maps[0] != "de_nuke" || p.Maps[1] != "de_mirage" {
		t.Fatalf("maps = %v, want de_nuke then de_mirage", p.Maps)
	}
}

func TestCleanPlaylistRefusesWhatAPlaylistCannotHold(t *testing.T) {
	id := uuid.New()
	tooMany := make([]PlaylistRound, MaxPlaylistRounds+1)
	for i := range tooMany {
		tooMany[i] = PlaylistRound{MatchID: id, Round: i%MaxPlaylistRound + 1}
	}
	tooMany[MaxPlaylistRounds].MatchID = uuid.New()
	for name, p := range map[string]Playlist{
		"no name":               {},
		"a long folder":         {Name: "x", Folder: strings.Repeat("a", MaxPlaylistFolderRunes+1)},
		"a control character":   {Name: "x\x00y"},
		"a round without match": {Name: "x", Rounds: []PlaylistRound{{Round: 1}}},
		"round zero":            {Name: "x", Rounds: []PlaylistRound{{MatchID: id}}},
		"a bad hash":            {Name: "x", Rounds: []PlaylistRound{{MatchID: id, Round: 1, DemoSHA256: "abc"}}},
		"a bad map":             {Name: "x", Rounds: []PlaylistRound{{MatchID: id, Round: 1, Map: "../x"}}},
		"too many rounds":       {Name: "x", Rounds: tooMany},
	} {
		if _, err := CleanPlaylist(p); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
