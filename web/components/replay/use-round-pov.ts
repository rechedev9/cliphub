'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '@/lib/api';
import { parseFailureReason } from '@/lib/api/failure-reason';
import { createPlayerJob, getLibraryMatch, isLibraryOffline } from '@/lib/api/library';
import { approveFullDemo, FULL_DEMO_CAPTURE_VARIANT, fullDemoPlanEdit, loadFullDemoPlan, saveFullDemoPlan } from '@/lib/full-demo-plan';
import { RoundPovFailure, roundPovBlocked, roundPovDone, roundPovError, roundPovFollowing, roundPovKey, roundPovOptions } from '@/lib/replay/round-pov';
import type { RoundPovRun } from '@/lib/replay/round-pov';

const PARSE_POLL_MS = 1000;
const PARSE_ATTEMPTS = 300;
const VIDEO_POLL_MS = 3000;

export type RoundPovRequest = { matchId: string; round: number; steamId: string; name: string };

function sleep(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    const timer = window.setTimeout(resolve, ms);
    signal.addEventListener('abort', () => { window.clearTimeout(timer); resolve(); }, { once: true });
  });
}

async function waitParsed(jobId: string, signal: AbortSignal): Promise<void> {
  for (let attempt = 0; attempt < PARSE_ATTEMPTS && !signal.aborted; attempt++) {
    const match = await api.getMatch(jobId);
    if (match?.status === 'parsed') return;
    if (match?.status === 'failed') throw new RoundPovFailure(parseFailureReason(match.failureReason, { job: true }).message);
    await sleep(PARSE_POLL_MS, signal);
  }
  if (!signal.aborted) throw new RoundPovFailure('El jugador tarda demasiado en prepararse. Inténtalo otra vez.');
}

/**
 * Records a player's POV of one round with the Full Demo capture and follows
 * the video until it is in Mis vídeos. The runs live for this visit only.
 */
export function useRoundPov(): { runs: readonly RoundPovRun[]; record: (request: RoundPovRequest) => void } {
  const [runs, setRuns] = useState<readonly RoundPovRun[]>([]);
  const runsRef = useRef(runs);
  runsRef.current = runs;
  const videoIds = useRef(new Map<string, string>());
  const controller = useRef<AbortController | null>(null);
  // One recording at a time per player: a second click waits until the first one has queued its video.
  const chains = useRef(new Map<string, Promise<void>>());

  useEffect(() => {
    const own = new AbortController();
    controller.current = own;
    return () => own.abort();
  }, []);

  const patch = useCallback((key: string, change: (run: RoundPovRun) => RoundPovRun) => {
    setRuns((current) => current.map((run) => (run.key === key ? change(run) : run)));
  }, []);

  const record = useCallback((request: RoundPovRequest) => {
    const signal = controller.current?.signal;
    const key = roundPovKey(request);
    const existing = runsRef.current.find((run) => run.key === key);
    if (signal === undefined || (existing !== undefined && existing.phase !== 'failed')) return;
    setRuns((current) => [{ key, round: request.round, name: request.name, phase: 'preparing' }, ...current.filter((run) => run.key !== key)]);
    const run = async (): Promise<void> => {
      try {
        const match = await getLibraryMatch(request.matchId);
        const job = await createPlayerJob(match.demo_id, request.steamId);
        await waitParsed(job.job_id, signal);
        if (signal.aborted) return;
        const plan = await loadFullDemoPlan(job.job_id, signal);
        const planned = await saveFullDemoPlan(job.job_id, roundPovOptions(plan.defaults, request.round), signal);
        const blocker = planned.blockers?.[0];
        if (blocker) throw roundPovBlocked(blocker.code);
        const video = await api.createVideo({
          matchId: job.job_id,
          playIds: planned.rounds.map((round) => round.round_id),
          mode: 'clean',
          variant: FULL_DEMO_CAPTURE_VARIANT,
          editConfig: fullDemoPlanEdit(approveFullDemo(planned)),
          signal,
        }).catch((failure: unknown) => {
          throw new RoundPovFailure(failure instanceof Error ? failure.message : 'No se pudo encolar el vídeo.');
        });
        videoIds.current.set(key, video.id);
        patch(key, (run) => roundPovFollowing(run, video));
      } catch (failure) {
        if (signal.aborted) return;
        const error = isLibraryOffline(failure) ? 'El servicio local no responde. Vuelve a intentarlo.' : roundPovError(failure);
        patch(key, (run) => ({ ...run, phase: 'failed', error }));
      }
    };
    const player = `${request.matchId}:${request.steamId}`;
    const next = (chains.current.get(player) ?? Promise.resolve()).then(run);
    chains.current.set(player, next);
  }, [patch]);

  const following = runs.some((run) => !roundPovDone(run) && videoIds.current.has(run.key));
  useEffect(() => {
    if (!following) return;
    const timer = window.setInterval(() => {
      void api.listVideos().then((videos) => {
        setRuns((current) => current.map((run) => {
          const id = videoIds.current.get(run.key);
          const video = id === undefined || roundPovDone(run) ? undefined : videos.find((candidate) => candidate.id === id);
          return video === undefined ? run : roundPovFollowing(run, video);
        }));
      }, () => undefined);
    }, VIDEO_POLL_MS);
    return () => window.clearInterval(timer);
  }, [following]);

  return { runs, record };
}
