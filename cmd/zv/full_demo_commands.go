package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/pathguard"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const fullDemoUsage = `Usage:
  zv full-demo defaults [--out <options.json>] [--format text|json]
  zv full-demo import --demo <match.dem> --steamid <SteamID64> [--url <loopback>] [--dry-run] [--format text|json]
  zv full-demo asset --input <media> --provenance <provenance.json> [--url <loopback>] [--dry-run] [--format text|json]
  zv full-demo plan --job <uuid> --options <options.json> --out <plan.json> [--url <loopback>] [--dry-run] [--format text|json]
  zv full-demo inspect [--plan <plan.json> | --job <uuid>] [--document plan|status|approved|effective|audio|loudness|delivery] [--out <document.json>] [--url <loopback>] [--format text|json]
  zv full-demo execute --job <uuid> --plan <plan.json> --approve <plan-hash> --allow-safe-tail-trim=true [--url <loopback>] [--dry-run] [--format text|json]

The existing local orchestrator must be running for remote operations (zv serve).
Use ORCHESTRATOR_URL and ZV_MUTATION_TOKEN for its address and session token.
Import queues the existing parser; inspect --document status reports its progress.
Plan resolves facts, voice and assets and persists a draft; it never starts CS2.
Defaults are not an approval: enabled music and sponsor require declared assets.
Inspect prints the complete options, assets, round/timeline choices and blockers.
Execute approves that exact hash and queues the same capture/render flow as Studio.
It requires a reviewed creative brief and a current-run HLAE/CS2 hardware grant.
Dry-run validates local inputs only; it sends no requests and writes no files.
It cannot certify server freshness, capture, media QA or Windows/HLAE/CS2 behavior.
`

func runFullDemo(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, fullDemoUsage)
		return exitInvalidArgs
	}
	if isHelp(args[0]) || isSingleHelp(args[1:]) {
		fmt.Fprint(stdout, fullDemoUsage)
		return exitSuccess
	}
	if issue := validateSkillCommand(append([]string{"full-demo"}, args...)); issue != "" {
		return writeCommandError(args, stdout, stderr, fmt.Errorf("%s", issue), fullDemoUsage, exitInvalidArgs)
	}
	fs := flag.NewFlagSet("full-demo "+args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jobID, demo, steamID := fs.String("job", "", "parsed job UUID"), fs.String("demo", "", "demo file"), fs.String("steamid", "", "target SteamID64")
	input, provenance := fs.String("input", "", "asset file"), fs.String("provenance", "", "permission declaration JSON")
	options, plan := fs.String("options", "", "complete options JSON"), fs.String("plan", "", "server-persisted document JSON")
	out, format := fs.String("out", "", "JSON destination"), fs.String("format", "text", "text or json")
	base, document := fs.String("url", "", "loopback orchestrator URL"), fs.String("document", "plan", "document to inspect")
	approved := fs.String("approve", "", "approved plan hash")
	allowTrim, dry := fs.Bool("allow-safe-tail-trim", false, "approve bounded safety trimming"), fs.Bool("dry-run", false, "validate local inputs without requests or writes")
	fail := func(err error, code int) int { return writeCommandError(args, stdout, stderr, err, "", code) }
	if err := fs.Parse(args[1:]); err != nil {
		return fail(err, exitInvalidArgs)
	}
	if fs.NArg() != 0 || (*format != "text" && *format != "json") {
		return fail(fmt.Errorf("unexpected positional arguments or output format"), exitInvalidArgs)
	}
	if *jobID != "" {
		id, err := uuid.Parse(*jobID)
		if err != nil || id == uuid.Nil || id.String() != *jobID {
			return fail(fmt.Errorf("invalid --job UUID"), exitInvalidArgs)
		}
	}
	if *out != "" {
		if err := pathguard.RejectOutputAliases(*out, pathguard.Input{Flag: "--options", Path: *options}, pathguard.Input{Flag: "--plan", Path: *plan}); err != nil {
			return fail(err, exitInvalidArgs)
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, stop := context.WithTimeout(ctx, 30*time.Minute)
	defer stop()
	client, err := fullDemoCLIClient(*base)
	if err != nil {
		return fail(err, exitInvalidArgs)
	}
	var result any
	switch args[0] {
	case "defaults":
		result = recapplan.DefaultOptions()
	case "import":
		if len(*steamID) != 17 || strings.IndexFunc(*steamID, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return fail(fmt.Errorf("invalid --steamid"), exitInvalidArgs)
		}
		if err := regularFullDemoInput(*demo, 8<<30); err != nil {
			return fail(err, exitInvalidArgs)
		}
		if !*dry {
			result, err = client.CreateJob(ctx, *demo, *steamID)
		}
	case "asset":
		if err := regularFullDemoInput(*input, 2<<30); err != nil {
			return fail(err, exitInvalidArgs)
		}
		var p mediaassets.Provenance
		if err := readFullDemoJSON(*provenance, &p, 16<<10); err != nil {
			return fail(err, exitInvalidArgs)
		}
		// The declaration file carries the version and actual file digest. The
		// server independently hashes upload bytes and preserves provenance.
		if err := p.Validate(); err != nil {
			return fail(err, exitInvalidArgs)
		}
		digest, hashErr := mediaassets.FileDigest(ctx, *input, 2<<30)
		if hashErr != nil {
			return fail(hashErr, exitUnexpected)
		}
		if digest != p.AssetSHA256 {
			return fail(fmt.Errorf("provenance SHA-256 differs from the input file"), exitInvalidArgs)
		}
		b, _ := json.Marshal(p)
		if !*dry {
			result, err = client.UploadFullDemoAsset(ctx, *input, b)
		}
	case "plan":
		var o recapplan.Options
		if err := readFullDemoJSON(*options, &o, 4<<20); err != nil {
			return fail(err, exitInvalidArgs)
		}
		b, _ := json.Marshal(o)
		if !*dry {
			var raw json.RawMessage
			raw, err = client.PlanFullDemo(ctx, *jobID, b)
			if err == nil {
				var d recapplan.Document
				if err = json.Unmarshal(raw, &d); err == nil {
					err = d.Validate()
				}
				result = d
			}
		}
	case "inspect":
		if (*plan == "") == (*jobID == "") {
			return fail(fmt.Errorf("inspect requires exactly one of --plan or --job"), exitInvalidArgs)
		}
		if *plan != "" {
			if *document != "plan" {
				return fail(fmt.Errorf("local --plan only supports --document plan"), exitInvalidArgs)
			}
			result, err = recapplan.ReadDocumentFile(*plan)
		} else {
			switch *document {
			case "plan":
				result, err = client.GetFullDemoPlan(ctx, *jobID)
			case "status":
				result, err = client.GetJob(ctx, *jobID)
			default:
				result, err = client.GetFullDemoEvidence(ctx, *jobID, *document)
			}
		}
	case "execute":
		d, readErr := recapplan.ReadDocumentFile(*plan)
		if readErr != nil {
			return fail(readErr, exitInvalidArgs)
		}
		snapshot := recapplan.Snapshot{Document: d, Approval: recapplan.Approval{PlanHash: *approved, AllowSafeTailTrim: *allowTrim, Timestamp: time.Now().UTC()}}
		if err := snapshot.Validate(); err != nil {
			return fail(err, exitInvalidArgs)
		}
		edit := renderplan.FullDemoEditRequest(snapshot)
		if err := edit.Validate(); err != nil {
			return fail(err, exitInvalidArgs)
		}
		b, _ := json.Marshal(edit)
		if !*dry {
			result, err = client.GenerateFullDemo(ctx, *jobID, b)
		}
	}
	if err != nil {
		return fail(err, exitUnexpected)
	}
	if *dry {
		result = map[string]any{"ok": true, "dry_run": true, "executed": false, "server_validated": false, "command": "full-demo " + args[0]}
	}
	if *out != "" && !*dry {
		if err := writeJSONArtifact(*out, result); err != nil {
			return fail(err, exitUnexpected)
		}
	}
	// Text inspection also prints the complete document so no editorial choice
	// disappears from the brief; --format json provides the same machine schema.
	if err := writeJSON(stdout, result); err != nil {
		return fail(err, exitUnexpected)
	}
	return exitSuccess
}

func fullDemoCLIClient(base string) (*tuiclient.Client, error) {
	if base == "" {
		base = os.Getenv("ORCHESTRATOR_URL")
	}
	if base == "" {
		base = tuiclient.DefaultBaseURL
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("--url must be an HTTP loopback origin")
	}
	host := u.Hostname()
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return nil, fmt.Errorf("--url must address the local orchestrator")
	}
	return tuiclient.New(tuiclient.Config{BaseURL: strings.TrimRight(base, "/"), HTTPClient: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}), nil
}

func regularFullDemoInput(path string, limit int64) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect input: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > limit {
		return fmt.Errorf("input must be a nonempty regular file within its resource limit")
	}
	return nil
}

func readFullDemoJSON(path string, out any, limit int64) error {
	if err := regularFullDemoInput(path, limit); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	dec := json.NewDecoder(io.LimitReader(f, limit+1))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("expected exactly one JSON document")
	}
	return nil
}
