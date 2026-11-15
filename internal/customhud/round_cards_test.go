package customhud

import (
	"fmt"
	"strings"
	"testing"

	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	st "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/sendtables"
)

func roundState(round, ct, tr int, swapped bool) Snapshot {
	s := Example()
	s.Round, s.CTScore, s.TScore = round, ct, tr
	if swapped {
		for i := range s.Players {
			s.Players[i].Side = map[string]string{"CT": "T", "T": "CT"}[s.Players[i].Side]
		}
	}
	return s
}

func sceneNodes(t *testing.T, s Snapshot) map[string]Node {
	t.Helper()
	r, err := NewRenderer("arena")
	if err != nil {
		t.Fatal(err)
	}
	nodes := map[string]Node{}
	for _, n := range r.Scene(s, ExampleTarget) {
		nodes[n.ID] = n
	}
	return nodes
}

func TestRoundTrackerFollowsTheTeamAcrossHalftime(t *testing.T) {
	tracker := &roundTracker{}
	var got Snapshot
	tracker.next(roundState(1, 0, 0, false))
	// The first team on CT wins four rounds in a row.
	for round := 1; round <= 4; round++ {
		got = tracker.next(roundState(round, round, 0, false))
	}
	if got.Streak != (Streak{Side: "CT", Rounds: 4}) {
		t.Fatalf("streak before halftime=%+v", got.Streak)
	}
	// Halftime swaps sides and scores without a round win.
	got = tracker.next(roundState(5, 0, 4, true))
	if got.Streak != (Streak{Side: "T", Rounds: 4}) {
		t.Fatalf("halftime moved or broke the streak: %+v", got.Streak)
	}
	got = tracker.next(roundState(5, 0, 5, true))
	if got.Streak != (Streak{Side: "T", Rounds: 5}) {
		t.Fatalf("streak after halftime=%+v", got.Streak)
	}
	got = tracker.next(roundState(6, 1, 5, true))
	if got.Streak != (Streak{Side: "CT", Rounds: 1}) {
		t.Fatalf("a win by the other team did not end the streak: %+v", got.Streak)
	}
}

func TestRoundTrackerCountsKillsSinceTheRoundBegan(t *testing.T) {
	tracker := &roundTracker{}
	start := roundState(3, 0, 2, false)
	tracker.next(start)
	live := roundState(3, 0, 2, false)
	live.Players[1].Kills += 3
	live.Players[1].Known = true
	got := tracker.next(live)
	if got.Players[1].RoundKills != 3 || got.Players[0].RoundKills != 0 {
		t.Fatalf("round kills=%d/%d", got.Players[1].RoundKills, got.Players[0].RoundKills)
	}
	if live.Players[1].RoundKills != 0 {
		t.Fatal("tracker mutated the recorded snapshot")
	}
	next := roundState(4, 0, 3, false)
	next.Players[1].Kills = live.Players[1].Kills
	if got := tracker.next(next); got.Players[1].RoundKills != 0 {
		t.Fatalf("kills carried into the next round: %d", got.Players[1].RoundKills)
	}
}

func TestRoundCardsShowOnlyRecordedFacts(t *testing.T) {
	s := Example()
	s.Players[1].RoundKills = 3
	s.Players[8].RoundKills = 5
	s.Streak = Streak{Side: "T", Rounds: 6}
	nodes := sceneNodes(t, s)
	for id, want := range map[string]string{
		"player/" + s.Players[1].SteamID + "/round-kills": "3K",
		"player/" + s.Players[8].SteamID + "/round-kills": "ACE",
		"score/t-streak": "6 IN A ROW",
	} {
		if nodes[id].Text != want || !nodes[id].FadeIn {
			t.Fatalf("%s=%+v, want %q with a fade", id, nodes[id], want)
		}
	}
	if _, ok := nodes["score/ct-streak"]; ok {
		t.Fatal("streak drawn on the losing side")
	}
	if _, ok := nodes["player/"+s.Players[0].SteamID+"/round-kills"]; ok {
		t.Fatal("badge for fewer than three kills")
	}
	s.Players[2].Known = false
	if _, ok := sceneNodes(t, s)["score/ct-alive"]; ok {
		t.Fatal("alive count drawn although one CT slot is unknown")
	}

	s = Example()
	s.Phase = "defusing"
	s.Defuse = &Defuse{Kit: true, StartTick: 10, EndTick: 330}
	nodes = sceneNodes(t, s)
	if nodes["defuse/label"].Text != "WITH KIT" || nodes["defuse/kit"].ID == "" {
		t.Fatalf("kit defuse=%+v", nodes["defuse/label"])
	}
	s.Defuse.Kit = false
	if nodes = sceneNodes(t, s); nodes["defuse/label"].Text != "NO KIT" || nodes["defuse/kit"].ID != "" {
		t.Fatalf("no-kit defuse=%+v", nodes["defuse/label"])
	}

	s = Example()
	s.Phase = "ended"
	s.Players[6].RoundKills = 2
	s.MVP = &MVP{SteamID: s.Players[6].SteamID, Reason: MVPBombPlanted}
	nodes = sceneNodes(t, s)
	if nodes["mvp/name"].Text != s.Players[6].Name || nodes["mvp/detail"].Text != "2 KILLS · BOMB PLANTED" {
		t.Fatalf("mvp card=%q %q", nodes["mvp/name"].Text, nodes["mvp/detail"].Text)
	}
	s.Players[6].RoundKills, s.MVP.Reason = 0, ""
	if _, ok := sceneNodes(t, s)["mvp/detail"]; ok {
		t.Fatal("mvp card invented a reason")
	}
	s.Phase = "live"
	if _, ok := sceneNodes(t, s)["mvp/name"]; ok {
		t.Fatal("mvp card outside the round end")
	}
}

func defuseTimeline(kit bool, seconds int) Timeline {
	d := testTimeline()
	d.EndTick = 2000
	planted := Example()
	planted.Tick, planted.Phase = 100, "planted"
	d.Snapshots = []Snapshot{planted}
	for tick := 164; tick <= 164+seconds*64; tick += 64 {
		s := Example()
		s.Tick, s.Phase, s.TimeRemaining = tick, "defusing", 30-(tick-164)/64
		s.Defuse = &Defuse{Kit: kit, StartTick: 164, EndTick: 164 + seconds*64}
		d.Snapshots = append(d.Snapshots, s)
	}
	ended := Example()
	ended.Tick, ended.Phase = 164+seconds*64+1, "ended"
	return Timeline{Version: d.Version, DemoSHA256: d.DemoSHA256, TargetSteamID: d.TargetSteamID, TickRate: 64, EndTick: d.EndTick, Snapshots: append(d.Snapshots, ended)}
}

func TestDefuseFillFollowsTheRecordedCompletionAcrossTrims(t *testing.T) {
	r, err := NewRenderer("arena")
	if err != nil {
		t.Fatal(err)
	}
	x, y, w, h := defuseTrack()
	for _, tc := range []struct {
		kit     bool
		seconds int
		offset  int64
		want    string
	}{
		// A kit defuse fills in five seconds from the first defusing frame.
		{true, 5, 0, fmt.Sprintf(`Dialogue: 6,0:00:01.00,0:00:06.01,HUD,defuse,0,0,0,,{\clip(%d,%d,%d,%d)\t(0,5000,\clip(%d,%d,%d,%d))`, x, y, x, y+h, x, y, x+w, y+h)},
		// A trim halfway through a ten second defuse starts the fill at half.
		{false, 10, 360, fmt.Sprintf(`Dialogue: 6,0:00:00.00,0:00:05.01,HUD,defuse,0,0,0,,{\clip(%d,%d,%d,%d)\t(0,5000,\clip(%d,%d,%d,%d))`, x, y, x+w/2, y+h, x, y, x+w, y+h)},
	} {
		ass, err := r.ASS(defuseTimeline(tc.kit, tc.seconds), Window{StartTick: 100, SourceOffsetFrames: tc.offset, Frames: 900})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(ass, tc.want) {
			t.Fatalf("missing defuse fill %s in\n%s", tc.want, ass)
		}
	}
	d := defuseTimeline(true, 5)
	for i := range d.Snapshots {
		if d.Snapshots[i].Defuse != nil {
			d.Snapshots[i].Defuse.EndTick = 0
		}
	}
	ass, err := r.ASS(d, Window{StartTick: 100, Frames: 900})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ass, ",defuse,") {
		t.Fatal("drew a defuse fill without the recorded completion time")
	}
}

func TestRoundCardsFadeInOnlyWhenTheyAppear(t *testing.T) {
	r, err := NewRenderer("arena")
	if err != nil {
		t.Fatal(err)
	}
	d := testTimeline()
	ended := Example()
	ended.Tick, ended.Phase = 300, "ended"
	ended.MVP = &MVP{SteamID: ended.Players[1].SteamID, Reason: MVPEliminations}
	d.Snapshots = append(d.Snapshots, ended)
	fade := fmt.Sprintf(`{\fad(%d,0)`, cardFadeFrames*1000/60)
	ass, err := r.ASS(d, Window{StartTick: 100, Frames: 240})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ass, fade) || !strings.Contains(ass, "MOST ELIMINATIONS") {
		t.Fatal("mvp card did not fade in when the round ended")
	}
	ass, err = r.ASS(d, Window{StartTick: 300, Frames: 60})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ass, fade) {
		t.Fatal("a card already on screen at the window start faded in again")
	}
}

func TestRoundFactsComeFromTheBombAndTheGame(t *testing.T) {
	bomb := propertyEntity{values: map[string]st.PropertyValue{"m_flDefuseCountDown": {Any: float32(410)}}}
	if got := defuseEndTick(bomb, 25600, 64, 9000); got != 9000+10*64 {
		t.Fatalf("defuse end=%d", got)
	}
	if defuseEndTick(nil, 25600, 64, 9000) != 0 || defuseEndTick(propertyEntity{}, 25600, 64, 9000) != 0 || defuseEndTick(bomb, 26300, 64, 9000) != 0 {
		t.Fatal("invented a defuse length")
	}
	d := extractFrames(t,
		func(p *extractionParser) {
			p.state.tick = 100
			p.dispatch.Dispatch(events.RoundFreezetimeEnd{})
		},
		func(p *extractionParser) { p.state.tick = 164; p.dispatch.Dispatch(events.BombPlanted{}) },
		func(p *extractionParser) {
			p.state.tick = 228
			p.dispatch.Dispatch(events.BombDefuseStart{HasKit: true})
		},
		func(p *extractionParser) { p.state.tick = 292; p.dispatch.Dispatch(events.RoundEnd{}) },
		func(p *extractionParser) {
			p.state.tick = 356
			p.dispatch.Dispatch(events.RoundMVPAnnouncement{Player: &common.Player{SteamID64: 76561199000000001}, Reason: events.MVPReasonBombDefused})
		},
		func(p *extractionParser) { p.state.tick, p.state.rounds, p.state.freeze = 420, 1, true },
	)
	defusing, _ := d.At(228)
	if defusing.Defuse == nil || !defusing.Defuse.Kit || defusing.Defuse.StartTick != 228 || defusing.Defuse.EndTick != 0 {
		t.Fatalf("defuse=%+v", defusing.Defuse)
	}
	if ended, _ := d.At(292); ended.Defuse != nil || ended.MVP != nil {
		t.Fatal("defuse outlived the round or mvp came before the announcement")
	}
	if mvp, _ := d.At(356); mvp.MVP == nil || *mvp.MVP != (MVP{SteamID: ExampleTarget, Reason: MVPBombDefused}) {
		t.Fatalf("mvp=%+v", mvp.MVP)
	}
	if freeze, _ := d.At(420); freeze.MVP != nil {
		t.Fatal("mvp card stayed into the next round")
	}
	bad := d
	bad.Snapshots = append([]Snapshot(nil), d.Snapshots...)
	bad.Snapshots[0].MVP = &MVP{SteamID: ExampleTarget}
	if bad.Validate() == nil {
		t.Fatal("accepted an mvp outside the round end")
	}
}
