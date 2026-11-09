import { parseFailureReason } from '../api/failure-reason.ts';
import type { Video } from '../api/types.ts';
import { currentFullDemoOptions, withFullDemoRounds } from '../full-demo-plan.ts';
import type { FullDemoOptions } from '../full-demo-plan.ts';

/**
 * A player's POV of one round, recorded from the Replay: the existing Full
 * Demo capture limited to that round, with Studio's default look and nothing
 * added (no clips, no effects between rounds, there is no other round).
 */
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
