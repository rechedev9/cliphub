# cliphub-render

Motor nativo de composición y renderizado de vídeo 2D acelerado por GPU en **Rust** (`wgpu 30` + `vello 0.12` + `skrifa` + FFmpeg `NVENC`), diseñado como reemplazo de alto rendimiento de Chromium/HyperFrames para los vídeo-ensayos de análisis y overlays programáticos de **ClipHub**.

## Arquitectura

Pipeline de 3 pasadas 100% nativo (cero Chromium, cero Electron, cero Python):

1. **Decodificación concurrente (`src/video.rs`):** Cada segmento de vídeo activo (POV, radar 2D, slow-mo `0.35x`, aceleraciones `×3`/`×8`) se decodifica en un hilo dedicado con *prefetch* acotado y reescalado Lanczos al tamaño exacto del PiP antes de subir a GPU.
2. **Compositor GPU en dos pasadas (`src/gpu.rs`):**
   - **Pasada 1 (WGSL en `wgpu`):** Fondo *desk* (degradado radial + líneas diagonales) y composición directa de texturas de vídeo/imagen con **SDF *rounded corners*, borde de color, sombra suave, opacidad y cámara**. Los frames de vídeo se actualizan in-place en texturas GPU persistentes sin tocar el atlas de imágenes de Vello.
   - **Pasada 2 (`vello 0.12` *compute shaders* + `skrifa`):** Todos los elementos vectoriales y tipografía OpenType (`Card` con esquina doblada y sombra gaussiana, `HudBox`, `Chip`, `Tag`, `Ring` con halo luminoso y pulso, `Stamp` doble borde inclinado, `ProgressBar`, `Legend`, `Text` con contadores y *keyframes*, y `Captions`).
   - **Pasada 3 (WGSL Composite + Vignette):** Mezcla la capa vectorial sobre la base de vídeo, aplica la viñeta radial y vuelca el frame RGBA a un buffer reciclado.
3. **Codificación NVENC y mezcla de audio (`src/encode.rs`):** Hilo dedicado escribiendo frames raw reciclados (cero *allocations* por frame) a `h264_nvenc` / `hevc_nvenc` / `av1_nvenc` (o `libx264`), seguido de mezcla multi-pista (locución, camas musicales con *fade*, audio de juego y SFX) y *mux* sin recodificar (`-c:v copy`).

## Benchmarks medidos (Ryzen 7 9800X3D + RTX 5080 Vulkan + `h264_nvenc`)

| Escena real (HUASOPEEK) | Resolución | Duración | Frames | Render + NVENC | Total (con init + mux audio) | Velocidad |
|---|---|---|---|---|---|---|
| `examples/c1_prueba.json` (2 vídeos 60fps simultáneos + `0.35x` slow-mo + HUD + anillo + subtítulos + 5 pistas audio) | 1920×1080 @ 60fps | 8,805 s | 529 | **2,80 s** | **4,85 s** | **183 FPS** (`1,81×` tiempo real) |
| `examples/hook_20.json` (zoom cámara + contador `0→20` + 20 fichas + sello con *camera shake* + 7 pistas audio) | 1920×1080 @ 60fps | 11,717 s | 704 | **1,76 s** | **3,81 s** | **380 FPS** (`3,07×` tiempo real) |
| 4 snapshots PNG (`hook_20`) | 1920×1080 | — | 4 | **0,49 s** | **0,49 s** | instantáneo |

> Un ensayo completo de **9 minutos a 1080p60** (~32.400 frames) pasa de 20–45+ min con 30 procesos de Chromium a **~1,5–2,5 min en un solo proceso** (y **< 45 s** paralelizando 3–4 escenas simultáneas aprovechando los dos motores NVENC de la RTX 5080).

## Uso

- El motor usa el FFmpeg de `ZV_FFMPEG_PATH` (Studio lo define con su FFmpeg fijado). Sin la variable busca `ffmpeg` en el `PATH`; si apunta a un archivo que no existe, falla.
- `render` nunca reemplaza un archivo: la salida no debe existir. Trabaja en `<salida>.<pid>.partial/` y publica el MP4 solo cuando el render termina entero.
- Una fuente de vídeo ilegible, vacía o más de 0,25 s más corta que su segmento hace fallar el render.

```powershell
# Ver GPU y encoders NVENC disponibles
.\target\release\cliphub-render.exe info

# Validar esquema JSON y rutas de assets
.\target\release\cliphub-render.exe validate examples\c1_prueba.json

# Sacar capturas PNG instantáneas para QA visual
.\target\release\cliphub-render.exe snapshot examples\c1_prueba.json --at 1.0,2.5,3.6,5.5 -o out\snaps

# Render final 1080p60 con h264_nvenc (o hevc_nvenc / av1_nvenc / libx264)
.\target\release\cliphub-render.exe render examples\c1_prueba.json -o out\c1_prueba.mp4
.\target\release\cliphub-render.exe render examples\hook_20.json -o out\hook_20.mp4 --codec hevc_nvenc
```

## Integración prevista con ClipHub (ADR 0003)

1. **`content/` MCP server (TypeScript):** Genera un `essay.json` declarativo a partir del guion, `words.json` (Whisper) y los clips de `zv-mcp`, en lugar de escribir scripts largos HTML+GSAP por vídeo.
2. **Render paralelo por beats:** Invocar `cliphub-render render` sobre cada beat/escena en paralelo (3–4 workers en tu RTX 5080) y concatenar con `ffmpeg -c copy` + una única pasada de audio maestro.
3. **Evolución futura en Studio:** El mismo binario puede sustituir progresivamente filtros lentos de CPU en `internal/editor` (`zoompan`, `gblur`, overlays compuestos).
