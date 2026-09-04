'use client';

import { use, useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { AlertTriangle, Film } from 'lucide-react';
import { toast } from 'sonner';
import {
  streamsApi,
  STREAM_VARIANTS,
  type StreamEditPlan,
  type StreamJob,
  type StreamRenderState,
  type StreamVariant,
} from '@/lib/api/streams';
import { clipEditIssue, streamRangesIssue } from '@/lib/clip-edit';
import {
  loadStreamDraft,
  reconcileStreamDraftAfterSave,
  saveStreamDraft,
  selectStreamDraftPlan,
  streamEditPlanFingerprint,
} from '@/lib/stream-draft';
import { isCurrentStreamEditorLoad, nextStreamEditorLoad, type StreamEditorLoad } from '@/lib/stream-editor-load';
import { streamRenderCanRetry } from '@/lib/stream-recovery';
import { shouldAutosaveStreamPlan, type AckedStreamPlanFingerprint } from '@/lib/streams/autosave';
import {
  STREAMER_NICK_RE,
  STREAM_OFFLINE_MESSAGE,
  errorMessage,
  fitPlanToSourceDuration,
  isServiceUnavailable,
  sleep,
  withDefaultStreamTitle,
} from '@/lib/streams/plan';
import { shortsWord, streamVariantLabel } from '@/lib/streams/editor';
import { affiliateFamilyLabel, isAffiliateStyle, stylesForFamily } from '@/lib/api/types';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { LongOperation } from '@/components/studio/long-operation';
import { Button } from '@/components/ui/button';
import { StreamEditor } from '@/components/streams/stream-editor';
import type { StreamAutosaveState } from '@/components/streams/stream-steps-rail';
import { useElapsedSeconds } from '@/components/streams/use-elapsed-seconds';

const STREAMS_HREF = '/streams';

type Stage = 'loading' | 'acquiring' | 'editing' | 'rendering' | 'rendered' | 'failed' | 'missing' | 'error';

/** /streams/[id]: the stage machine for one job; UI lives in components/streams. */
export default function StreamEditorPage({ params }: { params: Promise<{ id: string }> }): ReactNode {
  const { id } = use(params);
  const router = useRouter();
  const [stage, setStage] = useState<Stage>('loading');
  const [openAttempt, setOpenAttempt] = useState(0);
  const [job, setJob] = useState<StreamJob | null>(null);
  const [plan, setPlan] = useState<StreamEditPlan | null>(null);
  const [renderState, setRenderState] = useState<StreamRenderState | null>(null);
  /** The exact plan the shown render used; drives URLs and staleness. */
  const [renderedPlan, setRenderedPlan] = useState<StreamEditPlan | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [failureReason, setFailureReason] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [autosave, setAutosave] = useState<StreamAutosaveState>('saved');
  const autosaveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const autosaveGeneration = useRef(0);
  const autosaveChain = useRef<Promise<void>>(Promise.resolve());
  const autosaveInFlight = useRef(false);
  /** The banner text the last failed autosave wrote, so a later success can clear only that. */
  const autosaveError = useRef<string | null>(null);
  const serverPlanFingerprint = useRef<AckedStreamPlanFingerprint>(null);
  const draftSessionId = useRef('');
  const draftRevision = useRef(0);
  const editorLoad = useRef<StreamEditorLoad>({ generation: 0, jobId: '' });
  const pollGen = useRef(0);
  const acquiringElapsed = useElapsedSeconds(stage === 'acquiring');

  const fail = useCallback((message: string) => {
    pollGen.current += 1;
    editorLoad.current = nextStreamEditorLoad(editorLoad.current, '');
    setError(message);
    setStage('error');
    setPlan(null);
    setRenderState(null);
    setRenderedPlan(null);
    setFailureReason(null);
    serverPlanFingerprint.current = null;
  }, []);

  const loadEditor = useCallback(async (j: StreamJob, nextStage: 'editing' | 'rendering' = 'editing'): Promise<StreamEditPlan | null> => {
    const requestedLoad = nextStreamEditorLoad(editorLoad.current, j.id);
    editorLoad.current = requestedLoad;
    draftSessionId.current = window.crypto.randomUUID();
    draftRevision.current = 0;
    setJob(j);
    const duration = j.probe?.duration_seconds ?? 0;
    try {
      const browserDraft = typeof window === 'undefined' ? null : loadStreamDraft(window.localStorage, j.id);
      const serverPlan = j.edit_plan ?? (await streamsApi.getEditPlan(j.id));
      if (!isCurrentStreamEditorLoad(requestedLoad, editorLoad.current)) return null;
      serverPlanFingerprint.current = { jobId: j.id, fingerprint: streamEditPlanFingerprint(serverPlan) };
      // A browser draft is editable state only. An admitted/in-flight render is
      // bound to the persisted server revision and variant.
      const selectedPlan =
        nextStage === 'rendering' ? serverPlan : (selectStreamDraftPlan(browserDraft, serverPlan) ?? serverPlan);
      const editorPlan = withDefaultStreamTitle(
        fitPlanToSourceDuration(selectedPlan, duration),
        j.title,
        nextStage === 'editing',
      );
      setPlan(editorPlan);
      setStage(nextStage);
      return editorPlan;
    } catch (err) {
      if (!isCurrentStreamEditorLoad(requestedLoad, editorLoad.current)) return null;
      fail(errorMessage(err, 'No se pudo cargar el plan guardado. Vuelve a abrir el trabajo para reintentarlo.'));
      return null;
    }
  }, [fail]);

  const pollAcquiring = useCallback(
    async (jobId: string) => {
      const gen = ++pollGen.current;
      for (let attempt = 0; attempt < 200; attempt++) {
        await sleep(1200);
        if (pollGen.current !== gen) return; // superseded by a reset or unmount
        try {
          const j = await streamsApi.getJob(jobId);
          if (!j) {
            setStage('missing');
            return;
          }
          if (j.status === 'failed') {
            setJob(j);
            setFailureReason(j.failure_reason || 'no se pudo obtener el vídeo de origen');
            setStage('failed');
            return;
          }
          if (j.status !== 'acquiring') {
            void loadEditor(j);
            return;
          }
        } catch (err) {
          if (isServiceUnavailable(err)) {
            fail(STREAM_OFFLINE_MESSAGE);
            return;
          }
          // transient network hiccup; keep polling
        }
      }
      fail('Se agotó el tiempo esperando a que el vídeo de origen estuviera listo.');
    },
    [loadEditor, fail],
  );

  const pollRender = useCallback(
    async (jobId: string, variant: StreamVariant, attemptedPlan: StreamEditPlan, announce: boolean) => {
      const gen = ++pollGen.current;
      // No attempt cap: a real render can outlast any fixed budget, and a
      // capped loop previously reported a live render as failed. `pollGen`
      // still stops this loop on unmount, reset, or a superseding poll.
      for (;;) {
        try {
          const state = await streamsApi.getRenderState(jobId, variant);
          if (pollGen.current !== gen) return;
          setRenderState(state);
          if (state.status === 'rendered') {
            setRenderedPlan(attemptedPlan);
            setStage('rendered');
            if (announce) toast(`${shortsWord(state.videos.length)} listos`, { description: 'Descárgalos en Resultados' });
            return;
          }
          if (state.status === 'failed') {
            if (streamRenderCanRetry(state)) {
              setStage('editing');
              setError(
                state.error ||
                  (state.published
                    ? 'El nuevo render falló. La última versión publicada sigue disponible; revisa el plan y vuelve a intentarlo.'
                    : 'El plan cambió antes de publicar el render. Revísalo y vuelve a crear los Shorts.'),
              );
              return;
            }
            setStage('failed');
            setFailureReason(state.error || 'el render falló');
            return;
          }
        } catch (err) {
          if (isServiceUnavailable(err)) {
            fail(STREAM_OFFLINE_MESSAGE);
            return;
          }
        }
        await sleep(1500);
        if (pollGen.current !== gen) return;
      }
    },
    [fail],
  );

  const openJob = useCallback(
    (candidate: StreamJob) => {
      setError(null);
      setJob(candidate);
      if (candidate.status === 'acquiring') {
        setStage('acquiring');
        void pollAcquiring(candidate.id);
        return;
      }
      if (candidate.status === 'failed') {
        setFailureReason(candidate.failure_reason || 'no se pudo obtener el vídeo de origen');
        setStage('failed');
        return;
      }
      const resumesRender = candidate.status === 'rendering' || candidate.status === 'rendered';
      void loadEditor(candidate, resumesRender ? 'rendering' : 'editing').then((loadedPlan) => {
        if (!resumesRender || loadedPlan === null) return;
        void pollRender(candidate.id, loadedPlan.variant, loadedPlan, candidate.status === 'rendering');
      });
    },
    [loadEditor, pollAcquiring, pollRender],
  );

  useEffect(() => {
    let active = true;
    setStage('loading');
    streamsApi
      .getJob(id)
      .then((candidate) => {
        if (!active) return;
        if (candidate === null) setStage('missing');
        else openJob(candidate);
      })
      .catch((err: unknown) => {
        if (active) fail(errorMessage(err, 'No se pudo abrir ese stream.'));
      });
    return () => {
      active = false;
      pollGen.current += 1; // stop any in-flight poll loop on unmount
    };
  }, [id, openAttempt, openJob, fail]);

  const createShorts = useCallback(async () => {
    if (!job || !plan) return;
    const fittedPlan = fitPlanToSourceDuration(plan, job.probe?.duration_seconds ?? 0);
    const rangeIssue = streamRangesIssue(fittedPlan.clips, job.probe?.duration_seconds ?? 0);
    if (rangeIssue !== null) {
      setError(rangeIssue);
      return;
    }
    const needsFaceCrop =
      STREAM_VARIANTS.find((variant) => variant.value === fittedPlan.variant)?.needsFaceCrop ?? false;
    if (needsFaceCrop && fittedPlan.face_crop_reviewed !== true) {
      setError('Confirma manualmente el recorte de facecam antes de renderizar; no asumimos que el recorte automático contenga una cara.');
      return;
    }
    if (!STREAMER_NICK_RE.test(fittedPlan.streamer_banner?.nick?.trim() ?? '')) {
      setError('El nick debe tener hasta 25 letras, números o guiones bajos.');
      return;
    }
    const keyDropFamily = fittedPlan.keydrop_banner?.family?.trim() ?? '';
    const keyDropStyle = fittedPlan.keydrop_banner?.style?.trim() ?? '';
    if (keyDropStyle && !isAffiliateStyle(keyDropFamily, keyDropStyle)) {
      const names = stylesForFamily(keyDropFamily).map((entry) => entry.label).join(', ');
      const familyName = affiliateFamilyLabel(keyDropFamily, keyDropStyle) || 'afiliado';
      setError(`Elige un estilo ${familyName} válido (${names}).`);
      return;
    }
    const keyDropCode = fittedPlan.keydrop_banner?.code?.trim() ?? '';
    if (keyDropCode !== '' && !/^[A-Za-z0-9][A-Za-z0-9_-]{0,15}$/.test(keyDropCode)) {
      setError('El código de afiliado debe tener 1–16 letras, números, guiones o guiones bajos.');
      return;
    }
    const editIssue = clipEditIssue(fittedPlan.clips);
    if (editIssue !== null) {
      setError(editIssue);
      return;
    }
    setError(null);
    setSaving(true);
    // Only restored on failure if the optimistic render state below actually
    // ran; a throw earlier (e.g. the missing-revision check) must not clobber
    // whatever render state was already showing.
    let priorRenderState: StreamRenderState | null = null;
    let appliedOptimisticRenderState = false;
    try {
      autosaveGeneration.current += 1;
      if (autosaveTimer.current !== null) {
        clearTimeout(autosaveTimer.current);
        autosaveTimer.current = null;
      }
      await autosaveChain.current.catch(() => undefined);
      const submittedRevision = { editorSessionId: draftSessionId.current, revision: draftRevision.current };
      const saved = await streamsApi.putEditPlan(job.id, fittedPlan);
      if (typeof window !== 'undefined') {
        reconcileStreamDraftAfterSave(window.localStorage, job.id, fittedPlan, saved, submittedRevision);
      }
      serverPlanFingerprint.current = { jobId: job.id, fingerprint: streamEditPlanFingerprint(saved) };
      autosaveError.current = null;
      setPlan(saved);
      setAutosave('saved');
      if (!saved.updated_at) {
        throw new Error('El plan guardado no incluye una revisión verificable.');
      }
      setStage('rendering');
      setRenderState((previous) => {
        priorRenderState = previous;
        appliedOptimisticRenderState = true;
        return previous && (previous.published || previous.status === 'rendered')
          ? { ...previous, published: true, status: 'queued' }
          : { status: 'queued', videos: [] };
      });
      await streamsApi.startRender(job.id, saved.variant, saved.updated_at);
      toast(`${shortsWord(saved.clips.length)} en render`, {
        description: `FFmpeg · ${streamVariantLabel(saved)} · 1080×1920`,
      });
      void pollRender(job.id, saved.variant, saved, true);
    } catch (err) {
      setStage('editing');
      // The render never actually started; the optimistic "queued" state was
      // a lie the POST just disproved, so put the prior render state back.
      if (appliedOptimisticRenderState) setRenderState(priorRenderState);
      setError(errorMessage(err, 'No se pudo iniciar el render.'));
    } finally {
      setSaving(false);
    }
  }, [job, plan, pollRender]);

  useEffect(() => {
    if (!job || !plan || (stage !== 'editing' && stage !== 'rendered')) return;
    const revision = ++draftRevision.current;
    const submittedRevision = { editorSessionId: draftSessionId.current, revision };
    const requestedLoad = editorLoad.current;
    const fingerprint = streamEditPlanFingerprint(plan);
    if (typeof window !== 'undefined') {
      saveStreamDraft(
        window.localStorage,
        job.id,
        plan,
        undefined,
        serverPlanFingerprint.current?.jobId === job.id ? serverPlanFingerprint.current.fingerprint : fingerprint,
        submittedRevision,
      );
    }
    // Skip the PUT when the plan already matches the last server-acknowledged
    // revision: otherwise every open (initial load) and every bare `stage`
    // flip re-PUTs an unchanged plan and rewrites the server artifact.
    if (!shouldAutosaveStreamPlan(job.id, fingerprint, serverPlanFingerprint.current, autosaveInFlight.current)) return;
    const generation = ++autosaveGeneration.current;
    if (autosaveTimer.current !== null) clearTimeout(autosaveTimer.current);
    autosaveTimer.current = setTimeout(() => {
      autosaveTimer.current = null;
      setAutosave('saving');
      autosaveChain.current = autosaveChain.current
        .catch(() => undefined)
        .then(async () => {
          if (autosaveGeneration.current !== generation) return;
          autosaveInFlight.current = true;
          let saved: StreamEditPlan;
          try {
            saved = await streamsApi.putEditPlan(job.id, plan);
          } finally {
            autosaveInFlight.current = false;
          }
          if (typeof window !== 'undefined') {
            reconcileStreamDraftAfterSave(window.localStorage, job.id, plan, saved, submittedRevision);
          }
          if (isCurrentStreamEditorLoad(requestedLoad, editorLoad.current)) {
            serverPlanFingerprint.current = { jobId: job.id, fingerprint: streamEditPlanFingerprint(saved) };
          }
          if (autosaveGeneration.current !== generation) return;
          setAutosave('saved');
          if (autosaveError.current !== null) {
            const stale = autosaveError.current;
            autosaveError.current = null;
            setError((current) => (current === stale ? null : current));
          }
        })
        .catch((err: unknown) => {
          if (autosaveGeneration.current !== generation) return;
          // The local draft above still protects navigation/restart recovery;
          // surface the rejected save instead of silently claiming it landed.
          const message = errorMessage(err, 'No se pudo guardar el plan automáticamente.');
          autosaveError.current = message;
          setAutosave('failed');
          setError(message);
        });
    }, 500);
    return () => {
      if (autosaveTimer.current !== null) {
        clearTimeout(autosaveTimer.current);
        autosaveTimer.current = null;
      }
    };
  }, [job, plan, stage]);

  const goBack = useCallback(() => router.push(STREAMS_HREF), [router]);

  if (stage === 'loading') {
    return (
      <p role="status" className="flex items-center gap-2 font-mono text-meta uppercase tracking-wider text-fg-3">
        <span aria-hidden className="studio-spinner" />
        Abriendo el stream
      </p>
    );
  }
  if (stage === 'missing') {
    return (
      <StudioEmptyState
        icon={Film}
        accent="magenta"
        title="Ese stream ya no está"
        description="El trabajo no existe o se ha borrado. Vuelve a la lista para traer otro clip."
        actions={
          <Button asChild>
            <Link href={STREAMS_HREF}>Volver a Stream clips</Link>
          </Button>
        }
      />
    );
  }
  if (stage === 'error') {
    return (
      <div role="alert">
        <StudioEmptyState
          icon={AlertTriangle}
          accent="magenta"
          title="No se pudo abrir el stream"
          description={error ?? 'Algo salió mal.'}
          className="border-destructive/45"
          actions={
            <>
              <Button type="button" onClick={() => setOpenAttempt((n) => n + 1)}>
                Reintentar
              </Button>
              <Button asChild variant="outline">
                <Link href={STREAMS_HREF}>Volver</Link>
              </Button>
            </>
          }
        />
      </div>
    );
  }
  if (stage === 'acquiring') {
    return (
      <section className="studio-enter studio-panel studio-panel-raised measure-read flex flex-col gap-4 p-5 @[44rem]/content:p-7" aria-label="Trayendo el vídeo">
        <span className="flex items-center gap-2 font-mono text-meta uppercase tracking-widest text-stream-text">
          <span aria-hidden className="studio-spinner" />
          Trayendo el vídeo
        </span>
        <h1 className="font-display text-title font-bold uppercase text-fg-1">
          {job?.title?.trim() || 'Clip de stream'}
        </h1>
        <p className="text-body text-fg-2">
          Descargando y analizando el vídeo de origen en este PC. En cuanto termine se abre el editor con el plan guardado.
        </p>
        <LongOperation stage="Descarga + probe" detail="Sin cortes todavía" elapsedSec={acquiringElapsed} tone="stream" />
        <Button type="button" variant="outline" size="sm" onClick={goBack} className="self-start">
          Volver
        </Button>
      </section>
    );
  }
  if (stage === 'failed') {
    return (
      <div role="alert">
        <StudioEmptyState
          icon={AlertTriangle}
          accent="magenta"
          title="Ese trabajo falló"
          description={failureReason ?? 'Algo salió mal.'}
          className="border-destructive/45"
          actions={
            <Button type="button" onClick={goBack}>
              Empezar de nuevo
            </Button>
          }
        />
      </div>
    );
  }
  if (!job || !plan) return null;

  return (
    <StreamEditor
      job={job}
      plan={plan}
      onPlanChange={setPlan}
      stage={stage}
      renderState={renderState}
      renderedPlan={renderedPlan}
      error={error}
      saving={saving}
      autosave={autosave}
      onCreate={() => void createShorts()}
      onBack={goBack}
    />
  );
}
