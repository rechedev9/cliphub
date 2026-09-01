# Subir demo

Local `.dem` intake. The file never leaves the PC.

## Sub-features

- Sidebar `02 Subir demo` → `/upload`
- Dropzone / file picker
- Roster parse after upload
- Share-code import is a different door (Inicio / Steam), same CreateJob path

## How to get to it (user POV)

Click **Subir demo**, or follow Inicio into the upload door.

## What done looks like

A demo is accepted, roster appears, and a job exists in Partidas. No FACEIT Download API. No credential printed.

## Driving it with zv verify

```text
./bin/zv verify prove --feature subir-demo --format json
```

Cheap proof: map + upload page + stubbed upload e2e contract. A real `.dem` parse on Studio is the user path.

## Gotchas

- Real `.dem` files are not in git. Do not commit them.
- FACEIT Download API is not approved. Room/Watch download is a user-authorized source.
- `matchId` / `outcomeId` stay strings across HTTP (they exceed 2^53).
