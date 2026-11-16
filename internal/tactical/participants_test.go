package tactical

import (
	"strings"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/constants"
	st "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/sendtables"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// testEntity is a property bag standing in for a demo entity; a missing property panics like a real one.
type testEntity struct {
	id    int
	pos   r3.Vector
	props map[string]any
}

func (e *testEntity) ServerClass() st.ServerClass                    { return nil }
func (e *testEntity) ID() int                                        { return e.id }
func (e *testEntity) SerialNum() int                                 { return 0 }
func (e *testEntity) Properties() []st.Property                      { return nil }
func (e *testEntity) Property(string) st.Property                    { return nil }
func (e *testEntity) BindProperty(string, any, st.PropertyValueType) {}
func (e *testEntity) Position() r3.Vector                            { return e.pos }
func (e *testEntity) OnPositionUpdate(func(r3.Vector))               {}
func (e *testEntity) OnDestroy(func())                               {}
func (e *testEntity) Destroy()                                       {}
func (e *testEntity) OnCreateFinished(func())                        {}

func (e *testEntity) PropertyValue(name string) (st.PropertyValue, bool) {
	v, ok := e.props[name]
	return st.PropertyValue{Any: v}, ok
}

func (e *testEntity) PropertyValueMust(name string) st.PropertyValue {
	v, ok := e.props[name]
	if !ok {
		panic("testEntity has no property " + name)
	}
	return st.PropertyValue{Any: v}
}

// testWorld resolves pawn handles for the players built on it.
type testWorld struct{ pawns map[uint64]st.Entity }

func (w *testWorld) IngameTick() int                              { return 59020 }
func (w *testWorld) TickRate() float64                            { return 64 }
func (w *testWorld) FindPlayerByHandle(uint64) *common.Player     { return nil }
func (w *testWorld) FindPlayerByPawnHandle(uint64) *common.Player { return nil }
func (w *testWorld) FindWeaponByEntityID(int) *common.Equipment   { return nil }
func (w *testWorld) FindEntityByHandle(h uint64) st.Entity        { return w.pawns[h] }

type testPlayer struct {
	steamID   uint64
	userID    int
	entityID  int
	team      common.Team
	connected bool
	// pawn false is a controller whose pawn is gone, the shape a stale reconnect leaves.
	pawn  bool
	pos   r3.Vector
	money int32
	equip uint64
}

func (w *testWorld) player(tp testPlayer) *common.Player {
	pl := common.NewPlayer(w)
	pl.SteamID64 = tp.steamID
	pl.UserID = tp.userID
	pl.EntityID = tp.entityID
	pl.Name = "player"
	pl.Team = tp.team
	pl.IsConnected = tp.connected
	invalid := uint64(constants.InvalidEntityHandleSource2)
	controller := &testEntity{id: tp.entityID, props: map[string]any{
		"m_hPawn":                           invalid,
		"m_hPlayerPawn":                     invalid,
		"m_bPawnIsAlive":                    false,
		"m_pInGameMoneyServices.m_iAccount": tp.money,
	}}
	pl.Entity = controller
	if !tp.pawn {
		return pl
	}
	handle := uint64(1000 + tp.entityID)
	controller.props["m_hPawn"] = handle
	controller.props["m_hPlayerPawn"] = handle
	w.pawns[handle] = &testEntity{id: tp.entityID + 100, pos: tp.pos, props: map[string]any{
		"m_iHealth":                         int32(100),
		"m_ArmorValue":                      int32(100),
		"m_fFlags":                          uint64(1),
		"m_hGroundEntity":                   uint64(0),
		"m_bIsScoped":                       false,
		"m_angEyeAngles":                    [3]float32{0, 90, 0},
		"m_szLastPlaceName":                 "BombsiteA",
		"m_pItemServices.m_bHasDefuser":     false,
		"m_pItemServices.m_bHasHelmet":      true,
		"m_pWeaponServices.m_hActiveWeapon": invalid,
		"m_unCurrentEquipmentValue":         tp.equip,
	}}
	return pl
}

func testScanner() *scanner {
	s := newScanner(nil, Options{}, DefaultSampleHZ)
	s.tickrate = 64
	s.sampleTicks = 8
	s.current = &roundAcc{
		number:    1,
		tickStart: 58000,
		stats:     map[uint8]*playerRoundAcc{},
		sides:     map[uint8]tacticalplan.Side{},
	}
	return s
}

// duplicateCases list one player twice, the two shapes demoinfocs v5.3.0 produces, in both orders.
func duplicateCases(w *testWorld) map[string][]*common.Player {
	ct := w.player(testPlayer{steamID: 10, userID: 1, entityID: 1, team: common.TeamCounterTerrorists, connected: true, pawn: true, pos: r3.Vector{X: 100, Y: 100}, money: 1000, equip: 3000})
	live := w.player(testPlayer{steamID: 20, userID: 9, entityID: 2, team: common.TeamTerrorists, connected: true, pawn: true, pos: r3.Vector{X: -500, Y: 250, Z: 8}, money: 4000, equip: 4700})
	stale := w.player(testPlayer{steamID: 20, userID: 3, entityID: 7, team: common.TeamTerrorists, money: 800})
	// The stale controller still holds T while the new one is connected but not on a team yet.
	staleOnTeam := w.player(testPlayer{steamID: 20, userID: 4, entityID: 3, team: common.TeamTerrorists, pawn: true, pos: r3.Vector{X: -500, Y: 250, Z: 8}, money: 4000, equip: 4700})
	unassigned := w.player(testPlayer{steamID: 20, userID: 11, entityID: 8, team: common.TeamUnassigned, connected: true, pawn: true, pos: r3.Vector{X: 40, Y: 40}, money: 800})
	return map[string][]*common.Player{
		"reconnect leaves a stale player":       {ct, live, stale},
		"reconnect with the stale player first": {ct, stale, live},
		"same player under two user ids":        {ct, live, live},
		"new controller not on a team yet":      {ct, staleOnTeam, unassigned},
	}
}

func TestDuplicateParticipantsLeaveOneSamplePerSlot(t *testing.T) {
	for name, players := range duplicateCases(&testWorld{pawns: map[uint64]st.Entity{}}) {
		t.Run(name, func(t *testing.T) {
			s := testScanner()
			s.sampleFrame(59020, players)
			frames := s.current.frames
			if len(frames) != 1 || len(frames[0].Samples) != 2 {
				t.Fatalf("frames = %+v, want one frame with two samples", frames)
			}
			got := frames[0].Samples[1]
			if got.Slot != 1 || got.X != -500 || got.Y != 250 || !got.Flags.Has(tacticalplan.FlagAlive) {
				t.Fatalf("slot 1 sample = %+v, want the live player", got)
			}
			if !strings.Contains(strings.Join(s.warnings, "\n"), "listed twice at tick 59020") {
				t.Fatalf("warnings = %q, want the duplicate noted", s.warnings)
			}
			if _, err := s.finish(); err != nil {
				t.Fatalf("finish: %v", err)
			}
		})
	}
}

func TestEconomyCountsADuplicatedPlayerOnce(t *testing.T) {
	for name, players := range duplicateCases(&testWorld{pawns: map[uint64]st.Entity{}}) {
		t.Run(name, func(t *testing.T) {
			s := testScanner()
			s.recordEconomy(59020, players)
			eco := s.current.economy
			if eco.TMoney != 4000 || eco.TEquipValue != 4700 || eco.CTMoney != 1000 || eco.CTEquipValue != 3000 {
				t.Fatalf("economy = %+v, want each player counted once", eco)
			}
		})
	}
}

func TestPlayersBySlotKeepsThePreviousEntityOnATie(t *testing.T) {
	w := &testWorld{pawns: map[uint64]st.Entity{}}
	first := w.player(testPlayer{steamID: 20, userID: 1, entityID: 2, team: common.TeamTerrorists})
	second := w.player(testPlayer{steamID: 20, userID: 2, entityID: 5, team: common.TeamTerrorists})
	s := testScanner()
	if got := s.playersBySlot(1, []*common.Player{first, second}); got[0].player != second {
		t.Fatal("with nothing else to go on, the higher EntityID must win")
	}
	s.picker.lastEntity[0] = 2
	if got := s.playersBySlot(2, []*common.Player{second, first}); got[0].player != first {
		t.Fatal("the EntityID of the previous frame must win over a higher one")
	}
}
