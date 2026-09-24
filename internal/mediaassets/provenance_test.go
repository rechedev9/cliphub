package mediaassets

import (
	"strings"
	"testing"
)

// The web declares a file-only upload with these exact values
// (localFileProvenance in web/lib/full-demo-plan.ts); the server must keep
// accepting them while it rejects sources that are neither HTTP(S) nor local.
func TestProvenanceValidateLocalFileDeclaration(t *testing.T) {
	t.Parallel()
	declaration := func(title, source string) Provenance {
		return Provenance{
			SchemaVersion: "1.0", AssetSHA256: strings.Repeat("0", 64),
			Title: title, Creator: "No declarado", SourceURL: source,
			Permission: "Archivo local aportado para esta edición; licencia no declarada.",
		}
	}
	for _, tc := range []struct{ title, source string }{
		{"ZACK KEYDROP PREROLL.mp4", "local:ZACK%20KEYDROP%20PREROLL.mp4"},
		{"Intro #1.MP4", "local:Intro%20%231.MP4"},
		{"Narración ñ.wav", "local:Narraci%C3%B3n%20%C3%B1.wav"},
	} {
		if err := declaration(tc.title, tc.source).Validate(); err != nil {
			t.Errorf("Validate(%q) = %v", tc.source, err)
		}
	}
	for _, source := range []string{"ZACK KEYDROP", "www.zack.gg/preroll", "https:zack.gg", "https://user@zack.gg", "ftp://zack.gg"} {
		if err := declaration("preroll.mp4", source).Validate(); err == nil || !strings.Contains(err.Error(), "asset source") {
			t.Errorf("Validate(%q) = %v, want the source error", source, err)
		}
	}
}
