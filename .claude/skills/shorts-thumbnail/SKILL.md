---
name: shorts-thumbnail
description: Make a pixel art YouTube Shorts thumbnail for a CS2 demo with Aseprite, laid out so nothing is cut in the channel's Shorts grid. Use when the user asks for a thumbnail, miniatura or portada for a Short or a demo.
---

# Shorts thumbnail

Thumbnails are pixel art drawn by `template.lua` in Aseprite: 180x320 art, exported x6 to 1080x1920.

## Safe zone (why the layout looks like this)

The channel's Shorts tab shows each thumbnail at about 2:3, not 9:16. It crops about 154 px from
the top and 154 px from the bottom of a 1080x1920 image (26 px each in the 180x320 art). Keep all
text between y = 200 and y = 1720 in the final image. A name placed at the very top was cut in
the grid (Anubis 11-13, 2026-10-25).

The template already places every line inside that band and writes a `_grid-preview.png` with the
grid crop applied. Check the preview, not only the full image.

## Content

1. Read the demo with `bin\zv-demo-players.exe --demo <file.dem> --format text` (build with
   `.\scripts\build.ps1` if `bin\` is empty). It gives the map, kills and deaths per player.
   The channel player is `RSN`. Take the final score from the demo file name or ask the user.
2. One hook, one big value: kills (`34`), a multikill (`3K`, `ACE`), or a clutch (`1V4`).
   Short Spanish copy works on this channel: `Y PERDIMOS`, `PARA GANAR`.
3. Lines (all optional, uppercase, 5x7 font: A-Z, 0-9, `- % ! ? .` and space; no accents or Ñ):
   - `top`: player name, `big`: the hook value, `label`: what it is (`KILLS`).
   - `sub`: one short line, `score` with `result=win|loss` (green or red badge), `map`.
   Long lines shrink to fit the width. Keep `big` to 3 or 4 characters so it stays huge.

## Run

The Steam build of Aseprite returns at once when called directly, so wait for the process:

```powershell
$a = "C:\Program Files (x86)\Steam\steamapps\common\Aseprite\Aseprite.exe"
$t = "$PWD\.claude\skills\shorts-thumbnail\template.lua"
$params = [ordered]@{ out = "C:\Users\reche\Desktop"; name = "anubis_thumb"; top = "RSN"
  big = "34"; label = "KILLS"; sub = "Y PERDIMOS"; score = "11-13"; result = "loss"; map = "ANUBIS" }
# Start-Process joins arguments with spaces, so quote each key=value
$argv = @("-b") + ($params.GetEnumerator() | ForEach-Object { "--script-param", "`"$($_.Key)=$($_.Value)`"" }) + @("--script", "`"$t`"")
Start-Process -FilePath $a -ArgumentList $argv -Wait
```

It writes `<name>.aseprite` (editable 180x320 source), `<name>_1080x1920.png` (upload this) and
`<name>_grid-preview.png` (the grid crop) to `out`.

## Check

Open `_grid-preview.png` and the full PNG with Read before sending them to the user. Every line
must be readable in the preview, and the big value must not touch the frame edges. To change the
art (another map palette, icons), edit the `.aseprite` file or the template; keep the text inside
the safe band.
