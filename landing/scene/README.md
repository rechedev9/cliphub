# Puesto 07: the 3D scene of the landing

The landing shows a cybercafe desk from the CS 1.6 years. The scene is built in Blender from
`puesto07.py`, its light is baked into textures with `bake.py`, and `lib/ciber/scene.ts` shows the
result with three.js. Nothing here runs in the browser; the outputs go to `public/scene/`.

Generated files (`tex/`, `renders/`, `bake/`, `*.blend`) stay out of git.

## Tools

- Blender 5.2 with Cycles on an NVIDIA GPU (OptiX).
- Google Chrome, for the HTML textures.
- FFmpeg with libwebp, libvpx-vp9 and libx264.

## 1. Textures

Render each page in `textures/` to `tex/<name>.png` at its own size:

| Page | Size |
| --- | --- |
| `notepad.html` | 740x1050 |
| `poster.html`, `poster2.html`, `poster3.html` | 720x1040 |
| `clockface.html` | 512x512 |
| `ticket.html` | 480x300 |
| `postit.html` | 320x320 |
| `pad.html` | 960x800 |
| `can.html` | 1024x420 |

```bash
chrome --headless=new --hide-scrollbars --virtual-time-budget=4000 --window-size=740,1050 --screenshot=tex/notepad.png textures/notepad.html
```

The tubes in the bake and the stills use frames of the landing videos:

```bash
ffmpeg -ss 2.2 -i ../public/video/killfeed-loop.webm -frames:v 1 tex/frame-kf.png
ffmpeg -ss 1.4 -i ../public/video/hero-loop.webm -frames:v 1 tex/frame-hero.png
ffmpeg -ss 80.5 -i ../public/video/crt-jugada.webm -frames:v 1 -vf scale=1024:768 tex/screen-jugada.png
ffmpeg -i tex/frame-kf.png -filter_complex "[0]scale=1024:-1,crop=1024:768,boxblur=24,eq=brightness=-0.40[bg];[0]scale=-1:736,pad=iw+8:ih+8:4:4:color=0xff7a1e[fg];[bg][fg]overlay=(W-w)/2:12" tex/screen.png
ffmpeg -i tex/frame-hero.png -filter_complex "[0]scale=1024:-1,crop=1024:768,boxblur=24,eq=brightness=-0.40[bg];[0]scale=-1:736,pad=iw+8:ih+8:4:4:color=0xff7a1e[fg];[bg][fg]overlay=(W-w)/2:12" tex/screen4.png
ffmpeg -i tex/frame-hero.png -vf "crop=540:405:0:250,scale=1024:768" tex/screen2.png
ffmpeg -i tex/frame-hero.png -vf "crop=540:405:0:40,scale=1024:768" tex/screen3.png
```

The page shows the same frames on the tubes until each video plays (`FEEDS` in
`lib/ciber/scene.ts`). The two loops start at that frame; the CRT play tunes in through static, runs
once from the start and holds its last frame. With reduced motion the page never fetches the videos
unless the visitor presses "Reproducir vídeos":

```bash
ffmpeg -y -ss 80.5 -i ../public/video/crt-jugada.webm -frames:v 1 -c:v libwebp -quality 80 ../public/video/crt-jugada.webp
ffmpeg -y -ss 2.2 -i ../public/video/killfeed-loop.webm -frames:v 1 -vf scale=360:-2 -c:v libwebp -quality 80 ../public/video/killfeed-loop.webp
ffmpeg -y -ss 1.4 -i ../public/video/hero-loop.webm -frames:v 1 -vf scale=360:-2 -c:v libwebp -quality 80 ../public/video/hero-loop.webp
```

`public/video/crt-jugada.*` is NEO's whole 1v5 clutch for Virtus.pro at DreamHack Open: the first
83.2 s of youtube.com/watch?v=hLXQMMJmt-w, up to the fade after "Virtus.pro Wins The Round", cropped
to 4:3 and scaled to 512x384 with no audio:

```bash
ffmpeg -t 83.2 -i neo-1v5.webm -an -vf "crop=640:480:107:0,scale=512:384:flags=lanczos" -c:v libvpx-vp9 -b:v 0 -crf 46 -row-mt 1 -deadline good -cpu-used 2 ../public/video/crt-jugada.webm
ffmpeg -t 83.2 -i neo-1v5.webm -an -vf "crop=640:480:107:0,scale=512:384:flags=lanczos" -c:v libx264 -preset slow -crf 30 -pix_fmt yuv420p -movflags +faststart ../public/video/crt-jugada.mp4
```

## 2. Scene and stills

```bash
blender -b --factory-startup -P puesto07.py -- --only none
blender -b --factory-startup -P puesto07.py -- --only mesa,mesa_v,cd,stream,notepad,ticket,crt
```

The first command only writes `puesto07.blend`. The others render `renders/<camera>.png`; `--quick`
renders at half size with fewer samples. The monitor shows the CS clip unless `--crt` names another
texture; every station keeps the clip, so the `crt` still does too.
`mesa` and `mesa_v` come out wider than their shot (`OVERSCAN` in `puesto07.py`, the same factors as
`STILL_OVERSCAN` in `components/ciber-stage.tsx`), so the loading still also fills the area under the panel.

## 3. Bake and export

```bash
blender -b puesto07.blend -P bake.py -- --samples 384 --export
```

This joins the meshes of each atlas, drops faces no camera can see, unwraps them, bakes Cycles
light into `bake/<atlas>.png` (Khronos PBR Neutral, like the stills) and writes `bake/puesto07.glb`.
The tubes, the neon, the LEDs and the clock hands stay unbaked: three.js draws them live. Every
camera in `lib/ciber/stations.ts` must stay inside the box the face culling assumes (in front of
the desk, z between 0.95 and 1.62).

## 4. Publish into the landing

```bash
for f in bake/*.png; do n=$(basename "$f" .png); ffmpeg -y -i "$f" -c:v libwebp -quality 82 "../public/scene/tex/$n.webp"; ffmpeg -y -i "$f" -vf "scale=iw/2:ih/2:flags=lanczos" -c:v libwebp -quality 80 "../public/scene/tex/$n-m.webp"; done
cp bake/puesto07.glb ../public/scene/puesto07.glb
for c in mesa cd stream notepad ticket crt; do ffmpeg -y -i "renders/$c.png" -c:v libwebp -quality 80 "../public/scene/stills/$c.webp"; done
ffmpeg -y -i renders/mesa_v.png -c:v libwebp -quality 80 ../public/scene/stills/mesa-v.webp
ffmpeg -y -i renders/mesa.png -vf "crop=1714:900:223:0,scale=1200:630" -q:v 3 ../public/scene/stills/og.jpg
```
