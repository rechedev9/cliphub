# Biblioteca cards

Ready / in-flight / failed reel cards. Publication assistant is factual, not model-written.

## Sub-features

- `library-rail` — Sidebar `09 Biblioteca` → `/videos`.
- `library-empty` — Empty library when no reels exist.
- `library-ready` — MP4 download + PREPARAR PUBLICACIÓN as soon as the file exists.
- `library-failed` — Retry unless the job is gone.
- `library-filter` — Format filter including Full Demo.
- `library-nuevo` — `?nuevo=` scrolls the new card into view.

## How to get to it (user POV)

- Click **Biblioteca**.
- Follow `?nuevo=` after a generate.
- Capture waits also live here ([shorts-9x16-wait](shorts-9x16-wait.md), [full-demo-16x9-wait](full-demo-16x9-wait.md)).

## What done looks like

Header **BIBLIOTECA**. A ready reel shows the MP4 actions without requiring a cover pick. Failed reels stay on the poll so a retry is not stuck on FALLO. Cover JPGs may exist; picking one is not required in the Library ready card.

## Driving it with zv verify

Preconditions:

- Cheap: map present.
- Ready-card walk: a real MP4 on a live job.

- **Cheap contract.** `./bin/zv verify prove --feature biblioteca --format json`.
- **Live API.** When Studio is up, prove GETs `/api/demos/jobs` on Studio `web_url` (Library reconciles from local jobs). Empty list is success. `user_path` becomes inspected, never pass.
- **Open.** Click **Biblioteca**. Heading BIBLIOTECA.
- **Empty.** No reels → empty state, not an infinite skeleton.
- **Ready.** File exists → download + PREPARAR PUBLICACIÓN enabled. Cover pick is not a gate.
- **Offline.** Service-unavailable empty/error, poll does not die forever.
- **Highlight.** `/videos?nuevo={id}` scrolls `#reel-{id}`.

## Gotchas

- `recordAdmitted` after POST `/record` keeps a still-failed job in capture instead of latching FALLO.
- Library does not use the CLI thumbnail gate.
- Publish text in the pack is deterministic from demo facts. The assistant offers reel-derived alternatives only.
- Actualizar is the desktop updater (GitHub `releases/latest`), not this page.
