package store

// syncTable names one table the sync log tracks. KeyColumns is the table's
// primary key (or unique key) in schema order; localColumns never leave the device.
type syncTable struct {
	name         string
	keyColumns   []string
	localColumns []string
}

// syncTables lists every syncable table parents first, so a batch that holds a
// parent and its child applies the parent before the child's foreign key check.
// Migration 26 builds triggers from this list: a table added later needs a new step that creates its triggers.
var syncTables = []syncTable{
	{name: "library_demos", keyColumns: []string{"id"}, localColumns: []string{"path"}},
	{name: "library_teams", keyColumns: []string{"id"}},
	{name: "library_players", keyColumns: []string{"steam_id"}},
	{name: "library_series", keyColumns: []string{"id"}},
	{name: "library_matches", keyColumns: []string{"id"}},
	{name: "library_match_teams", keyColumns: []string{"match_id", "team_key"}},
	{name: "library_match_players", keyColumns: []string{"match_id", "steam_id"}},
	{name: "library_demo_tags", keyColumns: []string{"demo_id", "tag_key"}},
	{name: "library_team_aliases", keyColumns: []string{"alias_key"}},
	{name: "library_team_profiles", keyColumns: []string{"team_id"}},
	{name: "library_team_roles", keyColumns: []string{"team_id", "map"}},
	{name: "library_annotations", keyColumns: []string{"id"}},
	{name: "library_callout_names", keyColumns: []string{"map", "token"}},
	{name: "library_calls", keyColumns: []string{"id"}},
	{name: "library_lineups", keyColumns: []string{"id"}},
	{name: "library_patterns", keyColumns: []string{"id"}},
	{name: "library_pattern_matches", keyColumns: []string{"pattern_id", "match_id", "team_key"}},
	{name: "library_plays", keyColumns: []string{"id"}},
	{name: "library_playlists", keyColumns: []string{"id"}},
	{name: "library_playlist_rounds", keyColumns: []string{"playlist_id", "position"}},
	{name: "library_reports", keyColumns: []string{"id"}},
	{name: "library_pro_follows", keyColumns: []string{"team_id"}},
	{name: "library_match_voice", keyColumns: []string{"match_id"}},
	{name: "library_voice_speakers", keyColumns: []string{"team_id", "speaker_key"}},
	{name: "library_voice_tracks", keyColumns: []string{"match_id", "track"}},
	{name: "library_voice_lines", keyColumns: []string{"match_id", "track", "start_ms"}},
	{name: "library_voice_recordings", keyColumns: []string{"id"}},
	{name: "library_pinned_players", keyColumns: []string{"steamid64"}},
}

var (
	syncTableByName = func() map[string]syncTable {
		m := make(map[string]syncTable, len(syncTables))
		for _, t := range syncTables {
			m[t.name] = t
		}
		return m
	}()
	syncTableIndex = func() map[string]int {
		m := make(map[string]int, len(syncTables))
		for i, t := range syncTables {
			m[t.name] = i
		}
		return m
	}()
)
