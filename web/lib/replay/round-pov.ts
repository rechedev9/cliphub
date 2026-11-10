import { parseFailureReason } from '../api/failure-reason.ts';
import type { Video } from '../api/types.ts';
import { currentFullDemoOptions, withFullDemoRounds } from '../full-demo-plan.ts';
import type { FullDemoOptions } from '../full-demo-plan.ts';

/** A player's POV of one round: the Full Demo capture limited to that round, with the default look and no clips or effects. */
export function roundPovOptions(defaults: FullDemoOptions, round: number): FullDemoOptions {
  const { bumpers: _bumpers, ...rest } = currentFullDemoOptions(withFullDemoRounds(defaults, [round]));
  return { ...rest, transitions: rest.transitions ? { ...rest.transitions, enabled: false } : rest.transitions };
}

/** Before the video exists the run is `preparing`; after, it follows the video. */
export type RoundPovPhase = 'preparing' | Video['status'];

export type RoundPovRun = {
  key: string;
  round: number;
  name: string;
  phase: RoundPovPhase;
  percent?: number;
  error?: string;
};

export function roundPovKey(input: { matchId: string; round: number; steamId: string }): string {
  return `${input.matchId}:${input.round}:${input.steamId}`;
}

const PHASE_LABEL: Record<RoundPovPhase, string> = {
  preparing: 'Preparando',
  queued: 'En cola',
  recording: 'Grabando',
  composing: 'Montando',
  ready: 'Listo',
  failed: 'No se pudo grabar',
};

export function roundPovLabel(run: Pick<RoundPovRun, 'phase' | 'percent'>): string {
  const label = PHASE_LABEL[run.phase];
  return run.percent !== undefined && (run.phase === 'recording' || run.phase === 'composing') ? `${label} ${run.percent}%` : label;
}

/** A run is over once its video is ready or failed. */
export function roundPovDone(run: Pick<RoundPovRun, 'phase'>): boolean {
  return run.phase === 'ready' || run.phase === 'failed';
}

/** The run's state after reading its video; the percent only shows while a worker has it. */
export function roundPovFollowing(run: RoundPovRun, video: Pick<Video, 'status' | 'captureProgress' | 'failureReason'>): RoundPovRun {
  const { percent: _percent, error: _error, ...rest } = run;
  const percent = video.captureProgress?.percent;
  return {
    ...rest,
    phase: video.status,
    ...(percent === undefined ? {} : { percent: Math.round(percent) }),
    ...(video.status === 'failed' ? { error: parseFailureReason(video.failureReason, { fullDemo: true }).message } : {}),
  };
}

/** A failure whose text is already in Spanish and fit to show. */
export class RoundPovFailure extends Error {}

const BUSY = 'Este jugador ya tiene una grabación en marcha; espera a que termine.';
const NOT_RECORDABLE = 'Esta ronda no se puede grabar desde su POV.';
const NO_VOICES = 'Esta demo no tiene voces del equipo que se puedan usar; no se puede preparar su audio.';
const PARSE_FIRST = 'Hay que analizar de nuevo a este jugador antes de grabarlo; reanaliza la demo.';
const GENERIC = 'No se pudo preparar la grabación. Inténtalo otra vez.';

const CODE_MESSAGE: Readonly<Record<string, string>> = {
  generate_work_active: BUSY,
  pov_contract_failed: NOT_RECORDABLE,
  pov_dead_in_freeze: NOT_RECORDABLE,
  voice_unavailable: NO_VOICES,
  voice_decode_failed: NO_VOICES,
  full_demo_facts_insufficient: PARSE_FIRST,
  full_demo_plan_stale: PARSE_FIRST,
};

function codeOf(failure: unknown): string | undefined {
  if (!(failure instanceof Error) || !('code' in failure)) return undefined;
  return typeof failure.code === 'string' ? failure.code : undefined;
}

/** The Spanish text for a failed recording: known codes first, then our own messages, then a generic one. */
export function roundPovError(failure: unknown): string {
  const code = codeOf(failure);
  const known = code === undefined ? undefined : CODE_MESSAGE[code];
  if (known !== undefined) return known;
  return failure instanceof RoundPovFailure ? failure.message : GENERIC;
}

/** What the planner blocked the round for, in Spanish. */
export function roundPovBlocked(code: string): RoundPovFailure {
  return new RoundPovFailure(CODE_MESSAGE[code] ?? NOT_RECORDABLE);
}
