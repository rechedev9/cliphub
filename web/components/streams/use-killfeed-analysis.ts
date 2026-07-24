'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import {
  KILLFEED_ANALYSIS_STATUS,
  streamsApi,
  type KillfeedAnalysisState,
  type StreamClipRange,
  type StreamEditPlan,
} from '@/lib/api/streams';
import {
  appliedKillfeedEventReference,
  killfeedAnalysisInputsFingerprint,
  killfeedAnalysisNeeded,
  killfeedStateNeedsRefreshForRead,
} from '@/lib/killfeed-analysis';
import { applyClipKillfeedRead } from '@/lib/killfeed-plan';
import {
  clipsAreValid,
  errorMessage,
  isMissingXaiKey,
  killfeedAnalysisIsPending,
  sleep,
} from '@/lib/streams/plan';

/** Stable key for one cue inside one clip. */
export function killfeedCueKey(clipId: string, cue: number): string {
  return `${clipId}@${cue}`;
}

export interface KillfeedAnalysis {
  state: KillfeedAnalysisState | null;
  error: string | null;
  requestBusy: boolean;
  analyzing: boolean;
  weapons: string[];
  readingCueKey: string | null;
  readErrors: Record<string, string>;
  readNotice: string | null;
  /** Runs analysis now against `candidatePlan`, saving it first. */
  run: (candidatePlan: StreamEditPlan) => Promise<void>;
  /** Debounced run, used after a crop or range edit settles. */
  schedule: (candidatePlan: StreamEditPlan, delay?: number) => void;
  /** Stops polling and clears any pending debounce, leaving the plan alone. */
  cancel: () => void;
  /** Stops waiting on a running analysis and says so. */
  cancelWait: () => void;
  /** Keeps the debounce's view of the plan current without a re-render. */
  noteLatestPlan: (plan: StreamEditPlan) => void;
  /** Reads one cue's kills from the source frame with the vision model. */
  readCue: (clip: StreamClipRange, cue: number) => Promise<void>;
}

/**
 * Automatic killfeed analysis: the debounced per-frame pass, its polling and
 * apply step, the weapon catalog, and the AI read of a single cue.
 *
 * Moved out of the page unchanged. The invariants it protects are subtle and
 * expensive to rediscover: the cue is stored against the first verifiable
 * source frame, a read must run against the SAVED plan (the endpoint reads
 * durable state, not the editor's memory), and a generation change between save
 * and read has to fail loudly rather than align a cue to a stale frame.
 */
export function useKillfeedAnalysis({
  jobId,
  plan,
  enabled,
  onPlanChange,
  onAnalysisRecovered,
  onCueAligned,
}: {
  jobId: string;
  plan: StreamEditPlan;
  enabled: boolean;
  onPlanChange: (plan: StreamEditPlan) => void;
  onAnalysisRecovered: () => void;
  /** Moves the shared preview cursor onto a newly aligned event. */
  onCueAligned: (seconds: number) => void;
}): KillfeedAnalysis {
  const [state, setState] = useState<KillfeedAnalysisState | null>(null);
  const [requestBusy, setRequestBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [weapons, setWeapons] = useState<string[]>([]);
  const [readingCueKey, setReadingCueKey] = useState<string | null>(null);
  const [readErrors, setReadErrors] = useState<Record<string, string>>({});
  const [readNotice, setReadNotice] = useState<string | null>(null);
  const pollGen = useRef(0);
  const debounce = useRef<ReturnType<typeof setTimeout> | null>(null);
  const runActive = useRef(false);
  const latestPlan = useRef(plan);
  latestPlan.current = plan;

  const pollState = useCallback(
    async (initial?: KillfeedAnalysisState): Promise<KillfeedAnalysisState> => {
      const gen = ++pollGen.current;
      let next = initial ?? (await streamsApi.getKillfeedAnalysisState(jobId));
      if (pollGen.current !== gen) return next;
      setState(next);
      for (let attempt = 0; attempt < 240 && killfeedAnalysisIsPending(next); attempt++) {
        await sleep(1250);
        if (pollGen.current !== gen) return next;
        next = await streamsApi.getKillfeedAnalysisState(jobId);
        if (pollGen.current !== gen) return next;
        setState(next);
      }
      if (pollGen.current !== gen) return next;
      if (killfeedAnalysisIsPending(next)) {
        setState(null);
        throw new Error('El análisis de killfeed sigue pendiente. Puedes reintentarlo sin perder el plan.');
      }
      if (next.status === KILLFEED_ANALYSIS_STATUS.failed) {
        throw new Error(next.error || 'El análisis automático de killfeed falló.');
      }
      if (next.status === KILLFEED_ANALYSIS_STATUS.reviewRequired) {
        setError('Hay avisos cuyo fotograma de origen no pudo resolverse. Corrígelos o vuelve a analizar.');
        return next;
      }
      if (next.status === KILLFEED_ANALYSIS_STATUS.ready) {
        const appliedPlan = await streamsApi.applyKillfeedAnalysis(jobId, next.generation_id);
        if (pollGen.current !== gen) return next;
        latestPlan.current = appliedPlan;
        onPlanChange(appliedPlan);
        onAnalysisRecovered();
        next = { ...next, status: KILLFEED_ANALYSIS_STATUS.applied };
        setState(next);
      }
      return next;
    },
    [jobId, onAnalysisRecovered, onPlanChange],
  );

  const run = useCallback(
    async (candidatePlan: StreamEditPlan): Promise<void> => {
      if (runActive.current || !candidatePlan.killfeed_crop || !clipsAreValid(candidatePlan.clips)) return;
      runActive.current = true;
      setRequestBusy(true);
      setError(null);
      try {
        const saved = await streamsApi.putEditPlan(jobId, candidatePlan);
        latestPlan.current = saved;
        onPlanChange(saved);
        const started = await streamsApi.startKillfeedAnalysis(jobId);
        await pollState(started);
      } catch (err) {
        setError(errorMessage(err, 'No se pudo analizar automáticamente la killfeed.'));
      } finally {
        runActive.current = false;
        setRequestBusy(false);
      }
    },
    [jobId, onPlanChange, pollState],
  );

  const schedule = useCallback(
    (candidatePlan: StreamEditPlan, delay = 750): void => {
      if (debounce.current !== null) clearTimeout(debounce.current);
      const expectedInputs = killfeedAnalysisInputsFingerprint(candidatePlan);
      latestPlan.current = candidatePlan;
      debounce.current = setTimeout(() => {
        debounce.current = null;
        const current = latestPlan.current;
        if (
          !current.killfeed_crop ||
          !killfeedAnalysisNeeded(current) ||
          killfeedAnalysisInputsFingerprint(current) !== expectedInputs
        ) {
          return;
        }
        void run(current);
      }, delay);
    },
    [run],
  );

  const cancel = useCallback((): void => {
    pollGen.current += 1;
    if (debounce.current !== null) {
      clearTimeout(debounce.current);
      debounce.current = null;
    }
  }, []);

  useEffect(() => {
    if (!enabled) {
      pollGen.current += 1;
      if (debounce.current !== null) {
        clearTimeout(debounce.current);
        debounce.current = null;
      }
      setState(null);
      setError(null);
      return;
    }
    let active = true;
    void streamsApi
      .getKillfeedAnalysisState(jobId)
      .then(async (next) => {
        if (!active) return;
        setState(next);
        if (killfeedAnalysisIsPending(next) || next.status === KILLFEED_ANALYSIS_STATUS.ready) {
          await pollState(next);
          return;
        }
        if (
          (next.status === KILLFEED_ANALYSIS_STATUS.none || next.status === KILLFEED_ANALYSIS_STATUS.applied) &&
          killfeedAnalysisNeeded(latestPlan.current)
        ) {
          schedule(latestPlan.current);
        }
      })
      .catch((err: unknown) => {
        if (active) setError(errorMessage(err, 'No se pudo consultar el análisis de killfeed.'));
      });
    return () => {
      active = false;
      pollGen.current += 1;
      if (debounce.current !== null) {
        clearTimeout(debounce.current);
        debounce.current = null;
      }
    };
  }, [jobId, enabled, pollState, schedule]);

  useEffect(() => {
    if (!enabled || weapons.length > 0) return;
    let active = true;
    streamsApi
      .listKillfeedWeapons()
      .then((next) => {
        if (active) setWeapons(next);
      })
      .catch(() => {
        // The weapon <select> stays empty; a render still validates server-side.
      });
    return () => {
      active = false;
    };
  }, [enabled, weapons.length]);

  const readCue = async (clip: StreamClipRange, cue: number): Promise<void> => {
    const key = killfeedCueKey(clip.id, cue);
    setReadingCueKey(key);
    setReadErrors((prev) => {
      const { [key]: _removed, ...rest } = prev;
      return rest;
    });
    try {
      // Persist first so the orchestrator can locate this clip/cue for the job;
      // the read endpoint reads the saved plan, not the in-memory edits.
      const saved = await streamsApi.putEditPlan(jobId, plan);
      let resolvedState = state;
      if (killfeedStateNeedsRefreshForRead(saved, resolvedState)) {
        resolvedState = await streamsApi.getKillfeedAnalysisState(jobId);
        setState(resolvedState);
      }
      if (killfeedStateNeedsRefreshForRead(saved, resolvedState)) {
        throw new Error('La generación exacta de killfeed cambió. Recarga el análisis antes de leer esta marca.');
      }
      const eventReference = appliedKillfeedEventReference(saved, resolvedState, clip.id, cue);
      const read = await streamsApi.readKillfeed(jobId, clip.id, cue, eventReference);
      const clips = saved.clips.map((c) => (c.id === clip.id ? applyClipKillfeedRead(c, cue, read.events) : c));
      onPlanChange({ ...saved, clips });
      const reviewNote = read.review_required
        ? ` ${read.warnings?.join(' ') || 'Revisa manualmente el resultado antes de renderizar.'}`
        : '';
      if (read.aligned && read.events.length > 0) {
        const newest = read.events[read.events.length - 1];
        onCueAligned(newest.cue_seconds);
        setReadNotice(
          `IA ajustó ${read.events.length === 1 ? 'la marca' : `${read.events.length} marcas`} al instante real de ${read.events.length === 1 ? 'la kill' : 'las kills'}.${reviewNote}`,
        );
      } else {
        setReadNotice(`No se pudo detectar el borde temporal; se conservó la marca elegida.${reviewNote}`);
      }
    } catch (err) {
      const message = isMissingXaiKey(err)
        ? 'Configura tu clave de xAI en Ajustes para leer la killfeed con IA.'
        : errorMessage(err, 'No se pudieron leer las kills de esta marca.');
      setReadErrors((prev) => ({ ...prev, [key]: message }));
    } finally {
      setReadingCueKey(null);
    }
  };

  const cancelWait = (): void => {
    pollGen.current += 1;
    setRequestBusy(false);
    setState(null);
    setError('Espera cancelada. El análisis puede continuar en segundo plano; puedes retomarlo después.');
  };

  const noteLatestPlan = (next: StreamEditPlan): void => {
    latestPlan.current = next;
  };

  return {
    state,
    error,
    requestBusy,
    analyzing: killfeedAnalysisIsPending(state),
    weapons,
    readingCueKey,
    readErrors,
    readNotice,
    run,
    schedule,
    cancel,
    cancelWait,
    noteLatestPlan,
    readCue,
  };
}
