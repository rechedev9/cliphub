package demooverlay

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/htmlstill"
)

func shortSubscribeRows(t *testing.T, card SubscribeCard, size htmlstill.Size) []string {
	t.Helper()
	pages, err := ShortSubscribeHTML(card, size)
	if err != nil {
		t.Fatal(err)
	}
	sprite := ShortSubscribeSpriteFor(size)
	var rows []string
	for _, page := range pages {
		html := string(page)
		if strings.Count(html, `<div class="row">`)*sprite.CellHeight > size.Height || !strings.Contains(html, fmt.Sprintf("height:%dpx;overflow", size.Height)) {
			t.Fatalf("%v page does not fit the renderer's viewport", size)
		}
		if strings.Contains(html, "http://") || strings.Contains(html, "https://") || strings.Contains(html, "<script") {
			t.Fatalf("%v page loads something from outside", size)
		}
		rows = append(rows, strings.Split(html, `<div class="row">`)[1:]...)
	}
	return rows
}

func TestShortSubscribeSpriteLikesThenSubscribesThenRingsTheBell(t *testing.T) {
	for _, size := range []htmlstill.Size{htmlstill.Portrait, htmlstill.Landscape} {
		sprite := ShortSubscribeSpriteFor(size)
		rows := shortSubscribeRows(t, SubscribeCard{ChannelName: "ClipHub"}, size)
		if len(rows) != sprite.Frames+1 || sprite.Frames != ShortSubscribeFollowLast {
			t.Fatalf("%v sprite has %d rows; want %d", size, len(rows), sprite.Frames+1)
		}
		for i, row := range rows {
			liked := strings.Contains(row, `class="like liked"`)
			subscribed := strings.Contains(row, `class="sub subscribed"`)
			swinging := !strings.Contains(row, "transform:rotate(0deg)")
			if liked != (i > ShortSubscribeLikeFirst) {
				t.Fatalf("%v row %d liked = %v; the thumb fills on the like's second frame and stays filled", size, i, liked)
			}
			if subscribed != (i >= ShortSubscribeFollowFirst+2) || strings.Contains(row, ">SUBSCRIBED<") != subscribed || strings.Contains(row, ">SUBSCRIBE<") == subscribed {
				t.Fatalf("%v row %d subscribed = %v; SUBSCRIBE turns into SUBSCRIBED after the press and stays", size, i, subscribed)
			}
			if swinging != (i >= ShortSubscribeBellFirst && i < ShortSubscribeFollowLast) {
				t.Fatalf("%v row %d bell swinging = %v; only the follow clip's bell frames swing", size, i, swinging)
			}
		}
		if last := rows[len(rows)-1]; !strings.Contains(last, `class="like liked"`) || !strings.Contains(last, ">SUBSCRIBED<") || !strings.Contains(last, "transform:rotate(0deg)") {
			t.Fatalf("%v last row must hold liked, subscribed and the bell at rest", size)
		}
		if bottom := sprite.CellTop + sprite.CellHeight; sprite.CellTop < 0 || bottom > size.Height {
			t.Fatalf("%v cell %d+%d leaves the frame", size, sprite.CellTop, sprite.CellHeight)
		}
	}
}

func TestShortSubscribeCardIsSmallerThanTheOldBannerAndCarriesTheKit(t *testing.T) {
	portrait := ShortSubscribeSpriteFor(htmlstill.Portrait)
	if portrait.CellHeight > 150 || portrait.CellTop+portrait.CellHeight > 1500 {
		t.Fatalf("portrait cell %+v; want at most 150 px tall and clear of the Shorts title area", portrait)
	}
	logo := filepath.Join(t.TempDir(), "logo.png")
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logo, encoded.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	rows := shortSubscribeRows(t, SubscribeCard{LogoPath: logo, ChannelName: "<i>ClipHub</i>"}, htmlstill.Portrait)
	if !strings.Contains(rows[0], `src="data:image/png;base64,`) || !strings.Contains(rows[0], "&lt;i&gt;ClipHub&lt;/i&gt;") || strings.Contains(rows[0], "<i>ClipHub") {
		t.Fatal("card lacks the logo or the escaped channel name")
	}
	plain := shortSubscribeRows(t, SubscribeCard{}, htmlstill.Portrait)
	if strings.Contains(plain[0], "<img") || strings.Contains(plain[0], `class="name"`) || !strings.Contains(plain[0], `class="like"`) || !strings.Contains(plain[0], ">SUBSCRIBE<") {
		t.Fatal("a card without a kit must still show the like and subscribe buttons")
	}
}
