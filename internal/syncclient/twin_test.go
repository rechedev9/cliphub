package syncclient_test

import (
	"context"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/syncclient"
)

func (d *device) notes(t *testing.T, matchID string) []string {
	t.Helper()
	rows, err := d.repos.DB().Query(`SELECT text FROM library_annotations WHERE match_id = ? ORDER BY text`, matchID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			t.Fatal(err)
		}
		out = append(out, text)
	}
	return out
}

func TestTwoDevicesWithTheSameDemoMeetOnOneID(t *testing.T) {
	hub, box := startHub(t)
	postJSON(t, hub.URL+"/auth/signup", map[string]string{"email": "luis@example.com", "username": "luis", "password": "correct horse battery"})
	postJSON(t, hub.URL+"/auth/confirm", map[string]string{"token": box.token(t, "luis@example.com", "Confirma"), "password": "correct horse battery"})
	ctx := context.Background()
	login := syncclient.ConnectInput{ServerURL: hub.URL, Login: "luis@example.com", Password: "correct horse battery"}

	// Both computers import the same demo before either syncs, so each gives it its own ids.
	sha := strings.Repeat("cd", 32)
	pc, laptop := newDevice(t, "PC"), newDevice(t, "Portátil")
	pcMatch, laptopMatch := pc.addMatch(t, sha), laptop.addMatch(t, sha)
	demoKey := artifacts.LibraryDemoKey(laptopMatch.DemoID)
	if err := laptop.files.Put(demoKey, strings.NewReader("demo")); err != nil {
		t.Fatal(err)
	}
	if _, err := laptop.repos.DB().Exec(`UPDATE library_demos SET path = ? WHERE id = ?`, demoKey, laptopMatch.DemoID.String()); err != nil {
		t.Fatal(err)
	}
	voiceKey := path.Join(laptopMatch.ID.String(), "track-1.ogg")
	if err := laptop.voice.Put(voiceKey, strings.NewReader("ogg")); err != nil {
		t.Fatal(err)
	}
	for _, n := range []struct {
		d     *device
		match library.Match
		text  string
	}{{pc, pcMatch, "nota del PC"}, {laptop, laptopMatch, "nota del portátil"}} {
		if err := n.d.repos.Library.CreateAnnotation(ctx, &library.Annotation{MatchID: n.match.ID, Round: 1, Text: n.text}); err != nil {
			t.Fatal(err)
		}
	}

	for _, d := range []*device{pc, laptop} {
		if _, err := d.client.Connect(ctx, login); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range []*device{pc, laptop, pc} {
		if err := d.client.Sync(ctx); err != nil {
			t.Fatalf("Sync: %v", err)
		}
	}

	// The laptop took the ids the PC gave the workspace first, and its files followed.
	var demoID, demoPath string
	if err := laptop.repos.DB().QueryRow(`SELECT d.id, d.path FROM library_matches m JOIN library_demos d ON d.id = m.demo_id WHERE m.id = ?`, pcMatch.ID.String()).Scan(&demoID, &demoPath); err != nil {
		t.Fatalf("the laptop has no match %s: %v", pcMatch.ID, err)
	}
	if demoID != pcMatch.DemoID.String() || demoPath != artifacts.LibraryDemoKey(pcMatch.DemoID) || laptop.matchCount(t) != 1 {
		t.Fatalf("laptop demo = %s %q, %d matches", demoID, demoPath, laptop.matchCount(t))
	}
	for key, want := range map[string]bool{
		artifacts.LibraryDemoKey(pcMatch.DemoID):              true,
		artifacts.LibraryTacticalIndexKey(pcMatch.DemoID):     true,
		artifacts.LibraryDemoKey(laptopMatch.DemoID):          false,
		artifacts.LibraryTacticalIndexKey(laptopMatch.DemoID): false,
	} {
		if ok, _ := laptop.files.Exists(key); ok != want {
			t.Errorf("laptop has %s = %v, want %v", key, ok, want)
		}
	}
	if ok, _ := laptop.voice.Exists(path.Join(pcMatch.ID.String(), "track-1.ogg")); !ok {
		t.Error("the laptop's voice track did not follow the match id")
	}

	// Each note reaches the other computer on the one match.
	want := []string{"nota del PC", "nota del portátil"}
	for name, d := range map[string]*device{"PC": pc, "laptop": laptop} {
		if got := d.notes(t, pcMatch.ID.String()); !slices.Equal(got, want) {
			t.Errorf("%s notes = %v, want %v", name, got, want)
		}
		if st := d.client.Status(ctx); st.Pending != 0 || st.LastError != "" {
			t.Errorf("%s status = %+v", name, st)
		}
	}
}
