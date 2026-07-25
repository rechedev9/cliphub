# Tactical Analysis

Deterministic round analysis and 2D replay for CS2 demos, built for professional preparation work.

No model, no heuristic scoring, and no inference from rendered video: the demo is the only source.
Every judgement the classifier makes is recorded as a reason on the round it applies to, so a number an analyst disagrees with can be traced to the rule and the threshold that produced it.

## What it produces

One scan of a `.dem` produces two durable artifacts.

| Artifact | Contents | Size for a 24-round match |
|---|---|---|
| `tactical.json` | round index, economy, classification, events, identity table, map geometry, blob descriptor | ~0.2 MB |
| `positions.zvpos` | sampled player positions, one record per player per sample | ~2.1 MB at 8 Hz |

The document is the contract; `internal/tacticalplan` owns it and imports no demo parser, so the HTTP API, the CLI, and the web UI all read it without pulling the parsing stack in.
`internal/tactical` owns the scan, `internal/radarmap` owns the radar transform.

### Position sampling

Positions are sampled at 8 Hz by default (`--hz`, capped at 64).
125 ms between samples means a sprinting player moves about six pixels on a 1024 px radar, which is smoother than a 2D replay needs.
Samples are emitted on elapsed ticks rather than on a modulo of the tick number, because CS2 GOTV recordings skip and repeat frames around round transitions and a modulo silently drops or duplicates those.
The achieved rate is recorded in the descriptor and a warning is emitted when the observed interval drifts.

The `zvpos1` blob is a flat little-endian format: a 32-byte header, then per frame a tick, a presence mask, and 10 bytes per present player.
Each round carries a byte offset, so one round is decodable without touching the rest of the file.
The document records the blob's SHA-256; a reader that finds a mismatch must treat the analysis as stale and re-run it rather than draw half of it.

## Round classification

### Buy types

Thresholds are per player and multiplied by the number of players the side actually fielded, so a 4-man side is judged on what four players could afford.
They match the values the CS analysis ecosystem converged on, which keeps the numbers comparable with published team statistics.

| Buy | Rule |
|---|---|
| `pistol` | first round of a half, regulation only |
| `eco` | equipment value ≤ $1,000 per player |
| `full` | ≥ $4,500 per player (CT) or ≥ $4,000 per player (T) |
| `force` | between those bands, **and** the previous round was lost, **and** the side holds ≤ $400 per player |
| `semi` | anything else between the bands |

Two decisions worth stating explicitly.

A force buy is not a value band.
It is a half buy made while broke after a loss, which is what separates spending everything in desperation from a planned half buy.

The first round of an overtime half is not a pistol round.
Overtime halves start with $10,000, so labelling them pistol rounds would misreport every overtime economy.

Equipment value is sampled **7 seconds after freeze-time end**.
Players keep buying for a few seconds past the freeze, and by the end of the buy window they have already thrown utility and died, so both edges misreport what the team actually took into the round.

Anti-eco is not a buy type.
It is a matchup, so it is carried as a round tag and as the buy-versus-opponent-buy cross-tabulation in the aggregate.

### Round shapes

A **commitment** is three attackers reaching one bombsite within 15 seconds of each other, counted as they arrive rather than as a head count in one frame: an execute where the first man in trades immediately never has three live attackers on the site at once, but it is still an execute.
A bomb plant also proves a commitment even when fewer attackers survived to reach the site.

| T side | Rule |
|---|---|
| `eco_rush` | eco or force buy committing within 20 s of freeze end |
| `split` | attackers arrived from two or more separate directions (bearings clustered around the site, ≥ 2 attackers per direction, separated by ≥ 70° of empty arc) |
| `execute` | ≥ 3 pieces of T utility in the 10 s before the commit |
| `fast` | committed within 20 s of freeze end |
| `default` | committed later, or never reached a site but did trade fights |
| `save` | no commitment, no plant, round lost with ≥ 2 survivors |
| `unknown` | no commitment and no contact at all |

| CT side | Rule |
|---|---|
| `retake` | bomb planted and a defender was alive at the plant, or it was defused |
| `stack` | ≥ 3 defenders on one site during setup — before first contact, within 30 s of freeze end |
| `aggression` | a defender took the round's first kill within 25 s, away from any site |
| `save` | round lost with ≥ 2 survivors and no plant |
| `hold` | anything else, including a site lost before the plant with nobody left to retake |

Bombsite locations are derived from the demo itself: bomb plants first, because a plant is proof, and the map's own place names second.
Nothing is hard-coded per map, so a workshop map is analysed the same way.

### Facts, not judgements

The opening duel (first non-team kill of the round), the trade, first contact, and the bomb timeline are facts read straight from the demo and are never overwritten by classification.

A trade is a kill on the player who killed a teammate within **5 seconds**, matched on slot rather than on name so a mid-match name change cannot break it.
This is the same window the rest of the product uses for KAST, so two views of the same demo never disagree about what a trade is.

## The radar

No radar image is bundled and none is required.

`internal/radarmap` carries the numeric calibration CS2 publishes for each map — the world coordinate of the overview's upper-left corner and the world units per radar pixel — so `(world − pos) / scale` yields native radar pixels, with the Y axis inverted.
Split-level maps (Nuke, Vertigo, Train) select a layer by altitude.
A map with no shipped calibration gets one derived from the positions observed in the demo; that framing is stable within the demo but not comparable across demos, and the document says so in its warnings.

The drawable map itself is derived from play: an occupancy grid of every sampled position, at 64 world units per cell, plus the centre of mass of each of the map's own callouts.
That is the only map shape a demo can prove, it carries no third-party asset, and it works on any map.
Because the calibration is the standard one, an official overview image dropped in later aligns pixel-exactly with no other change.

## Aggregation

Tendencies are computed over whatever set of rounds a filter selects, so the numbers always match the visible round list.
A filter can follow one team across the side swap (`team`), or one side of the server (`side`).

Every rate ships with the count it came from, and a rate resting on fewer than 4 rounds is flagged as unreliable rather than presented as a tendency.
This is deliberate: a 1-of-1 site take is not a read, and a scouting report that hides its denominator is how analysts get misled.

The filter vocabulary is defined once, in `tacticalplan.FilterFromValues`, and the CLI and the HTTP API both parse through it.

## Known limits

The classifier describes what happened, not what was called.
A default that ends in a site take and an executed take of the same site look the same to it unless the utility density differs.

Approach clustering uses bearings around the site centre, so two corridors that converge before the site read as one approach.

Freeze-time positions are recorded and count towards the occupancy grid, which is intentional — setups and default spawns are exactly what a tactical tool is for — but they inflate spawn callouts in the callout ranking.
