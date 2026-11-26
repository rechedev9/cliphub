package demooverlay

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func decodeCameraPNG(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func alphaAt(img image.Image, x, y int) uint8 {
	return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA).A
}

func grayAt(img image.Image, x, y int) uint8 {
	return color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y
}

func TestSecondCameraMaskRoundsOnlyTheTopCorners(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mask.png")
	if err := RenderSecondCameraMaskPNG(path); err != nil {
		t.Fatal(err)
	}
	img := decodeCameraPNG(t, path)
	if b := img.Bounds(); b.Dx() != SecondCameraWidth || b.Dy() != SecondCameraHeight {
		t.Fatalf("mask is %dx%d", b.Dx(), b.Dy())
	}
	right, bottom := SecondCameraWidth-1, SecondCameraHeight-1
	if grayAt(img, 0, 0) != 0 || grayAt(img, right, 0) != 0 {
		t.Fatal("the top corners must be cut out")
	}
	if grayAt(img, 0, bottom) != 255 || grayAt(img, right, bottom) != 255 || grayAt(img, SecondCameraWidth/2, SecondCameraHeight/2) != 255 {
		t.Fatal("the bottom corners and the middle must stay opaque, the label closes the window")
	}
}

func TestSecondCameraLabelIsABarWithRoundedBottomCorners(t *testing.T) {
	dir := t.TempDir()
	for name, value := range map[string]string{
		"short":    "s1mple",
		"long":     "a name far too long to fit in the label of the second camera window of the full demo video render",
		"cyrillic": "Игрок",
	} {
		path := filepath.Join(dir, name+".png")
		if err := RenderSecondCameraLabelPNG(value, nil, path); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		img := decodeCameraPNG(t, path)
		if b := img.Bounds(); b.Dx() != SecondCameraWidth || b.Dy() != SecondCameraLabelHeight {
			t.Fatalf("%s: label is %dx%d", name, b.Dx(), b.Dy())
		}
		right, bottom := SecondCameraWidth-1, SecondCameraLabelHeight-1
		if alphaAt(img, 0, bottom) != 0 || alphaAt(img, right, bottom) != 0 {
			t.Fatalf("%s: the bottom corners must be transparent", name)
		}
		if alphaAt(img, 0, 0) == 0 || alphaAt(img, right, 0) == 0 {
			t.Fatalf("%s: the top edge must meet the video", name)
		}
		if alphaAt(img, right-2, SecondCameraLabelHeight/2) == 255 {
			t.Fatalf("%s: the name must stay inside the padding", name)
		}
	}
	if fitted := fitCameraLabelForTest(t, "a name far too long to fit in the label of the second camera window of the full demo video render"); []rune(fitted)[len([]rune(fitted))-1] != '…' {
		t.Fatalf("a long name must be ellipsized: %q", fitted)
	}
}

func fitCameraLabelForTest(t *testing.T, value string) string {
	t.Helper()
	face, err := secondCameraFace(value)
	if err != nil {
		t.Fatal(err)
	}
	defer face.Close()
	return fitCameraLabel(value, face, SecondCameraWidth-2*secondCameraLabelPadding)
}
