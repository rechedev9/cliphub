'use client';

import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { AlertTriangle } from 'lucide-react';
import {
  KILLFEED_ANALYSIS_STATUS,
  STREAM_VARIANTS,
  streamsApi,
  type KillfeedKill,
  type NormalizedRect,
  type StreamClipRange,
  type StreamEditPlan,
  type StreamJob,
  type StreamRenderState,
  type StreamVariant,
} from '@/lib/api/streams';
import { captionInputsFingerprint, captionsNeedReview, invalidateCaptionReview, streamHasAudio } from '@/lib/caption-review';
import {
  killfeedAnalysisInputsFingerprint,
  killfeedAnalysisNeeded,
  killfeedManualCueIssue,
  invalidateKillfeedAnalysis,
} from '@/lib/killfeed-analysis';
import { addClipCue, normalizeKillfeedPlan, removeClipCue, setClipCueKills } from '@/lib/killfeed-plan';
import {
  advanceMontagePlayback,
  clampStreamerBannerPosition,
  representativeFrameTime,
  resolveStreamerBannerPosition,
  startMontagePlayback,
} from '@/lib/stream-preview';
import { canRequestCaptionCandidates, streamRenderNeedsKillfeedReanalysis } from '@/lib/stream-recovery';
import {
  DEFAULT_KILLFEED_CROP,
  STREAMER_NICK_RE,
  clipsAreValid,
  detectedKillfeedEventCount,
  formatStreamTimestamp,
  planFingerprint,
} from '@/lib/streams/plan';
import { StreamFrameSession } from '@/components/streams/stream-frame-session';
import { StreamJobHeader } from '@/components/streams/job-header';
import { StreamLayoutPicker } from '@/components/streams/layout-picker';
import { StreamKillfeedPanel } from '@/components/streams/killfeed-panel';
import { StreamBannerControls } from '@/components/streams/banner-controls';
import { StreamClipEditor } from '@/components/streams/clip-editor';
import { StreamCaptionsPanel } from '@/components/streams/captions-panel';
import { StreamMusicCard } from '@/components/streams/music-card';
import { StreamRenderBar } from '@/components/streams/render-bar';
import { StreamRenderStage } from '@/components/streams/render-stage';
import { StreamRenderResults } from '@/components/streams/render-results';
import { StreamPreviewColumn } from '@/components/streams/preview-column';
import { useCaptionReview } from '@/components/streams/use-caption-review';
import { useKillfeedAnalysis } from '@/components/streams/use-killfeed-analysis';

/**
 * The stream edit workspace: one persisted plan, four panels that write to it,
 * and a live 9:16 monitor that reads it.
 *
 * This component owns the plan setters and the two analysis subsystems; the
 * panels are presentational and receive exactly what they render. The plan is
 * canonical for ranges, order, crop, audio, fades, text, captions, killfeed and
 * music volume — nothing here reaches around it.
 */
export function StreamEditor({
  job,
  plan,
  onPlanChange,
  stage,
  renderState,
  renderedPlan,
  error,
  saving,
  onCreate,
  onKillfeedAnalysisRecovered,
  onStartOver,
}: {
  job: StreamJob;
  plan: StreamEditPlan;
  onPlanChange: (plan: StreamEditPlan) => void;
  stage: 'editing' | 'rendering' | 'rendered';
  renderState: StreamRenderState | null;
  renderedPlan: StreamEditPlan | null;
  error: string | null;
  saving: boolean;
  onCreate: () => void;
  onKillfeedAnalysisRecovered: () => void;
  onStartOver: () => void;
}): ReactNode {
  const videoSrc = streamsApi.sourceUrl(job.id);
  const variantMeta = STREAM_VARIANTS.find((v) => v.value === plan.variant) ?? STREAM_VARIANTS[0];
  const stale = useMemo(
    () => renderedPlan !== null && plan !== null && planFingerprint(renderedPlan) !== planFingerprint(plan),
    [renderedPlan, plan],
  );

  const probedDuration = job.probe?.duration_seconds ?? 0;
  const sourceDuration = Number.isFinite(probedDuration) && probedDuration > 0 ? probedDuration : 0;
  const sourceHasAudio = streamHasAudio(job.probe);

  const [previewSeconds, setPreviewSeconds] = useState(() => representativeFrameTime(sourceDuration));
  const previewSecondsRef = useRef(previewSeconds);
  previewSecondsRef.current = previewSeconds;
  const [previewPlaying, setPreviewPlaying] = useState(false);
  const previewAudioRef = useRef<HTMLAudioElement>(null);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [previewReload, setPreviewReload] = useState(0);

  useEffect(() => {
    if (!previewPlaying || sourceDuration <= 0) return;
    const audio = previewAudioRef.current;
    let cursor = startMontagePlayback(plan.clips, previewSecondsRef.current);
    if (!cursor) {
      setPreviewPlaying(false);
      return;
    }
    const playCursor = (next: typeof cursor): void => {
      if (!next) return;
      cursor = next;
      setPreviewSeconds(next.sourceSeconds);
      if (audio) {
        audio.currentTime = next.sourceSeconds;
        audio.playbackRate = next.playbackRate;
        audio.volume = Math.min(1, Math.max(0, plan.clips[next.clipIndex]?.edit?.source_volume ?? 1));
        void audio.play().catch(() => {
          setPreviewPlaying(false);
          setPreviewError('El navegador no pudo iniciar el audio de la preview. Pulsa reintentar y vuelve a reproducir.');
        });
      }
    };
    playCursor(cursor);
    const timer = setInterval(() => {
      if (!cursor) return;
      const sourceSeconds =
        audio && !audio.paused
          ? audio.currentTime
          : previewSecondsRef.current + 0.125 * cursor.playbackRate;
      const next = advanceMontagePlayback(plan.clips, cursor.clipIndex, sourceSeconds);
      if (!next) {
        const restart = startMontagePlayback(plan.clips, Number.NaN);
        if (restart) setPreviewSeconds(restart.sourceSeconds);
        setPreviewPlaying(false);
        return;
      }
      if (next.clipIndex !== cursor.clipIndex) {
        playCursor(next);
        return;
      }
      cursor = next;
      setPreviewSeconds(next.sourceSeconds);
    }, 125);
    return () => {
      clearInterval(timer);
      audio?.pause();
    };
  }, [plan.clips, previewPlaying, sourceDuration]);

  const captions = useCaptionReview({ jobId: job.id, plan, sourceHasAudio, onPlanChange });
  const killfeedEnabled = plan.killfeed_crop !== undefined;
  const killfeed = useKillfeedAnalysis({
    jobId: job.id,
    plan,
    enabled: killfeedEnabled,
    onPlanChange,
    onAnalysisRecovered: onKillfeedAnalysisRecovered,
    onCueAligned: setPreviewSeconds,
  });

  const captionReviewBlocked = captionsNeedReview(plan, sourceHasAudio);
  const canGenerateCaptions = canRequestCaptionCandidates(captionReviewBlocked, captions.state);
  const busy =
    stage === 'rendering' ||
    saving ||
    captions.requestBusy ||
    captions.generating ||
    killfeed.requestBusy ||
    killfeed.analyzing ||
    captions.reviewingClipId !== null ||
    killfeed.readingCueKey !== null;

  const killfeedRenderNeedsReanalysis = streamRenderNeedsKillfeedReanalysis(renderState);
  const killfeedManualCueError = killfeedManualCueIssue(plan, killfeed.state);
  const killfeedAnalysisBlocked =
    killfeedEnabled &&
    (killfeedAnalysisNeeded(plan) ||
      killfeed.state?.status !== KILLFEED_ANALYSIS_STATUS.applied ||
      killfeedRenderNeedsReanalysis ||
      killfeedManualCueError !== undefined);

  const containingClipIndex = plan.clips.findIndex(
    (clip) =>
      Number.isFinite(clip.start_seconds) &&
      Number.isFinite(clip.end_seconds) &&
      clip.start_seconds <= previewSeconds &&
      previewSeconds < clip.end_seconds,
  );
  const containingClip = containingClipIndex >= 0 ? plan.clips[containingClipIndex] : undefined;
  const cueAlreadyExists = containingClip?.killfeed_seconds?.includes(previewSeconds) ?? false;
  const canAddKillfeedCue =
    killfeedEnabled && sourceDuration > 0 && containingClip !== undefined && !cueAlreadyExists;
  let killfeedCueStatus = `La marca se añadirá a Clip ${containingClipIndex + 1}, cuyo rango contiene este tiempo.`;
  if (sourceDuration <= 0) {
    killfeedCueStatus = 'La duración del vídeo no está disponible; todavía no se puede añadir una marca.';
  } else if (containingClip === undefined) {
    killfeedCueStatus = `El tiempo ${formatStreamTimestamp(previewSeconds)} queda fuera de todos los rangos de clip. Mueve el cursor o ajusta los rangos.`;
  } else if (cueAlreadyExists) {
    killfeedCueStatus = `Ese tiempo ya está marcado en Clip ${containingClipIndex + 1}.`;
  }

  const setVariant = (variant: StreamVariant) => onPlanChange({ ...plan, variant });
  const setFaceCrop = (rect: NormalizedRect) =>
    onPlanChange({ ...plan, face_crop: rect, face_crop_reviewed: false });
  const confirmFaceCrop = () => onPlanChange({ ...plan, face_crop_reviewed: true });

  const setKillfeedEnabled = (enabled: boolean) => {
    if (enabled) {
      const next = invalidateKillfeedAnalysis({
        ...plan,
        killfeed_crop: plan.killfeed_crop ?? DEFAULT_KILLFEED_CROP,
      });
      killfeed.noteLatestPlan(next);
      onPlanChange(next);
      killfeed.schedule(next);
      return;
    }
    killfeed.cancel();
    const withoutKillfeed = invalidateKillfeedAnalysis(plan);
    delete withoutKillfeed.killfeed_crop;
    killfeed.noteLatestPlan(withoutKillfeed);
    onPlanChange(withoutKillfeed);
  };

  const setKillfeedCrop = (rect: NormalizedRect) => {
    const next = invalidateKillfeedAnalysis({ ...plan, killfeed_crop: rect });
    killfeed.noteLatestPlan(next);
    onPlanChange(next);
    killfeed.schedule(next);
  };

  const addKillfeedCue = () => {
    if (!canAddKillfeedCue) return;
    const clips = plan.clips.map((clip, index) =>
      index === containingClipIndex ? addClipCue(clip, previewSeconds) : clip,
    );
    onPlanChange({ ...plan, clips });
  };

  const removeKillfeedCue = (clipId: string, cue: number) => {
    const clips = plan.clips.map((clip) => (clip.id === clipId ? removeClipCue(clip, cue) : clip));
    onPlanChange({ ...plan, clips });
  };

  const setCueKills = (clipId: string, cue: number, kills: KillfeedKill[]) => {
    const clips = plan.clips.map((clip) => (clip.id === clipId ? setClipCueKills(clip, cue, kills) : clip));
    onPlanChange({ ...plan, clips });
  };

  const bannerPosition = resolveStreamerBannerPosition(plan.variant, plan.streamer_banner?.position_y);
  const setStreamerNick = (nick: string) =>
    onPlanChange({ ...plan, streamer_banner: { ...plan.streamer_banner, nick } });
  const setStreamerPosition = (position: number) =>
    onPlanChange({
      ...plan,
      streamer_banner: { ...plan.streamer_banner, position_y: clampStreamerBannerPosition(position) },
    });
  const resetStreamerPosition = () => {
    const { position_y: _position, ...banner } = plan.streamer_banner ?? {};
    onPlanChange({ ...plan, streamer_banner: banner });
  };
  const setStreamerSlide = (slideEnabled: boolean) =>
    onPlanChange({ ...plan, streamer_banner: { ...plan.streamer_banner, slide_enabled: slideEnabled } });

  const setCaptionsEnabled = (enabled: boolean) =>
    onPlanChange({ ...plan, captions: { enabled, language: 'es' } });

  const setClips = (clips: StreamClipRange[]) => {
    const beforeFingerprint = captionInputsFingerprint(plan.clips);
    const beforeKillfeedFingerprint = killfeedAnalysisInputsFingerprint(plan);
    const clipsWithValidReviews = clips.map((clip) => {
      const previous = plan.clips.find((item) => item.id === clip.id);
      if (!previous || captionInputsFingerprint([previous]) === captionInputsFingerprint([clip])) return clip;
      return invalidateCaptionReview(clip);
    });
    let nextPlan = normalizeKillfeedPlan({ ...plan, clips: clipsWithValidReviews });
    if (beforeFingerprint !== captionInputsFingerprint(nextPlan.clips)) {
      captions.discardCandidates(nextPlan);
    }
    if (killfeedEnabled && beforeKillfeedFingerprint !== killfeedAnalysisInputsFingerprint(nextPlan)) {
      nextPlan = invalidateKillfeedAnalysis(nextPlan);
      killfeed.schedule(nextPlan);
    }
    killfeed.noteLatestPlan(nextPlan);
    onPlanChange(nextPlan);
  };

  const setMusicKey = (key: string) =>
    onPlanChange({ ...plan, music: key ? { key, volume: plan.music?.volume } : {} });
  const setMusicVolume = (volume: number) =>
    onPlanChange({ ...plan, music: { key: plan.music?.key, volume } });
  const setGrade = (grade: boolean) => onPlanChange({ ...plan, effects: { grade } });

  return (
    <StreamFrameSession
      key={previewReload}
      videoSrc={videoSrc}
      frameSeconds={previewSeconds}
      onMediaError={() => {
        setPreviewPlaying(false);
        setPreviewError('No se pudo decodificar o leer el MP4 de origen. Comprueba que el archivo siga disponible y reintenta la vista previa.');
      }}
    >
      <div className="grid gap-6 @[64rem]/content:grid-cols-[minmax(0,1fr)_19rem]">
        <div className="@container/editor flex min-w-0 flex-col gap-5">
          <StreamJobHeader job={job} />

          <div className="studio-panel flex flex-col gap-5 p-5 sm:p-6">
            <StreamLayoutPicker
              variant={plan.variant}
              faceCrop={plan.face_crop}
              faceCropReviewed={plan.face_crop_reviewed === true}
              needsFaceCrop={variantMeta.needsFaceCrop}
              previewSeconds={previewSeconds}
              busy={busy}
              onVariantChange={setVariant}
              onFaceCropChange={setFaceCrop}
              onConfirmFaceCrop={confirmFaceCrop}
            />

            <StreamKillfeedPanel
              enabled={killfeedEnabled}
              crop={plan.killfeed_crop}
              clips={plan.clips}
              weapons={killfeed.weapons}
              busy={busy}
              analyzing={killfeed.analyzing || killfeed.requestBusy}
              clipsValid={clipsAreValid(plan.clips)}
              hasAnalysis={plan.killfeed_analysis !== undefined}
              analysisApplied={killfeed.state?.status === KILLFEED_ANALYSIS_STATUS.applied}
              detectedEvents={detectedKillfeedEventCount(killfeed.state)}
              error={killfeed.error}
              warnings={killfeed.state?.warnings}
              needsReanalysis={killfeedRenderNeedsReanalysis}
              readNotice={killfeed.readNotice}
              readingCueKey={killfeed.readingCueKey}
              readErrors={killfeed.readErrors}
              previewSeconds={previewSeconds}
              sourceDuration={sourceDuration}
              canAddCue={canAddKillfeedCue}
              cueStatus={killfeedCueStatus}
              onToggle={setKillfeedEnabled}
              onAnalyze={() => void killfeed.run(plan)}
              onCancelWait={killfeed.cancelWait}
              onCropChange={setKillfeedCrop}
              onPreviewSecondsChange={setPreviewSeconds}
              onAddCue={addKillfeedCue}
              onRemoveCue={removeKillfeedCue}
              onCueKillsChange={setCueKills}
              onReadCue={(clip, cue) => void killfeed.readCue(clip, cue)}
            />

            <StreamBannerControls
              nick={plan.streamer_banner?.nick ?? ''}
              nickValid={STREAMER_NICK_RE.test(plan.streamer_banner?.nick?.trim() ?? '')}
              position={bannerPosition}
              hasExplicitPosition={plan.streamer_banner?.position_y !== undefined}
              slideEnabled={plan.streamer_banner?.slide_enabled ?? false}
              busy={busy}
              onNickChange={setStreamerNick}
              onPositionChange={setStreamerPosition}
              onResetPosition={resetStreamerPosition}
              onSlideChange={setStreamerSlide}
            />
          </div>

          <div className="studio-panel p-5 sm:p-6">
            <StreamClipEditor
              clips={plan.clips}
              sourceDuration={sourceDuration}
              onChange={setClips}
              disabled={busy}
            />
          </div>

          <div className="studio-panel p-5 sm:p-6">
            <StreamCaptionsPanel
              enabled={plan.captions?.enabled === true}
              clips={plan.clips}
              videoSrc={videoSrc}
              captionState={captions.state}
              captionDrafts={captions.drafts}
              captionError={captions.error}
              captionLoading={captions.loading}
              captionRequestBusy={captions.requestBusy}
              captionGenerating={captions.generating}
              captionReviewBlocked={captionReviewBlocked}
              canGenerateCaptions={canGenerateCaptions}
              sourceHasAudio={sourceHasAudio}
              reviewingClipId={captions.reviewingClipId}
              busy={busy}
              onToggle={setCaptionsEnabled}
              onGenerate={() => void captions.generate()}
              onCancelWait={captions.cancelWait}
              onWordsChange={captions.updateDraft}
              onApprove={(clip) => void captions.review(clip, false)}
              onNoSpeech={(clip) => void captions.review(clip, true)}
            />
          </div>

          <div className="studio-panel p-5 sm:p-6">
            <StreamMusicCard
              musicKey={plan.music?.key ?? ''}
              volume={plan.music?.volume ?? 0.25}
              grade={plan.effects?.grade ?? false}
              busy={busy}
              onMusicKey={setMusicKey}
              onMusicVolume={setMusicVolume}
              onGrade={setGrade}
            />
          </div>

          {error ? (
            <p
              role="alert"
              className="flex items-start gap-2 border border-destructive/45 bg-destructive/10 px-3.5 py-3 text-body-sm text-destructive"
            >
              <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
              {error}
            </p>
          ) : null}

          {stage === 'rendering' ? (
            <StreamRenderStage
              clips={plan.clips}
              renderState={renderState}
              variantLabel={variantMeta.label.toUpperCase()}
            />
          ) : null}

          <StreamRenderBar
            rendering={stage === 'rendering'}
            busy={busy}
            captionReviewBlocked={captionReviewBlocked}
            killfeedBlockedReason={
              killfeedAnalysisBlocked
                ? (killfeedManualCueError ?? 'Espera al análisis automático de la killfeed para continuar.')
                : null
            }
            onCreate={onCreate}
            onStartOver={onStartOver}
          />

          {renderedPlan && (stage === 'rendered' || renderState?.published) ? (
            <StreamRenderResults
              renderState={renderState}
              job={job}
              renderedPlan={renderedPlan}
              stale={stale}
            />
          ) : null}
        </div>

        <StreamPreviewColumn
          variant={plan.variant}
          faceCrop={plan.face_crop}
          gameplayCrop={plan.gameplay_crop}
          killfeedCrop={plan.killfeed_crop}
          clips={plan.clips}
          frameSeconds={previewSeconds}
          sourceDuration={sourceDuration}
          streamerNick={plan.streamer_banner?.nick?.trim()}
          streamerPositionY={plan.streamer_banner?.position_y}
          streamerSlideEnabled={plan.streamer_banner?.slide_enabled}
          playing={previewPlaying}
          canPlay={sourceDuration > 0 && startMontagePlayback(plan.clips, previewSeconds) !== null}
          previewError={previewError}
          videoSrc={videoSrc}
          audioRef={previewAudioRef}
          audioKey={previewReload}
          busy={busy}
          onStreamerPositionChange={setStreamerPosition}
          onTogglePlay={() => {
            setPreviewError(null);
            setPreviewPlaying((current) => !current);
          }}
          onAudioPause={() => setPreviewPlaying(false)}
          onAudioError={() => {
            setPreviewPlaying(false);
            setPreviewError('No se pudo decodificar la pista de audio de la preview. Revisa el MP4 y reintenta.');
          }}
          onRetry={() => {
            setPreviewError(null);
            setPreviewReload((current) => current + 1);
          }}
        />
      </div>
    </StreamFrameSession>
  );
}
