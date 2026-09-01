package verify

// SchemaVersion is the machine-readable doctor / feature-map contract.
const SchemaVersion = 1

const (
	SkillRelPath       = ".cursor/skills/verify-cliphub/SKILL.md"
	FeatureMapRelDir   = ".cursor/skills/verify-cliphub/references/features"
	ClosedCaptureGapID = "hlae_cs2_windows_studio"
	OverlayWalkGapID   = "studio_overlay_walk"
	ClosedCaptureGap   = "HLAE/CS2 / Windows Studio gap: this host cannot recertify capture or Full Demo 16:9 recap. Cloud Linux cannot launch CS2. Hosted CI green is not HLAE/CS2 proof. Do not treat compile, lint, or unit tests as a Pass on Full Demo or live overlay walks."
	OverlayWalkGap     = "Live Studio overlay walks (9:16 Shorts wait, Full Demo 16:9 wait, PR #120 progress snapshot) are unproven on this run. Doctor never fakes a Pass. Do not merge live-overlay work until a Windows Studio walk."
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
		},
		{
			ID:         "partidas",
			Title:      "Partidas",
			Route:      "/matches",
			NavLabel:   "Partidas",
			MapFile:    "partidas.md",
			CheapProof: "nav route /matches plus matches list page",
		},
		{
			ID:         "subir-demo",
			Title:      "Subir demo",
			Route:      "/upload",
			NavLabel:   "Subir demo",
			MapFile:    "subir-demo.md",
			CheapProof: "nav route /upload plus upload roster flow unit/e2e stubs",
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
			ID:         "cheaterdetect",
			Title:      "CheaterDetect",
			Route:      "/cheaters",
			NavLabel:   "CheaterDetect",
			MapFile:    "cheaterdetect.md",
			CheapProof: "nav route /cheaters plus anticheat unit tests; screening never writes job.Status",
		},
		{
			ID:         "tactica",
			Title:      "Táctica",
			Route:      "/tactical",
			NavLabel:   "Táctica",
			MapFile:    "tactica.md",
			CheapProof: "nav route /tactical plus tactical unit tests",
		},
		{
			ID:         "jugadores",
			Title:      "Jugadores",
			Route:      "/players",
			NavLabel:   "Jugadores",
			MapFile:    "jugadores.md",
			CheapProof: "nav route /players plus FACEIT client tests; Download API stays unapproved",
		},
		{
			ID:         "clips-de-stream",
			Title:      "Clips de stream",
			Route:      "/streams",
			NavLabel:   "Clips de stream",
			MapFile:    "clips-de-stream.md",
			CheapProof: "nav route /streams plus stream plan/render unit tests",
		},
		{
			ID:         "biblioteca",
			Title:      "Biblioteca cards",
			Route:      "/videos",
			NavLabel:   "Biblioteca",
			MapFile:    "biblioteca.md",
			CheapProof: "nav route /videos plus reel-reconcile and ready-card unit tests",
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
