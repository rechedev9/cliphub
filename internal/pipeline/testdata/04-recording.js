"use strict";
{
    const id = "zackvideo/generated-recorder";

    if (globalThis[id] !== undefined) {
        globalThis[id].unregister();
        delete globalThis[id];
    }

    const schedule = [
      {
        "tick": 25,
        "key": "stream-setup-01",
        "commands": [
          "cl_demo_predict 0"
        ]
      },
      {
        "tick": 25,
        "key": "stream-setup-02",
        "commands": [
          "cl_trueview_show_status 0"
        ]
      },
      {
        "tick": 25,
        "key": "stream-setup-03",
        "commands": [
          "voice_enable 0"
        ]
      },
      {
        "tick": 25,
        "key": "stream-setup-04",
        "commands": [
          "tv_listen_voice_indices 0"
        ]
      },
      {
        "tick": 25,
        "key": "stream-setup-05",
        "commands": [
          "tv_listen_voice_indices_h 0"
        ]
      },
      {
        "tick": 25,
        "key": "stream-setup-06",
        "commands": [
          "mirv_panorama panelstyle panelId=trueview_row opacity=0"
        ]
      },
      {
        "tick": 25,
        "key": "stream-setup-07",
        "commands": [
          "mirv_streams record name \"$WORK/recording\""
        ]
      },
      {
        "tick": 25,
        "key": "stream-setup-08",
        "commands": [
          "mirv_streams record fps 60"
        ]
      },
      {
        "tick": 25,
        "key": "stream-setup-09",
        "commands": [
          "mirv_streams record screen enabled 1"
        ]
      },
      {
        "tick": 25,
        "key": "stream-setup-10",
        "commands": [
          "mirv_streams settings add ffmpeg zvFfmpegYuv420pCrf18 \"-c:v libx264 -preset fast -crf 18 -pix_fmt yuv420p {QUOTE}{AFX_STREAM_PATH}\\video.mp4{QUOTE}\""
        ]
      },
      {
        "tick": 25,
        "key": "stream-setup-11",
        "commands": [
          "mirv_streams record screen settings zvFfmpegYuv420pCrf18"
        ]
      },
      {
        "tick": 25,
        "key": "stream-setup-12",
        "commands": [
          "spec_show_xray 0"
        ]
      },
      {
        "tick": 25,
        "key": "stream-setup-13",
        "commands": [
          "cl_spec_show_bindings 0"
        ]
      },
      {
        "tick": 25,
        "key": "stream-setup-14",
        "commands": [
          "cl_drawhud 1"
        ]
      },
      {
        "tick": 25,
        "key": "stream-setup-15",
        "commands": [
          "cl_draw_only_deathnotices 0"
        ]
      },
      {
        "tick": 25,
        "key": "stream-setup-16",
        "commands": [
          "cl_show_observer_crosshair 2"
        ]
      },
      {
        "tick": 25,
        "key": "stream-setup-17",
        "commands": [
          "crosshair 1"
        ]
      },
      {
        "tick": 50,
        "key": "timescale-up-preamble",
        "commands": [
          "demo_timescale 8"
        ]
      },
      {
        "tick": 17303,
        "key": "camera-warmup-seg-002",
        "commands": [
          "spec_autodirector 0",
          "spec_mode 2"
        ]
      },
      {
        "tick": 17591,
        "key": "timescale-reset-seg-002",
        "commands": [
          "demo_timescale 1"
        ]
      },
      {
        "tick": 17713,
        "key": "hide-demoui",
        "commands": [
          "demoui"
        ]
      },
      {
        "tick": 17718,
        "key": "camera-lock-seg-002",
        "commands": [
          "spec_autodirector 0",
          "spec_mode 2"
        ]
      },
      {
        "tick": 17719,
        "key": "record-start-seg-002",
        "commands": [
          "mirv_streams record start"
        ]
      },
      {
        "tick": 17751,
        "key": "camera-relock-seg-002",
        "commands": [
          "spec_autodirector 0",
          "spec_mode 2"
        ]
      },
      {
        "tick": 18103,
        "key": "record-end-seg-002",
        "commands": [
          "mirv_streams record end"
        ]
      },
      {
        "tick": 18107,
        "key": "timescale-up-seg-002",
        "commands": [
          "demo_timescale 8"
        ]
      },
      {
        "tick": 22344,
        "key": "camera-warmup-seg-004",
        "commands": [
          "spec_autodirector 0",
          "spec_mode 2"
        ]
      },
      {
        "tick": 22632,
        "key": "timescale-reset-seg-004",
        "commands": [
          "demo_timescale 1"
        ]
      },
      {
        "tick": 22759,
        "key": "camera-lock-seg-004",
        "commands": [
          "spec_autodirector 0",
          "spec_mode 2"
        ]
      },
      {
        "tick": 22760,
        "key": "record-start-seg-004",
        "commands": [
          "mirv_streams record start"
        ]
      },
      {
        "tick": 22792,
        "key": "camera-relock-seg-004",
        "commands": [
          "spec_autodirector 0",
          "spec_mode 2"
        ]
      },
      {
        "tick": 23215,
        "key": "record-end-seg-004",
        "commands": [
          "mirv_streams record end"
        ]
      },
      {
        "tick": 23219,
        "key": "timescale-up-seg-004",
        "commands": [
          "demo_timescale 8"
        ]
      },
      {
        "tick": 61108,
        "key": "camera-warmup-seg-011",
        "commands": [
          "spec_autodirector 0",
          "spec_mode 2"
        ]
      },
      {
        "tick": 61396,
        "key": "timescale-reset-seg-011",
        "commands": [
          "demo_timescale 1"
        ]
      },
      {
        "tick": 61523,
        "key": "camera-lock-seg-011",
        "commands": [
          "spec_autodirector 0",
          "spec_mode 2"
        ]
      },
      {
        "tick": 61524,
        "key": "record-start-seg-011",
        "commands": [
          "mirv_streams record start"
        ]
      },
      {
        "tick": 61556,
        "key": "camera-relock-seg-011",
        "commands": [
          "spec_autodirector 0",
          "spec_mode 2"
        ]
      },
      {
        "tick": 61828,
        "key": "record-end-seg-011",
        "commands": [
          "mirv_streams record end"
        ]
      },
      {
        "tick": 61832,
        "key": "timescale-up-seg-011",
        "commands": [
          "demo_timescale 8"
        ]
      },
      {
        "tick": 61924,
        "key": "timescale-reset-shutdown",
        "commands": [
          "demo_timescale 1"
        ]
      },
      {
        "tick": 61928,
        "key": "shutdown",
        "commands": []
      }
    ];
    const seeks = [
      {
        "after": 50,
        "target": 17271
      },
      {
        "after": 18135,
        "target": 22312
      },
      {
        "after": 23247,
        "target": 61076
      }
    ];
    const captureWindows = [
      {
        "segmentId": "seg-002",
        "lockFrom": 17303,
        "recordStart": 17719,
        "verifyUntil": 17783,
        "recordEnd": 18103
      },
      {
        "segmentId": "seg-004",
        "lockFrom": 22344,
        "recordStart": 22760,
        "verifyUntil": 22895,
        "recordEnd": 23215
      },
      {
        "segmentId": "seg-011",
        "lockFrom": 61108,
        "recordStart": 61524,
        "verifyUntil": 61700,
        "recordEnd": 61828
      }
    ];
    const targetSteamId = "76561198000000100";

    const fired = {};
    let scheduleIndex = 0;
    let armed = false;
    let seekIdx = 0;
    let frame = 0;
    let lastSeekFrame = -999;
    let seekAttempts = 0;
    const maxSeekAttempts = 6000;
    let seekStallFrames = 0;
    let lastSeekTick = -1;
    const maxSeekStallFrames = 1500;
    let lastLockFrame = -999;
    let activeSegment = null;
    let unknownObserverFrames = 0;
    const maxUnknownObserverFrames = 3;
    let driftedObserverFrames = 0;
    const maxDriftedObserverFrames = 12;
    let demoEndedFrames = 0;
    const demoEndedGraceFrames = 30;
    let fatal = false;
    let pendingQuitFrames = 0;
    const softQuitClientFrames = 45;
    const lockAttempts = {};
    const beginSoftQuit = () => {
        // disconnect first; quit a few client frames later so CS2 can leave
        // demo playback without the native Afx/CS2 hard-crash dialog.
        if (pendingQuitFrames > 0) return;
        mirv.exec("voice_enable 1");
        mirv.exec("tv_listen_voice_indices -1");
        mirv.exec("tv_listen_voice_indices_h -1");
        mirv.exec("disconnect");
        pendingQuitFrames = softQuitClientFrames;
    };
    const entityFromHandle = (handle) => {
        if (!mirv.isHandleValid(handle)) return null;
        return mirv.getEntityFromIndex(mirv.getHandleEntryIndex(handle));
    };
    const targetControllerIndex = () => {
        const highest = mirv.getHighestEntityIndex();
        for (let index = 1; index <= highest; index++) {
            const entity = mirv.getEntityFromIndex(index);
            if (entity === null || !entity.isPlayerController()) continue;
            try {
                if (entity.getSteamId().toString() === targetSteamId) return index;
            } catch (_) {
                // Entity handles can be replaced while a seek settles.
            }
        }
        return null;
    };
    const observedSteamId = () => {
        try {
            const localController = mirv.getEntityFromSplitScreenPlayer(0);
            if (localController === null) return null;
            const localPawn = entityFromHandle(localController.getPlayerPawnHandle());
            if (localPawn === null || !localPawn.isPlayerPawn()) return null;
            const observedPawn = entityFromHandle(localPawn.getObserverTargetHandle());
            if (observedPawn === null || !observedPawn.isPlayerPawn()) return null;
            const observedController = entityFromHandle(observedPawn.getPlayerControllerHandle());
            if (observedController === null || !observedController.isPlayerController()) return null;
            return observedController.getSteamId().toString();
        } catch (_) {
            return null;
        }
    };
    const lockTarget = (segmentId) => {
        const targetIndex = targetControllerIndex();
        const attempt = (lockAttempts[segmentId] ?? 0) + 1;
        lockAttempts[segmentId] = attempt;
        if (attempt === 1 || attempt % 60 === 0) {
            mirv.message(`[zackvideo] pov-lock ${segmentId} attempt ${attempt} controller ${targetIndex ?? "missing"}\n`);
        }
        if (targetIndex === null) return;
        mirv.exec("spec_autodirector 0");
        mirv.exec("spec_mode 2");
        mirv.exec(`spec_player ${targetIndex}`);
    };
    const failCapture = (reason) => {
        if (fatal) return;
        fatal = true;
        mirv.warning(`[zackvideo] capture_failed: ${reason}\n`);
        mirv.warning("ZACKVIDEO_CAPTURE_FAILED_OBSERVER_STEAMID_V1:dry-run-no-runtime-attestation\n");
        mirv.exec("echo ZACKVIDEO_CAPTURE_FAILED_OBSERVER_STEAMID_V1:dry-run-no-runtime-attestation");
        if (activeSegment !== null) mirv.exec("mirv_streams record end");
        activeSegment = null;
        beginSoftQuit();
    };
    const run = (item) => {
        if (fired[item.key]) return;
        fired[item.key] = true;
        mirv.message(`[zackvideo] ${item.key}: ${item.commands.join(" | ")}\n`);
        for (const cmd of item.commands) {
            const trimmed = cmd.trim();
            if (trimmed.length > 0) mirv.exec(trimmed);
        }
    };

    mirv.events.clientFrameStageNotify.on(id, (e) => {
        if (e.isBefore) return;
        // Soft-quit countdown must run after fatal/demo-end disconnect so CS2
        // still receives quit even when the capture schedule is frozen.
        if (pendingQuitFrames > 0) {
            pendingQuitFrames--;
            if (pendingQuitFrames === 0) {
                mirv.exec("quit");
            }
            return;
        }
        if (fatal) return;
        // A local empty server can advance its tick before the delayed +playdemo
        // command starts playback. getDemoTick alone therefore cannot prove that a
        // demo is active. Wait for HLAE's engine-backed playback check first.
        if (!mirv.isPlayingDemo()) {
            // A final segment can reach the demo's last tick, ending playback
            // before the scheduled shutdown tick. Ticks stop advancing then, so
            // the schedule can never fire: attest completion (or fail) from here.
            if (!armed || fired["shutdown"]) return;
            demoEndedFrames++;
            if (demoEndedFrames < demoEndedGraceFrames) return;
            const complete = activeSegment === null && captureWindows.every((window) => fired[`record-end-${window.segmentId}`]);
            if (!complete) {
                failCapture("demo playback ended before every protected segment completed");
                return;
            }
            fired["shutdown"] = true;
            mirv.message("ZACKVIDEO_CAPTURE_VERIFIED_OBSERVER_STEAMID_V1:dry-run-no-runtime-attestation\n");
            mirv.exec("echo ZACKVIDEO_CAPTURE_VERIFIED_OBSERVER_STEAMID_V1:dry-run-no-runtime-attestation");
            beginSoftQuit();
            return;
        }
        demoEndedFrames = 0;
        const tick = mirv.getDemoTick();
        if (tick === undefined || tick < 0) return;
        if (!armed) {
            armed = true;
            mirv.message(`[zackvideo] armed at tick ${tick}\n`);
        }
        frame++;
        // Seek to each segment in order, re-issuing demo_gototick until the demo
        // actually reaches the target (a one-shot seek can be dropped if issued a
        // hair too early). Hold the segment's schedule until its seek has landed.
        if (seekIdx < seeks.length) {
            const s = seeks[seekIdx];
            if (tick >= s.after) {
                if (tick + 8 < s.target) {
                    // Stall diagnostic: a healthy demo_gototick can freeze the demo tick
                    // for the duration of its load, and the client FPS is uncapped between
                    // record windows, so a frame-counted budget cannot tell a slow jump
                    // from a wedged seek without a wall clock (HLAE's JS engine has no
                    // reliable one). Never abort here; a truly stuck seek is still bounded
                    // by maxSeekAttempts. Warn once at the budget so the console log
                    // records that a seek showed no tick progress while still retrying.
                    if (tick !== lastSeekTick) {
                        lastSeekTick = tick;
                        seekStallFrames = 0;
                    } else {
                        seekStallFrames++;
                        if (seekStallFrames === maxSeekStallFrames) {
                            mirv.warning(`[zackvideo] seek ${seekIdx + 1} -> ${s.target} showed no tick progress for ${maxSeekStallFrames} frames; still retrying\n`);
                        }
                    }
                    if (frame - lastSeekFrame >= 8) {
                        seekAttempts++;
                        if (seekAttempts > maxSeekAttempts) {
                            failCapture(`seek ${seekIdx + 1} did not reach tick ${s.target} after ${maxSeekAttempts} attempts`);
                            return;
                        }
                        if (seekAttempts === 1 || seekAttempts % 60 === 0) {
                            mirv.message(`[zackvideo] seek ${seekIdx + 1} -> ${s.target} attempt ${seekAttempts} (at ${tick})\n`);
                        }
                        mirv.exec(`demo_gototick ${s.target}`);
                        lastSeekFrame = frame;
                    }
                    return;
                }
                mirv.message(`[zackvideo] seek-landed -> ${s.target} (at ${tick})\n`);
                seekIdx++;
                lastLockFrame = -999; // Entity/observer state may have been replaced by the seek.
                seekAttempts = 0;
                seekStallFrames = 0;
                lastSeekTick = -1;
            }
        }
        const captureWindow = captureWindows.find((window) => tick >= window.lockFrom && tick <= window.verifyUntil);
        if (captureWindow !== undefined) {
            const observed = observedSteamId();
            if (activeSegment === captureWindow.segmentId) {
                if (observed === null) {
                    unknownObserverFrames++;
                    driftedObserverFrames = 0;
                    if (unknownObserverFrames >= maxUnknownObserverFrames) {
                        failCapture(`observer target remained unknown during ${captureWindow.segmentId}`);
                        return;
                    }
                } else if (observed !== targetSteamId) {
                    unknownObserverFrames = 0;
                    driftedObserverFrames++;
                    if (frame - lastLockFrame >= 8) {
                        lockTarget(captureWindow.segmentId);
                        lastLockFrame = frame;
                    }
                    if (driftedObserverFrames >= maxDriftedObserverFrames) {
                        failCapture(`observer target ${observed} drifted from ${targetSteamId} during ${captureWindow.segmentId}`);
                        return;
                    }
                } else {
                    unknownObserverFrames = 0;
                    driftedObserverFrames = 0;
                }
            } else {
                unknownObserverFrames = 0;
                driftedObserverFrames = 0;
            }
            if (activeSegment === null && observed !== targetSteamId && frame - lastLockFrame >= 8) {
                lockTarget(captureWindow.segmentId);
                lastLockFrame = frame;
            }
        } else {
            unknownObserverFrames = 0;
            driftedObserverFrames = 0;
        }
        // The schedule is immutable and sorted by tick/key. Its consumed prefix
        // never runs again, even if demo ticks rewind; fired still owns key deduplication.
        while (scheduleIndex < schedule.length) {
            const item = schedule[scheduleIndex];
            if (tick < item.tick) break;
            scheduleIndex++;
            if (fired[item.key]) continue;
            if (item.key.startsWith("record-start-")) {
                const window = captureWindows.find((candidate) => `record-start-${candidate.segmentId}` === item.key);
                if (window === undefined || activeSegment !== null) {
                    failCapture(`capture start ${item.key} overlaps active segment ${activeSegment ?? "none"}`);
                    return;
                }
                const observed = observedSteamId();
                if (observed !== targetSteamId) {
                    failCapture(`observer target ${observed ?? "unknown"} does not match ${targetSteamId} before ${item.key}`);
                    return;
                }
                run(item);
                activeSegment = window.segmentId;
                continue;
            }
            if (item.key.startsWith("record-end-")) {
                const window = captureWindows.find((candidate) => `record-end-${candidate.segmentId}` === item.key);
                if (window === undefined || activeSegment !== window.segmentId) {
                    failCapture(`capture end ${item.key} does not match active segment ${activeSegment ?? "none"}`);
                    return;
                }
                run(item);
                activeSegment = null;
                continue;
            }
            if (item.key === "shutdown") {
                const complete = activeSegment === null && captureWindows.every((window) => fired[`record-end-${window.segmentId}`]);
                if (!complete) {
                    failCapture("capture reached shutdown before every protected segment completed");
                    return;
                }
                mirv.message("ZACKVIDEO_CAPTURE_VERIFIED_OBSERVER_STEAMID_V1:dry-run-no-runtime-attestation\n");
                mirv.exec("echo ZACKVIDEO_CAPTURE_VERIFIED_OBSERVER_STEAMID_V1:dry-run-no-runtime-attestation");
                run(item);
                beginSoftQuit();
                continue;
            }
            run(item);
        }
    });

    globalThis[id] = {
        unregister: () => mirv.events.clientFrameStageNotify.off(id)
    };
}
