import type { EditConfig } from './api/types.ts';

export const FULL_DEMO_PROFILE = 'full-demo-pov-chill-v1';
export const FULL_DEMO_CAPTURE_VARIANT = 'gameplay-pov-60';
const UUID = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;
const SHA256 = /^[0-9a-f]{64}$/;

// Validate the wire at the boundary. Keep the original document, including demo
// evidence: projecting only UI fields would change the server-approved hash.
type Guard<T> = (value: unknown) => value is T;
type Guarded<G> = G extends Guard<infer T> ? T : never;
function record(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}
function object<S extends Record<string, Guard<unknown>>>(shape: S, optional: readonly string[] = []): Guard<{ [K in keyof S]: Guarded<S[K]> }> {
  return (value): value is { [K in keyof S]: Guarded<S[K]> } => record(value)
    && Object.keys(value).every((key) => Object.hasOwn(shape, key))
    && Object.entries(shape).every(([key, guard]) => optional.includes(key) && !Object.hasOwn(value, key) || guard(value[key]));
}
function oneOf<const T extends string[]>(...values: T): Guard<T[number]> {
  return (value): value is T[number] => typeof value === 'string' && values.some((entry) => entry === value);
}
function number(min: number, max: number): Guard<number> {
  return (value): value is number => typeof value === 'number' && Number.isFinite(value) && value >= min && value <= max;
}
const integer: Guard<number> = (value): value is number => typeof value === 'number' && Number.isSafeInteger(value) && value >= 0;
const string: Guard<string> = (value): value is string => typeof value === 'string' && value.length <= 16384;
const boolean: Guard<boolean> = (value): value is boolean => typeof value === 'boolean';
const hash: Guard<string> = (value): value is string => typeof value === 'string' && SHA256.test(value);
const uuid: Guard<string> = (value): value is string => typeof value === 'string' && UUID.test(value) && value !== '00000000-0000-0000-0000-000000000000';
function nullable<T>(guard: Guard<T>): Guard<T | null> {
  return (value): value is T | null => value === null || guard(value);
}
function array<T>(guard: Guard<T>, limit = 10000): Guard<T[]> {
  return (value): value is T[] => Array.isArray(value) && value.length <= limit && value.every(guard);
}
const assetRef = object({ id: uuid, sha256: hash });
export type FullDemoAssetRef = Guarded<typeof assetRef>;
const optionsShape = object({
  profile_id: oneOf(FULL_DEMO_PROFILE), source_kind: oneOf('demo', 'premier', 'professional', 'faceit'),
  capture: object({
    hud_profile: oneOf('native-clean-spectator', 'native'), xray: (value): value is false => value === false,
    camera_policy: oneOf('strict-first-person'), contract_version: oneOf('full-demo-observer-v1'),
    crosshair: object({ mode: oneOf('observed', 'provided-code'), code: string, allow_capture_default: boolean }),
  }),
  editorial: object({
    freeze_seconds: number(0, 20), keep_freeze_voice: boolean, voice_context_seconds: number(0, 3), max_freeze_seconds: number(0, 60),
    death_tail_seconds: number(0, 3), round_tail_seconds: number(0, 2), allow_safe_tail_trim: boolean,
    manual_ranges: array(object({ round_id: string, start_tick: integer, end_tick: integer }), 200),
  }),
  audio: object({
    voice: object({ enabled: boolean, gain: number(0, 2), team_policy: oneOf('same-side-at-packet'), normalization: oneOf('bounded-activity-v1', 'none'), approved_fallback: oneOf('block', 'without-voice') }),
    game: object({ gain: number(0, 2), voice_priority: boolean }),
    music: object({
      enabled: boolean, assets: array(assetRef, 20), reference_level: oneOf('track-lufs-minus-16-v1'), bed_gain_db: number(-24, -18), loop_policy: oneOf('ordered-loop', 'once-pad-silence'),
      ducking: object({ enabled: boolean, game_contribution: number(0, 1), attack_ms: number(1, 2000), release_ms: number(20, 5000), threshold: number(0.001, 1), ratio: number(1, 20) }),
    }),
    loudness: object({ target_i_lufs: number(-14, -14), target_tp_dbtp: number(-1.5, -1.5), target_lra: number(11, 11), policy_version: oneOf('program-aac-v1') }),
  }),
  sponsor: object({
    enabled: boolean, video: nullable(assetRef), narration: nullable(assetRef), audio_policy: oneOf('embedded', 'replace-narration'), short_narration_policy: oneOf('block', 'pad-silence'),
    placement_policy: oneOf('first-two-rounds', 'round-boundary', 'manual-frame'), window_start_seconds: number(0, 43200), window_end_seconds: number(0, 43200),
    after_round_id: string, manual_start_frame: nullable(integer), allow_split_round: boolean, music_policy: oneOf('pause-resume'),
  }),
  overlays: object({ roster: boolean, scoreboard: boolean, theme: oneOf('faceit-orange', 'neon-violet'), source: oneOf('demo', 'faceit') }),
  outputs: object({ media_profile: oneOf('h264-1080p60-aac48-stereo'), cover_policy: oneOf('no-cover', 'generated-gameplay'), metadata_policy: oneOf('factual-v1') }),
});
export type FullDemoOptions = Guarded<typeof optionsShape>;

export function isFullDemoOptions(value: unknown): value is FullDemoOptions {
  if (!optionsShape(value)) return false;
  const { editorial, sponsor, capture } = value;
  if (editorial.max_freeze_seconds < editorial.freeze_seconds || sponsor.window_end_seconds < sponsor.window_start_seconds) return false;
  if (sponsor.placement_policy === 'manual-frame' && sponsor.manual_start_frame === null) return false;
  if (sponsor.placement_policy === 'round-boundary' && sponsor.after_round_id === '') return false;
  if (capture.crosshair.mode === 'observed' && capture.crosshair.code !== '') return false;
  if (capture.crosshair.mode === 'provided-code' && !/^CSGO(?:-[ABCDEFGHJKLMNOPQRSTUVWXYZabcdefhijkmnopqrstuvwxyz23456789]{5}){5}$/.test(capture.crosshair.code)) return false;
  const ranges = editorial.manual_ranges;
  return new Set(ranges.map((range) => range.round_id)).size === ranges.length
    && ranges.every((range) => range.round_id !== '' && range.end_tick > range.start_tick);
}

const interval = object({ start: integer, end: integer });
const notice = object({ code: string, message: string, round_id: (value): value is string | undefined => value === undefined || string(value) }, ['round_id']);
const round = object({
  round_id: string, source_round_number: integer, live_start_tick: integer, round_end_tick: integer, death_tick: nullable(integer),
  requested_start_tick: integer, requested_end_tick: integer, live_end_tick: integer, capture_start_tick: integer, capture_end_tick: integer, effective_end_tick: integer,
  start_reason: string, end_reason: string, bounds_evidence: string, excluded_intervals: nullable(array(interval)),
  kills: nullable(array(record, 1000)), utility: nullable(array(record, 1000)),
});
const documentShape = object({
  schema_version: oneOf('1.0'), plan_id: uuid, revision: integer, plan_hash: hash, planner_version: oneOf('full-demo-editorial-v1'),
  crosshairs: nullable(array(object({ tick: integer, code: string }), 4096)),
  input: object({ demo_sha256: hash, target_steamid64: (value): value is string => typeof value === 'string' && /^\d{17}$/.test(value), facts_ref: string, facts_hash: hash }),
  clock: object({ source_clock_kind: oneOf('ingame_tick'), tick_rate: number(1, 1024), output_fps: number(60, 60), audio_sample_rate: number(48000, 48000) }),
  options: isFullDemoOptions, rounds: array(round, 200),
  voice: object({ availability: string, index_ref: string, index_hash: string, extractor_version: string, clock_kind: string, activity: nullable(array(interval)), selected_packets: integer, excluded_packets: integer }),
  assets: nullable(array(object({ ref: assetRef, duration_frames: integer, has_video: boolean, has_audio: boolean, title: string, creator: string, source_url: string, permission: string, attribution: string }), 100)),
  sponsor_placement: object({ boundary: string, start_frame: integer, duration_frames: integer, candidates: nullable(array(object({ after_round_id: string, frame: integer }), 200)) }),
  timeline: nullable(array(object({ role: oneOf('round', 'sponsor'), source_ref: string, source_start_tick: integer, source_end_tick: integer, source_offset_frames: integer, start_frame: integer, end_frame: integer, start_sample: integer, end_sample: integer, reason: string }))),
  warnings: nullable(array(notice, 1000)), blockers: nullable(array(notice, 1000)),
});
export type FullDemoDocument = Guarded<typeof documentShape>;
export type FullDemoRound = Guarded<typeof round>;
const snapshotShape = object({ document: documentShape, approval: object({ approved_plan_hash: hash, allow_safe_tail_trim: boolean, timestamp: string }) });
export type FullDemoSnapshot = Guarded<typeof snapshotShape>;

export function isFullDemoSnapshot(value: unknown): value is FullDemoSnapshot {
  return snapshotShape(value) && value.approval.approved_plan_hash === value.document.plan_hash
    && Number.isFinite(Date.parse(value.approval.timestamp))
    && value.approval.allow_safe_tail_trim === value.document.options.editorial.allow_safe_tail_trim
    && value.document.rounds.length > 0 && (value.document.timeline?.length ?? 0) > 0 && (value.document.blockers?.length ?? 0) === 0;
}

/** Canonical UI comparison ignores plan IDs, approval time and object key order. */
export function fullDemoOptionsKey(options: FullDemoOptions): string {
  return canonicalJSON(options);
}
function canonicalJSON(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(canonicalJSON).join(',')}]`;
  if (record(value)) return `{${Object.keys(value).sort().map((key) => `${JSON.stringify(key)}:${canonicalJSON(value[key])}`).join(',')}}`;
  return JSON.stringify(value) ?? 'null';
}
export function fullDemoApprovalKey(document: FullDemoDocument, options: FullDemoOptions): string | null {
  return fullDemoOptionsKey(document.options) === fullDemoOptionsKey(options) && document.rounds.length > 0
    && (document.timeline?.length ?? 0) > 0 && (document.blockers?.length ?? 0) === 0 ? document.plan_hash : null;
}
export function approveFullDemo(document: FullDemoDocument, timestamp = new Date().toISOString()): FullDemoSnapshot {
  const snapshot = { document, approval: { approved_plan_hash: document.plan_hash, allow_safe_tail_trim: document.options.editorial.allow_safe_tail_trim, timestamp } };
  if (!isFullDemoSnapshot(snapshot)) throw new Error('El plan tiene bloqueos o está incompleto.');
  return snapshot;
}
export function fullDemoPlanEdit(snapshot: FullDemoSnapshot): EditConfig {
  const options = snapshot.document.options;
  return {
    fullDemo: snapshot, format: 'landscape-16x9', killEffect: 'clean', transition: 'cut', intro: false, outro: false,
    hookText: false, killCounter: false, matchRecap: true, nativeHud: true, voiceComms: options.audio.voice.enabled,
    voiceVolume: options.audio.voice.gain, coverStrategy: options.outputs.cover_policy, overlayTheme: options.overlays.theme,
    ...(options.overlays.source === 'faceit' ? { demoSource: 'faceit' } : {}),
  };
}

function planURL(jobId: string): string {
  if (!UUID.test(jobId)) throw new Error('Partida desconocida.');
  return `/api/demos/${jobId}/full-demo/plan`;
}
async function responseJSON(response: Response): Promise<unknown> {
  const value: unknown = await response.json();
  if (!response.ok) throw new Error(record(value) && typeof value.error === 'string' ? value.error : `Solicitud fallida (${response.status}).`);
  return value;
}
const envelopeShape = object({ document: nullable(documentShape), defaults: isFullDemoOptions, compatibility: oneOf('legacy-until-planned-and-approved', 'editorial-v1') });
export async function loadFullDemoPlan(jobId: string, signal?: AbortSignal): Promise<Guarded<typeof envelopeShape>> {
  const value = await responseJSON(await fetch(planURL(jobId), { cache: 'no-store', signal }));
  if (!envelopeShape(value)) throw new Error('El servidor devolvió un plan Full Demo incompatible.');
  return value;
}
export async function saveFullDemoPlan(jobId: string, options: FullDemoOptions): Promise<FullDemoDocument> {
  if (!isFullDemoOptions(options)) throw new Error('Revisa los valores de captura, audio y sponsor.');
  const value = await responseJSON(await fetch(planURL(jobId), { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ options }) }));
  if (!documentShape(value)) throw new Error('El servidor devolvió un plan Full Demo incompatible.');
  return value;
}

export type FullDemoProvenance = { title: string; creator: string; source_url: string; permission: string; attribution: string };
export async function uploadFullDemoAsset(file: File, provenance: FullDemoProvenance, signal?: AbortSignal): Promise<FullDemoAssetRef> {
  const form = new FormData();
  form.set('video', file);
  form.set('config', JSON.stringify({ provenance }));
  const value = await responseJSON(await fetch('/api/editor/assets', { method: 'POST', body: form, signal }));
  if (!record(value)) throw new Error('El servidor no certificó el archivo subido.');
  // The guarded pair is independent from private storage fields in the upload response.
  const ref = { id: value.id, sha256: value.sha256 };
  if (!assetRef(ref)) throw new Error('Referencia de archivo inválida.');
  return ref;
}
