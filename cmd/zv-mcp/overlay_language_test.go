package main

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/overlaylang"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestGenerateShortSendsTheOverlayLanguage(t *testing.T) {
	edit, err := resolveEdit(shortChoices{OverlayLanguage: "EN"})
	if err != nil {
		t.Fatal(err)
	}
	if req := decodeEdit(t, edit); req.OverlayLanguage != overlaylang.English {
		t.Fatalf("overlay_language en sent %q", req.OverlayLanguage)
	}
	if summary := describeShortRequest(tuiclient.GenerateRequest{Edit: edit}); !strings.Contains(summary, "inglés") {
		t.Errorf("summary does not say the texts are English: %s", summary)
	}
	for _, code := range []string{"", "es"} {
		edit, err := resolveEdit(shortChoices{OverlayLanguage: code})
		if err != nil || edit.OverlayLanguage != overlaylang.Spanish {
			t.Errorf("overlay_language %q = %q, %v; want Spanish", code, edit.OverlayLanguage, err)
		}
		if summary := describeShortRequest(tuiclient.GenerateRequest{Edit: edit}); strings.Contains(summary, "inglés") {
			t.Errorf("a Spanish Short is summarised as English: %s", summary)
		}
	}
	if _, err := resolveEdit(shortChoices{OverlayLanguage: "de"}); err == nil || !strings.Contains(err.Error(), "es, en") {
		t.Errorf("an unknown language was accepted or the error hides the valid ids: %v", err)
	}
}

func TestShortOptionsPickerSendsTheOverlayLanguage(t *testing.T) {
	cs := connectApps(t, closedStudio(t))
	read, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: shortOptionsAppURI})
	if err != nil {
		t.Fatal(err)
	}
	if html := read.Contents[0].Text; !strings.Contains(html, `["overlay_language", `) {
		t.Error("the Short picker does not offer overlay_language among the choices it sends")
	}
}
