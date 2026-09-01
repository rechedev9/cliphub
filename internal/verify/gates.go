package verify

// Gate is one cheap hosted check that already exists. This lever does not add
// Playwright or HLAE/CS2 to CI.
type Gate struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Hosted     bool     `json:"hosted"`
	Playwright bool     `json:"playwright"`
	HLAE       bool     `json:"hlae"`
	Commands   []string `json:"commands"`
	Note       string   `json:"note,omitempty"`
}

// Gates catalogs the hosted quality CI that already runs on PRs and main.
func Gates() []Gate {
	return []Gate{
		{
			ID:     "ci_frontend",
			Title:  "CI frontend",
			Hosted: true,
			Commands: []string{
				"pnpm --dir web run lint",
				"pnpm --dir web run typecheck",
				"pnpm --dir web run test:unit",
				"pnpm --dir desktop run lint",
				"pnpm --dir desktop run typecheck",
				"pnpm --dir desktop run test:unit",
			},
			Note: "Not Playwright. web/e2e stays an explicit local command.",
		},
		{
			ID:     "ci_backend",
			Title:  "CI backend",
			Hosted: true,
			Commands: []string{
				"go vet ./...",
				"go test ./... -count=1 -timeout 3m",
				"go run ./cmd/zv check",
			},
			Note: "Not HLAE/CS2 E2E.",
		},
		{
			ID:     "ci_infra",
			Title:  "CI infra",
			Hosted: true,
			Commands: []string{
				"actionlint",
			},
			Note: "Unsigned-release contract. Do not add signing steps.",
		},
	}
}

// GateReport is the dumpable hosted-gate catalog.
type GateReport struct {
	SchemaVersion int      `json:"schema_version"`
	OK            bool     `json:"ok"`
	DryRun        bool     `json:"dry_run"`
	Playwright    bool     `json:"playwright"`
	HLAE          bool     `json:"hlae"`
	Gates         []Gate   `json:"gates"`
	Commands      []string `json:"commands"`
}

// InspectGates lists the cheap hosted gates. --run without --dry-run is the
// caller's choice; this constructor never executes them.
func InspectGates(dryRun bool) GateReport {
	gates := Gates()
	var commands []string
	for _, gate := range gates {
		commands = append(commands, gate.Commands...)
	}
	return GateReport{
		SchemaVersion: SchemaVersion,
		OK:            true,
		DryRun:        dryRun,
		Playwright:    false,
		HLAE:          false,
		Gates:         gates,
		Commands:      commands,
	}
}
