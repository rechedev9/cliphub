    // This module extends the existing scheduler. It does not own a second
    // capture loop; all starts, stops and observer checks remain in that loop.
    // The planner reserves two unrecorded seconds after round_start. Acquire
    // during the last second of preroll, when the new pawn can be selected.
    // Freeze demo time, not recording time: no duplicate frames or shifted
    // audio are admitted, and no approved source boundary is rewritten.
    let fullDemoAcquiring = null;
    const prepareFullDemoPOV = (window, tick) => {
        if (activeSegment !== null || fired[`record-end-${window.segmentId}`]) return true;
        if (tick < window.recordStart - fullDemoAcquireLeadTicks) return true;
        const observed = observedSteamId();
        if (fullDemoAcquiring === null && observed === targetSteamId) return true;
        if (tick >= window.recordStart) {
            failCapture(`pov_acquisition_failed: POV not settled before record-start-${window.segmentId} at tick ${tick}`);
            return false;
        }
        if (fullDemoAcquiring === null) {
            fullDemoAcquiring = {segmentId: window.segmentId, frames: 0, stable: 0};
            mirv.message(`[zackvideo] pov-acquire ${window.segmentId} at tick ${tick}\n`);
            mirv.exec("demo_pause");
            lockTarget(window.segmentId);
            lastLockFrame = frame;
        }
        const acquisition = fullDemoAcquiring;
        acquisition.frames++;
        // A bounded number of engine frames, also when ticks are paused.
        // Never reissue spec_player while the correct observer is settling.
        acquisition.stable = observed === targetSteamId ? acquisition.stable + 1 : 0;
        if (acquisition.stable >= 2) {
            mirv.message(`[zackvideo] pov-acquired ${window.segmentId} at tick ${tick}\n`);
            fullDemoAcquiring = null;
            mirv.exec("demo_resume");
            return false; // Let resume take effect before consuming the schedule.
        }
        if (acquisition.frames >= 600) {
            failCapture(`pov_acquisition_failed: observer did not settle for ${window.segmentId} at tick ${tick} after 600 frames`);
            return false;
        }
        if (observed !== targetSteamId && frame - lastLockFrame >= 8) {
            lockTarget(window.segmentId);
            lastLockFrame = frame;
        }
        return false;
    };
    const fullDemoSavedCvars = new Map();
    const fullDemoRequiredCvars = new Map();
    let fullDemoSettingsReady = false;
    let fullDemoSettingsRestored = false;
    const fullDemoCvarEquals = (a, b) => a === b || (typeof a === "number" && typeof b === "number" && Number.isFinite(a) && Number.isFinite(b) && Math.abs(a - b) <= 0.00001);
    const fullDemoEvidence = (kind, value) => {
        mirv.message(`ZV_FULL_DEMO:${fullDemoToken}:${JSON.stringify({kind, ...value})}\n`);
    };
    const fullDemoFindCvar = (name) => {
        const index = AdvancedfxCVar.getIndexFromName(name);
        if (index !== undefined) return new AdvancedfxCVar(index);
        // Hidden cvars remain accessible by index. Do not globally unhide or
        // unlock the user's console to operate a bounded capture profile.
        for (let i = 0; i < 8192; i++) {
            try { const cvar = new AdvancedfxCVar(i); if (cvar.name === name) return cvar; } catch (_) {}
        }
        throw new Error(`required cvar unavailable: ${name}`);
    };
    const fullDemoSaveCvar = (cvar) => {
        if (!fullDemoSavedCvars.has(cvar.name)) fullDemoSavedCvars.set(cvar.name, {cvar, value: cvar.value});
    };
    const fullDemoSetCvar = (name, value) => {
        const cvar = fullDemoFindCvar(name);
        fullDemoSaveCvar(cvar);
        const typed = typeof cvar.value === "boolean" ? Boolean(value) : value;
        cvar.value = typed;
        if (!fullDemoCvarEquals(cvar.value, typed)) throw new Error(`cvar readback differs: ${name}`);
        fullDemoRequiredCvars.set(name, {cvar, value: typed});
    };
    const ensureFullDemoSettings = () => {
        if (fullDemoSettingsReady) return true;
        try {
            if (typeof AdvancedfxCVar === "undefined") throw new Error("HLAE cvar readback API unavailable");
            // Snapshot every archived crosshair setting before importing a code.
            fullDemoSaveCvar(fullDemoFindCvar("host_framerate"));
            for (let i = 0; i < 8192; i++) {
                try { const cv = new AdvancedfxCVar(i); if (cv.name.startsWith("cl_crosshair")) fullDemoSaveCvar(cv); } catch (_) {}
            }
            const settings = {
                voice_modenable: false, snd_voipvolume: 0, tv_listen_voice_indices: 0, tv_listen_voice_indices_h: 0,
                spec_show_xray: 0, spec_autodirector: false, cl_drawhud: true, cl_draw_only_deathnotices: false,
                cl_show_observer_crosshair: fullDemoCapture.crosshair.mode === "observed" ? 2 : 0,
                crosshair: true, cl_demo_predict: 0, cl_trueview_show_status: 0
            };
            const broadcastHUD = ["broadcast-clean", "broadcast-clean-v2"].includes(fullDemoCapture.hud_profile);
            if (fullDemoCapture.hud_profile === "native-clean-spectator" || broadcastHUD) Object.assign(settings, {
                cl_spec_show_bindings: false, cl_drawhud_specvote: false, cl_teamid_overhead_mode: 0,
                cl_drawhud_force_teamid_overhead: -1, hud_showtargetid: false
            });
            // Read back these CS2 cvars like every other capture invariant.
            // The native crosshair, scope, radar and killfeed remain in the
            // capture; player panels are composed later from demo telemetry.
            if (broadcastHUD) Object.assign(settings, {
                cl_draw_only_deathnotices: true, cl_drawhud_force_radar: 1, cl_drawhud_force_deathnotices: 1
            });
            if (fullDemoCapture.hud_profile === "broadcast-clean-v2") Object.assign(settings, {
                cl_hud_radar_background_alpha: .35, cl_hud_radar_map_additive: false, cl_hud_radar_scale: .85,
                cl_hud_color: 0, safezonex: .97, safezoney: .95
            });
            Object.assign(settings, fullDemoCrosshairCvars);
            // Snapshot all values before changing the first one.
            for (const name of Object.keys(settings)) fullDemoSaveCvar(fullDemoFindCvar(name));
            fullDemoEvidence("settings_before", {values: Array.from(fullDemoSavedCvars, ([name, entry]) => ({name, value: entry.value}))});
            for (const [name, value] of Object.entries(settings)) fullDemoSetCvar(name, value);
            if (fullDemoCapture.hud_profile === "native-clean-spectator" || broadcastHUD) {
                for (const panel of ["HudDemoController", "Scoreboard", "HudVote", "HudDeathPanel", "HudSpectatorVignetting", "HudHealthBars", "Status", "HudChat"]) {
                    mirv.exec(`mirv_panorama panelStyle panelId=${panel} opacity=0`);
                }
            }
            fullDemoSettingsReady = true;
            fullDemoEvidence("settings_applied", {values: Array.from(fullDemoRequiredCvars, ([name, entry]) => ({name, value: entry.cvar.value}))});
            return true;
        } catch (err) { failCapture(`pov_contract_failed: ${err}`); return false; }
    };
    const verifyFullDemoSettings = () => {
        for (const [name, entry] of fullDemoRequiredCvars) {
            if (!fullDemoCvarEquals(entry.cvar.value, entry.value)) { failCapture(`pov_contract_failed: ${name} changed during capture`); return false; }
        }
        return true;
    };
    function restoreFullDemoSettings() {
        if (fullDemoAcquiring !== null) {
            fullDemoAcquiring = null;
            mirv.exec("demo_resume");
        }
        if (fullDemoSettingsRestored) return;
        const failures = [];
        for (const [name, entry] of fullDemoSavedCvars) {
            try { entry.cvar.value = entry.value; if (!fullDemoCvarEquals(entry.cvar.value, entry.value)) failures.push(name); } catch (_) { failures.push(name); }
        }
        fullDemoSettingsRestored = failures.length === 0;
        fullDemoEvidence("settings_restored", {success: fullDemoSettingsRestored, failures});
    }
    const finishFullDemoSettings = () => {
        restoreFullDemoSettings();
        if (!fullDemoSettingsRestored) {
            failCapture("pov_contract_failed: Full Demo settings restoration failed");
            return false;
        }
        return true;
    };
    const fullDemoEnd = (window, endTick, reason) => {
        fullDemoEvidence("certified_end", {round_id: window.segmentId, end_tick: endTick, reason});
    };
    let fullDemoLastKnownTick = null;
    const failOrTrimFullDemo = (window, tick, reason) => {
        // This callback precedes rendering. The first unconfirmed POV frame
        // is not recorded, so its tick can promise one more output frame than
        // the native capture contains. Certify the last confirmed POV tick,
        // independently of media length, and never cut inside the live interval.
        const endTick = fullDemoLastKnownTick;
        if (fullDemoAllowTailTrim && activeSegment === window.segmentId && Number.isInteger(endTick) && endTick < tick && endTick > window.liveEndTick && endTick > window.recordStart) {
            mirv.message(`[zackvideo] record-end-${window.segmentId}: certified tail trim\n`);
            mirv.exec("mirv_streams record end");
            fired[`record-end-${window.segmentId}`] = true;
            activeSegment = null;
            fullDemoEnd(window, endTick, reason);
            return;
        }
        failCapture(`pov_contract_failed: ${reason}`);
    };
