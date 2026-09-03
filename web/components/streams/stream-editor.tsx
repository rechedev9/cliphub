'use client';

import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { AlertTriangle, ArrowRight } from 'lucide-react';
import { toast } from 'sonner';
import {
  STREAM_VARIANTS,
  streamsApi,
  type NormalizedRect,
  type StreamClipRange,
  type StreamEditPlan,
  type StreamerBannerPlatform,
  type StreamJob,
  type StreamRenderState,
  type StreamVariant,
} from '@/lib/api/streams';
import { api } from '@/lib/api';
import { isKeyDropBannerStyle, type AffiliateFamily, type Song } from '@/lib/api/types';
import {
  advanceMontagePlayback,
  clampKeyDropBannerPosition,
  clampStreamerBannerPosition,
  keyDropPreviewSourceSeconds,
  representativeFrameTime,
  resolveKeyDropBannerPosition,
  resolveStreamerBannerPosition,
  startMontagePlayback,
} from '@/lib/stream-preview';
import {
  DEFAULT_KEYDROP_CODE,
  DEFAULT_KEYDROP_END_SECONDS,
  DEFAULT_KEYDROP_START_SECONDS,
  STREAMER_NICK_RE,
  formatStreamClock,
  insertClipSorted,
  nextClipId,
  planFingerprint,
  resolveFaceCrop,
  resolveStreamerBannerPlatform,
  streamSourceLabel,
  timelineClipAt,
} from '@/lib/streams/plan';
import { canCreateStreamShorts, streamCreativeBrief } from '@/lib/streams/brief';
import {
  STREAM_STEP,
  STREAM_STEP_LABEL,
  shortsWord,
  streamBriefCanBeApproved,
  streamCtaLabel,
  streamCtaTarget,
  streamEditorSteps,
  streamNextStep,
  streamOutputSummary,
  streamPlanBlocker,
  type StreamBlocker,
  type StreamStep,
} from '@/lib/streams/editor';
import { persistAffiliateFamily, selectAffiliateFamily, selectAffiliateOff, selectAffiliateStyle } from '@/lib/affiliate-banner';
import { StreamFrameSession } from '@/components/streams/stream-frame-session';
import { StreamCropStage } from '@/components/streams/stream-crop-stage';
import { StreamReviewStep } from '@/components/streams/stream-review-step';
import { StreamLayoutBar } from '@/components/streams/stream-layout-bar';
import { StreamStepsRail, type StreamAutosaveState } from '@/components/streams/stream-steps-rail';
import { StreamMonitor } from '@/components/streams/stream-monitor';
import { StreamSourceTimeline } from '@/components/streams/stream-source-timeline';
import { StreamLayoutStep, StreamStepPanel } from '@/components/streams/stream-step-panel';
import { StreamBannerControls } from '@/components/streams/banner-controls';
import { isKeyDropCodeValid, StreamKeyDropBannerControls } from '@/components/streams/keydrop-banner-controls';
import { StreamClipEditor } from '@/components/streams/clip-editor';
import { StreamMusicCard } from '@/components/streams/music-card';
import { StreamRenderStage } from '@/components/streams/render-stage';
import { StreamRenderResults } from '@/components/streams/render-results';
import { StreamFooter } from '@/components/streams/stream-footer';
import { Button } from '@/components/ui/button';

/** Panel titles that say more than the rail label; every other step reuses its rail entry. */
const STEP_SUBTITLE: Partial<Record<StreamStep, string>> = {
  layout: 'Layout y facecam',
  music: 'Música y efectos',
  results: 'Shorts renderizados',
};

/** One hint per blocker; the compiler refuses a new blocker without one. */
const BLOCKER_HINT: Record<StreamBlocker, string> = {
  layout: 'Confirma el recorte de facecam en el paso 01 para poder aprobar.',
  cuts: 'Añade al menos un corte en la timeline para poder aprobar.',
};

/** Stream edit workspace: rail, monitor + timeline, active step, approval footer. */
export function StreamEditor({
  job,
  plan,
  onPlanChange,
  stage,
  renderState,
  renderedPlan,
  error,
  saving,
  autosave,
  onCreate,
  onBack,
}: {
  job: StreamJob;
  plan: StreamEditPlan;
  onPlanChange: (plan: StreamEditPlan) => void;
  stage: 'editing' | 'rendering' | 'rendered';
  renderState: StreamRenderState | null;
  renderedPlan: StreamEditPlan | null;
  error: string | null;
  saving: boolean;
  autosave: StreamAutosaveState;
  onCreate: () => void;
  onBack: () => void;
}): ReactNode {
  const videoSrc = streamsApi.sourceUrl(job.id);
  const variantMeta = STREAM_VARIANTS.find((v) => v.value === plan.variant) ?? STREAM_VARIANTS[0];
  const stale = useMemo(
    () => renderedPlan !== null && planFingerprint(renderedPlan) !== planFingerprint(plan),
    [renderedPlan, plan],
  );
  const hasRender = renderedPlan !== null && (stage === 'rendered' || renderState?.published === true);

  const probedDuration = job.probe?.duration_seconds ?? 0;
  const sourceDuration = Number.isFinite(probedDuration) && probedDuration > 0 ? probedDuration : 0;

  const [activeStep, setActiveStep] = useState<StreamStep>(
    hasRender ? STREAM_STEP.results : (streamPlanBlocker(plan) ?? STREAM_STEP.cuts),
  );
  const [selectedClipId, setSelectedClipId] = useState<string | null>(plan.clips[0]?.id ?? null);
  const [songs, setSongs] = useState<Song[] | null>(null);
  const [previewSeconds, setPreviewSeconds] = useState(() => representativeFrameTime(sourceDuration));
  const previewSecondsRef = useRef(previewSeconds);
  previewSecondsRef.current = previewSeconds;
  const [previewPlaying, setPreviewPlaying] = useState(false);
  const previewAudioRef = useRef<HTMLAudioElement>(null);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [previewReload, setPreviewReload] = useState(0);
  const [briefApproved, setBriefApproved] = useState(false);
  const briefItems = useMemo(() => streamCreativeBrief(plan), [plan]);
  const planKey = useMemo(() => planFingerprint(plan), [plan]);
  // Only start/end/speed/volume should restart the playback transport; a title
  // edit must not pause the montage that is already playing.
  const clipsPlaybackKey = useMemo(
    () =>
      plan.clips
        .map((c) => `${c.id}:${c.start_seconds}:${c.end_seconds}:${c.edit?.speed ?? 1}:${c.edit?.source_volume ?? 1}`)
        .join('|'),
    [plan.clips],
  );
  const playbackClipsRef = useRef(plan.clips);
  playbackClipsRef.current = plan.clips;

  // Any plan mutation invalidates the creative brief (same contract as demo reels).
  useEffect(() => {
    setBriefApproved(false);
  }, [planKey]);

  useEffect(() => {
    if (stage === 'rendering') setActiveStep(STREAM_STEP.results);
  }, [stage]);

  useEffect(() => {
    let active = true;
    api
      .listSongs()
      .then((next) => {
        if (active) setSongs(next);
      })
      .catch(() => {
        if (active) setSongs([]);
      });
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    if (!previewPlaying || sourceDuration <= 0) return;
    const audio = previewAudioRef.current;
    const clips = playbackClipsRef.current;
    let cursor = startMontagePlayback(clips, previewSecondsRef.current);
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
        audio.volume = Math.min(1, Math.max(0, clips[next.clipIndex]?.edit?.source_volume ?? 1));
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
      const next = advanceMontagePlayback(clips, cursor.clipIndex, sourceSeconds);
      if (!next) {
        const restart = startMontagePlayback(clips, Number.NaN);
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
  }, [clipsPlaybackKey, previewPlaying, sourceDuration]);

  const busy = stage === 'rendering' || saving;

  const setVariant = (variant: StreamVariant) => onPlanChange({ ...plan, variant });
  const faceCrop = resolveFaceCrop(plan.face_crop);
  const setFaceCrop = (rect: NormalizedRect) =>
    onPlanChange({ ...plan, face_crop: rect, face_crop_reviewed: false });
  const confirmFaceCrop = () => onPlanChange({ ...plan, face_crop: faceCrop, face_crop_reviewed: true });

  const bannerPosition = resolveStreamerBannerPosition(plan.variant, plan.streamer_banner?.position_y);
  const bannerPlatform = resolveStreamerBannerPlatform(plan.streamer_banner?.platform, job.source_url);
  const keyDropPosition = resolveKeyDropBannerPosition(plan.keydrop_banner?.position_y);
  const setStreamerNick = (nick: string) =>
    onPlanChange({ ...plan, streamer_banner: { ...plan.streamer_banner, nick, platform: bannerPlatform } });
  const setStreamerPlatform = (platform: StreamerBannerPlatform) =>
    onPlanChange({ ...plan, streamer_banner: { ...plan.streamer_banner, platform } });
  const setStreamerPosition = (position: number) =>
    onPlanChange({
      ...plan,
      streamer_banner: { ...plan.streamer_banner, position_y: clampStreamerBannerPosition(position), platform: bannerPlatform },
    });
  const resetStreamerPosition = () => {
    const { position_y: _position, ...banner } = plan.streamer_banner ?? {};
    onPlanChange({ ...plan, streamer_banner: { ...banner, platform: bannerPlatform } });
  };
  const setStreamerSlide = (slideEnabled: boolean) =>
    onPlanChange({ ...plan, streamer_banner: { ...plan.streamer_banner, slide_enabled: slideEnabled, platform: bannerPlatform } });

  const longestClipSeconds = Math.max(
    0,
    ...plan.clips.map((c) => Math.max(0, c.end_seconds - c.start_seconds)),
  );
  const keyDropStart = plan.keydrop_banner?.start_seconds ?? DEFAULT_KEYDROP_START_SECONDS;
  const keyDropEndRaw = plan.keydrop_banner?.end_seconds;
  const keyDropEnd =
    keyDropEndRaw ??
    (longestClipSeconds > 0
      ? Math.min(DEFAULT_KEYDROP_END_SECONDS, longestClipSeconds)
      : DEFAULT_KEYDROP_END_SECONDS);

  /** Keep the 9:16 monitor inside the plate's on-screen window so code edits are visible. */
  const revealKeyDropOnPreview = (start: number, end: number) => {
    setPreviewPlaying(false);
    setPreviewSeconds(keyDropPreviewSourceSeconds(plan.clips, previewSecondsRef.current, start, end));
  };

  const setKeyDropFamily = (family: AffiliateFamily) => {
    const selected = selectAffiliateFamily(
      { family: plan.keydrop_banner?.family ?? '', style: plan.keydrop_banner?.style ?? '' },
      family,
    );
    setKeyDropStyle(selected.style, selected.family);
  };

  const setKeyDropStyle = (style: string, family = plan.keydrop_banner?.family ?? '') => {
    if (!style) {
      const off = selectAffiliateOff();
      onPlanChange({ ...plan, keydrop_banner: { ...plan.keydrop_banner, family: off.family, style: off.style } });
      return;
    }
    const selected = selectAffiliateStyle(family, style);
    const next = {
      ...plan.keydrop_banner,
      family: persistAffiliateFamily(selected.family, selected.style) || selected.family,
      style: selected.style,
    };
    // First enable: pin the default sponsor code and a short callout window so
    // the rendered plate never depends on an implicit ZACKCSGO fallback alone.
    if (!plan.keydrop_banner?.code?.trim()) {
      next.code = DEFAULT_KEYDROP_CODE;
    }
    if (plan.keydrop_banner?.start_seconds === undefined) {
      next.start_seconds = DEFAULT_KEYDROP_START_SECONDS;
    }
    if (plan.keydrop_banner?.end_seconds === undefined) {
      next.end_seconds =
        longestClipSeconds > 0
          ? Math.min(DEFAULT_KEYDROP_END_SECONDS, longestClipSeconds)
          : DEFAULT_KEYDROP_END_SECONDS;
    }
    const start = next.start_seconds ?? DEFAULT_KEYDROP_START_SECONDS;
    const end = next.end_seconds ?? DEFAULT_KEYDROP_END_SECONDS;
    revealKeyDropOnPreview(start, end);
    onPlanChange({ ...plan, keydrop_banner: next });
  };
  const setKeyDropCode = (code: string) => {
    revealKeyDropOnPreview(keyDropStart, keyDropEnd);
    onPlanChange({ ...plan, keydrop_banner: { ...plan.keydrop_banner, code } });
  };
  const setKeyDropPosition = (position: number) =>
    onPlanChange({
      ...plan,
      keydrop_banner: { ...plan.keydrop_banner, position_y: clampKeyDropBannerPosition(position) },
    });
  const resetKeyDropPosition = () => {
    const { position_y: _position, ...banner } = plan.keydrop_banner ?? {};
    onPlanChange({ ...plan, keydrop_banner: banner });
  };
  const setKeyDropSlide = (slideEnabled: boolean) =>
    onPlanChange({ ...plan, keydrop_banner: { ...plan.keydrop_banner, slide_enabled: slideEnabled } });
  const setKeyDropStart = (startSeconds: number) => {
    const start = Math.max(0, startSeconds);
    let end = keyDropEnd;
    if (end <= start) end = start + 0.5;
    revealKeyDropOnPreview(start, end);
    onPlanChange({ ...plan, keydrop_banner: { ...plan.keydrop_banner, start_seconds: start, end_seconds: end } });
  };
  const setKeyDropEnd = (endSeconds: number) => {
    let end = Math.max(0.1, endSeconds);
    if (longestClipSeconds > 0) end = Math.min(end, longestClipSeconds);
    let start = keyDropStart;
    if (end <= start) start = Math.max(0, end - 0.5);
    revealKeyDropOnPreview(start, end);
    onPlanChange({ ...plan, keydrop_banner: { ...plan.keydrop_banner, start_seconds: start, end_seconds: end } });
  };

  const setClips = (clips: StreamClipRange[]) => onPlanChange({ ...plan, clips });
  const selectClip = (clip: StreamClipRange) => {
    setSelectedClipId(clip.id);
    setPreviewPlaying(false);
    setPreviewSeconds(clip.start_seconds);
  };
  const addClipAt = (seconds: number) => {
    const range = timelineClipAt(plan.clips, seconds, sourceDuration);
    if (range === null) {
      toast('No cabe un corte aquí', { description: 'Elige un hueco libre de la timeline' });
      return;
    }
    const clip: StreamClipRange = { id: nextClipId(), ...range, title: '' };
    setClips(insertClipSorted(plan.clips, clip));
    setActiveStep(STREAM_STEP.cuts);
    selectClip(clip);
  };
  const removeClip = (clip: StreamClipRange) => {
    setClips(plan.clips.filter((c) => c.id !== clip.id));
    if (selectedClipId === clip.id) setSelectedClipId(null);
    toast('Corte quitado', { description: 'Un Short menos en el render' });
  };

  const setMusicKey = (key: string) =>
    onPlanChange({ ...plan, music: key ? { key, volume: plan.music?.volume } : {} });
  const setMusicVolume = (volume: number) =>
    onPlanChange({ ...plan, music: { key: plan.music?.key, volume } });
  const setGrade = (grade: boolean) => onPlanChange({ ...plan, effects: { grade } });

  const musicKey = plan.music?.key ?? '';
  const musicLabel = songs?.find((song) => song.id === musicKey)?.title ?? musicKey;
  const steps = streamEditorSteps({ plan, musicLabel, renderState, stale, rendering: stage === 'rendering', briefApproved });
  const activeClip =
    plan.clips.find((c) => previewSeconds >= c.start_seconds && previewSeconds < c.end_seconds) ??
    plan.clips.find((c) => c.id === selectedClipId);
  const clipProgress =
    activeClip && activeClip.end_seconds > activeClip.start_seconds
      ? ((previewSeconds - activeClip.start_seconds) / (activeClip.end_seconds - activeClip.start_seconds)) * 100
      : 0;
  const briefApprovable = streamBriefCanBeApproved(plan);
  const onReview = activeStep === STREAM_STEP.review;
  const ctaLabel = streamCtaLabel({ plan, briefApproved, rendering: stage === 'rendering', hasRender, onReview });
  // While something blocks the render the CTA names it but stays a real link
  // to that step instead of a disabled dead end.
  const ctaTarget = streamCtaTarget(plan, activeStep);
  const ctaDisabled = busy ? true : ctaTarget === null && !canCreateStreamShorts({ briefApproved, busy });
  const blocker = streamPlanBlocker(plan);
  const activeEntry = steps.find((step) => step.key === activeStep);
  const panelTitle =
    activeEntry === undefined
      ? STREAM_STEP_LABEL[activeStep]
      : `${activeEntry.number} · ${STEP_SUBTITLE[activeStep] ?? activeEntry.label}`;
  const cropStage = activeStep === STREAM_STEP.layout && variantMeta.needsFaceCrop;
  const nextStep = stage === 'rendering' ? null : streamNextStep(activeStep);
  const stepAction =
    nextStep === null ? null : (
      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={() => setActiveStep(nextStep)}
        className="w-full justify-between border-stream/45 font-display uppercase tracking-wide text-stream-text hover:border-stream hover:bg-stream/10"
      >
        Siguiente · {STREAM_STEP_LABEL[nextStep]}
        <ArrowRight aria-hidden />
      </Button>
    );
  const sourceMeta = [streamSourceLabel(job.source_url) ?? 'Archivo local', sourceDuration > 0 ? formatStreamClock(sourceDuration) : null]
    .filter((part): part is string => part !== null)
    .join(' · ');

  let stepContent: ReactNode;
  if (activeStep === STREAM_STEP.layout) {
    stepContent = (
      <StreamLayoutStep
        needsFaceCrop={variantMeta.needsFaceCrop}
        faceCropReviewed={plan.face_crop_reviewed === true}
        busy={busy}
        onConfirmFaceCrop={confirmFaceCrop}
      />
    );
  } else if (activeStep === STREAM_STEP.banners) {
    stepContent = (
      <>
        <StreamBannerControls
          nick={plan.streamer_banner?.nick ?? ''}
          nickValid={STREAMER_NICK_RE.test(plan.streamer_banner?.nick?.trim() ?? '')}
          platform={bannerPlatform}
          position={bannerPosition}
          hasExplicitPosition={plan.streamer_banner?.position_y !== undefined}
          slideEnabled={plan.streamer_banner?.slide_enabled ?? false}
          busy={busy}
          onNickChange={setStreamerNick}
          onPlatformChange={setStreamerPlatform}
          onPositionChange={setStreamerPosition}
          onResetPosition={resetStreamerPosition}
          onSlideChange={setStreamerSlide}
        />
        <StreamKeyDropBannerControls
          family={plan.keydrop_banner?.family ?? ''}
          style={isKeyDropBannerStyle(plan.keydrop_banner?.style) ? plan.keydrop_banner.style : ''}
          code={plan.keydrop_banner?.code ?? ''}
          codeValid={isKeyDropCodeValid(plan.keydrop_banner?.code ?? '')}
          position={keyDropPosition}
          hasExplicitPosition={plan.keydrop_banner?.position_y !== undefined}
          slideEnabled={plan.keydrop_banner?.slide_enabled ?? false}
          startSeconds={keyDropStart}
          endSeconds={keyDropEnd}
          clipDurationSeconds={longestClipSeconds}
          busy={busy}
          onFamilyChange={setKeyDropFamily}
          onStyleChange={setKeyDropStyle}
          onCodeChange={setKeyDropCode}
          onPositionChange={setKeyDropPosition}
          onResetPosition={resetKeyDropPosition}
          onSlideChange={setKeyDropSlide}
          onStartChange={setKeyDropStart}
          onEndChange={setKeyDropEnd}
        />
      </>
    );
  } else if (activeStep === STREAM_STEP.cuts) {
    stepContent = (
      <StreamClipEditor
        clips={plan.clips}
        sourceDuration={sourceDuration}
        selectedClipId={selectedClipId}
        onChange={setClips}
        onSelect={selectClip}
        onRemove={removeClip}
        disabled={busy}
      />
    );
  } else if (activeStep === STREAM_STEP.music) {
    stepContent = (
      <StreamMusicCard
        songs={songs}
        musicKey={musicKey}
        volume={plan.music?.volume ?? 0.25}
        grade={plan.effects?.grade ?? false}
        busy={busy}
        onMusicKey={setMusicKey}
        onMusicVolume={setMusicVolume}
        onGrade={setGrade}
      />
    );
  } else if (activeStep === STREAM_STEP.review) {
    stepContent = (
      <StreamReviewStep
        items={briefItems}
        approved={briefApproved}
        approvable={briefApprovable}
        blockerHint={blocker === null ? null : BLOCKER_HINT[blocker]}
        busy={busy}
        onApprovedChange={setBriefApproved}
      />
    );
  } else if (stage === 'rendering') {
    stepContent = (
      <StreamRenderStage clips={plan.clips} renderState={renderState} variantLabel={variantMeta.label.toUpperCase()} />
    );
  } else if (renderedPlan && hasRender) {
    stepContent = <StreamRenderResults renderState={renderState} job={job} renderedPlan={renderedPlan} stale={stale} />;
  } else {
    stepContent = <p className="text-body-sm text-fg-3">Todavía no hay un render de este stream.</p>;
  }

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
      <div className="studio-enter -mx-(--shell-gutter) -my-10 flex flex-col @[64rem]/content:h-[calc(100vh-var(--shell-strip-height))]">
        <StreamLayoutBar variant={plan.variant} disabled={busy} onVariantChange={setVariant} />

        <div className="flex min-h-0 flex-1 flex-col @[64rem]/content:grid @[64rem]/content:grid-cols-[clamp(150px,17vw,236px)_minmax(260px,1fr)_clamp(220px,25vw,320px)] @[64rem]/content:grid-rows-[minmax(0,1fr)] @[64rem]/content:overflow-hidden">
          <StreamStepsRail
            steps={steps}
            activeStep={activeStep}
            sourceTitle={job.title?.trim() || 'Clip de stream'}
            sourceMeta={sourceMeta}
            autosave={autosave}
            onSelectStep={setActiveStep}
          />

          <section className="flex min-h-0 min-w-0 flex-col gap-3 overflow-hidden px-7 pt-4" aria-label="Monitor">
            {cropStage ? (
              <StreamCropStage
                rect={faceCrop}
                disabled={busy}
                onChange={setFaceCrop}
                preview={{
                  variant: plan.variant,
                  faceCrop,
                  gameplayCrop: plan.gameplay_crop,
                  clips: plan.clips,
                  frameSeconds: previewSeconds,
                  streamerNick: plan.streamer_banner?.nick?.trim(),
                  streamerPlatform: bannerPlatform,
                  streamerPositionY: plan.streamer_banner?.position_y,
                  disabled: true,
                  className: 'h-full w-auto min-h-[120px]',
                }}
              />
            ) : (
            <StreamMonitor
              preview={{
                variant: plan.variant,
                faceCrop,
                gameplayCrop: plan.gameplay_crop,
                clips: plan.clips,
                frameSeconds: previewSeconds,
                streamerNick: plan.streamer_banner?.nick?.trim(),
                streamerPlatform: bannerPlatform,
                streamerPositionY: plan.streamer_banner?.position_y,
                streamerSlideEnabled: plan.streamer_banner?.slide_enabled,
                keyDropFamily: plan.keydrop_banner?.family ?? '',
                keyDropStyle: isKeyDropBannerStyle(plan.keydrop_banner?.style) ? plan.keydrop_banner.style : '',
                keyDropCode: plan.keydrop_banner?.code,
                keyDropPositionY: plan.keydrop_banner?.position_y,
                keyDropSlideEnabled: plan.keydrop_banner?.slide_enabled,
                keyDropStartSeconds: keyDropStart,
                keyDropEndSeconds: keyDropEnd,
                onKeyDropPositionChange: busy ? undefined : setKeyDropPosition,
                onStreamerPositionChange: setStreamerPosition,
                disabled: busy,
                playheadPercent: clipProgress,
                className: 'h-full w-auto min-h-[120px]',
              }}
              frameSeconds={previewSeconds}
              sourceDuration={sourceDuration}
              playing={previewPlaying}
              canPlay={sourceDuration > 0 && startMontagePlayback(plan.clips, previewSeconds) !== null}
              previewError={previewError}
              videoSrc={videoSrc}
              audioRef={previewAudioRef}
              audioKey={previewReload}
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
            )}
            {cropStage ? null : (
            <div className="pb-3">
              <StreamSourceTimeline
                clips={plan.clips}
                sourceDuration={sourceDuration}
                selectedClipId={selectedClipId}
                playheadSeconds={previewSeconds}
                disabled={busy}
                onAddAt={addClipAt}
                onSelect={selectClip}
              />
            </div>
            )}
          </section>

          <StreamStepPanel title={panelTitle} action={stepAction}>
            {stepContent}
          </StreamStepPanel>
        </div>

        {error ? (
          <p
            role="alert"
            className="mx-(--shell-gutter) mb-2 flex items-start gap-2 border border-destructive/45 bg-destructive/10 px-3.5 py-2.5 text-body-sm text-destructive"
          >
            <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
            {error}
          </p>
        ) : null}

        <StreamFooter
          countLabel={shortsWord(plan.clips.length)}
          summary={streamOutputSummary(plan, stale)}
          ctaLabel={ctaLabel}
          ctaDisabled={ctaDisabled}
          rendering={stage === 'rendering'}
          busy={saving}
          onCreate={() => {
            if (ctaTarget !== null) {
              setActiveStep(ctaTarget);
              return;
            }
            onCreate();
          }}
          onBack={onBack}
        />
      </div>
    </StreamFrameSession>
  );
}
