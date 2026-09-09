# HUD artwork

Weapon and status silhouettes in this directory are from
[Lexogrine CS2 React HUD](https://github.com/lexogrine/cs2-react-hud), commit
`7874750c97fcecd8f72eb3fad382917e035ec651`.

- `weapons/*.svg`: upstream `src/assets/weapons/*.svg`.
- `status/*.svg`: upstream `src/assets/images/*.svg`.
- The upstream MIT license is reproduced in `LICENSE.lexogrine.txt`.

The files are kept as source vectors. ClipHub's layouts, palettes and animation
logic are independent of the upstream React application. Unknown equipment must
retain its source label; it must never be assigned an unrelated weapon silhouette.

Paths are normalized to absolute cubic coordinates with SVG transforms applied.
Even-odd contours are converted to nonzero winding using fonttools and
skia-pathops 0.9.2, preserving interior cutouts in libass and SVG alike.
These are build-time asset transformations; exports require no Python packages.
