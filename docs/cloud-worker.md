# ClipHub cloud worker runbook

The "ClipHub cloud" is one Windows PC we own (the capture machine) running
ClipHub Studio with two extra environment variables. Studio's orchestrator
then pulls jobs from the portal queue (`internal/cloudbridge`), captures and
renders them like a local user would, and uploads the videos. It only makes
outbound HTTPS requests to the portal. The operator panel is `/admin` on the
portal.

## One-time setup

1. **Windows user.** A dedicated, non-administrator account (for example `capture`) with
   automatic logon, so a reboot comes back to a desktop session without
   anyone typing a password. CS2 runs demos from strangers with `-insecure`;
   nothing of value belongs on this account.
2. **No lock, no sleep.** Disable the lock screen, the screen saver and
   sleep. CS2 only renders into an interactive desktop.
3. **Steam.** Install Steam with a dedicated account that owns nothing,
   enable "remember me", and put Steam in the user's Startup folder. Install
   CS2 and start it once by hand, so first-run prompts are out of the way.
4. **Studio.** Install ClipHub Studio and put it in the Startup folder too.
   Start it once and complete the first-run guide. Studio installs the
   newest official HLAE at every start.
5. **Worker credential.** In the panel, Workers, create a worker. The token
   is shown once.
6. **Environment variables** for that Windows user (`setx NAME value`, then
   log off and on):

   | Variable | Value |
   |---|---|
   | `ZV_BRIDGE_URL` | `https://cliphub.gravityroom.app` |
   | `ZV_BRIDGE_TOKEN` | the worker token (64 hex characters) |
   | `ZV_RECORD_TIMEOUT` | `45m` (default 20m; bounds one whole CS2 run) |
   | `ZV_RENDER_TIMEOUT` | `60m` (default 20m) |
   | `ZV_CLOUD_WORKER_MIN_FREE_BYTES` | optional, default 32212254720 (30 GiB) |

   Both bridge variables must be set, or neither. Studio refuses to start
   with only one, and requires `https://` unless the URL is loopback.
7. **Watchdog.** Add the scheduled task from "Watchdog for the whole app".
8. **Check.** Restart Studio. Its log shows
   `cloudbridge: worker enabled, taking jobs from ...`, and within 15 seconds
   the worker card in the panel is online with the Studio, CS2 and HLAE
   versions, Steam running and the free disk space.

## Remote access

Never leave the session by closing an RDP window. Disconnecting RDP detaches
the desktop, CS2 stops rendering and every capture after that fails. Either
use a remote tool that shares the console session, or hand the session back
to the console before you leave, from an elevated prompt:

```
tscon %sessionname% /dest:console
```

## What the worker does on its own

- **Heartbeat** every 15 seconds: state, versions (Studio, kill plan schema,
  CS2, HLAE), disk and the progress of its jobs.
- **Preflight** before every claim. If the machine cannot capture, the worker
  reports "blocked" and claims nothing. The queue is not touched.
- **One capture at a time.** The next job is claimed only when the previous
  one has finished rendering. Uploading does overlap with the next capture.
- **Bounded uploads.** While three finished jobs wait for their upload the
  worker claims nothing, and the portal refuses its claims with
  `upload_backlog`. A slow uplink slows the queue down instead of piling up
  videos that would be lost.
- **No claim without a lease.** If the portal answers the heartbeat with an
  error other than a rejected token, no lease is being renewed, so the worker
  stops claiming until a heartbeat is accepted again. Its log says
  `the portal refused the heartbeat`.
- **Cancel, timeout and lost lease.** The worker ends the recorder or the
  editor together with everything they started (CS2, ffmpeg, the overlay
  renderer), waits until those processes are gone, deletes the local job and
  only then claims again.
- **Black capture check.** Before a video is handed over, the worker reads
  the bitrate of the raw takes. If the whole capture averages under 0.04
  bits per pixel (about 5 Mb/s at 1080p60; a normal capture has about 40),
  nothing is delivered: the job goes back to the queue and the portal pauses
  the worker with `capture_incompatible: black capture suspected ...`.
- **Restart safe.** Studio or the PC can be restarted at any moment. A
  capture that was running is reported as `interrupted` at the next start
  and the portal requeues it (attempt 2). An upload continues from the parts
  the portal already has, without capturing again, as long as the worker
  token is the same (see "Rotate the token").
- **Backend restart.** If the backend (`zv-orchestrator.exe`) stops, Studio
  shows its stop screen and starts it again by itself: after 5 seconds, then
  10, 20 and so on up to 5 minutes. After 10 failures in a row it stops
  trying and waits for someone to press Reintentar; a backend that stayed up
  for 10 minutes starts the count over. This only happens when both bridge
  variables are set. `studio.log` has a line
  `[boot] cloud worker: restarting the backend in N s` for each restart.
- **Leftovers of a crash.** At every start, before the first claim, the
  worker ends any `zv-recorder.exe` and `zv-editor.exe` of this install that
  a dead backend left running (which also closes their CS2 and ffmpeg) and
  deletes the `zv-<stage>-<job id>-*` work directories in `%TEMP%`.
- **Cleanup.** After delivery or failure the worker deletes the local job
  (demo, raw capture, renders) and the downloaded demo. A janitor deletes
  leftovers of jobs the worker no longer holds at every start and every ten
  minutes while the worker is between jobs.

Local files: `<data dir>\cloudbridge\state.json` (what the worker holds) and
`<data dir>\cloudbridge\incoming\` (demos being processed).

**One Studio on this machine.** Because of the two cleanup steps above, do
not run a second Studio, or `zv-editor` by hand from the installed folder,
on the worker: the next start of the worker would end those processes and
delete their work directories.

## Watchdog for the whole app

The backend restart above lives inside Studio, so it cannot help when Studio
itself is gone (the Electron process crashed, or someone closed the window).
A reboot is already covered by automatic logon plus the Startup folder. For
the rest, add a scheduled task for the worker's Windows user that starts
Studio when it is not running:

1. Save this as `C:\ClipHub\studio-watchdog.ps1` on the server. It is not
   part of the installer. Set `$exe` to where Studio was installed: the
   installer lets you choose the folder, and the target of the Start menu
   shortcut is the path to copy.

   ```powershell
   $exe = 'C:\Studio\ClipHub Studio.exe'
   if (-not (Get-Process -Name 'ClipHub Studio' -ErrorAction SilentlyContinue)) {
       Start-Process -FilePath $exe
   }
   ```

2. Create the task from a prompt of that user (not elevated), so it runs
   only while the user is logged on, inside the desktop session CS2 needs:

   ```
   schtasks /Create /TN "ClipHub Studio watchdog" /SC MINUTE /MO 5 /F /TR "conhost.exe --headless powershell.exe -NoProfile -ExecutionPolicy Bypass -File C:\ClipHub\studio-watchdog.ps1"
   ```

3. Test it once: close Studio and check that it is back within five minutes
   and that the worker card goes online again.

Two rules for this task. It must never start Studio while Studio is running:
a second start brings the Studio window to the front, and a window that
takes the focus during a capture can ruin it. For the same reason it must
not open a window of its own, which is what `conhost.exe --headless` is for;
watch one run during a capture after you set it up. Before maintenance,
disable the task (`schtasks /Change /TN "ClipHub Studio watchdog" /DISABLE`)
or it will start Studio again while you work.

## Panel states and what to do

| Panel shows | Meaning | Action |
|---|---|---|
| Sin conexión | No heartbeat for 60 seconds: Studio is closed, its backend stopped and the automatic restarts ran out, the PC is off or has no network, or the token was revoked | Look at the server. If Studio shows its stop screen, read the log tail on it and press Reintentar. If Studio is closed, start it. If its log says `the portal rejected the worker token`, see "Rotate the token" |
| Bloqueado: `steam_unavailable` | `steam.exe` is not running in the session | Open Steam and make sure the account is logged in |
| Bloqueado: `cs2_running` | A `cs2.exe` is already running, so a capture cannot start. The same code is used when the capture or render of the previous job has not stopped 90 seconds after it was canceled; the detail then names the local job | Close CS2. If nobody opened it, a capture left it behind: end the process. For a previous job that will not stop, end `zv-recorder.exe`, `zv-editor.exe` and `ffmpeg.exe`, or restart Studio |
| Bloqueado: `disk_full` | Free space is under the floor | The worker already deletes what it no longer needs (see Cleanup and Leftovers of a crash), so look for what it does not own: old manual jobs in Studio, `<data dir>\lab`, files outside the data directory. Restarting Studio runs both cleanups at once |
| Bloqueado: `tools_missing` | Studio did not find the recorder, HLAE, CS2 or the editor at start | Open Studio and read its capture readiness message, fix it, restart Studio |
| Pausado automáticamente: `capture_incompatible` | A CS2 update broke HLAE: either the hook failed (`Error - AfxHookSource2`) or the capture came out black (`black capture suspected`, with the measured bitrate). The job went back to the queue without losing an attempt. A job that causes this three times is failed for good, so one bad demo cannot hold the queue | See "After a CS2 update" |
| Pausado automáticamente: `consecutive_failures` | Three jobs in a row failed on this worker | Read the real cause in Historial (`failureDetail`). Usual suspects: Steam logged out, CS2 waiting for an update, a driver dialog on screen. Fix it, then Reanudar |
| Pausado (admin) | Someone pressed Pausar | Reanudar when ready |
| Online, idle, with jobs in "uploading" and a queue that does not move | Three finished jobs are waiting to upload (`upload_backlog`) | Check the server's uplink. The queue moves again as soon as an upload finishes |

A blocked worker recovers by itself as soon as the cause is gone. A paused
worker waits for Reanudar in the panel. A job that was running when the pause
arrived is finished first.

An automatic pause names the job behind it. The card says "Lo causó el
trabajo" with its title, and the stored reason reads
`<code>: job <id> "<title>": <detail>`. When the same job is named twice,
suspect its demo before the machine. The third time that job is failed
instead of pausing the worker again, and its user is told that the cloud
tried several times and could not record it.

## After a CS2 update

1. Compare the CS2 patch version and the HLAE version on the worker card with
   the latest advancedfx release notes.
2. Restart Studio on the server. It fetches the newest official HLAE at start
   (see `docs/incidents.md`, "HLAE pin and CS2 updates").
3. If advancedfx has not shipped a fix yet, leave the worker paused. Users
   keep their place in the queue and can record on their own PC.
4. Before pressing Reanudar, capture one Short by hand in Studio on the
   server and look at the video. Check frames and bitrate, not only that it
   finished: the black-capture incident passed every exit code with a
   2.4 Mb/s file where a normal capture has about 40 Mb/s
   (`chperf captures` flags it). The worker's own black capture check only
   catches a capture that is black as a whole. It does not replace looking
   at the video.

## Updating Studio on the worker

The worker reports the kill plan schema its Studio parses demos with, and
the portal refuses new jobs from a Studio whose plan uses another one ("Tu
Studio y la nube usan versiones distintas"). So after a release that changes
how demos are cut into plays, update the worker the same day: until then
every user who already updated cannot submit, and after it every user who
has not is told to update. Pause the worker, wait for its jobs to finish,
install the new Studio, start it and press Reanudar.

## Rotate the token

A new token is a new worker for the portal. The jobs the old worker holds
do not move over: an upload that is still pending after the change is
refused (`lease_lost`), its finished videos are discarded, and the job is
captured again once its lease runs out, up to 30 minutes later. So rotate
with nothing in flight:

1. Panel, Workers: press Pausar on the current worker and wait until its card
   shows no job at all, neither running nor uploading.
2. Create a new worker and copy its token.
3. On the server: close Studio, `setx ZV_BRIDGE_TOKEN <new token>`, log off
   and on (or restart), start Studio.
4. Confirm the new worker is online, then revoke the old one.

If a token leaked and has to go now, revoke it first and accept that the
jobs it held are captured again.

A revoked token answers 401 everywhere. The worker then stops claiming,
abandons a capture in progress (the portal requeues it when its lease
expires) and keeps heartbeating once a minute, so its log keeps saying why.
Only a restart with a valid token brings it back.

## Stopping the worker for maintenance

Disable the watchdog task, press Pausar in the panel and wait until the
worker card shows no running job. Uploads may still be in progress; closing
Studio then is safe, they continue at the next start with the same token.
Closing Studio during a capture is also safe, it only costs the time of
that capture. Enable the watchdog task again when you are done.
