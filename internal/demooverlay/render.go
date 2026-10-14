package demooverlay

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/argv"
)

const mutedColor = "0xA1A1AA"

const (
	outroScoreSize    = 34
	outroColLabelSize = 14
	outroNameSize     = 22
	outroTopNameSize  = 24
	outroStatSize     = 21
	outroPOVLabelSize = 10
)

func ffmpegFilterPath(path string) string {
	path = strings.ReplaceAll(path, `\\`, `/`)
	path = strings.ReplaceAll(path, `'`, `\\'`)
	path = strings.ReplaceAll(path, `:`, `\\:`)
	path = strings.ReplaceAll(path, `[`, `\\[`)
	path = strings.ReplaceAll(path, `]`, `\\]`)
	return path
}

func ffmpegDrawtextText(text string) string {
	text = strings.ReplaceAll(text, `\\`, `\\\\`)
	text = strings.ReplaceAll(text, `'`, `\\'`)
	text = strings.ReplaceAll(text, `:`, `\\:`)
	return text
}
