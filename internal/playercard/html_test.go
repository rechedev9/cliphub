package playercard

import (
	"bytes"
	"image"
	"image/png"
	"regexp"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func fullCard() Card {
	return Card{
		SteamID64: "76561198000000001", Name: "<b>donk</b>", Team: "Spirit", Role: "entry", Map: "de_mirage",
		Result: &Result{Own: 13, Rival: 7}, Rounds: 20, Kills: 24, Deaths: 10,
		Rating: ptr(1.62), ADR: ptr(112.4), KAST: ptr(80.0), HSPct: ptr(54.2), OpeningKills: ptr(6), UtilityDamage: ptr(214),
		FACEIT: &FACEIT{Level: 10, ELO: 3120},
	}
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// visibleText strips the markup so assertions read what the card shows, in order.
func visibleText(page []byte) string {
	body := string(page)
	body = body[strings.Index(body, "<body"):]
	text := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(body, " ")
	return strings.Join(strings.Fields(text), " ")
}

func render(t *testing.T, template string, card Card, img []byte) (string, string) {
	t.Helper()
	page, err := HTML(template, card, img)
	if err != nil {
		t.Fatal(err)
	}
	html := string(page)
	if strings.Contains(html, "<canvas") || strings.Contains(html, "http://") || strings.Contains(html, "https://") {
		t.Fatalf("%s reaches the network or draws on a canvas", template)
	}
	return html, visibleText(page)
}

func TestCartaShowsRatingRoleAndSixStatsOfTheMatch(t *testing.T) {
	html, text := render(t, "carta", fullCard(), pngBytes(t))
	want := "1.62 RATING CLIPHUB ENTRY &lt;b&gt;donk&lt;/b&gt; Spirit NIVEL 10 3120 ELO K/D 2.40 ADR 112.4 KAST 80% HS 54% ENTRADAS 6 UTILIDAD 214"
	if text != want {
		t.Fatalf("carta reads\n%q\nwant\n%q", text, want)
	}
	if !strings.Contains(html, `src="data:image/png;base64,`) {
		t.Fatal("carta lost the player's image")
	}
}

func TestFichaShowsThePhotoNameTeamAndThreeFigures(t *testing.T) {
	_, text := render(t, "ficha", fullCard(), pngBytes(t))
	if want := "&lt;b&gt;donk&lt;/b&gt; Spirit ENTRY NIVEL 10 3120 ELO RATING CLIPHUB 1.62 K/D 2.40 ADR 112.4"; text != want {
		t.Fatalf("ficha reads\n%q\nwant\n%q", text, want)
	}
}

func TestMVPShowsTheResultAndTheBestFigure(t *testing.T) {
	_, text := render(t, "mvp", fullCard(), pngBytes(t))
	// Six entries in 20 rounds is three times the reference rate, the largest lead of the card.
	if want := "MVP VICTORIA 13-7 MIRAGE &lt;b&gt;donk&lt;/b&gt; Spirit NIVEL 10 3120 ELO 6 ENTRADAS"; text != want {
		t.Fatalf("mvp reads\n%q\nwant\n%q", text, want)
	}
}

func TestCardsWithoutFACEITLeaveTheBadgeOut(t *testing.T) {
	card := fullCard()
	card.FACEIT = nil
	for _, id := range []string{"carta", "ficha", "mvp"} {
		if _, text := render(t, id, card, pngBytes(t)); strings.Contains(text, "NIVEL") || strings.Contains(text, "ELO") {
			t.Fatalf("%s shows a FACEIT badge without FACEIT data: %q", id, text)
		}
	}
}

func TestCardsWithoutImageShowTheInitials(t *testing.T) {
	card := fullCard()
	card.Name = "Fallen Angel"
	for _, id := range []string{"carta", "ficha", "mvp"} {
		html, text := render(t, id, card, nil)
		if strings.Contains(html, "<img") || !strings.Contains(text, "FA") {
			t.Fatalf("%s without image should show the initials FA: %q", id, text)
		}
	}
	if _, text := render(t, "carta", card, []byte("not an image")); !strings.Contains(text, "FA") {
		t.Fatal("an unreadable image should fall back to the initials")
	}
}

func TestCardsLeaveOutTheFiguresTheDemoLacks(t *testing.T) {
	card := Card{Name: "solo", Kills: 3, Deaths: 0, Result: &Result{Own: 9, Rival: 13}}
	_, carta := render(t, "carta", card, nil)
	if want := "SO solo K/D 3.00"; carta != want {
		t.Fatalf("carta reads %q, want %q", carta, want)
	}
	_, mvp := render(t, "mvp", card, nil)
	if want := "MVP DERROTA 9-13 SO solo 3.00 K/D"; mvp != want {
		t.Fatalf("mvp reads %q, want %q", mvp, want)
	}
}

func TestUnknownTemplateIsRefused(t *testing.T) {
	if _, err := HTML("fifa", fullCard(), nil); err == nil {
		t.Fatal("rendered a template the catalog does not have")
	}
}
