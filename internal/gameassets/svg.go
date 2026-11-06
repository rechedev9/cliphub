package gameassets

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/rechedev9/cliphub/internal/mapgeo"
)

// ExtractSVG returns the SVG source a compiled .vsvg_c keeps in its DATA block
// after a short binary prefix.
func ExtractSVG(resource []byte) ([]byte, error) {
	blocks, err := mapgeo.ResourceBlocks(resource)
	if err != nil {
		return nil, err
	}
	data, ok := blocks["DATA"]
	if !ok {
		return nil, errors.New("vector graphic has no DATA block")
	}
	start := bytes.IndexByte(data, '<')
	if start < 0 || !bytes.Contains(data, []byte("<svg")) {
		return nil, errors.New("vector graphic holds no svg document")
	}
	return bytes.TrimRight(data[start:], "\x00"), nil
}

// svgElements are the drawing elements an icon may keep; anything else
// (script, foreignObject, image, animation, vendor metadata) is dropped with
// its whole subtree.
var svgElements = map[string]bool{
	"svg": true, "g": true, "path": true, "rect": true, "circle": true, "ellipse": true,
	"line": true, "polyline": true, "polygon": true, "defs": true, "use": true, "symbol": true,
	"linearGradient": true, "radialGradient": true, "stop": true, "clipPath": true, "mask": true,
	"title": true, "desc": true, "style": true,
}

// SanitizeSVG rewrites an SVG keeping only drawing elements and attributes
// that cannot run code or load anything: no event handlers, no DOCTYPE or
// processing instructions, and references only to fragments in the document.
func SanitizeSVG(src []byte) ([]byte, error) {
	dec := xml.NewDecoder(bytes.NewReader(src))
	dec.Strict = true
	var out bytes.Buffer
	var open []string // elements written and not yet closed
	skip, sawRoot := 0, false
	for {
		tok, err := dec.RawToken()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse svg: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			root := len(open) == 0 && skip == 0
			if root && (sawRoot || t.Name.Space != "" || t.Name.Local != "svg") {
				return nil, errors.New("svg must have a single svg root element")
			}
			if skip > 0 || t.Name.Space != "" || !svgElements[t.Name.Local] {
				skip++
				continue
			}
			sawRoot = true
			open = append(open, t.Name.Local)
			out.WriteString("<" + t.Name.Local)
			for _, a := range t.Attr {
				if name, ok := safeSVGAttr(a); ok {
					out.WriteString(" " + name + `="` + xmlEscaper.Replace(a.Value) + `"`)
				}
			}
			out.WriteString(">")
		case xml.EndElement:
			if skip > 0 {
				skip--
				continue
			}
			if len(open) == 0 {
				return nil, errors.New("parse svg: unbalanced end element")
			}
			out.WriteString("</" + open[len(open)-1] + ">")
			open = open[:len(open)-1]
		case xml.CharData:
			if skip == 0 && len(open) > 0 && !unsafeCSS(string(t)) {
				out.WriteString(xmlEscaper.Replace(string(t)))
			}
		}
	}
	if !sawRoot || len(open) > 0 {
		return nil, errors.New("svg has no complete root element")
	}
	return out.Bytes(), nil
}

// xmlEscaper escapes text and attribute values but keeps whitespace literal,
// so path data stays readable.
var xmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

// safeSVGAttr returns the name to write for an attribute, or false to drop it.
func safeSVGAttr(a xml.Attr) (string, bool) {
	name := a.Name.Local
	if a.Name.Space != "" {
		name = a.Name.Space + ":" + a.Name.Local
	}
	lower := strings.ToLower(a.Name.Local)
	switch {
	case strings.HasPrefix(lower, "on"):
		return "", false
	case a.Name.Space == "xmlns" || name == "xmlns":
		return name, a.Value == "http://www.w3.org/2000/svg" || a.Value == "http://www.w3.org/1999/xlink"
	case lower == "href":
		return name, strings.HasPrefix(a.Value, "#")
	case a.Name.Space != "" && name != "xml:space":
		return "", false
	}
	return name, !unsafeCSS(a.Value)
}

// unsafeCSS reports a value that could load a resource or run script:
// a url() that is not a local fragment, @import, or a script scheme.
func unsafeCSS(value string) bool {
	v := strings.ToLower(strings.Join(strings.Fields(value), ""))
	if strings.Contains(v, "javascript:") || strings.Contains(v, "@import") || strings.Contains(v, "expression(") {
		return true
	}
	for rest := v; ; {
		i := strings.Index(rest, "url(")
		if i < 0 {
			return false
		}
		rest = strings.TrimLeft(rest[i+4:], `'"`)
		if !strings.HasPrefix(rest, "#") {
			return true
		}
	}
}
