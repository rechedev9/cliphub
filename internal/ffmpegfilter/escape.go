// Package ffmpegfilter escapes values embedded in FFmpeg filtergraph strings.
package ffmpegfilter

import "strings"

var graphEscaper = strings.NewReplacer(`\`, `\\`, `'`, `\'`, `[`, `\[`, `]`, `\]`, `,`, `\,`, `;`, `\;`)

// Value escapes s as one unquoted filter option value at both filtergraph escaping levels
// (option, then graph). It leaves % alone, so a drawtext text= value needs expansion=none.
func Value(s string) string {
	return graphEscaper.Replace("'" + strings.ReplaceAll(s, "'", `'\''`) + "'")
}

// Path is Value for a file path, with forward slashes, which FFmpeg accepts on Windows.
func Path(p string) string {
	return Value(strings.ReplaceAll(p, `\`, "/"))
}
