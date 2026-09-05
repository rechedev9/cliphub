    // This module extends the existing scheduler. It does not own a second
    // capture loop; all starts, stops and observer checks remain in that loop.
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
            if (fullDemoCapture.hud_profile === "native-clean-spectator") Object.assign(settings, {
                cl_spec_show_bindings: false, cl_drawhud_specvote: false, cl_teamid_overhead_mode: 0,
                cl_drawhud_force_teamid_overhead: -1, hud_showtargetid: false
            });
            Object.assign(settings, fullDemoCrosshairCvars);
            // Snapshot all values before changing the first one.
            for (const name of Object.keys(settings)) fullDemoSaveCvar(fullDemoFindCvar(name));
            fullDemoEvidence("settings_before", {values: Array.from(fullDemoSavedCvars, ([name, entry]) => ({name, value: entry.value}))});
            for (const [name, value] of Object.entries(settings)) fullDemoSetCvar(name, value);
            if (fullDemoCapture.hud_profile === "native-clean-spectator") {
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
        if (fullDemoSettingsRestored) return;
        const failures = [];
        for (const [name, entry] of fullDemoSavedCvars) {
            try { entry.cvar.value = entry.value; if (!fullDemoCvarEquals(entry.cvar.value, entry.value)) failures.push(name); } catch (_) { failures.push(name); }
        }
        fullDemoSettingsRestored = failures.length === 0;
        fullDemoEvidence("settings_restored", {success: fullDemoSettingsRestored, failures});
    }
    const fullDemoEnd = (window, endTick, reason) => {
        fullDemoEvidence("certified_end", {round_id: window.segmentId, end_tick: endTick, reason});
    };
    const failOrTrimFullDemo = (window, tick, reason) => {
        if (fullDemoAllowTailTrim && activeSegment === window.segmentId && tick > window.liveEndTick && tick > window.recordStart) {
            mirv.message(`[zackvideo] record-end-${window.segmentId}: certified tail trim\n`);
            mirv.exec("mirv_streams record end");
            fired[`record-end-${window.segmentId}`] = true;
            activeSegment = null;
            fullDemoEnd(window, tick, reason);
            return;
        }
        failCapture(`pov_contract_failed: ${reason}`);
    };
