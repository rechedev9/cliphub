'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import {
  streamsApi,
  type CaptionGenerationState,
  type StreamCaptionWord,
  type StreamClipRange,
  type StreamEditPlan,
} from '@/lib/api/streams';
import {
  captionDraftDiffersFromReview,
  captionsNeedReview,
  captionWordsIssue,
  invalidateCaptionReview,
} from '@/lib/caption-review';
import { clipEditIssue } from '@/lib/clip-edit';
import {
  captionDraftsFromState,
  captionGenerationIsPending,
  clipsAreValid,
  errorMessage,
  sleep,
} from '@/lib/streams/plan';

export interface CaptionReview {
  state: CaptionGenerationState | null;
  drafts: Record<string, StreamCaptionWord[]>;
  error: string | null;
  loading: boolean;
  requestBusy: boolean;
  generating: boolean;
  reviewingClipId: string | null;
  /** Generates candidates for the saved ranges. */
  generate: () => Promise<void>;
  /** Stops waiting on a running generation without touching the plan. */
  cancelWait: () => void;
  updateDraft: (clipId: string, words: StreamCaptionWord[]) => void;
  review: (clip: StreamClipRange, noSpeech: boolean) => Promise<void>;
  /**
   * Drops candidates whose clip inputs no longer match, and explains why. The
   * caller runs this from its own clip setter so the discard happens in the
   * same update as the range change.
   */
  discardCandidates: (nextPlan: StreamEditPlan) => void;
}

/**
 * The caption-candidate subsystem: generation, polling, per-clip drafts and the
 * human review that turns evidence into publishable text.
 *
 * Extracted verbatim from the 2996-line page so the editor composes panels
 * instead of owning eleven pieces of caption state. Timing, poll cadence,
 * generation counters and every API call are unchanged: machine words never
 * reach the plan without an explicit approval or a no-speech confirmation.
 */
export function useCaptionReview({
  jobId,
  plan,
  sourceHasAudio,
  onPlanChange,
}: {
  jobId: string;
  plan: StreamEditPlan;
  sourceHasAudio: boolean;
  onPlanChange: (plan: StreamEditPlan) => void;
}): CaptionReview {
  const [state, setState] = useState<CaptionGenerationState | null>(null);
  const [drafts, setDrafts] = useState<Record<string, StreamCaptionWord[]>>({});
  const [loading, setLoading] = useState(false);
  const [requestBusy, setRequestBusy] = useState(false);
  const [reviewingClipId, setReviewingClipId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const pollGen = useRef(0);

  const captionsEnabled = plan.captions?.enabled;
  const clips = plan.clips;

  const pollState = useCallback(
    async (initial?: CaptionGenerationState): Promise<CaptionGenerationState> => {
      const gen = ++pollGen.current;
      let next = initial ?? (await streamsApi.getCaptionGenerationState(jobId));
      if (pollGen.current !== gen) return next;
      setState(next);
      for (let attempt = 0; attempt < 240 && captionGenerationIsPending(next); attempt++) {
        await sleep(1250);
        if (pollGen.current !== gen) return next;
        next = await streamsApi.getCaptionGenerationState(jobId);
        if (pollGen.current !== gen) return next;
        setState(next);
      }
      if (pollGen.current === gen && captionGenerationIsPending(next)) {
        setState(null);
        throw new Error('El análisis de subtítulos sigue pendiente. Actualiza el estado o inténtalo de nuevo.');
      }
      if (pollGen.current === gen && !captionGenerationIsPending(next)) {
        setDrafts(captionDraftsFromState(next));
      }
      return next;
    },
    [jobId],
  );

  useEffect(() => {
    if (!captionsEnabled) {
      pollGen.current += 1;
      setState(null);
      setDrafts({});
      setLoading(false);
      setError(null);
      return;
    }
    let active = true;
    setLoading(true);
    void pollState()
      .catch((err: unknown) => {
        if (active) {
          setState(null);
          setError(errorMessage(err, 'No se pudo consultar el análisis de subtítulos.'));
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
      pollGen.current += 1;
    };
  }, [jobId, captionsEnabled, pollState]);

  useEffect(() => {
    setDrafts((current) => {
      const next = { ...current };
      for (const clip of clips) {
        if (clip.caption_reviewed && next[clip.id] === undefined) {
          next[clip.id] = (clip.caption_words ?? []).map((word) => ({ ...word }));
        }
      }
      return next;
    });
  }, [clips]);

  const generate = async (): Promise<void> => {
    if (!clipsAreValid(plan.clips)) {
      setError('Corrige los rangos de clip antes de generar subtítulos.');
      return;
    }
    const editIssue = clipEditIssue(plan.clips);
    if (editIssue !== null) {
      setError(editIssue);
      return;
    }
    setRequestBusy(true);
    setError(null);
    setDrafts({});
    try {
      const saved = await streamsApi.putEditPlan(jobId, plan);
      onPlanChange(saved);
      const started = await streamsApi.startCaptionGeneration(jobId);
      await pollState(started);
    } catch (err) {
      setState(null);
      setError(errorMessage(err, 'No se pudieron generar los candidatos de subtítulos.'));
    } finally {
      setRequestBusy(false);
    }
  };

  const cancelWait = (): void => {
    pollGen.current += 1;
    setRequestBusy(false);
    setState(null);
    setError('Espera cancelada. El análisis puede continuar en segundo plano; vuelve a consultar cuando quieras.');
  };

  const updateDraft = (clipId: string, words: StreamCaptionWord[]): void => {
    setDrafts((current) => ({ ...current, [clipId]: words }));
    const clip = plan.clips.find((item) => item.id === clipId);
    if (clip && captionDraftDiffersFromReview(clip, words)) {
      onPlanChange({
        ...plan,
        clips: plan.clips.map((item) => (item.id === clipId ? invalidateCaptionReview(item) : item)),
      });
    }
  };

  const review = async (clip: StreamClipRange, noSpeech: boolean): Promise<void> => {
    if (!state || !state.generation_id) {
      setError('Genera candidatos actuales antes de guardar la revisión.');
      return;
    }
    const words = noSpeech ? [] : (drafts[clip.id] ?? []).map((word) => ({ ...word, word: word.word.trim() }));
    if (!noSpeech) {
      const issue = captionWordsIssue(words, clip.end_seconds - clip.start_seconds);
      if (issue !== null) {
        setError(issue);
        return;
      }
    }
    setReviewingClipId(clip.id);
    setError(null);
    try {
      const saved = await streamsApi.reviewCaptionCandidates(jobId, state.generation_id, [
        { clip_id: clip.id, words, no_speech: noSpeech || undefined },
      ]);
      onPlanChange(saved);
      setDrafts((current) => ({ ...current, [clip.id]: words }));
      try {
        setState(await streamsApi.getCaptionGenerationState(jobId));
      } catch {
        // The reviewed plan is already durable; a later page load can refresh its candidate state.
      }
    } catch (err) {
      setError(errorMessage(err, 'No se pudo guardar la revisión de este clip.'));
    } finally {
      setReviewingClipId(null);
    }
  };

  const discardCandidates = (nextPlan: StreamEditPlan): void => {
    pollGen.current += 1;
    setState(null);
    setDrafts({});
    if (captionsNeedReview(nextPlan, sourceHasAudio)) {
      setError('El rango o el audio del clip cambió. Genera candidatos nuevos antes de renderizar.');
    }
  };

  return {
    state,
    drafts,
    error,
    loading,
    requestBusy,
    generating: captionGenerationIsPending(state),
    reviewingClipId,
    generate,
    cancelWait,
    updateDraft,
    review,
    discardCandidates,
  };
}
