package recording

import (
	"encoding/json"
	"fmt"
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
	sb.WriteString("    let lastLockFrame = -999;\n")
	sb.WriteString("    let activeSegment = null;\n")
	sb.WriteString("    let unknownObserverFrames = 0;\n")
	sb.WriteString(fmt.Sprintf("    const maxUnknownObserverFrames = %d;\n", maxUnknownObserverFrames))
	sb.WriteString("    let demoEndedFrames = 0;\n")
	sb.WriteString(fmt.Sprintf("    const demoEndedGraceFrames = %d;\n", demoEndedGraceFrames))
	sb.WriteString("    let fatal = false;\n")
	sb.WriteString("    const lockAttempts = {};\n")
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
	sb.WriteString("        mirv.exec(\"disconnect\");\n")
	sb.WriteString("        mirv.exec(\"quit\");\n")
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
	sb.WriteString("        if (e.isBefore || fatal) return;\n")
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
	sb.WriteString("            mirv.exec(\"disconnect\");\n")
	sb.WriteString("            mirv.exec(\"quit\");\n")
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
	sb.WriteString("                seekIdx++;\n")
	sb.WriteString("                seekAttempts = 0;\n")
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

func buildSchedule(plan RecordingPlan) ([]scheduledCommand, []seekStep) {
	commands, seeks, _ := buildRuntimeSchedule(plan)
	return commands, seeks
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
			seekTick = plan.Segments[i-1].TickEnd + 32
		}
		recordStart := EffectiveRecordStartTick(s, plan.Tickrate)
		leadTicks := plan.Tickrate * 5
		seekTarget := max(1, s.TickStart-leadTicks)
		cameraWarmupTick := seekTarget + max(1, plan.Tickrate/2)
		cameraLead3Tick := recordStart - plan.Tickrate*3
		cameraLead2Tick := recordStart - plan.Tickrate*2
		cameraLead1Tick := recordStart - plan.Tickrate
		cameraLockTick := recordStart - 1
		if cameraWarmupTick >= recordStart {
			cameraWarmupTick = recordStart - max(2, plan.Tickrate/2)
		}
		verifyUntil := s.TickEnd
		if lastKill := lastKillTick(s); lastKill > 0 {
			// Once the final selected kill has happened, a spectator change can
			// be CS2's legitimate death cam during post-roll. Keep recording the
			// full segment, but stop treating that camera change as POV drift.
			verifyUntil = min(s.TickEnd, max(recordStart, lastKill))
		}
		windows = append(windows, captureWindow{
			SegmentID:   s.ID,
			LockFrom:    max(seekTarget+1, cameraWarmupTick),
			RecordStart: recordStart,
			VerifyUntil: verifyUntil,
			RecordEnd:   s.TickEnd,
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
			scheduledCommand{Tick: max(seekTarget+2, cameraLead3Tick), Key: "camera-lead-3s-" + s.ID, Commands: camera},
			scheduledCommand{Tick: max(seekTarget+3, cameraLead2Tick), Key: "camera-lead-2s-" + s.ID, Commands: camera},
			scheduledCommand{Tick: max(seekTarget+4, cameraLead1Tick), Key: "camera-lead-1s-" + s.ID, Commands: camera},
			scheduledCommand{Tick: max(seekTarget+5, cameraLockTick), Key: "camera-lock-" + s.ID, Commands: camera},
			scheduledCommand{Tick: recordStart + max(1, plan.Tickrate/2), Key: "camera-relock-" + s.ID, Commands: camera},
		)
		if i == 0 {
			commands = append(commands,
				scheduledCommand{Tick: max(seekTarget, recordStart-6), Key: "hide-demoui", Commands: []string{"demoui"}},
			)
		}

		if plan.Runtime.HostTimescale > 0 && plan.Runtime.HostTimescale != 1 {
			commands = append(commands,
				scheduledCommand{
					Tick:     max(1, recordStart-6),
					Key:      "timescale-up-" + s.ID,
					Commands: []string{fmt.Sprintf("host_timescale %s", formatFloat(plan.Runtime.HostTimescale))},
				},
			)
		}

		commands = append(commands,
			scheduledCommand{Tick: recordStart, Key: "record-start-" + s.ID, Commands: []string{"mirv_streams record start"}},
			scheduledCommand{Tick: s.TickEnd, Key: "record-end-" + s.ID, Commands: []string{"mirv_streams record end"}},
		)

		if plan.Runtime.HostTimescale > 0 && plan.Runtime.HostTimescale != 1 {
			commands = append(commands,
				scheduledCommand{Tick: s.TickEnd + 4, Key: "timescale-reset-" + s.ID, Commands: []string{"host_timescale 1"}},
			)
		}
	}

	lastEnd := plan.Segments[len(plan.Segments)-1].TickEnd
	pad := plan.Runtime.QuitTickPad
	if pad <= 0 {
		pad = 200
	}
	shutdownTick := lastEnd + max(8, pad/2)
	for i, cmd := range hudCleanupCommands(plan.Stream) {
		commands = append(commands, scheduledCommand{
			Tick:     shutdownTick - 4,
			Key:      fmt.Sprintf("hud-cleanup-%02d", i+1),
			Commands: []string{cmd},
		})
	}
	commands = append(commands,
		scheduledCommand{Tick: shutdownTick, Key: "shutdown", Commands: []string{"disconnect", "quit"}},
	)
	return commands, seeks, windows
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

func effectiveRecordStartTick(segment RecordingSegment, tickrate int) int {
	return EffectiveRecordStartTick(segment, tickrate)
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

func streamSetupCommands(plan RecordingPlan) []string {
	recordName := slashPath(plan.OutputDir)
	recordFPS := fmt.Sprintf("mirv_streams record fps %d", plan.Stream.FPS)
	commands := []string{
		"cl_demo_predict 0",
		"cl_trueview_show_status 0",
		"mirv_panorama panelstyle panelId=trueview_row opacity=0",
		fmt.Sprintf("mirv_streams record name %s", quoteConsoleArg(recordName)),
		recordFPS,
		"mirv_streams record screen enabled 1",
	}
	switch plan.Stream.Mode {
	case StreamModeTGASequence:
		commands = append(commands, "mirv_streams record screen settings afxClassic")
	default:
		settingName := ffmpegSettingName(plan.Stream.CRF)
		commands = append(commands,
			ffmpegSettingsCommand(settingName, plan.Stream.CRF),
			"mirv_streams record screen settings "+settingName,
		)
	}
	return append(commands, hudSetupCommands(plan)...)
}

func ffmpegSettingName(crf int) string {
	return fmt.Sprintf("zvFfmpegYuv420pCrf%d", crf)
}

func ffmpegSettingsCommand(name string, crf int) string {
	return fmt.Sprintf(
		`mirv_streams settings add ffmpeg %s "-c:v libx264 -preset fast -crf %d -pix_fmt yuv420p {QUOTE}{AFX_STREAM_PATH}\video.mp4{QUOTE}"`,
		name,
		crf,
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
