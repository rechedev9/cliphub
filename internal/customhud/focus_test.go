package customhud

import (
	"strings"
	"testing"

	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	st "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/sendtables"
)

func TestMovementRequiresRecordedPropertyAndIgnoresUnshownActions(t *testing.T) {
	for _, entity := range []st.Entity{nil, propertyEntity{}, propertyEntity{values: map[string]st.PropertyValue{"m_pMovementServices.m_nButtonDownMaskPrev": {Any: uint32(1)}}}} {
		if sourceMovement(entity) != nil {
			t.Fatal("missing input became released keys")
		}
	}
	for _, mask := range []uint64{0, uint64(common.ButtonForward | common.ButtonDuck | common.ButtonAttack)} {
		got := sourceMovement(propertyEntity{values: map[string]st.PropertyValue{"m_pMovementServices.m_nButtonDownMaskPrev": {Any: mask}}})
		if got == nil || *got != mask&^uint64(common.ButtonAttack) {
			t.Fatalf("movement mask=%v", got)
		}
	}
}

func TestFocusKeepsTargetAndSuppressesUnknownAndDeadInputs(t *testing.T) {
	r, _ := NewRenderer("focus")
	state := Example()
	nodes := func() map[string]Node {
		out := map[string]Node{}
		for _, node := range r.Scene(state, ExampleTarget) {
			out[node.ID] = node
		}
		return out
	}
	before := nodes()
	if before["keys/A/plate"].Color != r.Theme.T || before["keys/W/plate"].Color == r.Theme.T {
		t.Fatal("keys do not follow source movement")
	}
	if before["focus/money"].Text != "$2600" || before["focus/name"].Text != "donk" {
		t.Fatal("lost target statistics")
	}
	roster := 0
	for id, n := range before {
		if strings.HasPrefix(id, "player/") && strings.HasSuffix(id, "/name") {
			roster++
			if n.Y > 94 {
				t.Fatalf("roster left the top row: %+v", n)
			}
		}
	}
	if roster != 10 || before["score/time"].Text == "" {
		t.Fatalf("Focus lost the upper strip: roster=%d", roster)
	}
	state.Players[0], state.Players[1] = state.Players[1], state.Players[0]
	if nodes()["focus/name"].Text != "donk" {
		t.Fatal("target followed roster order")
	}
	r.Portrait = true
	if _, exists := nodes()["focus/side"]; exists {
		t.Fatal("portrait has a badge painted behind it")
	}
	for _, mode := range []string{"unknown", "dead", "disconnected", "missing-input"} {
		p := state.Players[0]
		switch mode {
		case "unknown":
			state.Players[0].Known = false
		case "dead":
			state.Players[0].Alive = false
		case "disconnected":
			state.Players[0].Inactive = true
		case "missing-input":
			state.Players[0].Movement = nil
		}
		for id := range nodes() {
			if strings.HasPrefix(id, "keys/") {
				t.Fatalf("%s displays factual keys", mode)
			}
		}
		state.Players[0] = p
	}
}

func TestFocusInputTimingSurvivesTrim(t *testing.T) {
	r, _ := NewRenderer("focus")
	d := testTimeline()
	forward := uint64(common.ButtonForward)
	d.Snapshots[1].Players[1].Movement = &forward
	var activeW Node
	for _, node := range r.Scene(d.Snapshots[1], ExampleTarget) {
		if node.ID == "keys/W/plate" {
			activeW = node
		}
	}
	ass, err := r.ASS(d, Window{StartTick: 100, SourceOffsetFrames: 30, Frames: 150})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ass, "Dialogue: 2,0:00:00.50,0:00:01.50,HUD,,0,0,0,,"+assNode(activeW)) {
		t.Fatal("trim shifted movement timing")
	}
}
