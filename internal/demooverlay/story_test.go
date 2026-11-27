package demooverlay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/htmlstill"
	"github.com/rechedev9/cliphub/internal/killplan"
)

func halftimeRounds(firstSide string, firstHalfWinners string, secondHalf int) []killplan.RoundResult {
	sides := map[byte]string{'C': "CT", 'T': "T"}
	var rounds []killplan.RoundResult
	for i := range len(firstHalfWinners) {
		rounds = append(rounds, killplan.RoundResult{Number: i + 1, Winner: sides[firstHalfWinners[i]], TargetSide: firstSide})
	}
	for range secondHalf {
		rounds = append(rounds, killplan.RoundResult{Number: len(rounds) + 1, Winner: "CT", TargetSide: oppositeSide(firstSide)})
	}
	return rounds
}

func halftimeDocument() Document {
	return Document{TargetSteamID64: "pov", Outro: Scoreboard{Teams: []TeamBoard{
		{Name: "Vitality", Side: "T", Players: []PlayerCard{{SteamID64: "pov"}}},
		{Name: "Spirit", Side: "CT", Players: []PlayerCard{{SteamID64: "rival"}}},
	}}}
}

func TestHalftimeScoreComesFromTheRoundsBeforeTheSideSwitch(t *testing.T) {
	rounds := halftimeRounds("CT", "CCCCCCCTTTTT", 6)
	got := BuildHalftime(halftimeDocument(), rounds, customhud.Teams{})
	if got == nil || got.Round != 13 {
		t.Fatalf("an MR12 match switches sides at round 13: %+v", got)
	}
	if got.Teams[0] != (HalftimeTeam{Name: "Vitality", Side: "T", Score: 7}) || got.Teams[1] != (HalftimeTeam{Name: "Spirit", Side: "CT", Score: 5}) {
		t.Fatalf("the POV team goes first with its first half score: %+v", got.Teams)
	}
	short := BuildHalftime(halftimeDocument(), halftimeRounds("T", "TTCTCTCT", 3), customhud.Teams{})
	if short == nil || short.Round != 9 || short.Teams[0].Score != 5 || short.Teams[1].Score != 3 {
		t.Fatalf("the switch round comes from the data, not from MR12: %+v", short)
	}
	overtime := halftimeRounds("CT", "CCCCCCTTTTTT", 12)
	for i := range 3 {
		overtime = append(overtime, killplan.RoundResult{Number: 25 + i, Winner: "T", TargetSide: "CT"})
	}
	if got := BuildHalftime(halftimeDocument(), overtime, customhud.Teams{}); got == nil || got.Round != 13 {
		t.Fatalf("an overtime switch must not move the halftime: %+v", got)
	}
}

func TestHalftimeTeamNamesFollowThePlanThenTheDemo(t *testing.T) {
	rounds := halftimeRounds("CT", "CCT", 2)
	named := BuildHalftime(halftimeDocument(), rounds, customhud.Teams{CTStart: " Mi equipo ", TStart: "Rival"})
	if named.Teams[0].Name != "Mi equipo" || named.Teams[1].Name != "Rival" {
		t.Fatalf("the plan names go by the side each team started on: %+v", named.Teams)
	}
	doc := halftimeDocument()
	doc.Outro.Teams[0].Name, doc.Outro.Teams[1].Name = "Terrorists", "Counter-Terrorists"
	sides := BuildHalftime(doc, rounds, customhud.Teams{})
	if sides.Teams[0].Name != "T" || sides.Teams[1].Name != "CT" {
		t.Fatalf("a demo without team names shows the sides of the second half: %+v", sides.Teams)
	}
}

func TestHalftimeIsUnknownWithoutCompleteRoundResults(t *testing.T) {
	full := halftimeRounds("CT", "CCCCCCCTTTTT", 4)
	cases := map[string][]killplan.RoundResult{
		"no rounds":      nil,
		"no side switch": halftimeRounds("CT", "CCCCCCCTTTTT", 0),
	}
	gap := append([]killplan.RoundResult{}, full[:5]...)
	cases["a round without a winner"] = append(gap, full[6:]...)
	unknown := append([]killplan.RoundResult{}, full...)
	unknown[11].TargetSide = ""
	cases["an unknown side before the switch"] = unknown
	draw := append([]killplan.RoundResult{}, full...)
	draw[3].Winner = ""
	cases["a draw"] = draw
	for name, rounds := range cases {
		if got := BuildHalftime(halftimeDocument(), rounds, customhud.Teams{}); got != nil {
			t.Errorf("%s produced a halftime score: %+v", name, got)
		}
	}
}

func TestHalftimeAndSubscribePagesFollowTheDesign(t *testing.T) {
	halftime := Halftime{Round: 13, Teams: [2]HalftimeTeam{{Name: `<b>Vitality</b>`, Side: "T", Score: 7}, {Name: "Spirit", Side: "CT", Score: 5}}}
	palette := &Palette{CT: "112233", T: "AABBCC", Background: "000000", Text: "FFFFFF", Muted: "999999", Radius: 4}
	for name, design := range map[string]*Palette{"neon": nil, "hud": palette} {
		card, err := HalftimeHTML(halftime, design)
		if err != nil {
			t.Fatalf("%s halftime: %v", name, err)
		}
		page := string(card)
		if !strings.Contains(page, ">DESCANSO<") || !strings.Contains(page, "&lt;b&gt;Vitality&lt;/b&gt;") || strings.Contains(page, "<b>Vitality") {
			t.Fatalf("%s halftime lacks its label or escaped names", name)
		}
		if strings.Index(page, "Vitality") > strings.Index(page, "Spirit") || !strings.Contains(page, `class="lead">7<`) || strings.Contains(page, `class="lead">5<`) {
			t.Fatalf("%s halftime must put the POV team first and mark the leader", name)
		}
		pages, err := SubscribeHTML(design, SubscribeCard{}, htmlstill.Landscape)
		button := bytes.Join(pages, nil)
		if err != nil {
			t.Fatalf("%s subscribe: %v", name, err)
		}
		for _, html := range []string{page, string(button)} {
			if strings.Contains(html, "http://") || strings.Contains(html, "https://") || strings.Contains(html, "<script") || strings.Contains(html, "<img") {
				t.Fatalf("%s page loads something from outside", name)
			}
			if strings.Contains(html, "#112233") != (design != nil) || strings.Contains(html, "#d99aff") != (design == nil) {
				t.Fatalf("%s page does not use its own colours", name)
			}
		}
		if !strings.Contains(string(button), ">SUBSCRIBE<") || !strings.Contains(string(button), "<svg") || !strings.Contains(string(button), `class="pill"`) {
			t.Fatalf("%s subscribe lacks its label or bell", name)
		}
	}
}

func TestSubscribeCardShowsTheChannelLogoAndName(t *testing.T) {
	logo := filepath.Join(t.TempDir(), "logo.png")
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logo, encoded.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	for size, layout := range map[htmlstill.Size]string{htmlstill.Landscape: "landscape", htmlstill.Portrait: "portrait"} {
		pages, err := SubscribeHTML(nil, SubscribeCard{LogoPath: logo, ChannelName: "<i>ClipHub</i>"}, size)
		if err != nil {
			t.Fatal(err)
		}
		html := string(bytes.Join(pages, nil))
		if !strings.Contains(html, `class="`+layout+`"`) || !strings.Contains(html, fmt.Sprintf("width:%dpx;height:%dpx", size.Width, size.Height)) {
			t.Fatalf("%s card is not laid out for its frame", layout)
		}
		if !strings.Contains(html, `src="data:image/png;base64,`) || !strings.Contains(html, "&lt;i&gt;ClipHub&lt;/i&gt;") || !strings.Contains(html, ">SUBSCRIBE<") || strings.Contains(html, `class="pill"`) {
			t.Fatalf("%s card lacks the logo, the escaped name or the button", layout)
		}
	}
	nameOnly, err := SubscribeHTML(nil, SubscribeCard{ChannelName: "ClipHub"}, htmlstill.Portrait)
	if err != nil || len(nameOnly) == 0 || strings.Contains(string(nameOnly[0]), "<img") || !strings.Contains(string(nameOnly[0]), ">ClipHub<") {
		t.Fatalf("a card without a logo: %v", err)
	}
	if err := os.WriteFile(logo, []byte("<svg/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := SubscribeHTML(nil, SubscribeCard{LogoPath: logo}, htmlstill.Portrait); err == nil {
		t.Fatal("a logo that is not a PNG or JPG was drawn")
	}
}

func TestOverlayDocumentWithoutHalftimeKeepsItsWireShape(t *testing.T) {
	raw, err := json.Marshal(Document{SchemaVersion: SchemaVersion})
	if err != nil || strings.Contains(string(raw), "halftime") {
		t.Fatalf("halftime reached a document without it: %s %v", raw, err)
	}
	doc := Document{SchemaVersion: SchemaVersion, Halftime: &Halftime{Round: 13, Teams: [2]HalftimeTeam{{Name: "A", Side: "T", Score: 7}, {Name: "B", Side: "CT", Score: 5}}}}
	raw, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var restored Document
	if err := json.Unmarshal(raw, &restored); err != nil || restored.Halftime == nil || *restored.Halftime != *doc.Halftime {
		t.Fatalf("halftime round trip: %+v %v", restored.Halftime, err)
	}
}

func TestHalftimeNeedsAProvenRegulationSwitch(t *testing.T) {
	lateOvertime := halftimeRounds("CT", "CCCCCCTTTTTT", 15)
	for i := range 12 {
		lateOvertime[i].TargetSide = ""
	}
	for i := range 3 {
		lateOvertime = append(lateOvertime, killplan.RoundResult{Number: 28 + i, Winner: "T", TargetSide: "CT"})
	}
	if got := BuildHalftime(halftimeDocument(), lateOvertime, customhud.Teams{}); got != nil {
		t.Fatalf("a POV first seen in the second half must not turn an overtime switch into halftime: %+v", got)
	}
	late := halftimeRounds("CT", "CCCCCCCTTTTT", 6)
	for i := range 4 {
		late[i].TargetSide = ""
	}
	if got := BuildHalftime(halftimeDocument(), late, customhud.Teams{}); got != nil {
		t.Fatalf("a POV who joins late has no proven halftime: %+v", got)
	}
	shortOvertime := halftimeRounds("T", "TTTCCC", 6)
	for i := range 6 {
		shortOvertime[i].TargetSide = ""
	}
	for i := range 6 {
		side := "CT"
		if i >= 3 {
			side = "T"
		}
		shortOvertime = append(shortOvertime, killplan.RoundResult{Number: 13 + i, Winner: "CT", TargetSide: side})
	}
	if got := BuildHalftime(halftimeDocument(), shortOvertime, customhud.Teams{}); got != nil {
		t.Fatalf("an MR6 overtime switch after 15 rounds is not halftime: %+v", got)
	}
	lateShort := halftimeRounds("T", "TTCTCTCT", 3)
	lateShort[0].TargetSide = ""
	if got := BuildHalftime(halftimeDocument(), lateShort, customhud.Teams{}); got != nil {
		t.Fatalf("a non-standard half needs the POV on one side through all of it: %+v", got)
	}
}

func TestSubscribeSpriteSwingsTheBellFromRest(t *testing.T) {
	for _, size := range []htmlstill.Size{htmlstill.Landscape, htmlstill.Portrait} {
		pages, err := SubscribeHTML(nil, SubscribeCard{ChannelName: "ClipHub"}, size)
		if err != nil {
			t.Fatal(err)
		}
		sprite := SubscribeSpriteFor(size)
		rows := 0
		for _, page := range pages {
			count := strings.Count(string(page), `class="row"`)
			if count*sprite.CellHeight > size.Height || !strings.Contains(string(page), fmt.Sprintf("height:%dpx;overflow", size.Height)) {
				t.Fatalf("%v page with %d rows does not fit the renderer's %dx%d viewport", size, count, size.Width, size.Height)
			}
			rows += count
		}
		html := string(bytes.Join(pages, nil))
		if rows != sprite.Frames+1 || strings.Count(html, `class="bell"`) != sprite.Frames+1 {
			t.Fatalf("%v sprite has %d rows; want %d", size, rows, sprite.Frames+1)
		}
		if first := strings.Index(html, "transform:rotate("); first < 0 || !strings.HasPrefix(html[first:], "transform:rotate(0deg)") || !strings.Contains(html, "transform:rotate(-") {
			t.Fatalf("%v bell must start at rest and swing both ways", size)
		}
		if bottom := sprite.CellTop + sprite.CellHeight; sprite.CellTop < 0 || bottom > size.Height {
			t.Fatalf("%v cell %d+%d leaves the frame", size, sprite.CellTop, sprite.CellHeight)
		}
	}
}

func TestSubscribeSpriteStacksThePagesCellByCell(t *testing.T) {
	size := htmlstill.Portrait
	sprite := SubscribeSpriteFor(size)
	rows := sprite.rowsPerPage(size)
	var pages []image.Image
	for p := 0; p*rows <= sprite.Frames; p++ {
		page := image.NewNRGBA(image.Rect(0, 0, size.Width, size.Height))
		for r := 0; r < rows; r++ {
			for y := r * sprite.CellHeight; y < (r+1)*sprite.CellHeight; y++ {
				page.SetNRGBA(0, y, color.NRGBA{R: uint8(p*rows + r), A: 255})
			}
		}
		pages = append(pages, page)
	}
	sheet := sprite.stack(pages, size)
	if sheet.Bounds().Dy() != sprite.Height() {
		t.Fatalf("sheet is %d px tall; want %d", sheet.Bounds().Dy(), sprite.Height())
	}
	for cell := 0; cell <= sprite.Frames; cell++ {
		for _, y := range []int{cell * sprite.CellHeight, (cell+1)*sprite.CellHeight - 1} {
			if got := sheet.NRGBAAt(0, y).R; int(got) != cell {
				t.Fatalf("row %d holds cell %d; want %d", y, got, cell)
			}
		}
	}
}
