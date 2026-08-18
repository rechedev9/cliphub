package streamcli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	exitSuccess     = 0
	exitUnexpected  = 1
	exitInvalidArgs = 2
)

func isHelp(value string) bool {
	return value == "-h" || value == "--help" || value == "help"
}

func isSingleHelp(args []string) bool {
	return len(args) == 1 && isHelp(args[0])
}

func parseFormatArgs(args []string) (string, []string, error) {
	format := "text"
	var rest []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--format":
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("missing value for --format")
			}
			i++
			format = args[i]
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
		default:
			rest = append(rest, arg)
		}
	}
	if format != "text" && format != "json" {
		return "", nil, fmt.Errorf("unsupported format %q", format)
	}
	return format, rest, nil
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

const streamUsage = `usage: zv-stream fetch [flags] | zv-stream variants [--format text|json] | zv-stream plan [flags] | zv-stream render [flags]
`

const streamFetchUsage = `usage: zv stream fetch --url <https://...> --out <stream.mp4> [flags]

Downloads an allowlisted Twitch or YouTube clip/VOD to a local MP4. The
destination is left untouched on --dry-run. Existing files are reused.

Flags:
  --max-bytes <n>              download size ceiling (default 8589934592)
  --ytdlp <path>               yt-dlp path; defaults to ZV_YTDLP_PATH or PATH
  --timeout <duration>         download timeout (default 20m)
  --dry-run                    validate the URL and destination without downloading
  --format <text|json>         output format (default text)
`

const streamVariantsUsage = `usage: zv stream variants [--format text|json]
`

const streamPlanUsage = `usage: zv stream plan --input <stream.mp4> --out <edit-plan.json> [flags]

Flags:
  --variant <name>             layout from "zv stream variants"
  --clip-id <id>               initial clip id (default clip-001)
  --clip-start <seconds>       initial clip start (default 0)
  --clip-end <seconds>         initial clip end (default full source duration)
  --title <text>               initial clip title
  --streamer <nick>            optional streamer banner
  --face-crop x,y,w,h          override normalized facecam crop
  --gameplay-crop x,y,w,h      override normalized gameplay crop
  --ffprobe <path>             ffprobe path; defaults to local discovery
  --dry-run                    probe and validate without writing the plan
  --format <text|json>         output format (default text)
`

const streamRenderUsage = `usage: zv stream render --input <stream.mp4> --plan <edit-plan.json> --out <run-dir> [flags]

The plan is the source of truth for ranges, crops, music, effects, and text. A
cover JPG is selected from a stable frame in the first third of the clip. Final
videos, covers, manifest, and gallery are copied to
<out>/shortslistosparasubir.

Flags:
  --title <text>               gallery title
  --ffmpeg <path>              ffmpeg path; defaults to local discovery
  --ffprobe <path>             ffprobe path; defaults to local discovery
  --timeout <duration>         render timeout (default 20m)
  --work-dir <dir>             temporary stage directory
  --music-dir <dir>            optional music catalog directory
  --dry-run                    probe and validate without rendering
  --format <text|json>         output format (default text)
`
