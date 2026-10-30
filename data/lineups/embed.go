// Package lineupdata embeds the smoke lineup catalog in data/lineups, so the
// orchestrator can compare a round's grenades with it without a catalog path.
package lineupdata

import "embed"

// Files holds the catalog JSON files at the root of the FS.
//
//go:embed *.json
var Files embed.FS
