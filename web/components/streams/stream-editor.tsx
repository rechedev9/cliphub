'use client';

import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { AlertTriangle } from 'lucide-react';
import {
  STREAM_VARIANTS,
  streamsApi,
  type NormalizedRect,
  type StreamClipRange,
  type StreamEditPlan,
  type StreamJob,
  type StreamRenderState,
  type StreamVariant,
} from '@/lib/api/streams';
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
  planFingerprint,
} from '@/lib/streams/plan';
import { streamCreativeBrief } from '@/lib/streams/brief';
import { StreamFrameSession } from '@/components/streams/stream-frame-session';
import { StreamJobHeader } from '@/components/streams/job-header';
import { StreamLayoutPicker } from '@/components/streams/layout-picker';
import { StreamBannerControls } from '@/components/streams/banner-controls';
import {
  isKeyDropCodeValid,
  StreamKeyDropBannerControls,
} from '@/components/streams/keydrop-banner-controls';
import { StreamClipEditor } from '@/components/streams/clip-editor';
import { StreamMusicCard } from '@/components/streams/music-card';
import { StreamRenderBar } from '@/components/streams/render-bar';
import { StreamRenderStage } from '@/components/streams/render-stage';
import { StreamRenderResults } from '@/components/streams/render-results';
import { StreamPreviewColumn } from '@/components/streams/preview-column';
import type { KeyDropBannerStyle } from '@/lib/api/streams';

/**
 * The stream edit workspace: one persisted plan, the panels that write to it,
 * and a live 9:16 monitor that reads it.
 *
 * This component owns the plan setters; the panels are presentational and
 * receive exactly what they render. The plan is canonical for ranges, order,
 * crop, audio, fades, text and music volume — nothing here reaches around it.
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

  // Any plan mutation invalidates the creative brief (same contract as demo reels).
  useEffect(() => {
    setBriefApproved(false);
  }, [planKey]);

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

  const busy = stage === 'rendering' || saving;

  const setVariant = (variant: StreamVariant) => onPlanChange({ ...plan, variant });
  const setFaceCrop = (rect: NormalizedRect) =>
    onPlanChange({ ...plan, face_crop: rect, face_crop_reviewed: false });
  const confirmFaceCrop = () => onPlanChange({ ...plan, face_crop_reviewed: true });

  const bannerPosition = resolveStreamerBannerPosition(plan.variant, plan.streamer_banner?.position_y);
  const keyDropPosition = resolveKeyDropBannerPosition(plan.keydrop_banner?.position_y);
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

  const longestClipSeconds = Math.max(
    0,
    ...plan.clips.map((c) => Math.max(0, c.end_seconds - c.start_seconds)),
  );
  const keyDropStart =
    plan.keydrop_banner?.start_seconds ?? DEFAULT_KEYDROP_START_SECONDS;
  const keyDropEndRaw = plan.keydrop_banner?.end_seconds;
  const keyDropEnd =
    keyDropEndRaw ??
    (longestClipSeconds > 0
      ? Math.min(DEFAULT_KEYDROP_END_SECONDS, longestClipSeconds)
      : DEFAULT_KEYDROP_END_SECONDS);

  /** Keep the 9:16 monitor inside the plate's on-screen window so code edits are visible. */
  const revealKeyDropOnPreview = (start: number, end: number) => {
    setPreviewPlaying(false);
    setPreviewSeconds(
      keyDropPreviewSourceSeconds(plan.clips, previewSecondsRef.current, start, end),
    );
  };

  const setKeyDropStyle = (style: KeyDropBannerStyle | '') => {
    if (!style) {
      onPlanChange({ ...plan, keydrop_banner: { ...plan.keydrop_banner, style: '' } });
      return;
    }
    const next = { ...plan.keydrop_banner, style };
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
      keydrop_banner: {
        ...plan.keydrop_banner,
        position_y: clampKeyDropBannerPosition(position),
      },
    });
  const resetKeyDropPosition = () => {
    const { position_y: _position, ...banner } = plan.keydrop_banner ?? {};
    onPlanChange({ ...plan, keydrop_banner: banner });
  };
  const setKeyDropSlide = (slideEnabled: boolean) =>
    onPlanChange({
      ...plan,
      keydrop_banner: { ...plan.keydrop_banner, slide_enabled: slideEnabled },
    });
  const setKeyDropStart = (startSeconds: number) => {
    const start = Math.max(0, startSeconds);
    let end = keyDropEnd;
    if (end <= start) end = start + 0.5;
    revealKeyDropOnPreview(start, end);
    onPlanChange({
      ...plan,
      keydrop_banner: { ...plan.keydrop_banner, start_seconds: start, end_seconds: end },
    });
  };
  const setKeyDropEnd = (endSeconds: number) => {
    let end = Math.max(0.1, endSeconds);
    if (longestClipSeconds > 0) end = Math.min(end, longestClipSeconds);
    let start = keyDropStart;
    if (end <= start) start = Math.max(0, end - 0.5);
    revealKeyDropOnPreview(start, end);
    onPlanChange({
      ...plan,
      keydrop_banner: { ...plan.keydrop_banner, start_seconds: start, end_seconds: end },
    });
  };

  const setClips = (clips: StreamClipRange[]) => onPlanChange({ ...plan, clips });

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
              busy={busy}
              onVariantChange={setVariant}
              onFaceCropChange={setFaceCrop}
              onConfirmFaceCrop={confirmFaceCrop}
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

            <StreamKeyDropBannerControls
              style={(plan.keydrop_banner?.style as KeyDropBannerStyle | '') ?? ''}
              code={plan.keydrop_banner?.code ?? ''}
              codeValid={isKeyDropCodeValid(plan.keydrop_banner?.code ?? '')}
              position={keyDropPosition}
              hasExplicitPosition={plan.keydrop_banner?.position_y !== undefined}
              slideEnabled={plan.keydrop_banner?.slide_enabled ?? false}
              startSeconds={keyDropStart}
              endSeconds={keyDropEnd}
              clipDurationSeconds={longestClipSeconds}
              busy={busy}
              onStyleChange={setKeyDropStyle}
              onCodeChange={setKeyDropCode}
              onPositionChange={setKeyDropPosition}
              onResetPosition={resetKeyDropPosition}
              onSlideChange={setKeyDropSlide}
              onStartChange={setKeyDropStart}
              onEndChange={setKeyDropEnd}
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
            briefItems={briefItems}
            briefApproved={briefApproved}
            onBriefApprovedChange={setBriefApproved}
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
          clips={plan.clips}
          frameSeconds={previewSeconds}
          sourceDuration={sourceDuration}
          streamerNick={plan.streamer_banner?.nick?.trim()}
          streamerPositionY={plan.streamer_banner?.position_y}
          streamerSlideEnabled={plan.streamer_banner?.slide_enabled}
          keyDropStyle={(plan.keydrop_banner?.style as KeyDropBannerStyle | '') ?? ''}
          keyDropCode={plan.keydrop_banner?.code}
          keyDropPositionY={plan.keydrop_banner?.position_y}
          keyDropSlideEnabled={plan.keydrop_banner?.slide_enabled}
          keyDropStartSeconds={keyDropStart}
          keyDropEndSeconds={keyDropEnd}
          playing={previewPlaying}
          canPlay={sourceDuration > 0 && startMontagePlayback(plan.clips, previewSeconds) !== null}
          previewError={previewError}
          videoSrc={videoSrc}
          audioRef={previewAudioRef}
          audioKey={previewReload}
          busy={busy}
          onStreamerPositionChange={setStreamerPosition}
          onKeyDropPositionChange={setKeyDropPosition}
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
