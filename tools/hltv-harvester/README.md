# HLTV harvester

An operator tool that runs on the owner's PC: a visible Electron window walks HLTV the way a person
would and downloads demo archives. It never solves a Cloudflare challenge; it waits for the user.
Downloads stay on the PC because HLTV's Cloudflare needs the home IP and CS2 capture needs Windows.

## One store, one registry

Every mode writes to one store, the folder given by `--out` (default `%USERPROFILE%\cliphub-hltv`):

| Path | Holds |
|---|---|
| `harvest-state.json` | one record per HLTV match id: `downloaded`, `skipped`, `no_demo` or `registered` |
| `downloads/` | `<match id>.rar` and its `.json` sidecar until `prepare-hltv` turns them into analyses |
| `analyses/` | `<match id>__<map entry>.zvpro.zst`, the files the VPS indexes |
| `demos/` | `<match id>__<map entry>.dem`, kept by the fast path for Studio |

The registry of what is already in the catalog is the community API on the VPS
(`GET /v1/pro-match-ids?ids=...`, from `pro_match_maps` in `services/community`). Before the
sweep downloads anything it asks which of its candidates are registered, skips them and records
them as `registered`. When the API is down or answers an error it logs `class=registry_unavailable`
and goes on with the local state, so a VPS outage never blocks a download. `--community-url URL`
or `CLIPHUB_COMMUNITY_URL` sets the server (default `https://cliphub.rechedev.cloud`; `off` turns
the check off).

Modes of the Electron app (`pnpm start -- <flags>`):

- `--days 7 --top 50 --ranked-only --out DIR`: the sweep over recent results.
- `--match ID --out DIR`: one match, unless its archive or kept demos are already in the store.
- `--refetch DIR`: download again the archives of a folder of sidecars, skipping the ones in the store.
- `--best-maps`, `--veto-backfill DIR`, `--faceit ROOMS`: see the top of `src/main.ts`.

## Ingest commands

`pnpm --dir tools/hltv-harvester run ingest -- <command> [--dry-run] [--config FILE]` runs the steps
around the Electron app. Both commands take `ingest.lock` in the store, so a sweep and a fast path
never run at once, and log to `ingest.log`. `--dry-run` prints every command and file change and runs
none of them; it still reads the store and asks the community API.

- `daily` is `daily.sh` in TypeScript, with the same steps: build the harvester, run the sweep, build
  `cliphub-community` from `origin/main` (on failure the current binary stays), run `prepare-hltv`,
  upload the analyses the VPS inbox lacks over ssh and start the indexer, then upload new lineup
  clips from `clips/` and rebuild the lineups. A failed harvest or clips sync does not stop the run;
  a failed `prepare-hltv` or upload does.
- `match <id>` is the fast path for one match: download it if the store lacks it, run `prepare-hltv`
  keeping its demos in `demos/`, and upload that match's analyses unless the community API already
  registers it.

Settings come from the environment, then from a JSON config file (`--config` or
`CLIPHUB_INGEST_CONFIG`) with the same keys in camelCase, then the defaults:

| Environment | Config key | Default |
|---|---|---|
| `CLIPHUB_HLTV_HOME` | `home` | `%USERPROFILE%\cliphub-hltv` |
| `CLIPHUB_REPO` | `repo` | this checkout |
| `CLIPHUB_COMMUNITY_URL` | `communityUrl` | `https://cliphub.rechedev.cloud` |
| `CLIPHUB_VPS_SSH` | `sshHost` | `openclaw` |
| `CLIPHUB_VPS_INBOX` | `vpsInbox` | `/var/lib/cliphub-community/hltv` |
| `CLIPHUB_VPS_CLIPS` | `vpsClips` | `/var/lib/cliphub-community/clips` |
| `CLIPHUB_ELECTRON` | `electron` | the `electron` package |
| `CLIPHUB_PNPM` | `pnpm` | `pnpm` |
| | `keepDemos` | `false`: the sweep keeps no demos |
| | `harvestArgs` | `["--days", "7", "--top", "50", "--ranked-only"]` |

The scheduled `daily.sh` outside the repo and the content task still run as before; moving them to
these commands is a later step with the owner.
