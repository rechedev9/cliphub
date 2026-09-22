package customhud

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

func movementBytes(field protowire.Number, value []byte) []byte {
	return protowire.AppendBytes(protowire.AppendTag(nil, field, protowire.BytesType), value)
}

func movementValue(buttons uint64) []byte {
	return movementBytes(1, movementBytes(3, protowire.AppendVarint(protowire.AppendTag(nil, 1, protowire.VarintType), buttons)))
}

func movementMessage(slot, number, tick int32, full, delta []byte) *msg.CSVCMsg_UserCommands {
	cmd := &msg.CMsgServerUserCmd{PlayerSlot: proto.Int32(slot), CmdNumber: proto.Int32(number), ServerTickExecuted: proto.Int32(tick), Data: full}
	if delta != nil {
		cmd.ProtoReflect().SetUnknown(movementBytes(6, delta))
	}
	return &msg.CSVCMsg_UserCommands{Commands: []*msg.CMsgServerUserCmd{cmd}}
}

func assertMovement(t *testing.T, m *movementCommands, controller int, tick uint32, want uint64) {
	t.Helper()
	got := m.at(controller, tick)
	if got == nil || *got != want {
		t.Fatalf("controller %d at %d: got %v, want %d", controller, tick, got, want)
	}
}

func TestMovementCommandFullDeltaAndSourceClock(t *testing.T) {
	var m movementCommands
	m.read(movementMessage(8, 100, 17549, movementValue(65544), nil))
	assertMovement(t, &m, 9, 17549, 65544) // Shift + W, server clock (demo tick 5800).
	for _, tick := range []uint32{0, 5800, 17548, 17550} {
		if m.at(9, tick) != nil {
			t.Fatalf("stale/future input leaked at %d", tick)
		}
	}
	// An unrelated field update retains the exact previous button state.
	m.read(movementMessage(8, 101, 17550, nil, movementBytes(1, []byte{0x10, 101})))
	assertMovement(t, &m, 9, 17550, 65544)
	m.read(movementMessage(8, 102, 17551, nil, movementValue(66056)))
	assertMovement(t, &m, 9, 17551, 66056) // Shift + W + A.
	// Full snapshots start from defaults and can also carry a following delta.
	m.read(movementMessage(8, 103, 17552, movementBytes(1, []byte{0x10, 102}), movementValue(movementMask|1)))
	assertMovement(t, &m, 9, 17552, movementMask) // Never display attack.
	m.read(movementMessage(8, 104, 17553, movementBytes(1, []byte{0x10, 103}), nil))
	assertMovement(t, &m, 9, 17553, 0)
	// Other slots never replace the selected controller's inputs.
	m.read(movementMessage(7, 104, 17553, movementValue(512), nil))
	assertMovement(t, &m, 9, 17553, 0)
	assertMovement(t, &m, 8, 17553, 512)
	m.clear(9)
	if m.at(9, 17553) != nil {
		t.Fatal("disconnect retained input")
	}
	m.read(movementMessage(8, 105, 17554, nil, movementValue(8)))
	if m.at(9, 17554) != nil {
		t.Fatal("new occupant inherited previous occupant's baseline")
	}
}

func TestMovementCommandResetMarkers(t *testing.T) {
	for name, delta := range map[string][]byte{
		"base":    {0x0f},
		"buttons": movementBytes(1, []byte{0x1f}),
		"mask":    movementBytes(1, movementBytes(3, []byte{0x0f})),
	} {
		t.Run(name, func(t *testing.T) {
			var m movementCommands
			m.read(movementMessage(8, 1, 100, movementValue(movementMask), nil))
			m.read(movementMessage(8, 2, 101, nil, delta))
			assertMovement(t, &m, 9, 101, 0)
		})
	}
	// Unrelated reset markers and custom repeated payloads are skipped.
	data := append([]byte{0x17}, movementBytes(4, []byte{0xff, 0xff})...)
	got, err := mergeMovement(data, 0, 8)
	if err != nil || got != 8 {
		t.Fatalf("unrelated fields changed buttons: %d, %v", got, err)
	}
}

func TestMovementCommandBrokenChainsStayUnknownUntilFullSnapshot(t *testing.T) {
	for name, corrupt := range map[string]*msg.CSVCMsg_UserCommands{
		"truncated tag":     movementMessage(8, 101, 501, nil, []byte{0x80}),
		"truncated message": movementMessage(8, 101, 501, nil, []byte{0x0a, 10}),
		"wrong leaf wire":   movementMessage(8, 101, 501, nil, movementBytes(1, movementBytes(3, []byte{0x0a, 0}))),
		"zero field":        movementMessage(8, 101, 501, nil, []byte{0}),
		"command rewind":    movementMessage(8, 99, 501, nil, movementValue(512)),
		"clock rewind":      movementMessage(8, 101, 499, nil, movementValue(512)),
	} {
		t.Run(name, func(t *testing.T) {
			var m movementCommands
			m.read(movementMessage(8, 100, 500, movementValue(8), nil))
			m.read(corrupt)
			m.read(movementMessage(8, 102, 502, nil, movementValue(16)))
			if m.at(9, 502) != nil {
				t.Fatal("broken delta chain looked factual")
			}
			m.read(movementMessage(8, 1, 503, movementValue(512), nil))
			assertMovement(t, &m, 9, 503, 512)
		})
	}
	var m movementCommands
	for _, slot := range []int32{-1, 64, 2000000} {
		m.read(movementMessage(slot, 1, 500, movementValue(8), nil))
	}
	for _, controller := range []int{-1, 0, 1, 65} {
		if m.at(controller, 500) != nil {
			t.Fatal("invalid slot produced input")
		}
	}
	if _, err := commandDelta([]byte{0x32, 0xff}); err == nil {
		t.Fatal("truncated delta envelope accepted")
	}
}

func TestExtractRealMovement(t *testing.T) {
	path, target := os.Getenv("FULL_DEMO_HUD_DEMO"), os.Getenv("FULL_DEMO_HUD_TARGET")
	if path == "" || target == "" || os.Getenv("FULL_DEMO_HUD_EXPECT_MOVEMENT") != "1" {
		t.Skip("set FULL_DEMO_HUD_DEMO, FULL_DEMO_HUD_TARGET and FULL_DEMO_HUD_EXPECT_MOVEMENT=1 for movement acceptance")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	d, err := Extract(context.Background(), f, fmt.Sprintf("%x", h.Sum(nil)), target, 64)
	if err != nil {
		t.Fatal(err)
	}
	var actions uint64
	states := 0
	r, _ := NewRenderer("focus")
	for _, s := range d.Snapshots {
		for _, p := range s.Players {
			if p.Movement == nil {
				continue
			}
			if p.SteamID != target || !p.Alive || !p.Known || p.Inactive {
				t.Fatal("movement attached to an ineligible player")
			}
			actions |= *p.Movement
			states++
			plates := 0
			for _, n := range r.Scene(s, target) {
				if n.ID == "keys/W/plate" {
					plates++
				}
			}
			if plates != 1 {
				t.Fatal("real movement missing from renderer")
			}
		}
	}
	if actions != movementMask || states < 100 {
		t.Fatalf("incomplete real fixture coverage: actions=%d states=%d", actions, states)
	}
	t.Logf("verified all seven recorded actions in %d real source states", states)
}
