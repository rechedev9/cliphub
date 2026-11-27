package workers

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/rechedev9/cliphub/internal/brandkit"
	"github.com/rechedev9/cliphub/internal/overlayassets"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/storage"
)

func TestSubscribeBannerArgsCarryTheVerifiedLogoAndName(t *testing.T) {
	t.Parallel()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	if err := png.Encode(&body, image.NewNRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	asset, err := overlayassets.Store(context.Background(), store, bytes.NewReader(body.Bytes()), "logo.png")
	if err != nil {
		t.Fatal(err)
	}
	logo := brandkit.ImageRef{ID: asset.ID.String(), SHA256: asset.SHA256}
	w := &RenderWorker{storage: store}
	workDir := t.TempDir()
	args, err := w.brandArgs(context.Background(), renderplan.EditRequest{SubscribeBanner: &brandkit.Subscribe{Logo: &logo, ChannelName: "ClipHub RSN"}}, workDir)
	if err != nil {
		t.Fatal(err)
	}
	logoPath := filepath.Join(workDir, "brand-subscribe-logo.media")
	if want := []string{"--subscribe", "--subscribe-logo", logoPath, "--subscribe-channel", "ClipHub RSN"}; !slices.Equal(args, want) {
		t.Fatalf("args = %q, want %q", args, want)
	}
	if copied, err := os.ReadFile(logoPath); err != nil || !bytes.Equal(copied, body.Bytes()) {
		t.Fatalf("the logo was not copied into the work dir: %v", err)
	}
	if plain, err := w.brandArgs(context.Background(), renderplan.EditRequest{SubscribeBanner: &brandkit.Subscribe{}}, t.TempDir()); err != nil || !slices.Equal(plain, []string{"--subscribe"}) {
		t.Fatalf("plain banner args = %q %v", plain, err)
	}
	forged := brandkit.ImageRef{ID: logo.ID, SHA256: "0000000000000000000000000000000000000000000000000000000000000000"}
	if _, err := w.brandArgs(context.Background(), renderplan.EditRequest{SubscribeBanner: &brandkit.Subscribe{Logo: &forged}}, t.TempDir()); err == nil {
		t.Fatal("a logo whose bytes changed was copied")
	}
}
