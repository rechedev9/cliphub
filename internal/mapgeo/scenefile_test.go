package mapgeo

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func testScene() *Scene {
	return &Scene{
		Map: "de_test", Build: "1",
		Textures: []SceneTexture{
			{Path: "a", Tex: Texture{Format: TexDXT1, Width: 4, Height: 4, Faces: 1, Data: make([]byte, 8), Levels: []int{8}}},
			{Path: "sky", Tex: Texture{Format: TexBC6H, Width: 4, Height: 4, Faces: 6, Data: make([]byte, 96), Levels: []int{96}}},
		},
		Materials: []SceneMaterial{{MaterialInfo: MaterialInfo{Name: "m", Tint: [3]float32{1, 1, 1}}, ColorTex: 0, Color2Tex: -1, BlendTex: -1}},
		Chunks: []SceneChunk{{
			Positions: []float32{0, 0, 0, 10, 0, 0, 0, 10, 5},
			Normals:   []float32{0, 0, 1, 0, 0, 1, 0, 0, 1},
			UVs:       []float32{0, 0, 1, 0, 0, 1},
			Indices:   []uint16{0, 1, 2},
			Min:       [3]float32{0, 0, 0}, Max: [3]float32{10, 10, 5},
		}},
		Sun: &Sun{ShadowChannel: -1},
		Sky: &Sky{Texture: 1, Brightness: 1, Tint: [3]float64{1, 1, 1}},
	}
}

func TestWriteSceneRoundTripsTheSummary(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteScene(&buf, testScene()); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	headerLength := int(binary.LittleEndian.Uint32(data[8:]))
	if blob := (12 + headerLength + 15) / 16 * 16; blob%16 != 0 || blob > len(data) {
		t.Fatalf("binary section starts at %d of %d", blob, len(data))
	}
	got, err := ReadSceneSummary(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	want := SceneSummary{Map: "de_test", Build: "1", Triangles: 1, Chunks: 1, Materials: 1, Textures: 2, Sun: true, Sky: true}
	if got != want {
		t.Fatalf("summary = %+v, want %+v", got, want)
	}
}

func TestReadSceneSummaryRefusesOtherFiles(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteScene(&buf, testScene()); err != nil {
		t.Fatal(err)
	}
	old := bytes.Clone(buf.Bytes())
	binary.LittleEndian.PutUint32(old[4:], SceneFileVersion-1)
	for name, data := range map[string][]byte{"magic": []byte("RIFF00000000"), "version": old, "short": []byte("CHMS")} {
		if _, err := ReadSceneSummary(bytes.NewReader(data)); err == nil {
			t.Errorf("%s: read a summary", name)
		}
	}
}

func TestCubeLevelsHoldSixFaces(t *testing.T) {
	h := vtexHeader{width: 8, height: 8, format: TexBC6H, faces: 6}
	if got := h.levelBytes(0); got != 2*2*16*6 {
		t.Fatalf("level 0 = %d bytes", got)
	}
	if got := h.levelBytes(3); got != 16*6 {
		t.Fatalf("level 3 = %d bytes", got)
	}
}
