import { readFile, writeFile, mkdir } from 'node:fs/promises';
import { dirname } from 'node:path';
import vm from 'node:vm';

const SCHEMA_VERSION = 1;
const VERIFIED_MARKER = 'ZACKVIDEO_CAPTURE_VERIFIED_OBSERVER_STEAMID_V1';
const FAILED_MARKER = 'ZACKVIDEO_CAPTURE_FAILED_OBSERVER_STEAMID_V1';

function requireObject(value, name) {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error(`${name} must be an object`);
  }
  return value;
}

function requireInteger(value, name, minimum = Number.MIN_SAFE_INTEGER) {
  if (!Number.isInteger(value) || value < minimum) {
    throw new Error(`${name} must be an integer >= ${minimum}`);
  }
  return value;
}

export function validateScenario(raw) {
  const scenario = requireObject(raw, 'scenario');
  if (scenario.schema_version !== SCHEMA_VERSION) {
    throw new Error(`scenario.schema_version must be ${SCHEMA_VERSION}`);
  }
  if (typeof scenario.name !== 'string' || scenario.name.trim() === '') {
    throw new Error('scenario.name must be a non-empty string');
  }
  if (typeof scenario.target_steamid !== 'string' || !/^\d{17}$/.test(scenario.target_steamid)) {
    throw new Error('scenario.target_steamid must be a 17-digit SteamID64');
  }
  requireInteger(scenario.start_tick ?? 0, 'scenario.start_tick', 0);
  requireInteger(scenario.tick_step ?? 1, 'scenario.tick_step', 1);
  requireInteger(scenario.max_frames ?? 10000, 'scenario.max_frames', 1);
  requireInteger(scenario.seek_delay_frames ?? 0, 'scenario.seek_delay_frames', 0);
  if (scenario.tick_overrides !== undefined && !Array.isArray(scenario.tick_overrides)) {
    throw new Error('scenario.tick_overrides must be an array');
  }
  for (const [index, override] of (scenario.tick_overrides ?? []).entries()) {
    requireObject(override, `scenario.tick_overrides[${index}]`);
    requireInteger(override.frame, `scenario.tick_overrides[${index}].frame`, 1);
    requireInteger(override.tick, `scenario.tick_overrides[${index}].tick`, 0);
  }
  if (scenario.demo_end_tick !== undefined) {
    requireInteger(scenario.demo_end_tick, 'scenario.demo_end_tick', 0);
  }
  if (scenario.observer_overrides !== undefined && !Array.isArray(scenario.observer_overrides)) {
    throw new Error('scenario.observer_overrides must be an array');
  }
  for (const [index, override] of (scenario.observer_overrides ?? []).entries()) {
    requireObject(override, `scenario.observer_overrides[${index}]`);
    requireInteger(override.from_tick, `scenario.observer_overrides[${index}].from_tick`, 0);
    requireInteger(override.to_tick, `scenario.observer_overrides[${index}].to_tick`, override.from_tick);
    if (override.observed_steamid !== null && override.observed_steamid !== undefined &&
        (typeof override.observed_steamid !== 'string' || !/^\d{17}$/.test(override.observed_steamid))) {
      throw new Error(`scenario.observer_overrides[${index}].observed_steamid must be null or a SteamID64`);
    }
  }
  if (scenario.expect !== undefined) {
    requireObject(scenario.expect, 'scenario.expect');
    if (!['verified', 'failed', 'incomplete'].includes(scenario.expect.outcome)) {
      throw new Error('scenario.expect.outcome must be verified, failed, or incomplete');
    }
  }
  return structuredClone(scenario);
}

function stateAtTick(scenario, tick) {
  const state = {
    observedSteamID: scenario.default_observed_steamid === undefined
      ? scenario.target_steamid
      : scenario.default_observed_steamid,
    targetPresent: scenario.target_present !== false,
    observerMode: scenario.observer_mode ?? 2,
  };
  for (const override of scenario.observer_overrides ?? []) {
    if (tick < override.from_tick || tick > override.to_tick) continue;
    if (Object.hasOwn(override, 'observed_steamid')) state.observedSteamID = override.observed_steamid;
    if (Object.hasOwn(override, 'target_present')) state.targetPresent = override.target_present;
    if (Object.hasOwn(override, 'observer_mode')) state.observerMode = override.observer_mode;
  }
  return state;
}

function includesMarker(events, marker) {
  return events.some((event) => event.value.includes(marker));
}

function normalizedEvent(frame, tick, kind, value) {
  return { frame, tick, kind, value: String(value).replaceAll('\r\n', '\n').trimEnd() };
}

function evaluateExpectations(summary, scenario) {
  const expect = scenario.expect;
  if (!expect) return [];
  const failures = [];
  if (summary.outcome !== expect.outcome) {
    failures.push(`outcome=${summary.outcome}, want ${expect.outcome}`);
  }
  if (expect.failure_contains && !summary.failure_messages.some((value) => value.includes(expect.failure_contains))) {
    failures.push(`failure messages do not contain ${JSON.stringify(expect.failure_contains)}`);
  }
  for (const wanted of expect.must_exec ?? []) {
    if (!summary.executed_commands.some((command) => command.includes(wanted))) {
      failures.push(`executed commands do not contain ${JSON.stringify(wanted)}`);
    }
  }
  for (const banned of expect.must_not_exec ?? []) {
    if (summary.executed_commands.some((command) => command.includes(banned))) {
      failures.push(`executed commands unexpectedly contain ${JSON.stringify(banned)}`);
    }
  }
  if (expect.recorded_segments) {
    const actual = summary.recorded_segments.join(',');
    const wanted = expect.recorded_segments.join(',');
    if (actual !== wanted) failures.push(`recorded segments=${actual}, want ${wanted}`);
  }
  if (expect.soft_quit === true) {
    if (summary.disconnect_frame === null) failures.push('disconnect was not executed');
    if (summary.quit_frame === null) failures.push('quit was not executed');
    if (summary.disconnect_frame !== null && summary.quit_frame !== null && summary.quit_frame <= summary.disconnect_frame) {
      failures.push(`quit frame ${summary.quit_frame} must be after disconnect frame ${summary.disconnect_frame}`);
    }
  }
  return failures;
}

export async function runSimulation(scriptSource, rawScenario) {
  const scenario = validateScenario(rawScenario);
  const callbacks = new Map();
  const events = [];
  const commands = [];
  const openSegments = [];
  const recordedSegments = [];
  const integrityFailures = [];
  let unlabeledRecordEnds = 0;
  let frame = 0;
  let tick = scenario.start_tick ?? 0;
  let playing = scenario.start_playing !== false;
  let pendingSeek = null;
  let disconnected = false;
  let quit = false;
  let disconnectFrame = null;
  let quitFrame = null;

  function emit(kind, value) {
    events.push(normalizedEvent(frame, tick, kind, value));
  }

  function exec(command) {
    command = String(command);
    commands.push(command);
    emit('exec', command);
    if (command.startsWith('demo_gototick ')) {
      const target = Number.parseInt(command.slice('demo_gototick '.length), 10);
      if (Number.isInteger(target) && scenario.seek_behavior !== 'never' && pendingSeek?.target !== target) {
        pendingSeek = { target, remaining: scenario.seek_delay_frames ?? 0 };
      }
    } else if (command === 'mirv_streams record start') {
      const marker = [...events].reverse().find((event) =>
        event.frame === frame && event.kind === 'message' && /\[zackvideo\] record-start-([^:]+):/.test(event.value));
      const id = marker?.value.match(/\[zackvideo\] record-start-([^:]+):/)?.[1] ?? '';
      if (!id) integrityFailures.push(`record start at frame ${frame} has no same-frame segment marker`);
      if (openSegments.length > 0) integrityFailures.push(`nested record start at frame ${frame}`);
      openSegments.push({ id, frame, tick });
    } else if (command === 'mirv_streams record end') {
      const marker = [...events].reverse().find((event) =>
        event.frame === frame && event.kind === 'message' && /\[zackvideo\] record-end-([^:]+):/.test(event.value));
      const id = marker?.value.match(/\[zackvideo\] record-end-([^:]+):/)?.[1] ?? '';
      const opened = openSegments.pop();
      if (!opened) integrityFailures.push(`record end at frame ${frame} has no open recording`);
      if (!id) unlabeledRecordEnds++;
      if (opened && id && opened.id !== id) {
        integrityFailures.push(`record end ${id} does not match open segment ${opened.id}`);
      }
      recordedSegments.push({
        id: opened?.id ?? id,
        start_frame: opened?.frame ?? null,
        start_tick: opened?.tick ?? null,
        end_frame: frame,
        end_tick: tick,
      });
    } else if (command === 'disconnect') {
      disconnected = true;
      playing = false;
      disconnectFrame = frame;
    } else if (command === 'quit') {
      quit = true;
      quitFrame = frame;
    }
  }

  function currentEntities() {
    const state = stateAtTick(scenario, tick);
    const localController = {
      isPlayerController: () => true,
      isPlayerPawn: () => false,
      getSteamId: () => ({ toString: () => '76561197960265729' }),
      getPlayerPawnHandle: () => 2,
    };
    const localPawn = {
      isPlayerController: () => false,
      isPlayerPawn: () => true,
      getObserverTargetHandle: () => state.observedSteamID === null ? -1 : 3,
      getObserverMode: () => state.observerMode,
      getPlayerControllerHandle: () => 1,
    };
    const observedPawn = {
      isPlayerController: () => false,
      isPlayerPawn: () => true,
      getPlayerControllerHandle: () => 4,
    };
    const observedController = {
      isPlayerController: () => true,
      isPlayerPawn: () => false,
      getSteamId: () => ({ toString: () => state.observedSteamID }),
      getPlayerPawnHandle: () => 3,
    };
    const targetController = {
      isPlayerController: () => true,
      isPlayerPawn: () => false,
      getSteamId: () => ({ toString: () => scenario.target_steamid }),
      getPlayerPawnHandle: () => 5,
    };
    const entities = new Map([[1, localController], [2, localPawn], [3, observedPawn], [4, observedController]]);
    if (state.targetPresent && state.observedSteamID !== scenario.target_steamid) entities.set(5, targetController);
    return { entities, localController };
  }

  const eventAPI = {
    on(id, callback) {
      if (typeof id !== 'string' || typeof callback !== 'function') throw new Error('invalid clientFrameStageNotify.on call');
      callbacks.set(id, callback);
    },
    off(id) { callbacks.delete(id); },
  };
  const mirv = {
    events: { clientFrameStageNotify: eventAPI },
    exec,
    message(value) { emit('message', value); },
    warning(value) { emit('warning', value); },
    isPlayingDemo() { return playing; },
    getDemoTick() { return tick; },
    isHandleValid(handle) { return Number.isInteger(handle) && handle > 0; },
    getHandleEntryIndex(handle) { return handle; },
    getHighestEntityIndex() {
      const { entities } = currentEntities();
      return Math.max(...entities.keys());
    },
    getEntityFromIndex(index) { return currentEntities().entities.get(index) ?? null; },
    getEntityFromSplitScreenPlayer(index) { return index === 0 ? currentEntities().localController : null; },
  };

  // Independent engine defaults for Full Demo's readback contract. Scenarios
  // can remove or refuse individual cvars; absent APIs must fail closed.
  const cvarValues = new Map(Object.entries({
    voice_modenable: true, snd_voipvolume: 0.63, tv_listen_voice_indices: 7, tv_listen_voice_indices_h: 9,
    spec_show_xray: 1, spec_autodirector: true, cl_drawhud: true, cl_draw_only_deathnotices: false,
    cl_show_observer_crosshair: 1, crosshair: true, cl_demo_predict: 1, cl_trueview_show_status: 2, host_framerate: 0,
    cl_spec_show_bindings: true, cl_drawhud_specvote: true, cl_teamid_overhead_mode: 3,
    cl_drawhud_force_teamid_overhead: 0, hud_showtargetid: true, cl_crosshairsize: 4,
    cl_crosshairgap: 0, cl_crosshair_outlinethickness: 0.5, cl_crosshaircolor_r: 0,
    cl_crosshaircolor_g: 0, cl_crosshaircolor_b: 0, cl_crosshairalpha: 255,
    cl_crosshair_dynamic_splitdist: 1, cl_crosshair_recoil: false, cl_fixedcrosshairgap: 0,
    cl_crosshaircolor: 4, cl_crosshair_drawoutline: false, cl_crosshair_dynamic_splitalpha_innermod: 1,
    cl_crosshair_dynamic_splitalpha_outermod: 0.5, cl_crosshair_dynamic_maxdist_splitratio: 0.5,
    cl_crosshairthickness: 1, cl_crosshairdot: true, cl_crosshairgap_useweaponvalue: false,
    cl_crosshairusealpha: false, cl_crosshair_t: true, cl_crosshairstyle: 4,
    ...(scenario.cvars ?? {}),
  }));
  for (const name of scenario.missing_cvars ?? []) cvarValues.delete(name);
  const cvarNames = [...cvarValues.keys()];
  class AdvancedfxCVar {
    static getIndexFromName(name) { const index = cvarNames.indexOf(name); return index < 0 ? undefined : index; }
    constructor(index) { if (!cvarNames[index]) throw new Error('cvar unavailable'); this.name = cvarNames[index]; }
    get value() { return cvarValues.get(this.name); }
    set value(value) { if (!(scenario.refuse_cvar_writes ?? []).includes(this.name)) cvarValues.set(this.name, value); }
  }
  const context = vm.createContext({ mirv, AdvancedfxCVar: scenario.cvar_api === false ? undefined : AdvancedfxCVar, console: Object.freeze({ log() {}, warn() {}, error() {} }) }, {
    name: `cliphub-capturelab-${scenario.name}`,
    codeGeneration: { strings: false, wasm: false },
  });
  const compiled = new vm.Script(scriptSource, { filename: 'recording.js' });
  compiled.runInContext(context, { timeout: scenario.script_timeout_ms ?? 1000 });
  if (callbacks.size !== 1) throw new Error(`recording.js registered ${callbacks.size} frame callbacks, want 1`);

  const maxFrames = scenario.max_frames ?? 10000;
  const tickOverrides = new Map((scenario.tick_overrides ?? []).map(({ frame, tick }) => [frame, tick]));
  for (frame = 1; frame <= maxFrames && !quit; frame++) {
    if (tickOverrides.has(frame)) tick = tickOverrides.get(frame);
    for (const callback of [...callbacks.values()]) callback({ isBefore: scenario.frame_stage === 'render-before', curStage: 12 });
    if (pendingSeek) {
      if (pendingSeek.remaining <= 0) {
        tick = pendingSeek.target;
        pendingSeek = null;
      } else {
        pendingSeek.remaining--;
      }
    } else if (playing) {
      tick += scenario.tick_step ?? 1;
    }
    if (playing && scenario.demo_end_tick !== undefined && tick > scenario.demo_end_tick) {
      playing = false;
    }
  }

  const verified = includesMarker(events, VERIFIED_MARKER);
  const failed = includesMarker(events, FAILED_MARKER);
  const outcome = failed ? 'failed' : verified ? 'verified' : 'incomplete';
  const startEvents = events.filter((event) => event.kind === 'message' && /\[zackvideo\] record-start-/.test(event.value));
  const endEvents = events.filter((event) => event.kind === 'message' && /\[zackvideo\] record-end-/.test(event.value));
  const markerStartIDs = startEvents.map((event) => event.value.match(/record-start-([^:]+):/)?.[1]).filter(Boolean);
  const markerEndIDs = endEvents.map((event) => event.value.match(/record-end-([^:]+):/)?.[1]).filter(Boolean);
  const recordedSegmentIDs = recordedSegments.map((segment) => segment.id).filter(Boolean);
  if (openSegments.length > 0) integrityFailures.push(`${openSegments.length} recording segment(s) remained open`);
  if (markerStartIDs.join(',') !== recordedSegmentIDs.join(',')) {
    integrityFailures.push(`record-start markers=${markerStartIDs.join(',')} do not match completed recordings=${recordedSegmentIDs.join(',')}`);
  }
  if (verified && unlabeledRecordEnds > 0) {
    integrityFailures.push(`${unlabeledRecordEnds} successful record end(s) had no same-frame segment marker`);
  }
  if (verified && markerEndIDs.join(',') !== recordedSegmentIDs.join(',')) {
    integrityFailures.push(`record-end markers=${markerEndIDs.join(',')} do not match completed recordings=${recordedSegmentIDs.join(',')}`);
  }
  const attestationIndex = events.findIndex((event) =>
    (event.kind === 'message' || event.kind === 'warning') &&
    (event.value.includes(VERIFIED_MARKER) || event.value.includes(FAILED_MARKER)));
  const finalRecordEndIndex = events.findLastIndex((event) => event.kind === 'exec' && event.value === 'mirv_streams record end');
  if (verified && (recordedSegments.length === 0 || attestationIndex <= finalRecordEndIndex)) {
    integrityFailures.push('verified attestation was not emitted after every recording closed');
  }
  const summary = {
    schema_version: SCHEMA_VERSION,
    scenario: scenario.name,
    outcome,
    frames_executed: Math.min(frame, maxFrames),
    final_tick: tick,
    verified_marker: verified,
    failed_marker: failed,
    disconnected,
    quit,
    disconnect_frame: disconnectFrame,
    quit_frame: quitFrame,
    recorded_segments: recordedSegmentIDs,
    record_operations: recordedSegments,
    record_start_count: startEvents.length,
    record_end_count: endEvents.length,
    executed_commands: commands,
    failure_messages: events.filter((event) => event.kind === 'warning' && event.value.includes('capture_failed:')).map((event) => event.value),
    events,
    final_cvars: Object.fromEntries(cvarValues),
    integrity_failures: integrityFailures,
  };
  summary.expectation_failures = evaluateExpectations(summary, scenario);
  summary.ok = summary.expectation_failures.length === 0 && summary.integrity_failures.length === 0;
  return summary;
}

export async function runFiles({ scriptPath, scenarioPath, outputPath }) {
  const [scriptSource, scenarioSource] = await Promise.all([
    readFile(scriptPath, 'utf8'),
    readFile(scenarioPath, 'utf8'),
  ]);
  const summary = await runSimulation(scriptSource, JSON.parse(scenarioSource));
  if (outputPath) {
    await mkdir(dirname(outputPath), { recursive: true });
    await writeFile(outputPath, `${JSON.stringify(summary, null, 2)}\n`, 'utf8');
  }
  return summary;
}
