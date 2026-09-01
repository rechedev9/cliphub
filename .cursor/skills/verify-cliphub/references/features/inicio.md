# Inicio

First-run door inside the Studio shell. Not a marketing page.

## Sub-features

- Sidebar `00 Inicio` → `/onboarding`
- Root `/` redirects to `/onboarding`
- Share-code door and recent Steam matches
- Guide stage: demo → 9:16 Shorts or Full Demo 16:9, entirely on this PC

## How to get to it (user POV)

Open ClipHub Studio. Land on EMPIEZA AQUÍ, or click **Inicio** in the numbered rail.

## What done looks like

The onboarding header is visible, the share-code door is there, and the empty state does not invent upload/stream links the first-run contract forbids. `/` never stays on a blank marketing root.

## Driving it with zv verify

```text
./bin/zv verify prove --feature inicio --format json
./bin/zv verify features --feature inicio --format json
```

Cheap proof: map + `web/lib/nav.ts` + onboarding/first-run tests. A live first-run walk needs Windows Studio.

## Gotchas

- Unsigned installer. Actualizar reads GitHub `releases/latest`. Vercel is not the updater.
- Steam GC is an explicit user action only. Never open it at startup.
- Decoding a share code without Steam configured returns `status: "decoded"`, not an error.
