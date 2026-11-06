package tacticalplan

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"flag"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var updateGearFixture = flag.Bool("update-gear-fixture", false, "rewrite the web decoder's zvpos gear fixture")

// gearFixturePath is read by web/lib/tactical-decode-gear.test.ts.
var gearFixturePath = filepath.Join("..", "..", "web", "lib", "testdata", "zvpos-gear-fixture.json")

func item(name string, count int) InventoryItem {
	return InventoryItem{Item: EquipmentByName(name), Count: count}
}

// Inventories are listed in id order, the order the decoder returns.
func rifleGear() *Gear {
	return &Gear{
		ActiveWeapon: EquipmentByName("ak47"),
		Inventory:    []InventoryItem{item("glock", 1), item("ak47", 1), item("flashbang", 2), item("smokegrenade", 1), item("c4", 1), item("knife", 1)},
		Armor:        100, Helmet: true, Money: 3150,
	}
}

func gearRounds() []RoundFrames {
	rifle := rifleGear()
	flashed := *rifle
	flashed.ActiveWeapon = EquipmentByName("flashbang")
	flashed.FlashSeconds = 2.3
	ct := &Gear{
		ActiveWeapon: EquipmentByName("m4a1_silencer"),
		Inventory:    []InventoryItem{item("usp_silencer", 1), item("m4a1_silencer", 1), item("hegrenade", 1), item("knife", 1)},
		Armor:        87, Defuser: true, Money: 16000,
	}
	return []RoundFrames{
		{Round: 1, Frames: []Frame{
			{Tick: 100, Samples: []Sample{
				{Slot: 0, X: 10, Yaw: 90, Health: 100, Flags: FlagAlive | FlagSideT, Pitch: -12.5, Gear: rifle},
				{Slot: 4, X: 200, Yaw: 180, Health: 100, Flags: FlagAlive, Pitch: 3, Gear: ct},
			}},
			{Tick: 108, Samples: []Sample{
				{Slot: 0, X: 12, Yaw: 91, Health: 100, Flags: FlagAlive | FlagSideT, Pitch: -12.5, Gear: rifle},
				{Slot: 4, X: 201, Yaw: 181, Health: 73, Flags: FlagAlive | FlagBlinded, Pitch: 45, Gear: ct},
			}},
			{Tick: 116, Samples: []Sample{
				{Slot: 0, X: 14, Yaw: 92, Health: 100, Flags: FlagAlive | FlagSideT, Pitch: 88, Gear: &flashed},
			}},
		}},
		{Round: 2, Frames: []Frame{
			{Tick: 900, Samples: []Sample{
				{Slot: 0, X: 0, Health: 100, Flags: FlagAlive | FlagSideT, Pitch: 0, Gear: rifle},
			}},
		}},
	}
}

func encodeGearRounds(t *testing.T) Blob {
	t.Helper()
	blob, err := EncodePositions(gearRounds(), 8, 64)
	if err != nil {
		t.Fatalf("EncodePositions: %v", err)
	}
	return blob
}

func TestGearRoundTrip(t *testing.T) {
	blob := encodeGearRounds(t)
	_, frames, err := DecodePositions(blob.Data)
	if err != nil {
		t.Fatalf("DecodePositions: %v", err)
	}
	in := gearRounds()
	want := append(append([]Frame(nil), in[0].Frames...), in[1].Frames...)
	for i, f := range frames {
		for j, got := range f.Samples {
			w := want[i].Samples[j]
			if math.Abs(got.Pitch-w.Pitch) > 0.36 {
				t.Fatalf("frame %d slot %d pitch = %v, want %v", i, got.Slot, got.Pitch, w.Pitch)
			}
			if !reflect.DeepEqual(*got.Gear, *w.Gear) {
				t.Fatalf("frame %d slot %d gear = %+v, want %+v", i, got.Slot, *got.Gear, *w.Gear)
			}
		}
	}
	// Unchanged gear is shared, a change makes a new value.
	if frames[0].Samples[0].Gear != frames[1].Samples[0].Gear {
		t.Fatal("an unchanged loadout must share its decoded Gear")
	}
	if frames[1].Samples[0].Gear == frames[2].Samples[0].Gear {
		t.Fatal("a changed loadout must decode to a new Gear")
	}
}

func TestGearWritesOnlyChangedFields(t *testing.T) {
	blob := encodeGearRounds(t)
	round1, _ := blob.Descriptor.Offset(1)
	pos := int(round1.ByteOffset)
	// Frame 1 slot 0: first sample of the round writes every field.
	pos += positionsFrameHead + positionsSampleSize
	if blob.Data[pos] != gearAll {
		t.Fatalf("first sample gear bits = %06b, want all", blob.Data[pos])
	}
	var gear gearEncoder
	head, err := appendFrame(nil, gearRounds()[0].Frames[0], blob.Descriptor.Origin, blob.Descriptor.Quantum, &gear)
	if err != nil {
		t.Fatal(err)
	}
	second, err := appendFrame(nil, gearRounds()[0].Frames[1], blob.Descriptor.Origin, blob.Descriptor.Quantum, &gear)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(blob.Data[round1.ByteOffset:round1.ByteOffset+int64(len(head)+len(second))], append(head, second...)) {
		t.Fatal("blob frames differ from frames appended in sequence")
	}
	// Slot 0 kept its pitch and gear: one byte. Slot 4 changed pitch only.
	slot0 := second[positionsFrameHead+positionsSampleSize:]
	if slot0[0] != 0 {
		t.Fatalf("unchanged sample gear bits = %06b, want none", slot0[0])
	}
	slot4 := slot0[1+positionsSampleSize:]
	if slot4[0] != gearPitch || len(slot4) != 2 {
		t.Fatalf("pitch-only sample = %v, want bits %06b and one pitch byte", slot4, gearPitch)
	}
}

func TestGearRoundsDecodeOnTheirOwn(t *testing.T) {
	blob := encodeGearRounds(t)
	round2, _ := blob.Descriptor.Offset(2)
	frames, err := DecodeFrames(blob.Data, round2.ByteOffset, round2.FrameCount, blob.Descriptor)
	if err != nil {
		t.Fatalf("DecodeFrames: %v", err)
	}
	if got := frames[0].Samples[0].Gear; got == nil || !reflect.DeepEqual(*got, *rifleGear()) {
		t.Fatalf("round 2 seeked alone lost its gear: %+v", got)
	}
}

func TestVersionOneBlobsDecodeWithoutGear(t *testing.T) {
	rounds := sampleRounds()
	data := make([]byte, positionsHeaderSize)
	copy(data, positionsMagic)
	binary.LittleEndian.PutUint16(data[6:], positionsVersionV1)
	binary.LittleEndian.PutUint16(data[8:], 4)
	binary.LittleEndian.PutUint32(data[12:], math.Float32bits(0.25))
	binary.LittleEndian.PutUint32(data[28:], 3)
	for _, r := range rounds {
		for _, f := range r.Frames {
			data = append(data, legacyFrameBytesV1(f, [3]float64{}, 0.25)...)
		}
	}
	_, frames, err := DecodePositions(data)
	if err != nil {
		t.Fatalf("a version 1 blob must still decode: %v", err)
	}
	if len(frames) != 3 || frames[0].Samples[1].Health != 56 {
		t.Fatalf("version 1 frames = %+v", frames)
	}
	for _, f := range frames {
		for _, s := range f.Samples {
			if s.Gear != nil || s.Pitch != 0 {
				t.Fatalf("version 1 sample carries gear: %+v", s)
			}
		}
	}
}

func legacyFrameBytesV1(f Frame, origin [3]float64, quantum float64) []byte {
	full := legacyFrameBytes(f, origin, quantum)
	// Drop each sample's gear block: the version 1 record ends at the flags.
	out := append([]byte(nil), full[:positionsFrameHead]...)
	for i := range f.Samples {
		start := positionsFrameHead + i*(positionsSampleSize+len(fullZeroGear))
		out = append(out, full[start:start+positionsSampleSize]...)
	}
	return out
}

func TestGearQuantisation(t *testing.T) {
	for _, tc := range []struct{ in, want float64 }{
		{0, 0}, {-90, -90}, {90, 90}, {270, -90}, {359, -1}, {120, 90}, {math.NaN(), 0},
	} {
		if got := decodePitch(encodePitch(tc.in)); math.Abs(got-tc.want) > 0.36 {
			t.Errorf("pitch %v decodes as %v, want %v", tc.in, got, tc.want)
		}
	}
	w := quantizeGear(&Sample{Gear: &Gear{
		Inventory:    []InventoryItem{item("smokegrenade", 1), item("flashbang", 1), item("flashbang", 1), {Item: 200, Count: 1}, item("knife", 0)},
		Armor:        300,
		Money:        -5,
		FlashSeconds: 99,
	}})
	want := []byte{byte(EquipmentByName("flashbang")) | 1<<6, byte(EquipmentByName("smokegrenade"))}
	if !bytes.Equal(w.inventory[:w.inventoryLen], want) || w.armor != 255 || w.money != 0 || w.flash != 255 {
		t.Fatalf("quantised gear = %+v", w)
	}
}

func TestEquipmentNamesAreStable(t *testing.T) {
	for name, id := range map[string]Equipment{"glock": 1, "ak47": 26, "m4a1_silencer": 28, "awp": 32, "flashbang": 35, "c4": 41, "knife": 42, "healthshot": 44} {
		if got := EquipmentByName(name); got != id {
			t.Errorf("%s = %d, want %d: ids are wire format", name, got, id)
		}
	}
	raw, err := json.Marshal(Gear{ActiveWeapon: EquipmentByName("awp"), Inventory: []InventoryItem{item("decoy", 1)}})
	if err != nil {
		t.Fatal(err)
	}
	var back Gear
	if err := json.Unmarshal(raw, &back); err != nil || back.ActiveWeapon != EquipmentByName("awp") || back.Inventory[0].Item != EquipmentByName("decoy") {
		t.Fatalf("gear JSON %s read back as %+v (%v)", raw, back, err)
	}
}

func TestDecodeRejectsTruncatedGear(t *testing.T) {
	blob := encodeGearRounds(t)
	// Round 2 holds one sample; cut inside its inventory list.
	round2, _ := blob.Descriptor.Offset(2)
	cut := round2.ByteOffset + positionsFrameHead + positionsSampleSize + 4
	if _, err := DecodeFrames(blob.Data[:cut], round2.ByteOffset, 1, blob.Descriptor); err == nil || !strings.Contains(err.Error(), "truncated gear") {
		t.Fatalf("truncated gear decoded with error %v", err)
	}
}

// gearFixture is the web decoder's contract: Go encodes, the node test decodes
// the same bytes and must reach the frames Go decoded.
type gearFixture struct {
	Blob         string        `json:"blob_base64"`
	RoundOffsets []RoundOffset `json:"round_offsets"`
	Frames       []Frame       `json:"frames"`
}

func TestWebGearFixtureMatchesEncoder(t *testing.T) {
	blob := encodeGearRounds(t)
	_, frames, err := DecodePositions(blob.Data)
	if err != nil {
		t.Fatal(err)
	}
	want, err := json.MarshalIndent(gearFixture{
		Blob: base64.StdEncoding.EncodeToString(blob.Data), RoundOffsets: blob.Descriptor.RoundOffsets, Frames: frames,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, '\n')
	if *updateGearFixture {
		if err := os.WriteFile(gearFixturePath, want, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(gearFixturePath)
	if err != nil {
		t.Fatalf("read %s (regenerate with -update-gear-fixture): %v", gearFixturePath, err)
	}
	if !bytes.Equal(bytes.ReplaceAll(got, []byte("\r\n"), []byte("\n")), want) {
		t.Fatalf("%s is stale; run go test ./internal/tacticalplan -run TestWebGearFixture -update-gear-fixture", gearFixturePath)
	}
}
