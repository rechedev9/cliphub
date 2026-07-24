'use client';

import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { AlertTriangle } from 'lucide-react';
import {
  streamsApi,
  STREAM_VARIANTS,
  type StreamEditPlan,
  type StreamJob,
  type StreamRenderState,
  type StreamVariant,
} from '@/lib/api/streams';
import { captionsNeedReview, streamHasAudio } from '@/lib/caption-review';
import { clipEditIssue, streamRangesIssue } from '@/lib/clip-edit';
import { killfeedAnalysisNeeded } from '@/lib/killfeed-analysis';
import { fitPlanToSourceDuration, normalizeKillfeedPlan } from '@/lib/killfeed-plan';
import {
  loadStreamDraft,
  reconcileStreamDraftAfterSave,
  recoverableStreamJobs,
  saveStreamDraft,
  selectStreamDraftPlan,
  streamEditPlanFingerprint,
} from '@/lib/stream-draft';
import { isCurrentStreamEditorLoad, nextStreamEditorLoad, type StreamEditorLoad } from '@/lib/stream-editor-load';
import { streamRenderCanRetry, streamRenderNeedsKillfeedReanalysis } from '@/lib/stream-recovery';
import {
  STREAMER_NICK_RE,
  STREAM_OFFLINE_MESSAGE,
  blankClip,
  blankPlan,
  errorMessage,
  isServiceUnavailable,
  isStreamURLValidationError,
  nonVideoExtension,
  sleep,
} from '@/lib/streams/plan';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Button } from '@/components/ui/button';
import { StreamAcquiringCard } from '@/components/streams/acquiring-card';
import { StreamEditor } from '@/components/streams/stream-editor';
import { StreamSourceCard } from '@/components/streams/source-card';


type Stage = 'idle' | 'submitting' | 'acquiring' | 'editing' | 'rendering' | 'rendered' | 'failed';

/**
 * Stream Clips (/streams) — paste a Twitch clip/VOD URL or upload an MP4, then
 * lay out the facecam over gameplay and cut clip ranges before rendering
 * vertical Shorts. Mirrors /upload's stage machine (submit → wait → edit) but
 * against the /api/streams/* proxy, which forwards to the orchestrator's
 * stream-jobs pipeline (acquire/probe → edit plan → render).
 *
 * This file owns the stage machine, the polling loops and the autosave chain
 * only; every surface it dispatches to lives in `components/streams/`.
 */
export default function StreamsPage() {
  return <LocalStreamsPage />;
}

function LocalStreamsPage() {
  const [stage, setStage] = useState<Stage>('idle');
  const [job, setJob] = useState<StreamJob | null>(null);
  const [plan, setPlan] = useState<StreamEditPlan | null>(null);
  const [renderState, setRenderState] = useState<StreamRenderState | null>(null);
  /** The exact plan the shown render used; drives URLs and staleness. */
  const [renderedPlan, setRenderedPlan] = useState<StreamEditPlan | null>(null);
  const [sourceUrl, setSourceUrl] = useState('');
  const [title, setTitle] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [failureReason, setFailureReason] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [recoverableJobs, setRecoverableJobs] = useState<StreamJob[]>([]);
  const autosaveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const autosaveGeneration = useRef(0);
  const autosaveChain = useRef<Promise<void>>(Promise.resolve());
  const serverPlanFingerprint = useRef<{ jobId: string; fingerprint: string } | null>(null);
  const draftSessionId = useRef('');
  const draftRevision = useRef(0);
  const editorLoad = useRef<StreamEditorLoad>({ generation: 0, jobId: '' });

  const pollGen = useRef(0);

  const reset = useCallback((message: string) => {
    pollGen.current += 1;
    editorLoad.current = nextStreamEditorLoad(editorLoad.current, '');
    setError(message);
    setStage('idle');
    setJob(null);
    setPlan(null);
    setRenderState(null);
    setRenderedPlan(null);
    setFailureReason(null);
    serverPlanFingerprint.current = null;
  }, []);

  const loadEditor = useCallback(async (j: StreamJob) => {
    const requestedLoad = nextStreamEditorLoad(editorLoad.current, j.id);
    editorLoad.current = requestedLoad;
    draftSessionId.current = window.crypto.randomUUID();
    draftRevision.current = 0;
    setJob(j);
    const duration = j.probe?.duration_seconds ?? 0;
    try {
      const browserDraft = typeof window === 'undefined' ? null : loadStreamDraft(window.localStorage, j.id);
      const serverPlan = j.edit_plan ?? (await streamsApi.getEditPlan(j.id));
      if (!isCurrentStreamEditorLoad(requestedLoad, editorLoad.current)) return;
      serverPlanFingerprint.current = { jobId: j.id, fingerprint: streamEditPlanFingerprint(serverPlan) };
      const loadedPlan = fitPlanToSourceDuration(
        normalizeKillfeedPlan(selectStreamDraftPlan(browserDraft, serverPlan) ?? serverPlan),
        duration,
      );
      if (j.title?.trim() && loadedPlan.clips[0] && !loadedPlan.clips[0].title?.trim()) {
        loadedPlan.clips[0] = { ...loadedPlan.clips[0], title: j.title.trim() };
      }
      setPlan(
        loadedPlan.clips.length > 0 ? loadedPlan : { ...loadedPlan, clips: [blankClip(duration)] },
      );
    } catch {
      if (!isCurrentStreamEditorLoad(requestedLoad, editorLoad.current)) return;
      setPlan(blankPlan(duration));
    }
    if (!isCurrentStreamEditorLoad(requestedLoad, editorLoad.current)) return;
    setStage('editing');
  }, []);

  const pollAcquiring = useCallback(
    async (jobId: string) => {
      const gen = ++pollGen.current;
      for (let attempt = 0; attempt < 200; attempt++) {
        await sleep(1200);
        if (pollGen.current !== gen) return; // superseded by a new submission/reset
        try {
          const j = await streamsApi.getJob(jobId);
          if (!j) {
            reset('Ese trabajo ya no está disponible.');
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
            reset(STREAM_OFFLINE_MESSAGE);
            return;
          }
          // transient network hiccup; keep polling
        }
      }
      reset('Se agotó el tiempo esperando a que el vídeo de origen estuviera listo.');
    },
    [loadEditor, reset],
  );

  const resumeJob = useCallback(
    (candidate: StreamJob) => {
      setError(null);
      setJob(candidate);
      if (candidate.status === 'acquiring') {
        setStage('acquiring');
        void pollAcquiring(candidate.id);
        return;
      }
      void loadEditor(candidate);
    },
    [loadEditor, pollAcquiring],
  );

  const submitUrl = useCallback(async () => {
    const trimmed = sourceUrl.trim();
    if (!trimmed) {
      setError('Pega una URL de clip o VOD de Twitch o YouTube. Para un archivo local, usa un MP4.');
      return;
    }
    const badExt = nonVideoExtension(trimmed);
    if (badExt) {
      setError(
        `Esa URL apunta a un archivo .${badExt}, no a un vídeo. Pega el enlace de un clip o VOD de Twitch o YouTube, o usa “Subir un MP4”.`,
      );
      return;
    }
    setError(null);
    setStage('submitting');
    try {
      const j = await streamsApi.createFromUrl({ sourceUrl: trimmed, title: title.trim() || undefined });
      if (j.status === 'acquiring') {
        setJob(j);
        setStage('acquiring');
        void pollAcquiring(j.id);
      } else {
        void loadEditor(j);
      }
    } catch (err) {
      reset(errorMessage(err, 'No se pudo iniciar ese trabajo. Revisa la URL y vuelve a intentarlo.'));
    }
  }, [sourceUrl, title, pollAcquiring, loadEditor, reset]);

  const submitFile = useCallback(
    async (file: File) => {
      setError(null);
      setStage('submitting');
      try {
        const j = await streamsApi.createFromFile(file, title.trim() || undefined);
        if (j.status === 'acquiring') {
          setJob(j);
          setStage('acquiring');
          void pollAcquiring(j.id);
        } else {
          void loadEditor(j);
        }
      } catch (err) {
        reset(errorMessage(err, 'No se pudo procesar ese archivo. Prueba con otro MP4.'));
      }
    },
    [title, pollAcquiring, loadEditor, reset],
  );

  const pollRender = useCallback(
    async (jobId: string, variant: StreamVariant, attemptedPlan: StreamEditPlan) => {
      const gen = ++pollGen.current;
      for (let attempt = 0; attempt < 300; attempt++) {
        try {
          const state = await streamsApi.getRenderState(jobId, variant);
          if (pollGen.current !== gen) return;
          setRenderState(state);
          if (state.status === 'rendered') {
            setRenderedPlan(attemptedPlan);
            setStage('rendered');
            return;
          }
          if (state.status === 'failed') {
            if (streamRenderNeedsKillfeedReanalysis(state)) {
              setStage('editing');
              setError(
                state.error ||
                  'Las capturas exactas de la killfeed ya no están disponibles. Reanaliza la killfeed y vuelve a crear los Shorts.',
              );
              return;
            }
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
            reset(STREAM_OFFLINE_MESSAGE);
            return;
          }
        }
        await sleep(1500);
        if (pollGen.current !== gen) return;
      }
      setStage('failed');
      setFailureReason('se agotó el tiempo esperando a que terminara el render');
    },
    [reset],
  );

  const clearRecoverableKillfeedRender = useCallback(() => {
    if (!streamRenderNeedsKillfeedReanalysis(renderState)) return;
    setRenderState((current) => (current?.published ? { ...current, error: undefined, error_code: undefined } : null));
    setError(null);
  }, [renderState]);

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
    const editIssue = clipEditIssue(fittedPlan.clips);
    if (editIssue !== null) {
      setError(editIssue);
      return;
    }
    const sourceHasAudio = streamHasAudio(job.probe);
    if (captionsNeedReview(fittedPlan, sourceHasAudio)) {
      setError('Revisa los subtítulos de cada clip con audio antes de crear los Shorts.');
      return;
    }
    if (killfeedAnalysisNeeded(fittedPlan)) {
      setError('Espera a que termine el análisis automático de la killfeed antes de crear los Shorts.');
      return;
    }
    setError(null);
    setSaving(true);
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
      setPlan(saved);
      setStage('rendering');
      setRenderState((previous) =>
        previous && (previous.published || previous.status === 'rendered')
          ? { ...previous, published: true, status: 'queued' }
          : { status: 'queued', videos: [] },
      );
      await streamsApi.startRender(job.id, saved.variant);
      void pollRender(job.id, saved.variant, saved);
    } catch (err) {
      setStage('editing');
      setError(errorMessage(err, 'No se pudo iniciar el render.'));
    } finally {
      setSaving(false);
    }
  }, [job, plan, pollRender]);

  useEffect(() => {
    let active = true;
    void streamsApi
      .listJobs()
      .then((jobs) => {
        if (active) setRecoverableJobs(recoverableStreamJobs(jobs).slice(0, 5));
      })
      .catch(() => {
        // Source creation remains available if the recent-job read fails.
      });
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    if (!job || !plan || (stage !== 'editing' && stage !== 'rendered')) return;
    const revision = ++draftRevision.current;
    const submittedRevision = { editorSessionId: draftSessionId.current, revision };
    const requestedLoad = editorLoad.current;
    if (typeof window !== 'undefined') {
      saveStreamDraft(
        window.localStorage,
        job.id,
        plan,
        undefined,
        serverPlanFingerprint.current?.jobId === job.id
          ? serverPlanFingerprint.current.fingerprint
          : streamEditPlanFingerprint(plan),
        submittedRevision,
      );
    }
    const generation = ++autosaveGeneration.current;
    if (autosaveTimer.current !== null) clearTimeout(autosaveTimer.current);
    autosaveTimer.current = setTimeout(() => {
      autosaveTimer.current = null;
      autosaveChain.current = autosaveChain.current
        .catch(() => undefined)
        .then(async () => {
          if (autosaveGeneration.current !== generation) return;
          const saved = await streamsApi.putEditPlan(job.id, plan);
          if (typeof window !== 'undefined') {
            reconcileStreamDraftAfterSave(window.localStorage, job.id, plan, saved, submittedRevision);
          }
          if (isCurrentStreamEditorLoad(requestedLoad, editorLoad.current)) {
            serverPlanFingerprint.current = { jobId: job.id, fingerprint: streamEditPlanFingerprint(saved) };
          }
        });
      void autosaveChain.current.catch(() => {
        // The synchronous local draft still protects navigation/restart recovery.
      });
    }, 500);
    return () => {
      if (autosaveTimer.current !== null) {
        clearTimeout(autosaveTimer.current);
        autosaveTimer.current = null;
      }
    };
  }, [job, plan, stage]);

  useEffect(() => {
    return () => {
      pollGen.current += 1; // stop any in-flight poll loop on unmount
    };
  }, []);

  let stageContent: ReactNode;
  if (stage === 'idle' || stage === 'submitting') {
    stageContent = (
      <StreamSourceCard
        sourceUrl={sourceUrl}
        title={title}
        submitting={stage === 'submitting'}
        error={error}
        recoverableJobs={recoverableJobs}
        onSourceUrlChange={(value) => {
          setSourceUrl(value);
          if (isStreamURLValidationError(error)) setError(null);
        }}
        onTitleChange={setTitle}
        onSubmitUrl={() => void submitUrl()}
        onSubmitFile={(f) => void submitFile(f)}
        onResume={resumeJob}
      />
    );
  } else if (stage === 'acquiring') {
    stageContent = <StreamAcquiringCard title={job?.title} />;
  } else if (stage === 'failed') {
    stageContent = (
      <div role="alert">
        <StudioEmptyState
          icon={AlertTriangle}
          accent="magenta"
          title="Ese trabajo falló"
          description={failureReason ?? 'Algo salió mal.'}
          className="border-destructive/45"
          actions={
            <Button type="button" variant="hero" onClick={() => reset('')}>
              EMPEZAR DE NUEVO
            </Button>
          }
        />
      </div>
    );
  } else if (job && plan) {
    stageContent = (
      <StreamEditor
        job={job}
        plan={plan}
        onPlanChange={setPlan}
        stage={stage}
        renderState={renderState}
        renderedPlan={renderedPlan}
        error={error}
        saving={saving}
        onCreate={() => void createShorts()}
        onKillfeedAnalysisRecovered={clearRecoverableKillfeedRender}
        onStartOver={() => reset('')}
      />
    );
  } else {
    stageContent = null;
  }

  return (
    <div className="flex flex-col gap-8">
      <StudioPageHeader
        title="DE STREAM A SHORT"
        description={
          <p>
            Pega un clip de Twitch o YouTube, o sube un MP4. Córtalo en vertical con tu facecam y
            conserva la killfeed original cuando la necesites.
          </p>
        }
      />

      {stageContent}
    </div>
  );
}
