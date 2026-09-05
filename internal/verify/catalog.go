// Package verify is the Windows-first ClipHub control surface behind `zv verify`.
// Doctor inspects a live Studio (ports.json, jobs.db, HLAE, running cs2.exe).
// Linux fail-closes. Capture recertification never Passes on Cloud Linux.
package verify

// SchemaVersion is the machine-readable doctor / feature-map contract.
const SchemaVersion = 3

const (
	FeatureCatalogSource = "internal/verify/catalog.go"
	StudioAppDir         = "cliphub-studio"
	StudioPortsFile      = "ports.json"
	StudioJobsDBRel      = "data/jobs.db"
	CS2ImageName         = "cs2.exe"
	ClosedCaptureGapID   = "hlae_cs2_windows_studio"
	OverlayWalkGapID     = "studio_overlay_walk"
	StudioPortsGapID     = "studio_ports_missing"
	StudioJobsDBGapID    = "studio_jobs_db_missing"
	StudioDownGapID      = "studio_orchestrator_down"
	HLAEMissingGapID     = "hlae_not_detected"
	CS2NotRunningGapID   = "cs2_not_running"
	JobIDRequiredGapID   = "studio_job_id_required"
	ClosedCaptureGap     = "HLAE/CS2 / Windows Studio gap: this host cannot recertify capture or Full Demo 16:9 recap. The verification host of record is King's Windows ClipHub Studio, not the cloud VM. Cloud Linux cannot launch CS2. Hosted CI green is not HLAE/CS2 proof. Do not treat compile, lint, or unit tests as a Pass on Full Demo or live overlay walks."
	OverlayWalkGap       = "This CLI does not screenshot Studio and does not add Playwright to CI. Live overlay percent is inspected via GET /api/jobs/{id}?view=status progress.percent on King's Windows Studio. Do not call Full Demo Pass from Cloud Linux."
	StudioPortsGap       = "ClipHub Studio ports.json is missing under %APPDATA%\\cliphub-studio. Doctor cannot find the live orchestrator. Start Studio on this Windows host."
	StudioJobsDBGap      = "jobs.db is missing under %APPDATA%\\cliphub-studio\\data. Studio userData is not the live install doctor expected."
	StudioDownGap        = "Studio orchestrator is down. ports.json was read but GET /healthz is not {service:cliphub,status:ok} on 127.0.0.1. A Windows host with Studio down still fail-closes."
	HLAEMissingGap       = "HLAE not detected. Expected C:\\HLAE-*\\HLAE.exe (never C:\\HLAE\\HLAE.exe) or packaged %APPDATA%\\cliphub-studio\\tools\\hlae\\*\\HLAE.exe."
	CS2NotRunningGap     = "cs2.exe is not running. Doctor never fakes CS2. An installed cs2.exe path is not enough; start Counter-Strike 2 on this Windows host before capture recertification can Pass."
	JobIDRequiredGap     = "prove needs --job-id to inspect GET /api/jobs/{id}?view=status (status, capture-progress, overlay percent). This is not Full Demo Pass."
)

// Feature is one Studio user-path the lever can name.
type Feature struct {
	ID                    string `json:"id"`
	Title                 string `json:"title"`
	Route                 string `json:"route"`
	NavLabel              string `json:"nav_label,omitempty"`
	RequiresHLAECS2       bool   `json:"requires_hlae_cs2"`
	RequiresWindowsStudio bool   `json:"requires_windows_studio"`
	CheapProof            string `json:"cheap_proof"`
	ProbePath             string `json:"probe_path,omitempty"`
}

// Features is the compiled Studio feature catalog used by the CLI.
// Keep feature IDs stable for CLI callers; routes and rail labels follow
// web/lib/nav.ts, including the destinations of retired Studio doors.
func Features() []Feature {
	return []Feature{
		{
			ID:         "inicio",
			Title:      "Clips y vídeos",
			Route:      "/clips",
			NavLabel:   "Clips y vídeos",
			CheapProof: "route /clips plus hub empty and first-run tests",
			ProbePath:  "/api/demos/jobs",
		},
		{
			ID:         "partidas",
			Title:      "Partidas",
			Route:      "/clips",
			NavLabel:   "Clips y vídeos",
			CheapProof: "route /clips plus hub partida rows and status tests",
			ProbePath:  "/api/demos/jobs",
		},
		{
			ID:         "subir-demo",
			Title:      "Subir demo",
			Route:      "/clips/nueva",
			NavLabel:   "Clips y vídeos",
			CheapProof: "route /clips/nueva plus upload roster flow unit/e2e stubs",
			ProbePath:  "/api/capabilities",
		},
		{
			ID:                    "demo-completa",
			Title:                 "Demo completa",
			Route:                 "/clips",
			NavLabel:              "Clips y vídeos",
			RequiresHLAECS2:       true,
			RequiresWindowsStudio: true,
			CheapProof:            "route /clips; open a partida and choose Vídeo largo 16:9; FULL_DEMO_CONTRACT unit coverage",
		},
		{
			ID:         "tactica",
			Title:      "Táctica",
			Route:      "/tactical",
			NavLabel:   "Táctica",
			CheapProof: "nav route /tactical plus tactical unit tests",
			ProbePath:  "/api/demos/jobs",
		},
		{
			ID:         "cheaterdetect",
			Title:      "Anti-cheat",
			Route:      "/cheaters",
			NavLabel:   "Anti-cheat",
			CheapProof: "nav route /cheaters plus anticheat unit tests; screening never writes job.Status",
			ProbePath:  "/api/demos/jobs",
		},
		{
			ID:         "jugadores",
			Title:      "Jugadores",
			Route:      "/players",
			NavLabel:   "Jugadores",
			CheapProof: "nav route /players plus FACEIT client tests; Download API stays unapproved",
			ProbePath:  "/api/faceit/followed",
		},
		{
			ID:         "clips-de-stream",
			Title:      "Clips de stream",
			Route:      "/streams",
			NavLabel:   "Clips de stream",
			CheapProof: "nav route /streams plus stream plan/render unit tests",
			ProbePath:  "/api/streams",
		},
		{
			ID:         "editor",
			Title:      "Crear vídeo",
			Route:      "/clips",
			NavLabel:   "Clips y vídeos",
			CheapProof: "route /clips; open a partida to configure a Short or vídeo largo",
			ProbePath:  "/api/demos/jobs",
		},
		{
			ID:         "biblioteca",
			Title:      "Clips y vídeos · clips",
			Route:      "/clips?vista=clips",
			NavLabel:   "Clips y vídeos",
			CheapProof: "route /clips?vista=clips plus reel-reconcile and output-card unit tests",
			ProbePath:  "/api/demos/jobs",
		},
		{
			ID:         "feed",
			Title:      "Clips y vídeos · clips",
			Route:      "/clips?vista=clips",
			NavLabel:   "Clips y vídeos",
			CheapProof: "route /clips?vista=clips plus hub sort/empty/offline unit path",
			ProbePath:  "/api/demos/jobs",
		},
		{
			ID:         "ajustes",
			Title:      "Ajustes",
			Route:      "/settings",
			NavLabel:   "Ajustes",
			CheapProof: "nav route /settings plus Steam account and desktop-bridge unit path",
			ProbePath:  "/api/steam/account",
		},
		{
			ID:                    "shorts-9x16-wait",
			Title:                 "9:16 Shorts wait",
			Route:                 "/clips?vista=clips",
			RequiresHLAECS2:       true,
			RequiresWindowsStudio: true,
			CheapProof:            "capture-progress and shell-activity unit tests; live overlay is a named gap",
		},
		{
			ID:                    "full-demo-16x9-wait",
			Title:                 "Full Demo 16:9 wait",
			Route:                 "/clips?vista=clips",
			RequiresHLAECS2:       true,
			RequiresWindowsStudio: true,
			CheapProof:            "rendering-card landscape recap unit path; live overlay is a named gap",
		},
	}
}

// FeatureByID returns the catalog row or false.
func FeatureByID(id string) (Feature, bool) {
	for _, feature := range Features() {
		if feature.ID == id {
			return feature, true
		}
	}
	return Feature{}, false
}
