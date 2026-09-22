package customhud

import (
	"fmt"

	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"
	"google.golang.org/protobuf/encoding/protowire"
)

const movementMask = uint64(common.ButtonForward | common.ButtonBack | common.ButtonMoveLeft | common.ButtonMoveRight | common.ButtonJump | common.ButtonDuck | common.ButtonSpeed)

// CS2 records commands per controller slot, independently of pawn properties.
// Recent demos use CMsgServerUserCmd.delta_data (field 6), not declared in our
// pinned v5 parser. Its protobuf unknown fields preserve that payload verbatim.
// Valve deltas reset fields with wire type 7; they are NOT ordinary protobuf.
// Encoding reference: demoinfocs-golang commit
// 14db58bad6e6ac2cb794b441c7b3d0d2a6dd1752, s2_usercmd_buttons.go.
// Adapted button traversal: see LICENSE.demoinfocs.txt for its MIT notice.
// Keep this adapter limited to buttons rather than upgrading the entire parser.
type movementCommands struct {
	slots [64]movementCommand
}

type movementCommand struct {
	buttons uint64
	number  int32
	tick    int32
	known   bool
}

func (m *movementCommands) clear(controller int) {
	if controller > 0 && controller <= len(m.slots) {
		m.slots[controller-1] = movementCommand{}
	}
}

func (m *movementCommands) read(message *msg.CSVCMsg_UserCommands) {
	for _, cmd := range message.GetCommands() {
		if cmd == nil || cmd.GetPlayerSlot() < 0 || int(cmd.GetPlayerSlot()) >= len(m.slots) {
			continue
		}
		state := &m.slots[cmd.GetPlayerSlot()]
		next, err := decodeMovementCommand(cmd, *state)
		if err != nil {
			// A malformed delta invalidates its chain until a full snapshot.
			// Keeping the previous mask would paint keys that may be released.
			*state = movementCommand{}
			continue
		}
		*state = next
	}
}

func (m *movementCommands) at(controller int, serverTick uint32) *uint64 {
	if controller < 1 || controller > len(m.slots) || serverTick == 0 {
		return nil
	}
	s := m.slots[controller-1]
	// The network and demo seek clocks have different origins. Sample at
	// FrameDone using the network tick, then store on the demo's source tick.
	// Never extend a missing command across frames or read future inputs.
	if !s.known || int64(s.tick) != int64(serverTick) {
		return nil
	}
	buttons := s.buttons & movementMask
	return &buttons
}

func decodeMovementCommand(cmd *msg.CMsgServerUserCmd, previous movementCommand) (movementCommand, error) {
	invalid := func() (movementCommand, error) {
		return movementCommand{}, fmt.Errorf("unknown movement command baseline")
	}
	if cmd.CmdNumber == nil || cmd.ServerTickExecuted == nil || cmd.GetServerTickExecuted() < 0 {
		return invalid()
	}
	delta, err := commandDelta(cmd.ProtoReflect().GetUnknown())
	if err != nil {
		return movementCommand{}, err
	}
	if len(cmd.Data) == 0 && len(delta) == 0 {
		return previous, nil
	}
	var buttons uint64
	if len(cmd.Data) > 0 {
		buttons, err = mergeMovement(cmd.Data, 0, 0)
	} else {
		if !previous.known || cmd.GetCmdNumber() < previous.number || cmd.GetServerTickExecuted() < previous.tick {
			return invalid()
		}
		buttons = previous.buttons
	}
	if err == nil && len(delta) > 0 {
		buttons, err = mergeMovement(delta, 0, buttons)
	}
	if err != nil {
		return movementCommand{}, err
	}
	return movementCommand{buttons: buttons, number: cmd.GetCmdNumber(), tick: cmd.GetServerTickExecuted(), known: true}, nil
}

func commandDelta(unknown []byte) ([]byte, error) {
	var delta []byte
	for len(unknown) > 0 {
		field, wire, n := protowire.ConsumeTag(unknown)
		if n < 0 {
			return nil, protowire.ParseError(n)
		}
		unknown = unknown[n:]
		if field == 6 {
			if wire != protowire.BytesType {
				return nil, fmt.Errorf("invalid command delta wire type %d", wire)
			}
			delta, n = protowire.ConsumeBytes(unknown)
		} else {
			n = protowire.ConsumeFieldValue(field, wire, unknown)
		}
		if n < 0 {
			return nil, protowire.ParseError(n)
		}
		unknown = unknown[n:]
	}
	return delta, nil
}

// Follow only base(1).buttons_pb(3).buttonstate1(1). Skipping unrelated
// length-delimited payloads also skips Valve's custom repeated-field encoding.
func mergeMovement(payload []byte, depth int, buttons uint64) (uint64, error) {
	path := [...]protowire.Number{1, 3, 1}
	for len(payload) > 0 {
		field, wire, n := protowire.ConsumeTag(payload)
		if n < 0 {
			return 0, protowire.ParseError(n)
		}
		payload = payload[n:]
		if wire == 7 {
			if field == path[depth] {
				buttons = 0
			}
			continue
		}
		if field != path[depth] {
			n = protowire.ConsumeFieldValue(field, wire, payload)
		} else if depth == len(path)-1 && wire == protowire.VarintType {
			buttons, n = protowire.ConsumeVarint(payload)
		} else if depth < len(path)-1 && wire == protowire.BytesType {
			var nested []byte
			nested, n = protowire.ConsumeBytes(payload)
			if n >= 0 {
				var err error
				buttons, err = mergeMovement(nested, depth+1, buttons)
				if err != nil {
					return 0, err
				}
			}
		} else {
			return 0, fmt.Errorf("invalid movement wire type %d at depth %d", wire, depth)
		}
		if n < 0 {
			return 0, protowire.ParseError(n)
		}
		payload = payload[n:]
	}
	return buttons, nil
}
