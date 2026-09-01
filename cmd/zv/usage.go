package main

const usage = `zv - deterministic CS2 demo-to-video workflows

Usage:
  zv short <demo.dem> --prompt "<instruction>" [--output-format short-9x16|landscape-16x9] [--kill-effect <style>] [--transition <style>] [--preset <name>] [--out <dir>] [--music <audio>] [--target-steamid <SteamID64>] [--from-recording <recording-result.json>] [--dry-run] [--format text|json]
  zv batch <dir> [--recursive] [--steamid <id>] [--out <dir>] [--jobs <n>] [--format text|json] [--report <path>]
  zv metrics [--reset]
  zv errors [--tail <n>] [--json] [--clear]
  zv presets [--format text|json]
  zv capabilities [--format text|json]
  zv verify doctor [--format text|json]
  zv verify features [--feature <id>] [--format text|json]
  zv verify http [--url <loopback>] [--format text|json]
  zv verify gates [--run] [--dry-run] [--format text|json]
  zv verify prove --feature <id> [--dry-run] [--format text|json]
  zv faceit index --profile <url-or-nickname> --out <demo-index.json> [--from YYYY-MM-DD] [--to YYYY-MM-DD] [--format text|json]
  zv demo parse [zv-parser parse flags]
  zv demo players [zv-demo-players flags]
  zv demo moments --killplan <plan.json> [--top <n>] [--out <moments.json>] [--dry-run] [--format text|json]
  zv demo select --killplan <plan.json> --segments <ids> --out <selected-plan.json> [--dry-run] [--format text|json]
  zv demo anticheat --demo <match.dem> [--baseline <baseline.json>] [--out <anticheat.json>] [--dossier <SteamID64>] [--dry-run] [--format text|json]
  zv demo anticheat calibrate --demos <dir> --id <name> --out <baseline.json> [--dry-run] [--format text|json]
  zv demo probe --demo <match.dem> --out <playability.json> [--dry-run] [--format text|json]
  zv demo voice --demo <match.dem> --steamid <SteamID64> --out <voice-probe.json> [--extract <dir>] [--dry-run] [--format text|json]
  zv utility audit [zv-parser utility-audit flags]
  zv record [zv-recorder flags]
  zv compose final [zv-composer flags]
  zv shorts render [zv-editor flags]
  zv stream fetch --url <https://...> --out <stream.mp4> [--max-bytes <n>] [--dry-run] [--format text|json]
  zv stream variants [--format text|json]
  zv stream plan --input <stream.mp4> --out <edit-plan.json> [--variant <name>] [--dry-run] [--format text|json]
  zv stream render --input <stream.mp4> --plan <edit-plan.json> --out <run-dir> [--dry-run] [--format text|json]
  zv music analyze [zv-rhythm analyze flags]
  zv analysis tactical --demo <match.dem> --out <tactical.json> [--positions <positions.zvpos>] [--hz <n>] [--cell-size <n>] [--dry-run] [--format text|json]
  zv analysis rounds --tactical <tactical.json> [filters] [--format text|json]
  zv analysis tendencies --tactical <tactical.json> [filters] [--format text|json]
  zv analysis tactical-data [zv-tactical-data flags]
  zv analysis view [zv-analysis-viewer flags]
  zv gallery open --path <index.html>
  zv check
  zv skills list
  zv skills show <name>
  zv skills check
  zv workflows list
  zv workflows show <name>
  zv workflows validate <name> [--format text|json] -- [workflow flags]
  zv workflows run <name> -- [workflow flags]
  zv workflows check
  zv flows list [--format text|json]
  zv flows show <demo|stream> [--format text|json]
  zv flows run <demo|stream> --run-dir <dir> --dry-run [--format text|json]
  zv serve
  zv tui [--url <orchestrator>] [--token <token>]

Legacy pass-throughs:
  zv parser [zv-parser args]
  zv editor [zv-editor args]
  zv recorder [zv-recorder args]
  zv composer [zv-composer args]
  zv orchestrator [zv-orchestrator args]
  zv analysis-viewer [zv-analysis-viewer args]
  zv tactical-data [zv-tactical-data args]
  zv rhythm [zv-rhythm args]
  zv tui [zv-tui args]

Use "zv <command> --help" for the underlying command help.
`

const shortUsage = `usage: zv short <demo.dem> --prompt "<instruction>" [flags]

One command from demo to an upload-ready vertical Short or 16:9 long-form video:
parse -> record (HLAE/CS2) -> [music analyze] -> render + publish pack.

Flags:
  --prompt <text>            editing instruction (Spanish or English); required
  --preset <name>            render preset; overrides the prompt (see zv presets)
  --out <dir>                run output directory; defaults under data/runs
  --music <audio>            music file; required for beat-synced shorts
  --target-steamid <id>      target player SteamID64 when the prompt only names a player
  --hlae <HLAE.exe>          HLAE path; defaults to env or local autodetection
  --cs2 <cs2.exe>            CS2 path; defaults to env or Steam autodetection
  --from-recording <json>    existing recording-result.json; skips parse and record
  --output-format <format>   short-9x16 (TikTok/Shorts) or landscape-16x9 (YouTube)
  --kill-effect <style>      clean, punch-in, velocity, freeze-flash, shake, or glitch
  --transition <style>       cut, flash, whip, dip, glitch, or zoom-whip
  --intro / --outro          add professional title bookends
  --cover-first-frame        freeze the cover frame over the first frames so
                             YouTube's Shorts thumbnail selector can pick it
  --encoder <name>           capture encoder: nvenc-h264, amf-h264, qsv-h264, or libx264 (default)
  --gap-timescale <n>        demo_timescale between record windows (0 = default 8, 1 = off)
  --settle-seconds <n>       1x settle before each record window (0 = default 2)
  --threads <n>              render encoder threads per short (0 = FFmpeg default)
  --dry-run                  print the resolved plan without launching HLAE/CS2 or FFmpeg
  --format <text|json>       dry-run plan format (default text; JSON requires --dry-run)

Prompt rules (deterministic keywords, no model calls):
  "todas las kills" / "all kills"        one compiled Short with every kill (default)
  "mejores" / "best" / "highlights"      best-moments compilation (top segments)
  "música" / "music" / "beat" / "ritmo"  beat-synced edit; needs --music
  a SteamID64 in the prompt              selects the target player
  a preset name in the prompt            selects that preset
  "16:9" / "horizontal" / "video largo" selects landscape-16x9
`

const presetsUsage = `usage: zv presets [--format text|json]
`

const capabilitiesUsage = `usage: zv capabilities [--format text|json]
`

const verifyUsage = `usage: zv verify doctor [--format text|json] [--dry-run] [--user-data <dir>] | zv verify features [--feature <id>] [--format text|json] | zv verify http [--url <loopback>] [--format text|json] | zv verify gates [--run] [--dry-run] [--format text|json] | zv verify prove --feature <id> [--job-id <uuid>] [--dry-run] [--user-data <dir>] [--format text|json]

Windows-first ClipHub control CLI. Doctor inspects a live Studio: orchestrator
on 127.0.0.1 from %APPDATA%\cliphub-studio\ports.json, jobs.db, HLAE
(C:\HLAE-*\HLAE.exe, never C:\HLAE\HLAE.exe), and a running cs2.exe.
The verification host of record is King's Windows Studio, not the cloud VM.
Linux doctor fail-closes and names hlae_cs2_windows_studio. Windows Passes
only when Studio + HLAE + CS2 are actually up. Never fake CS2. Hosted CI
green is not HLAE/CS2 proof. JSON is the agent contract.

Subcommands:
  doctor    live Studio surface, HLAE, running CS2, skill, feature map, named gaps
  features  dump the Studio feature map
  http      GET /healthz on a loopback orchestrator if one is running
  gates     list cheap hosted CI commands (no Playwright, no HLAE)
  prove     inspect one mapped feature; cheap features GET a read-only API when Studio is up; HLAE features GET job status / progress

Flags:
  --format text|json   machine-readable JSON in both formats
  --feature <id>       catalog id (inicio, partidas, demo-completa, ...)
  --job-id <uuid>      GET /api/jobs/{id}?view=status (status, progress.percent)
  --user-data <dir>    Studio userData (default %APPDATA%\cliphub-studio)
  --url <loopback>     orchestrator base; default http://127.0.0.1:8080
  --run                with gates, still requires --dry-run (not a second CI)
  --dry-run            inspect only; no jobs.db writes, no capture enqueue, no HTTP

Fail-closed gaps:
  hlae_cs2_windows_studio   Linux / non-Windows host (never Pass recertification)
  studio_ports_missing      ports.json missing under Studio userData
  studio_jobs_db_missing    jobs.db missing under userData\data
  studio_orchestrator_down  ports.json present but /healthz is not live
  hlae_not_detected         no C:\HLAE-*\HLAE.exe (never C:\HLAE\HLAE.exe)
  cs2_not_running           cs2.exe process is not running (never faked)
  studio_job_id_required    prove of an HLAE feature without --job-id
`

const verifyDoctorUsage = `usage: zv verify doctor [--format text|json] [--dry-run] [--user-data <dir>]

Windows-first Studio doctor. Passes only when this Windows host has a live
ClipHub Studio (ports.json + jobs.db + /healthz), detected HLAE, and a
running cs2.exe. Linux always fail-closes.

Flags:
  --format text|json   machine-readable JSON in both formats
  --user-data <dir>    Studio userData (default %APPDATA%\cliphub-studio)
  --dry-run            do not GET /healthz; never writes jobs.db or enqueues capture

Fail-closed gaps:
  hlae_cs2_windows_studio   this host is not Windows Studio (Cloud Linux)
  studio_ports_missing      %APPDATA%\cliphub-studio\ports.json is missing
  studio_jobs_db_missing    %APPDATA%\cliphub-studio\data\jobs.db is missing
  studio_orchestrator_down  Studio is down (Windows host still fail-closes)
  hlae_not_detected         expected C:\HLAE-*\HLAE.exe; never C:\HLAE\HLAE.exe
  cs2_not_running           cs2.exe is not running; doctor never fakes CS2

Never Pass capture recertification on Cloud Linux. Never fake CS2.
This CLI does not screenshot Studio and does not add Playwright to CI.
`

const verifyFeaturesUsage = `usage: zv verify features [--feature <id>] [--format text|json]
`

const verifyHTTPUsage = `usage: zv verify http [--url <loopback>] [--format text|json]
`

const verifyGatesUsage = `usage: zv verify gates [--run] [--dry-run] [--format text|json]
`

const verifyProveUsage = `usage: zv verify prove --feature <id> [--job-id <uuid>] [--dry-run] [--user-data <dir>] [--format text|json]

Inspect one mapped Studio feature. Cheap features prove the feature-map
contract, emit drive.open_url, and GET probe_path (read-only) when Studio
is up. That inspect is never a capture Pass and never a UI walk. HLAE/CS2
features GET live job status and capture-progress percent when Studio is up.

Flags:
  --feature <id>       catalog id; required
  --job-id <uuid>      GET /api/jobs/{id}?view=status (status, progress.percent)
  --user-data <dir>    Studio userData (default %APPDATA%\cliphub-studio)
  --dry-run            print the plan; no HTTP, no jobs.db writes, no capture enqueue
  --format text|json   machine-readable JSON in both formats

Fail-closed gaps:
  hlae_cs2_windows_studio   Linux cannot recertify capture or Full Demo
  studio_job_id_required    HLAE feature prove without --job-id
  studio_overlay_walk       live GET failed; this is not Full Demo Pass

Never enqueue capture. Never fake CS2. Do not call Full Demo Pass from Cloud Linux.
`

const faceitUsage = `usage: zv faceit index [flags]

Index a player's FACEIT CS2 match history, statistics, demo availability, and
manual room links without downloading demos. Set FACEIT_API_KEY first.
`

const faceitIndexUsage = `usage: zv faceit index --profile <url-or-nickname> --out <demo-index.json> [flags]

Flags:
  --profile <value>      FACEIT player URL or nickname; required
  --out <path.json>      durable demo index artifact; required
  --from <YYYY-MM-DD>    first UTC date (default January 1 of current year)
  --to <YYYY-MM-DD>      last UTC date (default today)
  --format <text|json>   output format (default text)
  --dry-run              validate without network access or writing --out

The index ranks matches only for triage. Downloaded .dem files remain the source
of truth for kills, weapons, camera, and recording ranges. Authentication is
read only from FACEIT_API_KEY and is never written to the index.
`

const batchUsage = `usage: zv batch <dir> [flags]

Parse every .dem under <dir> in-process and record each failure to the local
error journal, so a folder of demos can be exercised without driving the CLI
once per demo. Exit code is non-zero when any demo failed.

Flags:
  --recursive            descend into subdirectories
  --steamid <id>         target SteamID64 for every demo; default auto-picks the top fragger
  --out <dir>            optional directory to write each kill plan into
  --obs-dir <dir>        observability directory (default data/obs or $ZV_DATA_DIR/obs)
  --jobs <n>             max concurrent demos; 0 picks a CPU-based default
  --segment-mode <mode>  kills, smokes, or utility (default kills)
  --format text|json     summary format (default text)
  --report <path>        also write the JSON summary report to <path>
`

const metricsUsage = `usage: zv metrics [--obs-dir <dir>] [--reset]

Print the local pipeline counters in Prometheus text format. --reset clears them.
`

const errorsUsage = `usage: zv errors [--obs-dir <dir>] [--tail <n>] [--json] [--clear]

Summarize the local error journal. --clear truncates it (use between fix-loop runs).
`

const demoUsage = `usage: zv demo parse [zv-parser parse flags] | zv demo players [zv-demo-players flags] | zv demo moments [flags] | zv demo select [flags] | zv demo anticheat [flags] | zv demo probe [flags] | zv demo voice [flags]
`

const demoAnticheatUsage = `usage: zv demo anticheat --demo <match.dem> [--baseline <baseline.json>] [--out <anticheat.json>] [--dossier <SteamID64>] [--dry-run] [--format text|json]
       zv demo anticheat calibrate --demos <dir> --id <name> --out <baseline.json> [--dry-run] [--format text|json]

Screen every player in a demo for cheat-suspicion signals and score them
against a professional-play baseline. The pass is demo-only: it never launches
CS2 or HLAE and never contacts a network service.

The result is an anomaly report, not a verdict of guilt. --dossier renders the
evidence pack for one player, including the legitimate channels through which
the user can file their own report; ClipHub never submits one.
`

const demoAnticheatCalibrateUsage = `usage: zv demo anticheat calibrate --demos <dir> --id <name> --out <baseline.json> [--dry-run] [--format text|json]

Measure a baseline from a directory of demos that are known to contain
professional play, replacing the shipped distribution. Metrics without
enough samples keep the estimate and are named in the baseline description.
`

const demoVoiceUsage = `usage: zv demo voice --demo <match.dem> --steamid <SteamID64> --out <voice-probe.json> [--extract <dir>] [--dry-run] [--format text|json]

Probe whether a CS2 demo carries svc_VoiceData packets. Lists the POV and
their teammates; other speakers are aggregated. --extract writes Ogg Opus
tracks for the POV team. --dry-run validates flags without parsing.
`

const demoProbeUsage = `usage: zv demo probe --demo <match.dem> --out <playability.json> [--dry-run] [--format text|json]

Classify whether CS2 can start this demo without crashing on the playdemo
rewind to tick 0. Does not launch CS2 or HLAE. --dry-run validates flags
and skips the walk and write.
`

const demoMomentsUsage = `usage: zv demo moments --killplan <plan.json> [--top <n>] [--out <moments.json>] [--dry-run] [--format text|json]

Score and rank every planned segment for review before expensive capture.
The JSON result includes stable segment ids, kill counts, weapons, victims,
reason codes, duration, and score. --out persists the same moments document;
--dry-run scores in memory and skips the write.
`

const demoSelectUsage = `usage: zv demo select --killplan <plan.json> (--segments <seg-ids> | --top <n>) --out <selected-plan.json> [--dry-run] [--format text|json]

Create a recorder-ready kill plan containing either the requested segments in
the exact order supplied or the highest-scoring N moments. This is the decision
boundary between review and HLAE/CS2 capture; use --dry-run before committing
expensive GPU work.
`

const utilityUsage = `usage: zv utility audit [zv-parser utility-audit flags]
`

const composeUsage = `usage: zv compose final [zv-composer flags]
`

const shortsUsage = `usage: zv shorts render [zv-editor flags]
`

const streamUsage = `usage: zv stream fetch [flags] | zv stream variants [--format text|json] | zv stream plan [flags] | zv stream render [flags]

Local CLI-first stream workflow. Generate an edit plan, review its clip ranges
and crops, then render production artifacts directly under
<out>/shortslistosparasubir without starting Studio or MCP.
`

const musicUsage = `usage: zv music analyze --input <audio-or-video> --out <rhythm.json> [--killplan <plan.json>|--recording-result <recording-result.json>] [--tail-trim <seconds>] [--rank-moments [--limit <n>]]
`

const analysisUsage = `usage: zv analysis tactical --demo <match.dem> --out <tactical.json> [flags]
  zv analysis rounds --tactical <tactical.json> [filters] [--format text|json]
  zv analysis tendencies --tactical <tactical.json> [filters] [--format text|json]
  zv analysis tactical-data [zv-tactical-data flags]
  zv analysis view [zv-analysis-viewer flags]
`

const analysisTacticalUsage = `usage: zv analysis tactical --demo <match.dem> --out <tactical.json> [flags]

Scan a CS2 demo into the durable tactical document: the round index with its
economy and deterministic classification, the per-round event list, and the map
geometry derived from where players actually walked.

Flags:
  --demo <match.dem>       demo to scan; required
  --out <tactical.json>    tactical document artifact; required
  --positions <path>       also write the sidecar position blob the document describes
  --hz <n>                 position sample rate in Hz (default 8, max 64)
  --cell-size <n>          occupancy grid resolution in world units (default 64)
  --dry-run                settle the argv and print the plan without reading the demo or writing
  --format <text|json>     output format (default text)
`

const analysisRoundsUsage = `usage: zv analysis rounds --tactical <tactical.json> [filters] [--format text|json]

List the rounds a filter selects, with the economy, site, patterns, winner, and
tags an analyst reads before deciding what to watch.

Flags:
  --tactical <path>        tactical document written by "zv analysis tactical"; required
  --format <text|json>     output format (default text)

Filters (AND across flags, OR within one; repeat or comma-separate a value):
  --side <CT|T>            perspective side
  --team <key>             team key, followed across the side swap
  --buy <type>             pistol, eco, semi, force, full, or unknown
  --opponent-buy <type>    the same vocabulary, for the opponent
  --site <a|b|mid|none>    where the round was decided
  --outcome <win|loss>     result from the perspective
  --t-pattern <pattern>    execute, default, split, fast, eco_rush, save, unknown
  --ct-pattern <pattern>   hold, retake, aggression, stack, save, unknown
  --tag <tag>              round tag that must be present
  --slot <n>               player slot that must have played the round
  --round-from <n>         first round number
  --round-to <n>           last round number
  --phase <regulation|overtime>
`

const analysisTendenciesUsage = `usage: zv analysis tendencies --tactical <tactical.json> [filters] [--format text|json]

Aggregate the rounds a filter selects into buys, matchups, sites, patterns,
opening duels, timings, and players. Every rate prints its denominator, and any
rate below the reliable sample size is marked low-sample.

Flags and filters are the same as "zv analysis rounds"; see its --help.
`

const galleryUsage = `usage: zv gallery open --path <index.html>
`

const serveUsage = `usage: zv serve
`

const checkUsage = `usage: zv check [--format text|json]
`

const skillsUsage = `usage: zv skills list [--format text|json] | zv skills show <name> [--format text|json] | zv skills check [--format text|json]
`

const skillsListUsage = `usage: zv skills list [--format text|json]
`

const skillsShowUsage = `usage: zv skills show <name> [--format text|json]
`

const skillsCheckUsage = `usage: zv skills check [--format text|json]
`

const workflowsUsage = `usage: zv workflows list [--format text|json] | zv workflows show <name> [--format text|json] | zv workflows validate <name> [--format text|json] -- [workflow flags] | zv workflows run <name> -- [workflow flags] | zv workflows check [--format text|json]
`

const workflowsListUsage = `usage: zv workflows list [--format text|json]
`

const workflowsShowUsage = `usage: zv workflows show <name> [--format text|json]
`

const workflowsRunUsage = `usage: zv workflows run <name> -- [workflow flags]
`

const workflowsValidateUsage = `usage: zv workflows validate <name> [--format text|json] -- [workflow flags]
`

const workflowsCheckUsage = `usage: zv workflows check [--format text|json]
`

const flowsUsage = `usage: zv flows list [--format text|json] | zv flows show <demo|stream> [--format text|json] | zv flows run <demo|stream> --run-dir <dir> --dry-run [--format text|json]

End-to-end production journeys for agents. Workflows describe atomic commands;
flows describe decision points and the safe order from source to upload pack.
"flows run" chains a whole journey in --dry-run mode.
`

const flowsListUsage = `usage: zv flows list [--format text|json]
`

const flowsShowUsage = `usage: zv flows show <demo|stream> [--format text|json]
`

const flowsRunUsage = `usage: zv flows run <demo|stream> --run-dir <dir> --dry-run [flags]

Chain a whole production journey safely: cheap deterministic stages run for real
and write chainable JSON into --run-dir, expensive capture/render stages run with
--dry-run, and creative gates are reported as skipped. Real execution stays stage
by stage behind the creative gates, so --dry-run is required.

Demo flags:
  --demo <dem>           demo to parse and capture from
  --steamid <SteamID64>  target POV player for demo parse
  --killplan <plan.json> existing kill plan; skips demo parse
  --run-dir <dir>        run output directory (required)

Stream flags:
  --input <mp4>          stream/VOD source (required)
  --run-dir <dir>        run output directory (required)

Common flags:
  --dry-run              required; the only supported execution mode
  --format <text|json>   report format (default text)
`
