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

func TestCleanPlaylistGivesANewNoteItsIDAndOrdersTheNotesByTick(t *testing.T) {
	kept := uuid.New()
	p, err := CleanPlaylist(Playlist{Name: "x", Rounds: []PlaylistRound{{MatchID: uuid.New(), Round: 2, Notes: []PlaylistNote{
		{ID: kept, Tick: 900, Text: " tarde "},
		{Tick: 100, Strokes: []Stroke{{Color: "t", Width: 4, Points: [][2]float64{{0, 0}, {10, 10}}}}},
	}}}})
	if err != nil {
		t.Fatal(err)
	}
	notes := p.Rounds[0].Notes
	if len(notes) != 2 || notes[0].Tick != 100 || notes[0].ID == uuid.Nil || notes[1].ID != kept || notes[1].Text != "tarde" {
		t.Fatalf("notes = %+v; want the drawing first with a new id, then the kept note trimmed", notes)
	}
	twice := []PlaylistNote{{ID: kept, Text: "a"}, {ID: kept, Text: "b"}}
	if _, err := CleanPlaylist(Playlist{Name: "x", Rounds: []PlaylistRound{{MatchID: uuid.New(), Round: 1, Notes: twice}}}); err == nil {
		t.Fatal("two notes with one id: no error")
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
		"an empty note":         {Name: "x", Rounds: []PlaylistRound{{MatchID: id, Round: 1, Notes: []PlaylistNote{{Tick: 5}}}}},
		"a note off the map": {Name: "x", Rounds: []PlaylistRound{{MatchID: id, Round: 1, Notes: []PlaylistNote{
			{Strokes: []Stroke{{Color: "accent", Width: 2, Points: [][2]float64{{0, 0}, {1e9, 0}}}}},
		}}}},
	} {
		if _, err := CleanPlaylist(p); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
