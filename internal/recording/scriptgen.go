package recording

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

type scheduledCommand struct {
	Tick     int      `json:"tick"`
	Key      string   `json:"key"`
	Commands []string `json:"commands"`
}

// seekStep tells the runtime to seek the demo to Target once playback has passed
// After. The runtime re-issues demo_gototick until the demo actually reaches
// Target, because a one-shot seek issued a hair too early is silently dropped
// ("Not currently playing back a demo").
type seekStep struct {
	After  int `json:"after"`
	Target int `json:"target"`
}

// captureWindow is the interval where the generated runtime must prove that
// the local spectator observes the selected SteamID. Recording never starts
// without that proof, and any drift before the final selected event fails the
// run instead of publishing a structurally valid clip from another POV.
type captureWindow struct {
	SegmentID   string `json:"segmentId"`
	LockFrom    int    `json:"lockFrom"`
	RecordStart int    `json:"recordStart"`
	VerifyUntil int    `json:"verifyUntil"`
	RecordEnd   int    `json:"recordEnd"`
}

const (
	minimumDemoSeekGapSeconds = 30
	maxUnknownObserverFrames  = 3
	demoEndedGraceFrames      = 30
	// softQuitClientFrames delays quit after disconnect. disconnect ends demo
	// playback so ticks stop; quit must be driven by client frames, not the
	// tick schedule. Same-frame disconnect+quit hard-crashes CS2/AfxHookSource2.
	softQuitClientFrames = 45
)

// GenerateHLAEJavaScript renders a self-contained HLAE 2.x mirv-script file.
func GenerateHLAEJavaScript(plan RecordingPlan) (string, error) {
	return generateHLAEJavaScript(plan, "dry-run-no-runtime-attestation")
}

// GenerateHLAEJavaScriptWithAttestation binds runtime completion markers to
// one recorder invocation so demo-controlled console text cannot spoof success.
func GenerateHLAEJavaScriptWithAttestation(plan RecordingPlan, token string) (string, error) {
	if strings.TrimSpace(token) == "" || strings.ContainsAny(token, "\r\n") {
		return "", fmt.Errorf("capture attestation token must be non-empty and single-line")
	}
	return generateHLAEJavaScript(plan, token)
}

func generateHLAEJavaScript(plan RecordingPlan, attestationToken string) (string, error) {
	plan.Stream = normalizeStreamConfig(plan.Stream)
	if err := plan.Validate(); err != nil {
		return "", err
	}

	schedule, seeks, windows := buildRuntimeSchedule(plan)
	sort.SliceStable(schedule, func(i, j int) bool {
		if schedule[i].Tick == schedule[j].Tick {
			return schedule[i].Key < schedule[j].Key
		}
		return schedule[i].Tick < schedule[j].Tick
	})

	scheduleJSON, err := json.MarshalIndent(schedule, "    ", "  ")
	if err != nil {
		return "", err
	}
	seeksJSON, err := json.MarshalIndent(seeks, "    ", "  ")
	if err != nil {
		return "", err
	}
	windowsJSON, err := json.MarshalIndent(windows, "    ", "  ")
	if err != nil {
		return "", err
	}
	targetSteamIDJSON, err := json.Marshal(plan.TargetSteamID64)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("\"use strict\";\n")
	sb.WriteString("{\n")
	sb.WriteString("    const id = \"zackvideo/generated-recorder\";\n\n")
	sb.WriteString("    if (globalThis[id] !== undefined) {\n")
	sb.WriteString("        globalThis[id].unregister();\n")
	sb.WriteString("        delete globalThis[id];\n")
	sb.WriteString("    }\n\n")
	sb.WriteString("    const schedule = ")
	sb.Write(scheduleJSON)
	sb.WriteString(";\n")
	sb.WriteString("    const seeks = ")
	sb.Write(seeksJSON)
	sb.WriteString(";\n")
	sb.WriteString("    const captureWindows = ")
	sb.Write(windowsJSON)
	sb.WriteString(";\n")
	sb.WriteString("    const targetSteamId = ")
	sb.Write(targetSteamIDJSON)
	sb.WriteString(";\n\n")
	sb.WriteString("    const fired = {};\n")
	sb.WriteString("    let armed = false;\n")
	sb.WriteString("    let seekIdx = 0;\n")
	sb.WriteString("    let frame = 0;\n")
	sb.WriteString("    let lastSeekFrame = -999;\n")
	sb.WriteString("    let seekAttempts = 0;\n")
	sb.WriteString("    const maxSeekAttempts = 6000;\n")
	sb.WriteString("    let seekStallFrames = 0;\n")
	sb.WriteString("    let lastSeekTick = -1;\n")
	sb.WriteString("    const maxSeekStallFrames = 1500;\n")
	sb.WriteString("    let lastLockFrame = -999;\n")
	sb.WriteString("    let activeSegment = null;\n")
	sb.WriteString("    let unknownObserverFrames = 0;\n")
	sb.WriteString(fmt.Sprintf("    const maxUnknownObserverFrames = %d;\n", maxUnknownObserverFrames))
	sb.WriteString("    let demoEndedFrames = 0;\n")
	sb.WriteString(fmt.Sprintf("    const demoEndedGraceFrames = %d;\n", demoEndedGraceFrames))
	sb.WriteString("    let fatal = false;\n")
	sb.WriteString("    let pendingQuitFrames = 0;\n")
	sb.WriteString(fmt.Sprintf("    const softQuitClientFrames = %d;\n", softQuitClientFrames))
	sb.WriteString("    const lockAttempts = {};\n")
	sb.WriteString("    const beginSoftQuit = () => {\n")
	sb.WriteString("        // disconnect first; quit a few client frames later so CS2 can leave\n")
	sb.WriteString("        // demo playback without the native Afx/CS2 hard-crash dialog.\n")
	sb.WriteString("        if (pendingQuitFrames > 0) return;\n")
	for _, cmd := range voiceRestoreCommands() {
		sb.WriteString(fmt.Sprintf("        mirv.exec(%q);\n", cmd))
	}
	sb.WriteString("        mirv.exec(\"disconnect\");\n")
	sb.WriteString("        pendingQuitFrames = softQuitClientFrames;\n")
	sb.WriteString("    };\n")
	sb.WriteString("    const entityFromHandle = (handle) => {\n")
	sb.WriteString("        if (!mirv.isHandleValid(handle)) return null;\n")
	sb.WriteString("        return mirv.getEntityFromIndex(mirv.getHandleEntryIndex(handle));\n")
	sb.WriteString("    };\n")
	sb.WriteString("    const targetControllerIndex = () => {\n")
	sb.WriteString("        const highest = mirv.getHighestEntityIndex();\n")
	sb.WriteString("        for (let index = 1; index <= highest; index++) {\n")
	sb.WriteString("            const entity = mirv.getEntityFromIndex(index);\n")
	sb.WriteString("            if (entity === null || !entity.isPlayerController()) continue;\n")
	sb.WriteString("            try {\n")
	sb.WriteString("                if (entity.getSteamId().toString() === targetSteamId) return index;\n")
	sb.WriteString("            } catch (_) {\n")
	sb.WriteString("                // Entity handles can be replaced while a seek settles.\n")
	sb.WriteString("            }\n")
	sb.WriteString("        }\n")
	sb.WriteString("        return null;\n")
	sb.WriteString("    };\n")
	sb.WriteString("    const observedSteamId = () => {\n")
	sb.WriteString("        try {\n")
	sb.WriteString("            const localController = mirv.getEntityFromSplitScreenPlayer(0);\n")
	sb.WriteString("            if (localController === null) return null;\n")
	sb.WriteString("            const localPawn = entityFromHandle(localController.getPlayerPawnHandle());\n")
	sb.WriteString("            if (localPawn === null || !localPawn.isPlayerPawn()) return null;\n")
	sb.WriteString("            const observedPawn = entityFromHandle(localPawn.getObserverTargetHandle());\n")
	sb.WriteString("            if (observedPawn === null || !observedPawn.isPlayerPawn()) return null;\n")
	sb.WriteString("            const observedController = entityFromHandle(observedPawn.getPlayerControllerHandle());\n")
	sb.WriteString("            if (observedController === null || !observedController.isPlayerController()) return null;\n")
	sb.WriteString("            return observedController.getSteamId().toString();\n")
	sb.WriteString("        } catch (_) {\n")
	sb.WriteString("            return null;\n")
	sb.WriteString("        }\n")
	sb.WriteString("    };\n")
	sb.WriteString("    const lockTarget = (segmentId) => {\n")
	sb.WriteString("        const targetIndex = targetControllerIndex();\n")
	sb.WriteString("        const attempt = (lockAttempts[segmentId] ?? 0) + 1;\n")
	sb.WriteString("        lockAttempts[segmentId] = attempt;\n")
	sb.WriteString("        if (attempt === 1 || attempt % 60 === 0) {\n")
	sb.WriteString("            mirv.message(`[zackvideo] pov-lock ${segmentId} attempt ${attempt} controller ${targetIndex ?? \"missing\"}\\n`);\n")
	sb.WriteString("        }\n")
	sb.WriteString("        if (targetIndex === null) return;\n")
	sb.WriteString("        mirv.exec(\"spec_autodirector 0\");\n")
	sb.WriteString("        mirv.exec(\"spec_mode 2\");\n")
	sb.WriteString("        mirv.exec(`spec_player ${targetIndex}`);\n")
	sb.WriteString("    };\n")
	sb.WriteString("    const failCapture = (reason) => {\n")
	sb.WriteString("        if (fatal) return;\n")
	sb.WriteString("        fatal = true;\n")
	sb.WriteString("        mirv.warning(`[zackvideo] capture_failed: ${reason}\\n`);\n")
	failedAttestation := CaptureFailedAttestation(attestationToken)
	verifiedAttestation := CaptureVerifiedAttestation(attestationToken)
	sb.WriteString(fmt.Sprintf("        mirv.warning(%q);\n", failedAttestation+"\\n"))
	sb.WriteString(fmt.Sprintf("        mirv.exec(%q);\n", "echo "+failedAttestation))
	sb.WriteString("        if (activeSegment !== null) mirv.exec(\"mirv_streams record end\");\n")
	sb.WriteString("        activeSegment = null;\n")
	sb.WriteString("        beginSoftQuit();\n")
	sb.WriteString("    };\n")
	sb.WriteString("    const run = (item) => {\n")
	sb.WriteString("        if (fired[item.key]) return;\n")
	sb.WriteString("        fired[item.key] = true;\n")
	sb.WriteString("        mirv.message(`[zackvideo] ${item.key}: ${item.commands.join(\" | \")}\\n`);\n")
	sb.WriteString("        for (const cmd of item.commands) {\n")
	sb.WriteString("            const trimmed = cmd.trim();\n")
	sb.WriteString("            if (trimmed.length > 0) mirv.exec(trimmed);\n")
	sb.WriteString("        }\n")
	sb.WriteString("    };\n\n")
	sb.WriteString("    mirv.events.clientFrameStageNotify.on(id, (e) => {\n")
	sb.WriteString("        if (e.isBefore) return;\n")
	sb.WriteString("        // Soft-quit countdown must run after fatal/demo-end disconnect so CS2\n")
	sb.WriteString("        // still receives quit even when the capture schedule is frozen.\n")
	sb.WriteString("        if (pendingQuitFrames > 0) {\n")
	sb.WriteString("            pendingQuitFrames--;\n")
	sb.WriteString("            if (pendingQuitFrames === 0) {\n")
	sb.WriteString("                mirv.exec(\"quit\");\n")
	sb.WriteString("            }\n")
	sb.WriteString("            return;\n")
	sb.WriteString("        }\n")
	sb.WriteString("        if (fatal) return;\n")
	sb.WriteString("        // A local empty server can advance its tick before the delayed +playdemo\n")
	sb.WriteString("        // command starts playback. getDemoTick alone therefore cannot prove that a\n")
	sb.WriteString("        // demo is active. Wait for HLAE's engine-backed playback check first.\n")
	sb.WriteString("        if (!mirv.isPlayingDemo()) {\n")
	sb.WriteString("            // A final segment can reach the demo's last tick, ending playback\n")
	sb.WriteString("            // before the scheduled shutdown tick. Ticks stop advancing then, so\n")
	sb.WriteString("            // the schedule can never fire: attest completion (or fail) from here.\n")
	sb.WriteString("            if (!armed || fired[\"shutdown\"]) return;\n")
	sb.WriteString("            demoEndedFrames++;\n")
	sb.WriteString("            if (demoEndedFrames < demoEndedGraceFrames) return;\n")
	sb.WriteString("            const complete = activeSegment === null && captureWindows.every((window) => fired[`record-end-${window.segmentId}`]);\n")
	sb.WriteString("            if (!complete) {\n")
	sb.WriteString("                failCapture(\"demo playback ended before every protected segment completed\");\n")
	sb.WriteString("                return;\n")
	sb.WriteString("            }\n")
	sb.WriteString("            fired[\"shutdown\"] = true;\n")
	sb.WriteString(fmt.Sprintf("            mirv.message(%q);\n", verifiedAttestation+"\\n"))
	sb.WriteString(fmt.Sprintf("            mirv.exec(%q);\n", "echo "+verifiedAttestation))
	sb.WriteString("            beginSoftQuit();\n")
	sb.WriteString("            return;\n")
	sb.WriteString("        }\n")
	sb.WriteString("        demoEndedFrames = 0;\n")
	sb.WriteString("        const tick = mirv.getDemoTick();\n")
	sb.WriteString("        if (tick === undefined || tick < 0) return;\n")
	sb.WriteString("        if (!armed) {\n")
	sb.WriteString("            armed = true;\n")
	sb.WriteString("            mirv.message(`[zackvideo] armed at tick ${tick}\\n`);\n")
	sb.WriteString("        }\n")
	sb.WriteString("        frame++;\n")
	sb.WriteString("        // Seek to each segment in order, re-issuing demo_gototick until the demo\n")
	sb.WriteString("        // actually reaches the target (a one-shot seek can be dropped if issued a\n")
	sb.WriteString("        // hair too early). Hold the segment's schedule until its seek has landed.\n")
	sb.WriteString("        if (seekIdx < seeks.length) {\n")
	sb.WriteString("            const s = seeks[seekIdx];\n")
	sb.WriteString("            if (tick >= s.after) {\n")
	sb.WriteString("                if (tick + 8 < s.target) {\n")
	sb.WriteString("                    // Stall diagnostic: a healthy demo_gototick can freeze the demo tick\n")
	sb.WriteString("                    // for the duration of its load, and the client FPS is uncapped between\n")
	sb.WriteString("                    // record windows, so a frame-counted budget cannot tell a slow jump\n")
	sb.WriteString("                    // from a wedged seek without a wall clock (HLAE's JS engine has no\n")
	sb.WriteString("                    // reliable one). Never abort here; a truly stuck seek is still bounded\n")
	sb.WriteString("                    // by maxSeekAttempts. Warn once at the budget so the console log\n")
	sb.WriteString("                    // records that a seek showed no tick progress while still retrying.\n")
	sb.WriteString("                    if (tick !== lastSeekTick) {\n")
	sb.WriteString("                        lastSeekTick = tick;\n")
	sb.WriteString("                        seekStallFrames = 0;\n")
	sb.WriteString("                    } else {\n")
	sb.WriteString("                        seekStallFrames++;\n")
	sb.WriteString("                        if (seekStallFrames === maxSeekStallFrames) {\n")
	sb.WriteString("                            mirv.warning(`[zackvideo] seek ${seekIdx + 1} -> ${s.target} showed no tick progress for ${maxSeekStallFrames} frames; still retrying\\n`);\n")
	sb.WriteString("                        }\n")
	sb.WriteString("                    }\n")
	sb.WriteString("                    if (frame - lastSeekFrame >= 8) {\n")
	sb.WriteString("                        seekAttempts++;\n")
	sb.WriteString("                        if (seekAttempts > maxSeekAttempts) {\n")
	sb.WriteString("                            failCapture(`seek ${seekIdx + 1} did not reach tick ${s.target} after ${maxSeekAttempts} attempts`);\n")
	sb.WriteString("                            return;\n")
	sb.WriteString("                        }\n")
	sb.WriteString("                        if (seekAttempts === 1 || seekAttempts % 60 === 0) {\n")
	sb.WriteString("                            mirv.message(`[zackvideo] seek ${seekIdx + 1} -> ${s.target} attempt ${seekAttempts} (at ${tick})\\n`);\n")
	sb.WriteString("                        }\n")
	sb.WriteString("                        mirv.exec(`demo_gototick ${s.target}`);\n")
	sb.WriteString("                        lastSeekFrame = frame;\n")
	sb.WriteString("                    }\n")
	sb.WriteString("                    return;\n")
	sb.WriteString("                }\n")
	sb.WriteString("                mirv.message(`[zackvideo] seek-landed -> ${s.target} (at ${tick})\\n`);\n")
	sb.WriteString("                seekIdx++;\n")
	sb.WriteString("                seekAttempts = 0;\n")
	sb.WriteString("                seekStallFrames = 0;\n")
	sb.WriteString("                lastSeekTick = -1;\n")
	sb.WriteString("            }\n")
	sb.WriteString("        }\n")
	sb.WriteString("        const captureWindow = captureWindows.find((window) => tick >= window.lockFrom && tick <= window.verifyUntil);\n")
	sb.WriteString("        if (captureWindow !== undefined) {\n")
	sb.WriteString("            const observed = observedSteamId();\n")
	sb.WriteString("            if (activeSegment === captureWindow.segmentId) {\n")
	sb.WriteString("                if (observed === null) {\n")
	sb.WriteString("                    unknownObserverFrames++;\n")
	sb.WriteString("                    if (unknownObserverFrames >= maxUnknownObserverFrames) {\n")
	sb.WriteString("                        failCapture(`observer target remained unknown during ${captureWindow.segmentId}`);\n")
	sb.WriteString("                        return;\n")
	sb.WriteString("                    }\n")
	sb.WriteString("                } else if (observed !== targetSteamId) {\n")
	sb.WriteString("                    unknownObserverFrames = 0;\n")
	sb.WriteString("                    failCapture(`observer target ${observed} drifted from ${targetSteamId} during ${captureWindow.segmentId}`);\n")
	sb.WriteString("                    return;\n")
	sb.WriteString("                } else {\n")
	sb.WriteString("                    unknownObserverFrames = 0;\n")
	sb.WriteString("                }\n")
	sb.WriteString("            } else {\n")
	sb.WriteString("                unknownObserverFrames = 0;\n")
	sb.WriteString("            }\n")
	sb.WriteString("            if (activeSegment === null && observed !== targetSteamId && frame - lastLockFrame >= 8) {\n")
	sb.WriteString("                lockTarget(captureWindow.segmentId);\n")
	sb.WriteString("                lastLockFrame = frame;\n")
	sb.WriteString("            }\n")
	sb.WriteString("        } else {\n")
	sb.WriteString("            unknownObserverFrames = 0;\n")
	sb.WriteString("        }\n")
	sb.WriteString("        for (const item of schedule) {\n")
	sb.WriteString("            if (fired[item.key] || tick < item.tick) continue;\n")
	sb.WriteString("            if (item.key.startsWith(\"record-start-\")) {\n")
	sb.WriteString("                const window = captureWindows.find((candidate) => `record-start-${candidate.segmentId}` === item.key);\n")
	sb.WriteString("                if (window === undefined || activeSegment !== null) {\n")
	sb.WriteString("                    failCapture(`capture start ${item.key} overlaps active segment ${activeSegment ?? \"none\"}`);\n")
	sb.WriteString("                    return;\n")
	sb.WriteString("                }\n")
	sb.WriteString("                const observed = observedSteamId();\n")
	sb.WriteString("                if (observed !== targetSteamId) {\n")
	sb.WriteString("                    failCapture(`observer target ${observed ?? \"unknown\"} does not match ${targetSteamId} before ${item.key}`);\n")
	sb.WriteString("                    return;\n")
	sb.WriteString("                }\n")
	sb.WriteString("                run(item);\n")
	sb.WriteString("                activeSegment = window.segmentId;\n")
	sb.WriteString("                continue;\n")
	sb.WriteString("            }\n")
	sb.WriteString("            if (item.key.startsWith(\"record-end-\")) {\n")
	sb.WriteString("                const window = captureWindows.find((candidate) => `record-end-${candidate.segmentId}` === item.key);\n")
	sb.WriteString("                if (window === undefined || activeSegment !== window.segmentId) {\n")
	sb.WriteString("                    failCapture(`capture end ${item.key} does not match active segment ${activeSegment ?? \"none\"}`);\n")
	sb.WriteString("                    return;\n")
	sb.WriteString("                }\n")
	sb.WriteString("                run(item);\n")
	sb.WriteString("                activeSegment = null;\n")
	sb.WriteString("                continue;\n")
	sb.WriteString("            }\n")
	sb.WriteString("            if (item.key === \"shutdown\") {\n")
	sb.WriteString("                const complete = activeSegment === null && captureWindows.every((window) => fired[`record-end-${window.segmentId}`]);\n")
	sb.WriteString("                if (!complete) {\n")
	sb.WriteString("                    failCapture(\"capture reached shutdown before every protected segment completed\");\n")
	sb.WriteString("                    return;\n")
	sb.WriteString("                }\n")
	sb.WriteString(fmt.Sprintf("                mirv.message(%q);\n", verifiedAttestation+"\\n"))
	sb.WriteString(fmt.Sprintf("                mirv.exec(%q);\n", "echo "+verifiedAttestation))
	sb.WriteString("                run(item);\n")
	sb.WriteString("                beginSoftQuit();\n")
	sb.WriteString("                continue;\n")
	sb.WriteString("            }\n")
	sb.WriteString("            run(item);\n")
	sb.WriteString("        }\n")
	sb.WriteString("    });\n\n")
	sb.WriteString("    globalThis[id] = {\n")
	sb.WriteString("        unregister: () => mirv.events.clientFrameStageNotify.off(id)\n")
	sb.WriteString("    };\n")
	sb.WriteString("}\n")
	return sb.String(), nil
}

func buildRuntimeSchedule(plan RecordingPlan) ([]scheduledCommand, []seekStep, []captureWindow) {
	commands := []scheduledCommand{}
	seeks := []seekStep{}
	windows := []captureWindow{}
	camera := []string{"spec_autodirector 0", "spec_mode 2"}
	setupTick := 25
	for i, cmd := range streamSetupCommands(plan) {
		commands = append(commands, scheduledCommand{
			Tick:     setupTick,
			Key:      fmt.Sprintf("stream-setup-%02d", i+1),
			Commands: []string{cmd},
		})
	}

	for i, s := range plan.Segments {
		seekTick := 50
		if i > 0 {
			seekTick = EffectiveRecordEndTick(plan.Segments[i-1], plan) + 32
		}
		recordStart := EffectiveRecordStartTick(s, plan.Tickrate)
		recordEnd := EffectiveRecordEndTick(s, plan)
		leadTicks := plan.Tickrate * 5
		seekTarget := max(1, s.TickStart-leadTicks)
		cameraWarmupTick := seekTarget + max(1, plan.Tickrate/2)
		cameraLockTick := recordStart - 1
		if cameraWarmupTick >= recordStart {
			cameraWarmupTick = recordStart - max(2, plan.Tickrate/2)
		}
		// Verification runs before scheduled commands at a tick. Stop one tick
		// before record-end so a legitimate entity teardown at the boundary does
		// not fail an otherwise complete capture.
		verifyUntil := max(recordStart, recordEnd-1)
		if lastKill := lastKillTick(s); lastKill > 0 {
			// Once the final selected kill has happened, a spectator change can
			// be CS2's legitimate death cam during post-roll. Keep recording the
			// full segment, but stop treating that camera change as POV drift.
			verifyUntil = min(verifyUntil, max(recordStart, lastKill))
		}
		windows = append(windows, captureWindow{
			SegmentID:   s.ID,
			LockFrom:    max(seekTarget+1, cameraWarmupTick),
			RecordStart: recordStart,
			VerifyUntil: verifyUntil,
			RecordEnd:   recordEnd,
		})

		// Short demo_gototick jumps can corrupt CS2's demo netchannel. Let nearby
		// segments advance naturally and reserve seeking for gaps worth skipping.
		if seekTarget-seekTick >= plan.Tickrate*minimumDemoSeekGapSeconds {
			// The seek is driven by the runtime (re-issued until it lands), not a
			// one-shot scheduled command, so it survives being attempted too early.
			seeks = append(seeks, seekStep{After: seekTick, Target: seekTarget})
		}

		commands = append(commands,
			scheduledCommand{Tick: max(seekTarget+1, cameraWarmupTick), Key: "camera-warmup-" + s.ID, Commands: camera},
			scheduledCommand{Tick: max(seekTarget+5, cameraLockTick), Key: "camera-lock-" + s.ID, Commands: camera},
			scheduledCommand{Tick: recordStart + max(1, plan.Tickrate/2), Key: "camera-relock-" + s.ID, Commands: camera},
		)
		if i == 0 {
			commands = append(commands,
				scheduledCommand{Tick: max(seekTarget, recordStart-6), Key: "hide-demoui", Commands: []string{"demoui"}},
			)
		}

		commands = append(commands,
			scheduledCommand{Tick: recordStart, Key: "record-start-" + s.ID, Commands: []string{"mirv_streams record start"}},
			scheduledCommand{Tick: recordEnd, Key: "record-end-" + s.ID, Commands: []string{"mirv_streams record end"}},
		)
	}

	lastEnd := EffectiveRecordEndTick(plan.Segments[len(plan.Segments)-1], plan)
	pad := plan.Runtime.QuitTickPad
	if pad <= 0 {
		pad = 200
	}
	shutdownTick := lastEnd + max(8, pad/2)
	commands = append(commands, playbackTimescaleCommands(plan, windows, shutdownTick)...)
	for i, cmd := range hudCleanupCommands(plan.Stream) {
		commands = append(commands, scheduledCommand{
			Tick:     shutdownTick - 4,
			Key:      fmt.Sprintf("hud-cleanup-%02d", i+1),
			Commands: []string{cmd},
		})
	}
	// Shutdown only disconnects via beginSoftQuit in the runtime; quit is
	// delayed by softQuitClientFrames because disconnect ends demo playback and
	// ticks stop advancing (a scheduled quit after disconnect would never fire).
	commands = append(commands,
		scheduledCommand{Tick: shutdownTick, Key: "shutdown", Commands: []string{}},
	)
	return commands, seeks, windows
}

func playbackTimescaleEnabled(plan RecordingPlan) bool {
	return plan.Runtime.Normalized().PlaybackTimescale != 1
}

func playbackTimescaleCommand(scale float64) string {
	return "demo_timescale " + formatFloat(scale)
}

// playbackTimescaleCommands speed unrecorded gaps and restore 1x before every
// record window. Commands are never scheduled inside [recordStart, recordEnd].
func playbackTimescaleCommands(plan RecordingPlan, windows []captureWindow, shutdownTick int) []scheduledCommand {
	if !playbackTimescaleEnabled(plan) || len(windows) == 0 {
		return nil
	}
	normalized := plan.Runtime.Normalized()
	speed := playbackTimescaleCommand(normalized.PlaybackTimescale)
	reset := playbackTimescaleCommand(1)
	commands := []scheduledCommand{}
	resetTicks := make([]int, len(windows))
	settleSeconds := normalized.PlaybackSettleSeconds
	if settleSeconds <= 0 {
		settleSeconds = DefaultPlaybackSettleSeconds
	}
	settleTicks := int(math.Round(settleSeconds * float64(plan.Tickrate)))
	if settleTicks < 1 {
		settleTicks = 1
	}
	for i, window := range windows {
		resetTick := window.RecordStart - settleTicks
		if i > 0 {
			minReset := windows[i-1].RecordEnd + 1
			if resetTick < minReset {
				resetTick = minReset
			}
		}
		if resetTick < 1 {
			resetTick = 1
		}
		if resetTick >= window.RecordStart {
			resetTicks[i] = 0
			continue
		}
		resetTicks[i] = resetTick
		commands = append(commands, scheduledCommand{
			Tick:     resetTick,
			Key:      "timescale-reset-" + window.SegmentID,
			Commands: []string{reset},
		})
	}
	if firstReset := resetTicks[0]; firstReset > 50 {
		commands = append(commands, scheduledCommand{
			Tick:     50,
			Key:      "timescale-up-preamble",
			Commands: []string{speed},
		})
	}
	for i, window := range windows {
		upTick := window.RecordEnd + 4
		nextBound := shutdownTick
		if i+1 < len(windows) {
			nextBound = windows[i+1].RecordStart
			if resetTicks[i+1] > 0 && resetTicks[i+1] < nextBound {
				nextBound = resetTicks[i+1]
			}
		}
		if upTick >= nextBound {
			continue
		}
		commands = append(commands, scheduledCommand{
			Tick:     upTick,
			Key:      "timescale-up-" + window.SegmentID,
			Commands: []string{speed},
		})
	}
	lastEnd := windows[len(windows)-1].RecordEnd
	if shutdownReset := shutdownTick - 4; shutdownReset > lastEnd {
		commands = append(commands, scheduledCommand{
			Tick:     shutdownReset,
			Key:      "timescale-reset-shutdown",
			Commands: []string{reset},
		})
	}
	return commands
}

func quoteConsoleArg(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

// EffectiveRecordStartTick returns the actual tick where HLAE starts recording
// a segment after applying recorder camera-settle timing.
func EffectiveRecordStartTick(segment RecordingSegment, tickrate int) int {
	if tickrate <= 0 || len(segment.Kills) == 0 {
		return segment.TickStart
	}
	firstKill := firstKillTick(segment)
	if firstKill <= 0 {
		return segment.TickStart
	}
	latestStart := firstKill - tickrate
	if latestStart <= segment.TickStart {
		return segment.TickStart
	}
	stabilizedStart := segment.TickStart + tickrate*2
	if stabilizedStart > latestStart {
		return latestStart
	}
	return stabilizedStart
}

// EffectiveRecordEndTick returns the tick where HLAE stops recording a segment.
// Middle-of-demo segments keep their planned end. Only ends that already
// approach demo EOF are pulled back so the glitchy last frames are not
// captured and record-end can still fire while playback is advancing.
//
// Last-event logic matches the parser clamp: kills and utility throw/pop ticks
// all count. When the last event sits inside the EOF safety margin, keep a
// short clean tail (and hard headroom before absolute duration) instead of
// soft-capping before that event.
func EffectiveRecordEndTick(segment RecordingSegment, plan RecordingPlan) int {
	end := segment.TickEnd
	if plan.DemoDurationTicks <= 1 || end <= 0 {
		return end
	}
	tickrate := plan.Tickrate
	if tickrate <= 0 {
		tickrate = 64
	}
	// Match parser demoEndSafetyMarginSeconds (2s) and short-tail (1s).
	const (
		safetySeconds    = 2
		shortTailSeconds = 1
	)
	margin := safetySeconds * tickrate
	if maxMargin := plan.DemoDurationTicks / 4; maxMargin > 0 && margin > maxMargin {
		margin = maxMargin
	}
	if margin < 1 {
		margin = 1
	}
	softCap := plan.DemoDurationTicks - margin
	if softCap < 1 {
		softCap = plan.DemoDurationTicks
	}
	// Segment is safely before the EOF zone: keep editorial post-roll as planned.
	if end <= softCap {
		return end
	}
	end = softCap
	lastEvent := lastSegmentEventTick(segment)
	shortTail := shortTailSeconds * tickrate
	if shortTail < 1 {
		shortTail = 1
	}
	// Soft margin may sit before the last selected kill/utility event. Keep a
	// short clean tail after that event rather than ending before it.
	if lastEvent > 0 && end < lastEvent {
		end = lastEvent + shortTail
	}
	if plan.DemoDurationTicks > 1 && end >= plan.DemoDurationTicks {
		if lastEvent > 0 && lastEvent < plan.DemoDurationTicks-1 {
			end = plan.DemoDurationTicks - 1
		} else if lastEvent <= 0 {
			end = plan.DemoDurationTicks - 1
		} else {
			end = plan.DemoDurationTicks
		}
	}
	if end > plan.DemoDurationTicks {
		end = plan.DemoDurationTicks
	}
	if end < segment.TickStart {
		return segment.TickEnd
	}
	// Never stop before the last selected event; fall back to the plan end.
	if lastEvent > 0 && end < lastEvent {
		return segment.TickEnd
	}
	return end
}

func firstKillTick(segment RecordingSegment) int {
	out := 0
	for _, kill := range segment.Kills {
		if kill.Tick <= 0 {
			continue
		}
		if out == 0 || kill.Tick < out {
			out = kill.Tick
		}
	}
	return out
}

func lastKillTick(segment RecordingSegment) int {
	out := 0
	for _, kill := range segment.Kills {
		if kill.Tick > out {
			out = kill.Tick
		}
	}
	return out
}

// lastSegmentEventTick is the latest kill or utility throw/pop in the segment.
// It must stay aligned with parser lastSegmentEventTick so capture ends cover
// the same selected events the plan validated.
func lastSegmentEventTick(segment RecordingSegment) int {
	last := lastKillTick(segment)
	for _, utility := range segment.Utility {
		if utility.ThrowTick > last {
			last = utility.ThrowTick
		}
		if utility.PopTick > last {
			last = utility.PopTick
		}
	}
	return last
}

func streamSetupCommands(plan RecordingPlan) []string {
	recordName := slashPath(plan.OutputDir)
	recordFPS := fmt.Sprintf("mirv_streams record fps %d", plan.Stream.FPS)
	commands := []string{
		"cl_demo_predict 0",
		"cl_trueview_show_status 0",
	}
	commands = append(commands, voiceMuteCommands()...)
	commands = append(commands,
		"mirv_panorama panelstyle panelId=trueview_row opacity=0",
		fmt.Sprintf("mirv_streams record name %s", quoteConsoleArg(recordName)),
		recordFPS,
		"mirv_streams record screen enabled 1",
	)
	switch plan.Stream.Mode {
	case StreamModeTGASequence:
		commands = append(commands, "mirv_streams record screen settings afxClassic")
	default:
		settingName := ffmpegSettingName(plan.Stream.Encoder, plan.Stream.CRF)
		commands = append(commands,
			ffmpegSettingsCommand(settingName, plan.Stream.CRF, plan.Stream.Encoder),
			"mirv_streams record screen settings "+settingName,
		)
	}
	return append(commands, hudSetupCommands(plan)...)
}

// voiceMuteCommands silence in-engine demo voice before HLAE records. CS2
// playback relays both teams; the render mix is extracted POV-team tracks.
func voiceMuteCommands() []string {
	return []string{
		"voice_enable 0",
		"tv_listen_voice_indices 0",
		"tv_listen_voice_indices_h 0",
	}
}

func voiceRestoreCommands() []string {
	return []string{
		"voice_enable 1",
		"tv_listen_voice_indices -1",
		"tv_listen_voice_indices_h -1",
	}
}

// ffmpegSettingName derives a unique HLAE accumulator setting name per
// (encoder, crf) so differently-configured captures never collide in HLAE's
// keyed ffmpeg setting store.
func ffmpegSettingName(encoder string, crf int) string {
	switch encoder {
	case EncoderNVENC:
		return fmt.Sprintf("zvFfmpegNvencYuv420pCrf%d", crf)
	case EncoderAMF:
		return fmt.Sprintf("zvFfmpegAmfYuv420pCrf%d", crf)
	case EncoderQSV:
		return fmt.Sprintf("zvFfmpegQsvYuv420pCrf%d", crf)
	default:
		return fmt.Sprintf("zvFfmpegYuv420pCrf%d", crf)
	}
}

func ffmpegSettingsCommand(name string, crf int, encoder string) string {
	var codec string
	switch encoder {
	case EncoderNVENC:
		codec = fmt.Sprintf("-c:v h264_nvenc -preset p5 -rc vbr -b:v 0 -cq %d -pix_fmt yuv420p", crf)
	case EncoderAMF:
		codec = fmt.Sprintf("-c:v h264_amf -quality balanced -rc cqp -qp_i %d -qp_p %d -qp_b %d -pix_fmt yuv420p", crf, crf, crf)
	case EncoderQSV:
		codec = fmt.Sprintf("-c:v h264_qsv -global_quality %d -pix_fmt yuv420p", crf)
	default:
		codec = fmt.Sprintf("-c:v libx264 -preset fast -crf %d -pix_fmt yuv420p", crf)
	}
	return fmt.Sprintf(
		`mirv_streams settings add ffmpeg %s "%s {QUOTE}{AFX_STREAM_PATH}\video.mp4{QUOTE}"`,
		name,
		codec,
	)
}

func hudSetupCommands(plan RecordingPlan) []string {
	var commands []string
	switch plan.Stream.HUDMode {
	case HUDModeClean:
		return []string{
			"spec_show_xray 0",
			"cl_drawhud 0",
		}
	case HUDModeDeathnotices:
		commands = []string{
			"spec_show_xray 0",
			"cl_spec_show_bindings 0",
			"cl_drawhud 1",
			"cl_draw_only_deathnotices 1",
			"cl_show_observer_crosshair 2",
			"crosshair 1",
		}
	default:
		commands = []string{
			"spec_show_xray 0",
			"cl_spec_show_bindings 0",
			"cl_drawhud 1",
			"cl_draw_only_deathnotices 0",
			"cl_show_observer_crosshair 2",
			"crosshair 1",
		}
	}
	if plan.Stream.HUDMode != HUDModeDeathnotices && !plan.Stream.PortraitSafeKillfeed {
		return commands
	}
	commands = append(commands,
		"mirv_deathmsg clear",
		"mirv_deathmsg filter clear",
		fmt.Sprintf("mirv_deathmsg filter add attackerMatch=!x%s block=1 lastRule=1", plan.TargetSteamID64),
		"mirv_deathmsg localPlayer -1",
		fmt.Sprintf("mirv_deathmsg lifetime %s", formatFloat(plan.Stream.DeathnoticeLifetime)),
	)
	if plan.Stream.PortraitSafeKillfeed && plan.Stream.HUDMode == HUDModeDeathnotices {
		commands = append(commands,
			fmt.Sprintf("safezonex %s", formatFloat(plan.Stream.DeathnoticeSafeZoneX)),
			fmt.Sprintf("safezoney %s", formatFloat(plan.Stream.DeathnoticeSafeZoneY)),
		)
	}
	return commands
}

func hudCleanupCommands(stream StreamConfig) []string {
	if stream.HUDMode != HUDModeDeathnotices && !stream.PortraitSafeKillfeed {
		return nil
	}
	commands := []string{
		"mirv_deathmsg clear",
		"mirv_deathmsg filter clear",
		"mirv_deathmsg localPlayer default",
		"mirv_deathmsg lifetime default",
	}
	if stream.PortraitSafeKillfeed && stream.HUDMode == HUDModeDeathnotices {
		commands = append(commands,
			"safezonex 1",
			"safezoney 1",
		)
	}
	return commands
}

func slashPath(path string) string {
	return strings.ReplaceAll(path, `\`, `/`)
}

func formatFloat(v float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", v), "0"), ".")
}
