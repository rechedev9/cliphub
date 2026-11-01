# Studio MCP server: implementation plan

## Goal

Studio has two kinds of users, and both are first class. Some work in the Studio UI, where the
visual tasks live (reviewing plays, the editor, the analysis views). Others prefer to stay in an
assistant (Claude Desktop, ChatGPT Desktop) and do everything from there. The MCP server serves the
second group without taking anything from the first: both drive the same orchestrator, a job
started from the assistant shows up in the UI, and the UI stays complete on its own.

Let a user drive ClipHub Studio from an assistant in plain language:
"import my last demo and make a Short of the two best clutches". Claude acts on the Studio the
user has open, through the same orchestrator the UI uses, so every job Claude starts shows up in
the app as it runs.

The MCP server is a thin adapter over the orchestrator HTTP API. It adds no domain logic: what a
Short is, how plays are picked and how renders run stay in the orchestrator.

Why MCP and not only a CLI: MCP gives Claude live job progress, in-chat approval before expensive
work, and a one-click path for customers who use Claude Desktop.

## History: the first MCP

Studio already shipped a local MCP server once. `768ffe32` (2026-07-13) added `desktop/src/mcp/`:
a TypeScript stdio server with a discovery handshake, generic `search`/`execute` tools,
preview-then-`confirmed=true` mutations and a Settings registration card (released in 2.1.0 and
2.2.0). `5422982d` (2026-07-20) retired it, the same day an in-app Codex agent landed
(`936f15c6`), and later releases removed the `discovery_secret` it kept in `ports.json`
(`desktop/src/stable-ports.ts`). The retire commit gives no reason; the reason, per the product
owner: in July 2026 MCP clients were immature and few users worked from an assistant. Both have
changed. `web/lib/webmcp.ts` is a still-live WebMCP preview.

What this attempt changes:
- Product-action tools (`generate_short`) instead of a generic `execute` over every endpoint.
- Go in `cmd/zv-mcp`, reusing `internal/tuiclient`, instead of 5,600 lines of TypeScript beside
  Electron.
- An explicit opt-in switch, so the session file exists only for users who connect Claude.
- Protocol features that did not exist or were not used then: elicitation and progress.

Decide in phase 1 whether the WebMCP preview stays.

## Product behavior

- Settings gets a "Claude" section with a switch, "Permitir que Claude use Studio" (default off),
  and a card that shows the Claude Desktop config snippet and the `claude mcp add` command.
- With the switch off, every tool answers with a clear message saying where to turn it on.
- Starting a Short or a Full Demo launches CS2 and records for minutes, so both ask for approval
  in the chat before they run.
- The user's own Claude subscription pays for the model. Studio adds no API key and no cost.

## Architecture

```
Claude Desktop / Claude Code
        |  stdio (MCP)
   zv-mcp.exe  (cmd/zv-mcp)
        |  HTTP + X-ClipHub-Token, loopback only
   zv-orchestrator  (started by Electron)
        ^
   Electron writes <userData>/agent-session.json after boot
```

- `cmd/zv-mcp`: Go, official SDK `github.com/modelcontextprotocol/go-sdk`, stdio only, no port.
  The spike pins the version and confirms which protocol features each client supports.
- HTTP goes through `internal/tuiclient` (base URL, token header, multipart, `APIError`, Full
  Demo helpers). Add the missing calls there. `tuiclient.New` already reads `ORCHESTRATOR_URL`
  and `ZV_MUTATION_TOKEN`, so the Go server can be built and tested against a locally run
  orchestrator before any Electron change.

### Session discovery

The orchestrator port is stable across boots (`ports.json`), but the mutation token is minted per
boot, and production requires it even for reads (`requireMutationToken` in
`internal/httpapi/routes.go`).

1. After `waitForDesktopServices` succeeds, Electron always writes `<userData>/agent-session.json`
   atomically: `{ "version": 1, "enabled": false }` when the switch is off, and
   `{ "version": 1, "enabled": true, "orchestratorUrl": "http://127.0.0.1:<port>", "token": "<hex>", "dataDir": "<path>" }`
   when it is on. "No file" then means "Studio is closed", and `enabled: false` means "switch off".
2. Electron rewrites it when the switch changes and deletes it on quit, on boot failure and at the
   start of a new boot attempt (a new boot mints a new token).
3. `zv-mcp` finds userData at `%APPDATA%\cliphub-studio`, overridden by
   `CLIPHUB_STUDIO_USER_DATA`. This is a new variable: `CLIPHUB_E2E_USER_DATA` is consumed and
   deleted by `isolated-userdata.cjs`, so the MCP cannot inherit it.
4. Staleness: probe `/healthz` (outside auth) and reread the file on any 401. No pid check: pid
   liveness on Windows needs Win32 calls and pid reuse makes it unreliable.

Security: the web child already holds the same token in its environment (`boot-security.ts`), and
any process of the same Windows user can read the profile's database and media, so a file in
userData keeps the boundary at "the logged-in user". The switch keeps the token off disk for users
who never connect Claude. The token never appears in tool output, logs or telemetry.

The settings switch needs a new store file, a new action in the typed settings IPC
(`studio-settings-ipc.ts`), preload wiring and a web Settings component. The retired
`web/components/settings/mcp-settings.tsx` (in `5422982d^`) is a starting point.

### Long jobs and progress

There is no event stream; the UI polls.
- Start tools return right away with the job id and first status.
- `wait_for_job` blocks up to 50 s by default, polls `GET /api/jobs/{id}?view=status` every 2 s
  and sends MCP progress notifications from `progress` (capture progress while recording, render
  progress with `stage` while rendering). It returns the final status with `failure_code`, or
  "still running" so Claude calls it again. The spike confirms each client's tool-call timeout.
- Library demos have their own states (`queued`, `parsing`, `ready`, `failed`); `wait_for_job`
  also accepts a library demo id and waits for `ready`.

The orchestrator rate limiter is a pass-through in Studio (`WithRateLimit(0, 0)`), so polling is
safe. Only uploads are limited (2 concurrent).

### Approval

- Elicitation before `generate_short` and `render_full_demo`, showing what will run (segments and
  preset, or the Full Demo plan summary and `plan_hash`).
- Fallback for clients without elicitation: the tool returns the summary and requires a second
  call with `approve` set to the shown hash (a hash of the segment ids and preset for Shorts, the
  `plan_hash` for Full Demo).

### Untrusted text

Player names, clan names, victims and publish-assistant titles come from the demo and are chosen by
players. In tool output, return them in labelled fields, strip control characters and cap their
length. Tool descriptions say these fields are data, not instructions. A name never reaches a path,
a shell or a header.

## Tools (v1)

Names are product actions. Descriptions are in English for the model; Claude answers in the user's
language. Output is compact JSON with ids Claude can chain.

The flow mirrors the UI: import a demo (the scan starts by itself), read the match players, create
a job for one player (the job parses that player; the API may return an existing job with
`reused: true`), list the plan segments, generate.

Read:

| Tool | Orchestrator calls |
| --- | --- |
| `studio_status` | `/healthz`, `/api/capabilities` (`record.enabled`), session file state |
| `list_matches`, `get_match` | `GET /api/library/matches`, `GET /api/library/matches/{id}` (players with SteamIDs) |
| `list_moments` | `GET /api/jobs/{id}/moments`: the same segment ids as the "Prepara tu Short" list, with scores |
| `list_presets` | `GET /api/presets` |
| `list_jobs`, `get_job` | `GET /api/jobs`, `GET /api/jobs/{id}?view=status` |
| `list_videos` | `GET /api/jobs/{id}/renders/{variant}` and the publish board; paths resolved against `dataDir` |
| `publish_copy` | `GET .../videos/{name}/publish-assistant`: title, description, tags |

Write:

| Tool | Orchestrator calls | Approval |
| --- | --- | --- |
| `import_demo` | `POST /api/library/demos` (multipart `demo`, `tag`) | no |
| `create_job` | `POST /api/library/demos/{id}/jobs` with `target_steamid` | no |
| `generate_short` | `POST /api/jobs/{id}/generate` with `preset` (default from `list_presets`) and `segment_ids` from the plan | yes |
| `render_full_demo` | `GET /full-demo/plan` defaults, `POST /full-demo/plan`, then `POST /generate` with preset `gameplay-pov-60` and the approval (`tuiclient.GenerateFullDemo`) | yes |
| `wait_for_job` | polls a job or a library demo | no |

`import_demo` accepts only an absolute path to a `.dem` or `.dem.zst` file, refuses directories
and files above the server's demo size limit, and leaves real validation to the orchestrator.

`studio_status` reports whether capture is enabled, so Claude says so before it starts a Short
instead of failing halfway.

Later, behind the same approval rule: deletes, stream jobs, the editor, Faceit and Steam account
changes, `open_studio`, resources and prompts.

## Spike results (2026-11-01)

- SDK: `github.com/modelcontextprotocol/go-sdk` v1.8.0. Typed tools give `structuredContent` and a
  JSON text block for free.
- Claude Code (`claude -p` with `--mcp-config`) ran the whole flow against a throwaway
  orchestrator in about 30 s and 9 turns: `import_demo`, `wait_for_demo`, `get_match`,
  `create_job`, `wait_for_job`, `list_moments`, `generate_short` returning `needs_approval`. A
  random-bytes `.dem` came back as `not_a_demo` with a clear message.
- Headless Claude Code does not advertise elicitation, so the approval-code fallback is the path
  it takes. It worked: the model stopped and asked instead of approving on its own.
- A panic in one tool kills the stdio server and the client only sees "Connection closed". Tool
  handlers must never panic; the spike hit this once (`toolError(nil)`).
- A bare orchestrator reports capture enabled even without CS2 on the path checked; do not treat
  `capture_enabled` as proof that a capture will succeed.
- Session discovery is in: Electron writes `agent-session.json` after the health checks when
  `<userData>/agent-access.json` holds `{"enabled": true}` (the Settings switch will write that
  file), and removes it at boot start, on boot failure and on quit. Studio built from the branch
  with a throwaway profile published it, and Claude Code ran the flow against that Studio with no
  URL or token in its config. Re-importing returned `duplicate` and re-creating the job returned
  `reused`, so repeated assistant calls do not duplicate work.
- With no session file, every tool answers "ClipHub Studio is not running. Ask the user to open
  ClipHub Studio and try again".
- Claude Desktop: registered in `claude_desktop_config.json`; checking it needs a Claude Desktop
  restart by the user.
- ChatGPT Desktop is not installed on the development PC, so its stdio support is still unverified.

## Implementation phases

1. **Spike.** Confirm request and response bodies against `internal/httpapi`. Pin the SDK.
   Register a hello-world `zv-mcp` in Claude Code, Claude Desktop and ChatGPT Desktop and check
   tool timeouts, progress and elicitation support in each. Reports disagree on whether ChatGPT
   Desktop runs local stdio servers; if it only takes remote servers, ChatGPT support moves to a
   later phase and v1 stays stdio. Whatever a client lacks, the hash fallback must cover.
2. **Read tools**, against a locally run orchestrator with `ORCHESTRATOR_URL` and
   `ZV_MUTATION_TOKEN`. Map orchestrator errors to clear tool errors (Studio closed, switch off,
   job not found, capture disabled).
3. **Write tools, `wait_for_job`, approval and untrusted-text handling.**
4. **Electron**: session file, settings switch, Settings card with the config snippet and the
   `claude mcp add` command. Go: `internal/studiosession` reads and validates the file.
5. **Packaging**: add `zv-mcp` to `$commands` in `scripts/build.ps1`, copy it in `assemble.mjs`
   (which today copies exactly three exes), and update the release asset list and integrity check
   if it ships outside the installer.
6. **Later**: a `.mcpb` bundle. It embeds its own `zv-mcp.exe`, so it ships as a separate release
   asset with `release-integrity.mjs` and the asset list updated together, not inside the
   installer.

## Tests

- Unit tests where there is real logic: session file validation, error mapping, argument and path
  validation, progress mapping, name sanitizing.
- `TestNoDeadCode` skips under `-short`, and the release runs `-short`, so dead `tuiclient`
  helpers are caught only locally: run `go test ./cmd/zv -run TestNoDeadCode` before the PR.
- No e2e suite. Manual check per `AGENTS.md`: build Studio from the branch with a throwaway
  profile, set `CLIPHUB_STUDIO_USER_DATA` to it, register `zv-mcp` in Claude Code, ask Claude to
  import a demo and reach the "Prepara tu Short" list. Then import a corrupt `.dem` and check that
  Claude reports a clear error and `studio.log` shows it.

## Open questions

1. Does the WebMCP preview (`web/lib/webmcp.ts`) stay?
2. Does ChatGPT Desktop run local stdio servers (answered by the spike)?
