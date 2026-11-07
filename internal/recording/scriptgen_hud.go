package recording

import "fmt"

func hudSetupCommands(plan RecordingPlan) []string {
	if plan.FullDemo != nil {
		return nil
	}
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
		killfeedFilter(plan.TargetSteamID64),
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

// killfeedFilter keeps only the kills of steamID in the CS2 killfeed.
func killfeedFilter(steamID string) string {
	return fmt.Sprintf("mirv_deathmsg filter add attackerMatch=!x%s block=1 lastRule=1", steamID)
}

// killfeedFiltered reports whether the capture filters the killfeed to the
// filmed player's kills.
func killfeedFiltered(plan RecordingPlan) bool {
	return plan.FullDemo == nil && (plan.Stream.HUDMode == HUDModeDeathnotices || plan.Stream.PortraitSafeKillfeed)
}

// windowSteamID is the player a capture window films: the victim on a replay,
// the segment's own POV in a multi-POV plan, else the plan target.
func windowSteamID(s RecordingSegment) string {
	if s.POVSteamID64 != "" {
		return s.POVSteamID64
	}
	if s.Observe != nil {
		return s.Observe.SteamID64
	}
	return ""
}

// povKillfeedCommands points the killfeed filter at the killer each window
// shows in a multi-POV plan; single-POV scripts keep the filter set at setup.
func povKillfeedCommands(plan RecordingPlan, s RecordingSegment, pass, tick int) []scheduledCommand {
	if !plan.IsMultiPOV() || !killfeedFiltered(plan) {
		return nil
	}
	return []scheduledCommand{{Pass: pass, Tick: tick, Key: "killfeed-pov-" + s.ID, Commands: []string{
		"mirv_deathmsg filter clear",
		killfeedFilter(plan.ObservedSteamID(s)),
	}}}
}
