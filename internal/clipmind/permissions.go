package clipmind

import "net/http"

// permission names what a route needs. Every route the server exposes from a
// workspace's orchestrator handlers is listed below with one; a route that is
// not listed answers 404, so a new orchestrator route is closed until someone
// opens it here on purpose.
type permission string

const (
	permRead   permission = "library:read"
	permWrite  permission = "library:write"
	permUpload permission = "library:upload"
	permSync   permission = "sync"
)

// rolePermissions: every member has the same rights in a workspace. Owner-only
// actions (expel, transfer, delete) are account routes, not library routes.
var rolePermissions = map[string][]permission{
	"owner":  {permRead, permWrite, permUpload, permSync},
	"member": {permRead, permWrite, permUpload, permSync},
}

func roleAllows(role string, need permission) bool {
	for _, p := range rolePermissions[role] {
		if p == need {
			return true
		}
	}
	return false
}

type routeRule struct {
	method  string
	pattern string
	perm    permission
}

func get(pattern string) routeRule     { return routeRule{http.MethodGet, pattern, permRead} }
func post(pattern string) routeRule    { return routeRule{http.MethodPost, pattern, permWrite} }
func put(pattern string) routeRule     { return routeRule{http.MethodPut, pattern, permWrite} }
func patch(pattern string) routeRule   { return routeRule{http.MethodPatch, pattern, permWrite} }
func del(pattern string) routeRule     { return routeRule{http.MethodDelete, pattern, permWrite} }
func syncGet(pattern string) routeRule { return routeRule{http.MethodGet, pattern, permSync} }

// workspaceRoutes is the analysis surface of the web edition: the library,
// the 2D replay data and the map assets. What it leaves out needs the demo
// file, the machine or another service: importing by path, re-analysing,
// capture, renders, voice audio, FACEIT and the voice plugin.
var workspaceRoutes = []routeRule{
	{http.MethodPost, "/api/library/uploads", permUpload},
	syncGet("/api/sync/changes"),
	{http.MethodPost, "/api/sync/changes", permSync},

	get("/api/maps/{map}/calibration"),
	get("/api/maps/{map}/radar"),
	get("/api/maps/{map}/shot"),
	get("/api/maps/{map}/geometry"),
	get("/api/maps/{map}/scene"),
	get("/api/maps/{map}/scene/summary"),
	get("/api/equipment/{name}/icon"),
	get("/api/teams/logo"),

	get("/api/library/annotations"),
	post("/api/library/annotations"),
	patch("/api/library/annotations/{id}"),
	del("/api/library/annotations/{id}"),
	get("/api/library/baseline"),
	get("/api/library/calls"),
	post("/api/library/calls"),
	get("/api/library/calls/{id}"),
	patch("/api/library/calls/{id}"),
	del("/api/library/calls/{id}"),
	get("/api/library/demos"),
	get("/api/library/demos/{id}"),
	patch("/api/library/demos/{id}"),
	del("/api/library/demos/{id}"),
	get("/api/library/leaderboard"),
	get("/api/library/libreta/export"),
	get("/api/library/lineups"),
	post("/api/library/lineups"),
	get("/api/library/lineups/{id}"),
	patch("/api/library/lineups/{id}"),
	del("/api/library/lineups/{id}"),
	get("/api/library/maps/{map}/callouts"),
	put("/api/library/maps/{map}/callouts/{token}"),
	del("/api/library/maps/{map}/callouts/{token}"),
	get("/api/library/matches"),
	get("/api/library/matches/{id}"),
	patch("/api/library/matches/{id}"),
	get("/api/library/matches/{id}/metrics"),
	get("/api/library/matches/{id}/patterns"),
	get("/api/library/matches/{id}/pro"),
	put("/api/library/matches/{id}/teams/{key}"),
	get("/api/library/matches/{id}/tactical"),
	get("/api/library/matches/{id}/tactical/aggregate"),
	get("/api/library/matches/{id}/tactical/heatmap"),
	get("/api/library/matches/{id}/tactical/overlay"),
	get("/api/library/matches/{id}/tactical/positions"),
	get("/api/library/matches/{id}/tactical/rounds/{round}"),
	get("/api/library/matches/{id}/tactical/rounds/{round}/lineups"),
	get("/api/library/matches/{id}/tactical/rounds/{round}/play"),
	get("/api/library/matches/{id}/voice"),
	patch("/api/library/matches/{id}/voice"),
	del("/api/library/matches/{id}/voice"),
	get("/api/library/matches/{id}/voice/comms"),
	put("/api/library/matches/{id}/voice/speakers"),
	patch("/api/library/matches/{id}/voice/tracks/{track}"),
	get("/api/library/norms"),
	get("/api/library/patterns"),
	post("/api/library/patterns"),
	get("/api/library/patterns/{id}"),
	patch("/api/library/patterns/{id}"),
	del("/api/library/patterns/{id}"),
	get("/api/library/patterns/{id}/rounds"),
	get("/api/library/pinned-players"),
	get("/api/library/players"),
	get("/api/library/players/{steamid}"),
	get("/api/library/players/{steamid}/metrics"),
	get("/api/library/players/{steamid}/positions"),
	get("/api/library/players/{steamid}/role"),
	get("/api/library/playlists"),
	post("/api/library/playlists"),
	get("/api/library/playlists/export"),
	post("/api/library/playlists/import"),
	get("/api/library/playlists/{id}"),
	patch("/api/library/playlists/{id}"),
	del("/api/library/playlists/{id}"),
	post("/api/library/playlists/{id}/rounds"),
	patch("/api/library/playlists/{id}/rounds/{match}/{round}"),
	del("/api/library/playlists/{id}/rounds/{match}/{round}"),
	get("/api/library/plays"),
	post("/api/library/plays"),
	get("/api/library/plays/export"),
	post("/api/library/plays/import"),
	get("/api/library/plays/{id}"),
	patch("/api/library/plays/{id}"),
	del("/api/library/plays/{id}"),
	get("/api/library/pro-follows"),
	post("/api/library/pro-follows"),
	del("/api/library/pro-follows/{team}"),
	get("/api/library/reports"),
	post("/api/library/reports"),
	get("/api/library/reports/{id}"),
	patch("/api/library/reports/{id}"),
	del("/api/library/reports/{id}"),
	get("/api/library/reports/{id}/metrics"),
	get("/api/library/reports/{id}/scout"),
	get("/api/library/search"),
	get("/api/library/series"),
	post("/api/library/series"),
	get("/api/library/series/{id}"),
	patch("/api/library/series/{id}"),
	del("/api/library/series/{id}"),
	get("/api/library/teams"),
	post("/api/library/teams"),
	get("/api/library/teams/{id}"),
	patch("/api/library/teams/{id}"),
	post("/api/library/teams/{id}/merge"),
	get("/api/library/teams/{id}/attack"),
	get("/api/library/teams/{id}/bombsites"),
	get("/api/library/teams/{id}/first-death"),
	get("/api/library/teams/{id}/grenades"),
	get("/api/library/teams/{id}/heatmap"),
	get("/api/library/teams/{id}/lineups"),
	get("/api/library/teams/{id}/map-counts"),
	get("/api/library/teams/{id}/maps"),
	get("/api/library/teams/{id}/overlay"),
	get("/api/library/teams/{id}/pattern"),
	get("/api/library/teams/{id}/playbook"),
	get("/api/library/teams/{id}/plays"),
	get("/api/library/teams/{id}/prep"),
	get("/api/library/teams/{id}/profile"),
	get("/api/library/teams/{id}/roles"),
	get("/api/library/teams/{id}/rounds"),
	get("/api/library/teams/{id}/scout"),
	get("/api/library/teams/{id}/side-stats"),
	get("/api/library/teams/{id}/veto"),
	get("/api/library/teams/{id}/warnings"),
	get("/api/library/voice/search"),
	get("/api/library/voice/summary"),
}
