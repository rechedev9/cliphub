# Thumbnail spec

Status: **draft, waiting for the owner's approval.** No renderer code lands before it is approved.

This file is the single source of truth for the thumbnails of the content product (`content/`):
the full-map POV (16:9) and the Short (9:16) of the YouTube channel @ClipHubRSN. It replaces the
Thumbnails section of the old `pro-pov-pipeline` skill and its `thumb.ts` and `portraits.ts`, which
this spec was read from (state of 2026-11-27). A renderer that passes the checks in
[Acceptance checks](#acceptance-checks) is the tool; a skill only says when to call it.

Environment details (browser profile, signed-in accounts, device ids, download folders, the FFmpeg
path) are not part of the spec. They come from `content/config.local.json`.

## 1. Who supplies what

| Input | From | Validated by the tool |
|---|---|---|
| `format` | the job: `pov` or `short` | one of the two |
| `name` | the player's HLTV nick (FACEIT nick when there is no HLTV page) | charset, uppercased |
| `hook` | the LLM, from facts the demo or the HLTV match page proves | charset, word count, kind (see 4) |
| `map` | the demo | one of the maps in the look table (see 5) |
| `elo` | `faceit_player` on the day the copy is written, FACEIT matches only | `/^\d{3,4}$/`; absent means no ELO line |
| `background` | Gemini (see 5) | decodes as JPEG or PNG; content checked by a person (see 9) |
| `portrait` | the player's HLTV bodyshot (see 6), or none | PNG signature, alpha, not the HLTV silhouette |

The LLM never draws text or faces. Gemini draws only the scene; the tool draws all text and
composites the real portrait.

## 2. Formats

| Format | Canvas | Use |
|---|---|---|
| POV | 1920x1080 PNG, 16:9 | long full-map POV |
| Short | 1080x1920 PNG, 9:16 | Short; never a 16:9 image on a Short |

**Short safe zone.** The channel grid shows a Short's thumbnail cropped around the centre. Every
text box and the face must lie inside the 3:4 window `crop=1080:1440:0:240` (y = 240 to 1680).
All text also stays inside y = 200 to 1720, the older 2:3 measurement of the YouTube Shorts tab
(see [Open questions](#open-questions) 1). The map box starts at y = 310.

## 3. Layout

All coordinates are in canvas pixels, origin top left. Layers, bottom to top: scene, shade, halo,
edge, portrait, hook, name, ELO, map box.

### Scene and shade

- The background is scaled to cover the canvas (`force_original_aspect_ratio=increase`) and
  centre-cropped, then `eq=saturation=1.15:contrast=1.06`.
- Shade: black over the lower half only. Alpha (0 to 255) at row `Y` is `0` above `H/2` and
  `170 * ((Y - H/2) / (H/2))^1.4` below, so the bottom row is at 170/255.

### Portrait

- Crop of the source: width 80 % starting at x = 10 %, height from the top: POV 66 %, Short 58 %
  (head and shoulders, cut below the collarbone).
- Scaled (Lanczos) to width POV 860, Short 940, then `unsharp=5:5:0.8`.
- Position: x = 20 on a POV, centred on a Short. The portrait's bottom sits 14 px below the canvas
  bottom (`y = H - h + 14`).
- Rim light from the cutout's own alpha:
  - halo: RGB (255, 120, 20), Gaussian blur sigma 26, alpha x2, at `y = H - h + 20`;
  - edge: RGB (255, 205, 110), Gaussian blur sigma 5, alpha x2, at `y = H - h + 8`.

### Large text (hook, name, ELO)

- Font: Saira ExtraBold (800). Each line is centred horizontally in its box `[x, x + w]` and
  vertically on `cy`.
- Size: `min(max, floor(w / (0.68 * length)))`, so a long line shrinks to fit.
- Shadow and outline layer: fill `#3A1200`, border width `round(size / 18)` in `#160600`, shadow
  black at 75 % offset `(0, round(size / 22))`.
- Fill: a vertical three-stop gradient from `cy - 0.42 * size` to `cy + 0.42 * size`, masked by
  the glyphs:

| Line | Stops (top, middle, bottom) |
|---|---|
| hook, name | gold `#FFF4B8`, `#FFC52E`, `#FF7A00` |
| ELO (`<elo> ELO`) | FACEIT orange `#FFD2A0`, `#FF7A1A`, `#FF4500` |

| Box | POV with portrait | POV without portrait | Short (with or without) |
|---|---|---|---|
| name | x 880, w 1000, cy 150, max 230 | x 120, w 1680, cy 290, max 250 | x 40, w 1000, cy 560, max 230 |
| ELO | x 880, w 1000, cy 390, max 165 | x 120, w 1680, cy 530, max 180 | x 40, w 1000, cy 780, max 160 |
| hook | x 600, w 1280, cy 905, max 280 | x 120, w 1680, cy 870, max 320 | x 40, w 1000, cy 1090, max 270 |

On a POV with a portrait the hook overlaps the portrait's shoulder a little. On a Short the hook
sits above the head and never covers the face.

### Map box (kill-feed style)

- Position: POV left 40, y 40, height 110. Short left 40, y 310, height 100.
- Font IBM Plex Mono SemiBold, colour `#FFC52E`, size `min(round(0.62 * h), floor(600 / (0.6 *
  length)))`.
- Padding `round(0.32 * h)`; box width `round(0.6 * size * length + 2 * padding)`.
- Fill `#0B0B0D` at 82 %; frame `#E0242B`, 5 px. Text centred in the box.

## 4. Text rules

- Every word is English. Text is uppercased, then must match `/^[A-Z0-9 .%!?-]+$/`; the tool
  refuses anything else (no accents, no `_`, no emoji).
- At most 5 words on a POV and 3 on a Short (see [Open questions](#open-questions) 2 for what is
  counted).
- Every fact comes from the demo or the HLTV match page, never from memory.

| Line | POV | Short |
|---|---|---|
| name | the nick | the nick |
| ELO | FACEIT matches with a readable ELO only, `<elo> ELO` | same |
| hook | the K-D, `22-8` | the play tag the demo proves: `ACE`, `4K`, `3K`, `1V2`, `PISTOL 3K` |
| map | the map name, `MIRAGE` | same |

- A FACEIT match whose ELO cannot be read gets no ELO line, never a guessed number.
- The rating (HLTV Rating 3.0) only picks the player; it never appears on a thumbnail.

## 5. Background (Gemini, model Neon)

- One new Gemini chat per image, model **Neon**, aspect ratio 16:9 for a POV and 9:16 for a Short.
  Never another generator or model; without Neon the thumbnail waits.
- Prompt template (fill `<Map>` and `<look>` from the table):

> Original YouTube gaming thumbnail background, <16:9 | 9:16>, Counter-Strike 2 map <Map>: <look>.
> Recognizable map geometry, vivid saturated color, crisp contrast, first-person action on the far
> side, and warm orange light from a small burst or sparks. Show first-person equipment only when
> it fits the featured play. <POV: Leave room for a large real player cutout on the left and a
> big statistic along the bottom. | Short: Focus the scene around the center, with room for the
> player cutout and a large lower-middle statistic.> No visible people, fake HUD, letters, numbers,
> logos or watermark.

| Map | Look |
|---|---|
| Mirage | sandstone Moroccan town, arches, palm trees, wooden boxes |
| Inferno | Italian village, terracotta roofs, bell tower, narrow streets |
| Nuke | nuclear plant, cooling silos, steel catwalks, industrial yard |
| Dust2 | desert town, sand-colored walls, blue wooden doors |
| Ancient | jungle temple ruins, Mayan stone carvings, vines |
| Anubis | Egyptian canals, sandstone temples, palm trees |
| Train | rail yard, freight trains, concrete platforms |
| Overpass | canal park under a highway bridge, graffiti |
| Vertigo | skyscraper under construction, scaffolding, city skyline |
| Cache | abandoned warehouse yard, Soviet concrete buildings |

- **Never in a background:** a person or a face, a word, letter or number, a logo, a watermark, a
  fake HUD. A bombsite letter painted on a wall is allowed.
- A background that breaks this is discarded and generated again; after 3 tries the thumbnail
  waits for the next run.
- Art direction: the channel [LOVE CS 2](https://www.youtube.com/@LOVE_CS_2/videos) is the visual
  reference (owner's order, 2026-11-25): a large player cutout, a vivid map scene and large gold
  text. Compositions are original.

## 6. Portraits

- The player's real HLTV bodyshot (`img.player-summary-stat-box-left-bodyshot` on the player page),
  fetched once per player and kept per player. Never an AI-drawn face.
- It must be a PNG (signature `89 50 4E 47 0D 0A 1A 0A`) with transparency. HLTV's grey silhouette
  (about 12 KB) is not a photo: treat it as no portrait.
- A player without a portrait (streamers such as m3wsu or StRoGo) gets the no-portrait layout.
- Someone looks at each new portrait once: the jersey must be the player's current team.

## 7. What the data says (as of 2026-11-26)

Sources: YouTube Studio export (28 days to 2026-11-24, read 2026-11-25) and the YouTube Data API
view counts read 2026-11-26 16:08 Madrid.

- **CTR exists only for the old pixel-art thumbnails.** All 53 thumbnails were switched to the
  Gemini design on 2026-11-25 between 16:55 and 18:05, and the large name and ELO layout arrived
  on 2026-11-27. No CTR or impressions have been read since, so no element of this spec is proven
  by CTR yet.
- Channel, 28 days to 2026-11-24: 18,100 impressions, **CTR 0.9 %** (a usual YouTube range is 2 to
  10 %).
- POV CTR with pixel-art thumbnails (Studio, to 2026-11-24):

| POV | Video | Impressions | CTR |
|---|---|---|---|
| EliGE, Nuke | `N8t6PuQuejI` | 1,156 | **1.5 %** (best) |
| nilo, Nuke | `W2WOhtpheoE` | 840 | 1.4 % |
| frozen, Nuke | `ABHJmTXIK2I` | 1,302 | 1.2 % |
| xiELO, Ancient | `rPfsHtPtnM4` | 893 | 1.2 % |
| ropz, Mirage | `ZXbmyirqaJw` | 2,657 | 0.8 % |
| poiii, Dust2 | `Hf6kS-zNgac` | 4,332 | 0.6 % |
| decenty, Mirage | `OIQ3wwGx3DY` | 2,338 | **0.3 %** (worst) |

- POV views (2026-11-26): best EliGE Nuke `N8t6PuQuejI` 197, Senzu ACE Dust2 `hzQGa8OEgJ4` 172,
  frozen Nuke `ABHJmTXIK2I` 170; worst Jorko Mirage `PV1XM4NsH-c`, jottAAA Mirage `YC-nKbp8OzI` and
  rain Dust2 `nX82FZdhra8`, 1 view each (all under 2 days old). POVs drew 6 % of the views of the
  videos published from 2026-11-24 (median 41 a POV).
- Short views (2026-11-26): best ropz 1v2 Mirage `8YG_9kHmkhY` 2,878, decenty pistol 3K
  `NuweLheksV8` 1,796, frozen 3 headshots `XEej0npeCsI` 1,712; worst Krimbo retake `00cQYXgn5Lg` 6,
  luchov 2K `mrNULpT0WrA` 14, stavn AWP `WZrEDZa3tVw` 16. Best and worst carry the same design.
  The gap follows the upload order: YouTube spread about the first 9 to 10 Shorts in any 24 h and
  left the rest under 50 views. **Short views say nothing about the thumbnail.**
- Reading: the POV thumbnail is the one that matters for clicks, and its CTR must be measured
  again with this design, from the planned `collect_metrics` (YouTube Analytics API, per video).

## 8. Examples

Good (follow these; thumbnails of 2026-11-27):

- donk, FACEIT Ancient POV (`dWPPR5XqqD8`): portrait left with the rim light, `DONK`, `5299 ELO`,
  `32-22`, map box `ANCIENT`. Name, ELO and K-D read at phone size.
- ropz 1v2 Mirage Short (`8YG_9kHmkhY`): `ROPZ` and `1V2` above the head, face whole in the 3:4
  window.

Bad (do not repeat):

- Labels that mix event and map (`BLAST MIRAGE`, `FACEIT MIRAGE`) or a hook that is not the K-D on
  a POV (`155 ADR`, `19 KILLS`): the 2026-11-25 backfill, replaced by this layout.
- A POV hook with a small or negative K-D (EliGE Cache `23-21`): see Open questions 3.
- A portrait whose jersey shows a betting sponsor in large type (ropz, luchov, donk): see Open
  questions 5.

## 9. Acceptance checks

A renderer test can verify each of these without a person:

1. Output is a PNG of exactly 1920x1080 (POV) or 1080x1920 (Short).
2. Rendering the same inputs twice gives byte-identical PNGs (pinned fonts and renderer).
3. Text outside `/^[A-Z0-9 .%!?-]+$/` after uppercasing is refused with an error, and no file is
   written.
4. More than 5 words on a POV or 3 on a Short is refused (per the rule chosen in Open questions 2).
5. `elo` that is not 3 or 4 digits is refused; with no `elo`, no pixel of the ELO box differs from
   the render without it.
6. Each text line's bounding box lies inside its box `[x, x + w]` and inside the canvas; its size
   is `min(max, floor(w / (0.68 * length)))`.
7. On a Short, every text bounding box and the map box lie inside y = 240 to 1680 and x = 0 to
   1080; the map box top is at y = 310.
8. The map box's top-left corner is at (40, 40) on a POV and (40, 310) on a Short, with a 5 px
   frame of `#E0242B`.
9. The pixel at the vertical centre of a hook glyph stem lies on the gold gradient (hue within the
   `#FFF4B8` to `#FF7A00` range); the ELO line on the orange one.
10. A portrait that is not a PNG or has no alpha channel is refused, and so is HLTV's silhouette
    (about 12 KB; the exact test is the renderer's); `none` gives the no-portrait layout.
11. Golden images: for a fixed background, portrait and text set per format (POV with and without
    portrait, Short, FACEIT with ELO), the output matches the committed golden within a small
    per-pixel tolerance. The goldens are generated, reviewed by the owner and are the switch from
    skill to tool.

Checks a person does (not testable): the background has no people, words or logos; the portrait is
the right player and team; every word is true for that video.

## Open questions

The sources disagree on these. Each needs the owner's decision before the renderer is written.

1. **Short safe zone.** `thumb.ts` uses TikTok's 3:4 profile grid (y = 240 to 1680) and places the
   map box at y = 310 for it. The repo's `shorts-thumbnail` skill measured the YouTube Shorts tab at
   about 2:3 (y = 154 to 1766, text inside 200 to 1720). TikTok and Instagram are out of the new
   design. Keep 3:4 as the safe zone, or move to the YouTube 2:3 measurement?
2. **Word limit.** "At most 5 words on a POV and 3 on a Short", but a FACEIT Short already has
   `DONK`, `5299 ELO`, `1V2` and `MIRAGE` (5 words). Does the limit count the hook only, or every
   line?
3. **POV hook with a poor K-D.** The pipeline makes the POV hook the K-D. The `youtube-channel`
   skill says a negative K-D stays out of the title and the thumbnail. What is the hook then (ADR,
   a multikill, nothing)? And is a K-D like `23-21` worth showing?
4. **Anime portraits.** On 2026-11-27 a mock-up (`mock-anime.ts`) drew AI anime versions of donk
   and luchov with a `FACEIT` context pill and no K-D. The adopted rule says never an AI-drawn face.
   Is the anime style dropped, or does it become a variant?
5. **Sponsor logos on jerseys.** Backgrounds must carry no logo, but HLTV bodyshots show team
   sponsors, several of them betting brands. Allowed as they are, or masked?
6. **Pixel-art Shorts.** The repo's `shorts-thumbnail` skill (Aseprite pixel art, Spanish copy such
   as `Y PERDIMOS`) is still listed for Short thumbnails, against the English-only rule and the
   Gemini design. Is it retired, or kept for the owner's own `RSN` Shorts?
7. **Background reuse.** The pipeline asks for one new Gemini image per thumbnail; a set of
   per-map backgrounds was also made on 2026-11-25. May a map background be reused across videos?
