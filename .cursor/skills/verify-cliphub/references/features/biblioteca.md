# Biblioteca cards

Ready / in-flight / failed reel cards. Publication assistant is factual, not model-written.

## Sub-features

- Sidebar `09 Biblioteca` → `/videos`
- Ready card: MP4 download + PREPARAR PUBLICACIÓN as soon as the file exists
- Failed card: Retry unless the job is gone
- Format filter including Full Demo

## How to get to it (user POV)

Click **Biblioteca**, or follow `?nuevo=` after a generate.

## What done looks like

A ready reel shows the MP4 actions without requiring a cover pick. Failed reels stay on the poll so a retry is not stuck on FALLO. Cover JPGs may exist; picking one is not required in the Library ready card.

## Driving it with zv verify

```text
./bin/zv verify prove --feature biblioteca --format json
```

Cheap proof: map + reel-reconcile tests. A ready reel walk needs a real MP4.

## Gotchas

- `recordAdmitted` after POST `/record` keeps a still-failed job in capture instead of latching FALLO.
- Library does not use the CLI thumbnail gate.
- Publish text in the pack is deterministic from demo facts. The assistant offers reel-derived alternatives only.
- Actualizar is the desktop updater (GitHub `releases/latest`), not this page.
