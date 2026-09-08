# Neon roster assets

The overlay is an HTML/CSS document rendered by the Chromium runtime bundled
with Studio. Fonts and flag SVGs are embedded in that document; rendering makes
no network requests. Profile avatars are fetched separately from FACEIT and
supplied as local data URLs.

- Roboto Regular, Medium and Bold: `googlefonts/roboto`, commit
  `38062f4b4a0be4346d07a928408da21602545e9e`, `src/hinted/`.
  Apache License 2.0, reproduced in `fonts/LICENSE-Roboto.txt`.
- Country flags: `flag-icons` 7.5.0, two-letter country SVGs from `flags/4x3/`.
  MIT license, reproduced in `flags/LICENSE.txt`.
- Membership, verification, trend and rank decorations are inline SVG paths in
  `neon-intro.html`. Badges appear only when their corresponding profile or
  ranking evidence exists.

The rendering template is `../neon-intro.html`. Statistics distinguish the
player's lifetime `Matches` from the last 20 matches and never substitute the
current demo's headshot rate into a FACEIT average.
