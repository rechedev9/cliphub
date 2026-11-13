package clipmind

import (
	"net/http"
	"strings"

	"github.com/rechedev9/cliphub/internal/httpapi"
)

// permission names what a route needs. The server exposes only the analysis
// routes of a workspace's orchestrator handlers; any other route answers 404.
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

// workspaceRoutes is the analysis surface Studio and the web share
// (httpapi.AnalysisRoutes), each with the permission it needs.
func workspaceRoutes() []routeRule {
	routes := httpapi.AnalysisRoutes()
	rules := make([]routeRule, 0, len(routes))
	for _, r := range routes {
		rules = append(rules, routeRule{r.Method, r.Pattern, routePermission(r)})
	}
	return rules
}

func routePermission(r httpapi.Route) permission {
	switch {
	case strings.HasPrefix(r.Pattern, "/api/sync/"):
		return permSync
	case r.Method == http.MethodPost && (r.Pattern == "/api/library/uploads" || r.Pattern == "/api/library/import-package"):
		return permUpload
	case r.Method == http.MethodGet:
		return permRead
	default:
		return permWrite
	}
}
