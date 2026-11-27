# Thumbnail spec

Status: decisions 1 to 9 were **approved by the owner on 2026-11-27**. Decision 10, the layout
after LOVE CS 2, was asked for by the owner the same day; its numbers are starting values that the
owner confirms at the golden review (check 11). See [Decisions](#decisions-2026-11-27). The
renderer is written against this file.

This file is the single source of truth for the thumbnails of the content product (`content/`,
not in the repository yet): the full-map POV (16:9) and the Short (9:16) of the YouTube channel
@ClipHubRSN. It replaces the Thumbnails section of the old `pro-pov-pipeline` skill and its
`thumb.ts` and `portraits.ts`, which this spec was read from (state of 2026-11-27). Those, like the
`youtube-channel` skill and `mock-anime.ts` named below, are the owner's local files, outside this
repository. A renderer that passes the checks in
[Acceptance checks](#9-acceptance-checks) is the tool; a skill only says when to call it.

Environment details (browser profile, signed-in accounts, device ids, download folders, the FFmpeg
path) are not part of the spec. They come from `content/config.local.json`, which `.gitignore`
keeps out of this public repository.

## 1. Who supplies what

| Input | From | Validated by the tool |
|---|---|---|
| `format` | the job: `pov` or `short` | one of the two |
| `name` | the player's HLTV nick (FACEIT nick when there is no HLTV page) | normalised (see 4), then charset |
| `hook` | the LLM, from facts the demo or the HLTV match page proves | charset, word count, kind (see 4) |
| `map` | the demo | one of the maps in the look table (see 5) |
| `elo` | `faceit_player` on the day the copy is written, FACEIT matches only | `/^\d{3,4}$/`; absent means no ELO pill |
| `kills` | the featured round (see 4), from Studio's `get_round` kill feed | 1 to 5 kills by the player, each with victim, weapon, headshot and both sides |
| `background` | the library or Gemini (see 5) | decodes as JPEG or PNG; content checked by a person (see 9) |
| `portrait` | the player's HLTV bodyshot (see 6), or none | PNG signature, alpha, not the HLTV silhouette |

The LLM never draws text or faces. Gemini draws only the scene; the tool draws all text, the pills
and the kill feed, and composites the real portrait.

## 2. Formats

| Format | Canvas | Use |
|---|---|---|
| POV | 1920x1080 PNG, 16:9 | long full-map POV |
| Short | 1080x1920 PNG, 9:16 | Short; never a 16:9 image on a Short |

**Short safe zone.** The channel's Shorts tab shows a Short's thumbnail cropped to about 2:3
around the centre (y = 154 to 1766). Every text box, pill and kill feed row lies inside y = 200 to
1720, and so does the face. The pill row's top is at y = 310.

## 3. Layout

All coordinates are in canvas pixels, origin top left. Layers, bottom to top: scene, shade, halo,
edge, portrait, text glow, large text, pills, kill feed.

The layout follows the reference in 5: a large real face on the left (at the bottom on a Short),
one large statistic over the player's chest, gold pills for context and the featured round's kill
feed in the top right.

### Scene and shade

- The background is scaled to cover the canvas (`force_original_aspect_ratio=increase`) and
  centre-cropped, then `eq=saturation=1.2:contrast=1.08`.
- Shade: black over the lower 40 % only. Alpha (0 to 255) at row `Y` is `0` above `0.6 * H` and
  `130 * ((Y - 0.6 * H) / (0.4 * H))^1.4` below, so the bottom row is at 130/255.

### Portrait

| | POV | Short |
|---|---|---|
| Crop of the source | width 70 % from x = 15 %, height 72 % from the top (head to waist) | width 80 % from x = 10 %, height 58 % from the top (head and shoulders) |
| Scale (Lanczos), then `unsharp=5:5:0.8` | to height 1094 | to width 940 |
| x | -100: the left shoulder leaves the canvas | centred |
| y | `H - h + 14`: the bottom sits 14 px below the canvas | same |

- From HLTV's 400x417 bodyshot the POV head is about 360 px tall, a third of the canvas, with its
  top near y = 45.
- On a Short the scaled portrait is at most 699 px tall, so its top stays at y = 1235 or lower,
  under the hook box. A taller source is cut from the bottom, nearer the chin; the head is never
  scaled down or moved up.
- Rim light from the cutout's own alpha:
  - halo: RGB (255, 130, 20), Gaussian blur sigma 32, alpha x2.5, at `y = H - h + 20`;
  - edge: RGB (255, 205, 110), Gaussian blur sigma 5, alpha x2, at `y = H - h + 8`;
  - both at the portrait's x.

### Large text (the hook, and the name when there is no portrait)

- Font: Saira ExtraBold (800). Each line is centred horizontally in its box `[x, x + w]` and
  vertically on `cy`.
- Size: start at `min(max, floor(w / (0.68 * length)))`, where `length` counts every character
  including spaces (a unit's characters count 0.72 each, see below), then shrink 1 px at a time
  until the line measured in the pinned font, border included, fits `w`. A long line shrinks to
  fit.
- Glow: the glyphs' alpha, Gaussian blur sigma `round(size / 10)`, colour `#FF8A00` at 60 %, under
  the shadow layer.
- Shadow and outline layer: fill `#3A1200`, border width `round(size / 18)` in `#160600`, shadow
  black at 75 % offset `(0, round(size / 22))`.
- Fill: a vertical gold gradient `#FFF4B8`, `#FFC52E`, `#FF7A00` (top, middle, bottom) from
  `cy - 0.42 * size` to `cy + 0.42 * size`, masked by the glyphs.
- Unit: a hook that ends in `ADR` draws that word at 0.72 of the size, on the same baseline, after
  a space of `0.2 * size`.

| Box | POV with portrait | POV without portrait | Short with portrait | Short without portrait |
|---|---|---|---|---|
| hook | x 420, w 1440, cy 890, max 300 | x 120, w 1680, cy 870, max 320 | x 40, w 1000, cy 1090, max 300 | x 40, w 1000, cy 1300, max 300 |
| name | none: the kill feed carries it | x 120, w 1680, cy 520, max 240 | none | x 40, w 1000, cy 1000, max 230 |

On a POV the hook lies over the portrait's chest, as in the reference. On a Short the hook sits
above the head and never covers the face (the 699 px cap in Portrait keeps it so).

### Pills

- A pill is a box of height `h` with fully rounded ends (radius `h / 2`), filled with a vertical
  gradient and framed 5 px in `#5A2A00`. Its text is Saira ExtraBold in `#2A1200`, centred, at
  `min(round(0.56 * h), fit)`, with `round(0.42 * h)` of padding on each side. The width follows
  the text up to the max below; a longer text shrinks to fit.
- Map pill, always: the map name, gold `#FFE58A`, `#F2B01E`, `#C97A00`.
- ELO pill, FACEIT matches with a readable ELO only: `<elo> ELO`, FACEIT orange `#FFD2A0`,
  `#FF7A1A`, `#FF4500`.

| | POV with portrait | POV without portrait | Short |
|---|---|---|---|
| map pill | left 840, top 70, h 120, max w 430 | left 120, top 70, h 120, max w 430 | left 40, top 310, h 100, max w 480 |
| ELO pill | left 840, top 215, h 120, max w 430 | left 120, top 215, h 120, max w 430 | left at the map pill's right + 20, top 310, h 100, max w 480 |

### Kill feed

The kills of the featured round (see 4), drawn like the kill feed of Studio's Focus HUD
(`internal/customhud/killfeed.go`), so the thumbnail matches the video.

- One row per kill, in the order they happened, top to bottom, at most 5. Each row is
  right-aligned and reads: the player's name in its side's colour, the weapon icon, the headshot
  icon when the kill was a headshot, the victim's name in its side's colour.
- Colours: CT `#9CC8FF`, T `#F2C14E`, icons `#FFFFFF`, fill `#0B0D10` at 64 %, frame 3 px
  `#E0313B` (CS2 frames the local player's kills in red), corner radius 6.
- Icons: `internal/customhud/assets/weapons/<weapon>.svg` and
  `internal/customhud/assets/status/icon_skull_default.svg` (Lexogrine, MIT). A weapon with no icon
  is drawn as its label, never as another weapon's icon.
- Names: Saira ExtraBold. A name wider than its max shrinks down to size 26, then loses characters
  from the end until it fits.

| | POV | Short |
|---|---|---|
| right edge | x 1880 | x 1040 |
| first row top | y 40 | y 450 |
| row height, gap | 62, 8 | 58, 8 |
| name size, max name width | 38, 180 | 36, 180 |
| weapon icon height, max width | 32, 120 | 30, 112 |
| headshot icon | 30 | 28 |
| padding, item gap | 18, 12 | 18, 12 |

A full row is at most 582 px wide on a POV, so the kill feed starts right of x = 1298 and clears
the pills, which end at x = 1270.

## 4. Text rules

- Every word of the large text and the pills is English. That text is uppercased, then must match
  `/^[A-Z0-9 .%!?-]+$/`; the tool refuses anything else (no accents, no emoji).
- The name is normalised first: uppercased, each `_` becomes a space, and any other character
  outside the charset is dropped. A name left empty is refused.
- Kill feed names keep their case, as CS2 shows them: the player's is the `name` input as HLTV
  writes it, a victim's is the demo's. Characters outside `[A-Za-z0-9._-]` are dropped; a name left
  empty is drawn as `?`.
- The hook has at most 5 words on a POV and 3 on a Short. The limit counts the hook only, not the
  name, the pills or the kill feed.
- Every fact comes from the demo or the HLTV match page, never from memory.

| Element | POV | Short |
|---|---|---|
| hook | the K-D, `22-8`, when kills exceed deaths; otherwise the best multikill the demo proves (`ACE`, `4K`, `3K`) and, without one, the ADR (`112 ADR`). Never a K-D with kills at or under deaths | the play tag the demo proves: `ACE`, `4K`, `3K`, `1V2`, `PISTOL 3K` |
| name | a large line only when there is no portrait; the player's name always leads each kill feed row | same |
| map pill | the map name, `MIRAGE` | same |
| ELO pill | FACEIT matches with a readable ELO only, `<elo> ELO` | same |
| featured round | the round with the most kills by the player; a tie goes to more headshots, then to the later round | the round of the play the Short shows |

- A FACEIT match whose ELO cannot be read gets no ELO pill, never a guessed number.
- The rating (HLTV Rating 3.0) only picks the player; it never appears on a thumbnail.

## 5. Background (Gemini, model Neon)

- Backgrounds form a library per map, format (16:9 for a POV, 9:16 for a Short) and weapon. The
  weapon is `AWP` when most of the featured round's kills were with the AWP; otherwise the
  player's side in that round decides: `AK-47` for T, `M4A1-S` for CT. A background may be reused
  across videos. A new one is generated only when its map, format and weapon have none, or when
  the owner asks for a fresh one.
- Each new background takes one new Gemini chat, model **Neon**. Never another generator or model;
  without Neon and without a library background the thumbnail waits.
- Prompt template (fill `<Map>` and `<look>` from the table and `<weapon>` from the rule above):

> Original YouTube gaming thumbnail background, <16:9 | 9:16>, Counter-Strike 2 map <Map>: <look>.
> First-person view: the player's gloved hands hold a <weapon> in the lower right with a bright
> muzzle flash, and one distant enemy in CS2 gear is hit at mid range, its face not visible.
> Recognizable map geometry, sunny, vivid saturated color, warm golden grade, crisp contrast,
> sparks and glowing embers in the air. <POV: Keep the left 45 % simple for a large player cutout,
> the top right simple for a kill feed and the bottom fifth for a big statistic. | Short: Keep the
> bottom third simple for the player cutout and the middle for a big statistic.> No HUD, letters,
> numbers, logos or watermark.

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

- **Never in a background:** a readable face, a word, letter or number, a logo, a watermark, a
  HUD. Allowed: a bombsite letter painted on a wall, the first-person hands and weapon, and one
  distant enemy whose face cannot be read.
- A background that breaks this is discarded and generated again; after 3 tries the thumbnail
  waits for the next run.

### Reference: LOVE CS 2

The owner named [LOVE CS 2](https://www.youtube.com/@LOVE_CS_2/videos) as the visual reference on
2026-11-25, and on 2026-11-27 asked to imitate it and improve it, with the face kept human. Read
from its videos page and thumbnails on 2026-11-27:

| What it does | Where this spec does it |
|---|---|
| One statistic of at most two words, about a fifth of the canvas tall, low and centred, over the player's chest | the hook box in 3 |
| A large face with a warm glow, the top of the head near the top edge | the POV portrait and its rim light in 3 |
| A gold pill with a status mark (`#1`) | the map and ELO pills in 3 |
| A five-row kill feed in the top right, framed in red | the kill feed in 3 |
| A bright, warm first-person scene: rifle and muzzle flash in the lower right, an enemy hit in the distance, embers | the prompt above |
| A smaller unit after the number (`170 ADR`) | Unit in 3 |
| The same layout in every video, so viewers know the series | one renderer with byte-identical output (check 2) |

What this spec does better:

- The face is the real photo and no part of the player is drawn by AI. LOVE CS 2's most viewed
  thumbnail (`JU15E0_KEag`) and its latest one put donk's head on an edited bare, muscular torso.
- The kill feed is the real round: victims, weapons and headshots from the demo. LOVE CS 2 uses a
  joke feed (`Good Grades`, `Social Life`, `Bank Balance`) or the same five victims in several
  thumbnails.
- Every number on the thumbnail is proven by the demo, so the thumbnail and the title agree.
- The rifle in the scene is the one the featured round was played with.
- The kill feed has the icons and colours of the video's own HUD.

Compositions, art and wording are ours: no LOVE CS 2 image, badge, emblem or joke is copied.

## 6. Portraits

- The player's real HLTV bodyshot (`img.player-summary-stat-box-left-bodyshot` on the player page),
  fetched once per player and kept per player outside the repository, in the folder
  `content/config.local.json` names. Never an AI-drawn face or body, and no anime or other
  AI-styled version of a player (the 2026-11-27 anime mock-up was dropped).
- HLTV serves the bodyshot at 400x417. The POV scales it about 3.6 times with Lanczos, never with
  an AI upscaler, which would redraw the face.
- The jersey stays as HLTV shows it, team sponsors included: it is the team's official kit, so
  nothing is masked.
- It must be a PNG (signature `89 50 4E 47 0D 0A 1A 0A`) with transparency. HLTV's grey silhouette
  (about 12 KB) is not a photo: treat it as no portrait.
- A player without a portrait (streamers such as m3wsu or StRoGo) gets the no-portrait layout.
- Someone looks at each new portrait once: the jersey must be the player's current team.

## 7. What the data says (as of 2026-11-27)

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
- Reference channel (videos page, 2026-11-27): LOVE CS 2 has 1.12 K subscribers and 35 videos, and
  its donk videos in this layout reached 506 K (`JU15E0_KEag`), 140 K (`6Rwsdaswbjs`), 140 K
  (`5ZOYxMIcT-Y`) and 93 K (`1wAJ4LE9EQo`) views. Views that far above the subscriber count come
  from browse and suggested traffic, where the thumbnail decides the click. donk is the most
  watched player of the moment, so the subject explains part of it.
- Reading: the POV thumbnail is the one that matters for clicks, and its CTR must be measured
  again with this design, from the planned `collect_metrics` (YouTube Analytics API, per video).

## 8. Examples

Reference (LOVE CS 2: follow the layout, never copy the art):

- `JU15E0_KEag` (506 K views): face on the left with a warm glow, `6000 ELO` low and centred, gold
  `#1` pill, five-row kill feed in the top right, rifle and muzzle flash in the lower right.
- `6Rwsdaswbjs` (140 K views): the same layout with the player in his team's jersey, the closest
  to ours.

Ours before this layout (2026-11-27, replaced by decision 10):

- donk, FACEIT Ancient POV (`dWPPR5XqqD8`): `DONK`, `5299 ELO` and `32-22` stacked on the right
  and a face a quarter of the canvas tall. Three lines compete for the eye.
- ropz 1v2 Mirage Short (`8YG_9kHmkhY`): `ROPZ` and `1V2` above the head, face whole inside the
  Shorts band. The new Short keeps the hook above the head.

Bad (do not repeat):

- Labels that mix event and map (`BLAST MIRAGE`, `FACEIT MIRAGE`) or a hook that is not the K-D on
  a POV (`155 ADR`, `19 KILLS`): the 2026-11-25 backfill.
- A K-D with kills at or under deaths as a POV hook: use the fallback in 4 instead.
- An AI-drawn or AI-edited face or body, or a kill feed the demo does not prove.

## 9. Acceptance checks

A renderer test can verify each of these without a person:

1. Output is a PNG of exactly 1920x1080 (POV) or 1080x1920 (Short).
2. Rendering the same inputs twice gives byte-identical PNGs (pinned fonts and renderer).
3. Large or pill text outside `/^[A-Z0-9 .%!?-]+$/` after uppercasing (and, for the name, after
   the normalisation in 4) is refused with an error, and no file is written.
4. A hook of more than 5 words on a POV or 3 on a Short is refused; the name, the pills and the
   kill feed do not count.
5. `elo` that is not 3 or 4 digits is refused. With no `elo` there is no ELO pill, and every pixel
   outside the ELO pill's box equals the render of the same inputs with an `elo`.
6. Each large text line's bounding box, border included, lies inside its box `[x, x + w]` and
   inside the canvas; its size is the largest one at or under `min(max, floor(w / (0.68 *
   length)))` that fits, with a unit's characters counted at 0.72.
7. On a Short, every text box, pill and kill feed row lies inside y = 200 to 1720 and x = 0 to
   1080, and the pill row's top is at y = 310.
8. The map pill's top-left corner is at (840, 70) on a POV with a portrait, (120, 70) without one
   and (40, 310) on a Short, with a 5 px frame of `#5A2A00`.
9. The pixel at the vertical centre of a hook glyph stem lies on the gold gradient (hue within the
   `#FFF4B8` to `#FF7A00` range); the ELO pill's fill lies on the orange one.
10. A portrait that is not a PNG or has no alpha channel is refused, and so is HLTV's silhouette
    (about 12 KB; the exact test is the renderer's); `none` gives the no-portrait layout.
11. Golden images: for a fixed background, portrait and inputs per case (POV with and without a
    portrait, Short, FACEIT with ELO, a five-kill round with a headshot and a weapon without an
    icon), the output matches the committed golden within a small per-pixel tolerance. The goldens
    are generated, reviewed by the owner and are the switch from skill to tool.
12. On a Short the portrait is at most 699 px tall and its top is at y = 1235 or lower.
13. The kill feed has one row per kill of the featured round, in order; each row's right edge is at
    x = 1880 (POV) or 1040 (Short) and carries a 3 px `#E0313B` frame. `kills` with no entry or
    more than 5 is refused.
14. A hook that ends in `ADR` draws that word at 0.72 of the number's size, on the same baseline.

Checks a person does (not testable): the background has no readable face, no words and no logos;
the portrait is the right player and team; every word is true for that video.

## Decisions (2026-11-27)

The owner settled these on 2026-11-27; each is already applied above.

1. **Short safe zone:** the YouTube Shorts tab's 2:3 band (y = 154 to 1766), with text, pills,
   kill feed and face inside y = 200 to 1720. TikTok's 3:4 grid is dropped: the channel publishes
   only on YouTube.
2. **Word limit:** it counts the hook only, at most 5 words on a POV and 3 on a Short.
3. **POV hook:** the K-D only when kills exceed deaths; otherwise the best multikill the demo
   proves, then the ADR.
4. **Anime portraits:** dropped; the `mock-anime.ts` mock-up does not become a variant.
5. **Sponsor logos on jerseys:** kept as HLTV shows them, since the kit is the team's own.
6. **Pixel-art Shorts:** retired. The renderer's PR deletes `.claude/skills/shorts-thumbnail`,
   which also removes the Windows user path it names, and the owner's `youtube-channel` skill stops
   pointing at it.
7. **Background reuse:** allowed, from a library per map, format and weapon (see 5).
8. **Tall portraits on a Short:** capped at 699 px tall and cut from the bottom.
9. **Nicks outside the charset:** uppercased, `_` becomes a space, other characters dropped; an
   empty result is refused.
10. **Layout after LOVE CS 2** (asked for on 2026-11-27: imitate it and improve it, face kept
    human): one large statistic, a large real face, gold pills and the featured round's real kill
    feed (see 3 and 5). Its numbers are starting values that the owner confirms at the golden
    review (check 11).
