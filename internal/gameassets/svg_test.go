package gameassets

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"
)

const hostileSVG = `<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE svg [<!ENTITY x SYSTEM "file:///c:/windows/win.ini">]>
<!-- Generator: Adobe Illustrator -->
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:evil="http://evil.example" viewBox="0 0 10 10" onload="alert(1)" evil:x="1">
<script>alert(2)</script>
<foreignObject><div xmlns="http://www.w3.org/1999/xhtml">hi</div></foreignObject>
<image href="https://evil.example/a.png"/>
<style>@import url(https://evil.example/a.css);</style>
<style>.a{fill:url(#grad)}</style>
<defs><linearGradient id="grad"><stop offset="0" stop-color="#fff"/></linearGradient></defs>
<g onclick="steal()" style="fill:url( 'https://evil.example/x' )">
<path d="M0,0
	L10,10" fill="#FFFFFF" fill-rule="evenodd"/>
<use xlink:href="#grad"/>
<use href="https://evil.example/sprite.svg#x"/>
<a href="javascript:alert(3)"><path d="M1,1"/></a>
<animate attributeName="href" to="javascript:alert(4)"/>
</g>
</svg>`

func TestSanitizeSVGStripsActiveContent(t *testing.T) {
	out, err := SanitizeSVG([]byte(hostileSVG))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, banned := range []string{"script", "alert", "onload", "onclick", "foreignObject", "<image", "evil", "@import", "DOCTYPE", "ENTITY", "<?xml", "<!--", "<a", "animate", "javascript"} {
		if strings.Contains(got, banned) {
			t.Errorf("sanitized svg still contains %q:\n%s", banned, got)
		}
	}
	for _, kept := range []string{`viewBox="0 0 10 10"`, `fill="#FFFFFF"`, `xlink:href="#grad"`, `fill:url(#grad)`, "<linearGradient", "M0,0\n\tL10,10"} {
		if !strings.Contains(got, kept) {
			t.Errorf("sanitized svg lost %q:\n%s", kept, got)
		}
	}
	if !strings.HasPrefix(got, "<svg ") || !wellFormed(out) {
		t.Errorf("sanitized svg is not a well-formed svg document:\n%s", got)
	}
}

func wellFormed(doc []byte) bool {
	dec := xml.NewDecoder(bytes.NewReader(doc))
	for {
		_, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return true
		}
		if err != nil {
			return false
		}
	}
}

func TestSanitizeSVGRejectsNonSVG(t *testing.T) {
	for _, src := range []string{
		`<html><body/></html>`,
		`<g><path d="M0,0"/></g>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><path d="M0,0"/>`,
		`<svg/><svg/>`,
		`plain text`,
	} {
		if out, err := SanitizeSVG([]byte(src)); err == nil {
			t.Errorf("SanitizeSVG(%q) = %q, want an error", src, out)
		}
	}
}

func TestExtractSVGFindsTheDocumentInTheDataBlock(t *testing.T) {
	doc := `<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`
	res := buildResource(append([]byte{0x56, 0xbc, 0x4c, 0x6d, 0, 0}, doc+"\x00\x00"...), nil)
	got, err := ExtractSVG(res)
	if err != nil || string(got) != doc {
		t.Fatalf("ExtractSVG = %q, %v; want %q", got, err, doc)
	}
	if _, err := ExtractSVG(buildResource([]byte("no markup"), nil)); err == nil {
		t.Error("ExtractSVG accepted a DATA block without an svg document")
	}
}
