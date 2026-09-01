// Package verify is the Windows-first ClipHub control surface behind `zv verify`.
// Doctor inspects a live Studio (ports.json, jobs.db, HLAE, running cs2.exe).
// Linux fail-closes. Capture recertification never Passes on Cloud Linux.
package verify

// SchemaVersion is the machine-readable doctor / feature-map contract.
const SchemaVersion = 2

const (
	SkillRelPath       = ".cursor/skills/verify-cliphub/SKILL.md"
	FeatureMapRelDir   = ".cursor/skills/verify-cliphub/references/features"
	StudioAppDir       = "cliphub-studio"
	StudioPortsFile    = "ports.json"
	StudioJobsDBRel    = "data/jobs.db"
	CS2ImageName       = "cs2.exe"
	ClosedCaptureGapID = "hlae_cs2_windows_studio"
	OverlayWalkGapID   = "studio_overlay_walk"
	StudioPortsGapID   = "studio_ports_missing"
	StudioJobsDBGapID  = "studio_jobs_db_missing"
	StudioDownGapID    = "studio_orchestrator_down"
	HLAEMissingGapID   = "hlae_not_detected"
	CS2NotRunningGapID = "cs2_not_running"
	JobIDRequiredGapID = "studio_job_id_required"
	ClosedCaptureGap   = "HLAE/CS2 / Windows Studio gap: this host cannot recertify capture or Full Demo 16:9 recap. The verification host of record is King's Windows ClipHub Studio, not the cloud VM. Cloud Linux cannot launch CS2. Hosted CI green is not HLAE/CS2 proof. Do not treat compile, lint, or unit tests as a Pass on Full Demo or live overlay walks."
	OverlayWalkGap     = "This CLI does not screenshot Studio and does not add Playwright to CI. Live overlay percent is inspected via GET /api/jobs/{id}?view=status progress.percent on King's Windows Studio. Do not call Full Demo Pass from Cloud Linux."
	StudioPortsGap     = "ClipHub Studio ports.json is missing under %APPDATA%\\cliphub-studio. Doctor cannot find the live orchestrator. Start Studio on this Windows host."
	StudioJobsDBGap    = "jobs.db is missing under %APPDATA%\\cliphub-studio\\data. Studio userData is not the live install doctor expected."
	StudioDownGap      = "Studio orchestrator is down. ports.json was read but GET /healthz is not {service:cliphub,status:ok} on 127.0.0.1. A Windows host with Studio down still fail-closes."
	HLAEMissingGap     = "HLAE not detected. Expected C:\\HLAE-*\\HLAE.exe (never C:\\HLAE\\HLAE.exe) or packaged %APPDATA%\\cliphub-studio\\tools\\hlae\\*\\HLAE.exe."
	CS2NotRunningGap   = "cs2.exe is not running. Doctor never fakes CS2. An installed cs2.exe path is not enough; start Counter-Strike 2 on this Windows host before capture recertification can Pass."
	JobIDRequiredGap   = "prove needs --job-id to inspect GET /api/jobs/{id}?view=status (status, capture-progress, overlay percent). This is not Full Demo Pass."
)

// RequiredFeatureHeadings are the H2s every feature-map file must carry.
var RequiredFeatureHeadings = []string{
	"## Sub-features",
	"## How to get to it (user POV)",
	"## What done looks like",
	"## Driving it with zv verify",
	"## Gotchas",
}

// Feature is one Studio user-path the lever can name.
type Feature struct {
	ID                    string `json:"id"`
	Title                 string `json:"title"`
	Route                 string `json:"route"`
	NavLabel              string `json:"nav_label,omitempty"`
	MapFile               string `json:"map_file"`
	RequiresHLAECS2       bool   `json:"requires_hlae_cs2"`
	RequiresWindowsStudio bool   `json:"requires_windows_studio"`
	CheapProof            string `json:"cheap_proof"`
	ProbePath             string `json:"probe_path,omitempty"`
}

// Features is the closed Studio map the skill and CLI must stay aligned with.
func Features() []Feature {
	return []Feature{
		{
			ID:         "inicio",
			Title:      "Inicio",
			Route:      "/onboarding",
			NavLabel:   "Inicio",
			MapFile:    "inicio.md",
			CheapProof: "nav route /onboarding plus onboarding page and first-run tests",
			ProbePath:  "/api/steam/account",
		},
		{
			ID:         "partidas",
			Title:      "Partidas",
			Route:      "/matches",
			NavLabel:   "Partidas",
			MapFile:    "partidas.md",
			CheapProof: "nav route /matches plus matches list page",
			ProbePath:  "/api/demos/jobs",
		},
		{
			ID:         "subir-demo",
			Title:      "Subir demo",
			Route:      "/upload",
			NavLabel:   "Subir demo",
			MapFile:    "subir-demo.md",
			CheapProof: "nav route /upload plus upload roster flow unit/e2e stubs",
			ProbePath:  "/api/capabilities",
		},
		{
			ID:                    "demo-completa",
			Title:                 "Demo completa",
			Route:                 "/full-demo",
			NavLabel:              "Demo completa",
			MapFile:               "demo-completa.md",
			RequiresHLAECS2:       true,
			RequiresWindowsStudio: true,
			CheapProof:            "nav route /full-demo plus FULL_DEMO_CONTRACT unit coverage",
		},
		{
			ID:         "tactica",
			Title:      "Táctica",
			Route:      "/tactical",
			NavLabel:   "Táctica",
			MapFile:    "tactica.md",
			CheapProof: "nav route /tactical plus tactical unit tests",
			ProbePath:  "/api/demos/jobs",
		},
		{
			ID:         "cheaterdetect",
			Title:      "CheaterDetect",
			Route:      "/cheaters",
			NavLabel:   "CheaterDetect",
			MapFile:    "cheaterdetect.md",
			CheapProof: "nav route /cheaters plus anticheat unit tests; screening never writes job.Status",
			ProbePath:  "/api/demos/jobs",
		},
		{
			ID:         "jugadores",
			Title:      "Jugadores",
			Route:      "/players",
			NavLabel:   "Jugadores",
			MapFile:    "jugadores.md",
			CheapProof: "nav route /players plus FACEIT client tests; Download API stays unapproved",
			ProbePath:  "/api/faceit/followed",
		},
		{
			ID:         "clips-de-stream",
			Title:      "Clips de stream",
			Route:      "/streams",
			NavLabel:   "Clips de stream",
			MapFile:    "clips-de-stream.md",
			CheapProof: "nav route /streams plus stream plan/render unit tests",
			ProbePath:  "/api/streams",
		},
		{
			ID:         "editor",
			Title:      "Editor",
			Route:      "/editor",
			NavLabel:   "Editor",
			MapFile:    "editor.md",
			CheapProof: "nav route /editor plus editor home empty/create unit path",
			ProbePath:  "/api/editor/projects",
		},
		{
			ID:         "biblioteca",
			Title:      "Biblioteca cards",
			Route:      "/videos",
			NavLabel:   "Biblioteca",
			MapFile:    "biblioteca.md",
			CheapProof: "nav route /videos plus reel-reconcile and ready-card unit tests",
			ProbePath:  "/api/demos/jobs",
		},
		{
			ID:         "feed",
			Title:      "Feed",
			Route:      "/feed",
			NavLabel:   "Feed",
			MapFile:    "feed.md",
			CheapProof: "nav route /feed plus feed sort/empty/offline unit path",
			ProbePath:  "/api/capabilities",
		},
		{
			ID:         "ajustes",
			Title:      "Ajustes",
			Route:      "/settings",
			NavLabel:   "Ajustes",
			MapFile:    "ajustes.md",
			CheapProof: "nav route /settings plus Steam account and desktop-bridge unit path",
			ProbePath:  "/api/steam/account",
		},
		{
			ID:                    "shorts-9x16-wait",
			Title:                 "9:16 Shorts wait",
			Route:                 "/videos",
			MapFile:               "shorts-9x16-wait.md",
			RequiresHLAECS2:       true,
			RequiresWindowsStudio: true,
			CheapProof:            "capture-progress and shell-activity unit tests; live overlay is a named gap",
		},
		{
			ID:                    "full-demo-16x9-wait",
			Title:                 "Full Demo 16:9 wait",
			Route:                 "/videos",
			MapFile:               "full-demo-16x9-wait.md",
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
