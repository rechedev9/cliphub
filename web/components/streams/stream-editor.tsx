'use client';

import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { AlertTriangle } from 'lucide-react';
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
  clampKeyDropBannerPosition,
  clampStreamerBannerPosition,
  keyDropPreviewSourceSeconds,
  resolveKeyDropBannerPosition,
  resolveStreamerBannerPosition,
} from '@/lib/stream-preview';
import {
  DEFAULT_KEYDROP_CODE,
  DEFAULT_KEYDROP_END_SECONDS,
  DEFAULT_KEYDROP_START_SECONDS,
  STREAMER_NICK_RE,
  formatStreamClock,
  clipOutputDuration,
  insertClipSorted,
  nextClipId,
  planFingerprint,
  resolveFaceCrop,
  resolveStreamerBannerPlatform,
  streamSourceLabel,
} from '@/lib/streams/plan';
import { streamCreativeBrief } from '@/lib/streams/brief';
import { PLAYBACK_STATUS, type PlaybackStatus } from '@/lib/playback-session';
import { STREAM_PLAYBACK_MODE, streamPlaybackIndex, type StreamPlaybackMode } from '@/lib/stream-playback';
import {
  STREAM_STEP,
  STREAM_STEP_LABEL,
  shortsWord,
  streamBlockerHint,
  streamCtaLabel,
  streamEditorSteps,
  streamOutputSummary,
  streamPlanBlocker,
  type StreamStep,
} from '@/lib/streams/editor';
import {
  persistAffiliateFamily,
  selectAffiliateFamily,
  selectAffiliateOff,
  selectAffiliateStyle,
} from '@/lib/affiliate-banner';
import { StreamFrameSession } from '@/components/streams/stream-frame-session';
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
import { MomentRange } from '@/components/streams/moment-range';
import { CreativeBriefList } from '@/components/studio/creative-brief';
import { streamRangeIssue, streamRangeOverlapIssue, streamRangesIssue } from '@/lib/clip-edit';
import { StreamSaveButton } from '@/components/streams/stream-save-button';

/** Guided stream editor: moments, appearance, review, then finished videos. */
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

  const [activeStep, setActiveStep] = useState<StreamStep>(hasRender ? STREAM_STEP.results : STREAM_STEP.cuts);
  const [selectedClipId, setSelectedClipId] = useState<string | null>(plan.clips[0]?.id ?? null);
  const resultVideos = renderState?.videos ?? [];
  const resultIndex = Math.max(
    0,
    resultVideos.findIndex((v) => v.clip_id === selectedClipId),
  );
  const resultClip = resultVideos[resultIndex];
  const resultTitle = resultClip?.title || `Short ${resultIndex + 1}`;
  const hasRenderedVideos = hasRender && resultVideos.length > 0;
  const [draftRange, setDraftRange] = useState({ start_seconds: 0, end_seconds: Math.min(sourceDuration, 30) });
  const [creatingMoment, setCreatingMoment] = useState(plan.clips.length === 0);
  const [playbackMode, setPlaybackMode] = useState<StreamPlaybackMode>(STREAM_PLAYBACK_MODE.source);
  const panelRef = useRef<HTMLDivElement>(null);
  const latestPlanRef = useRef(plan);
  latestPlanRef.current = plan;
  const selectedClip = plan.clips.find((c) => c.id === selectedClipId) ?? plan.clips[0];
  const [songs, setSongs] = useState<Song[] | null>(null);
  const [previewSeconds, setPreviewSeconds] = useState(0);
  const previewSecondsRef = useRef(previewSeconds);
  previewSecondsRef.current = previewSeconds;
  const [previewPlaying, setPreviewPlaying] = useState(false);
  const [playbackStatus, setPlaybackStatus] = useState<PlaybackStatus>(PLAYBACK_STATUS.loading);
  const [previewSeek, setPreviewSeek] = useState({ seconds: 0, revision: 0 });
  const [playbackLoop, setPlaybackLoop] = useState(false);
  const [playbackClipId, setPlaybackClipId] = useState(selectedClipId);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [previewReload, setPreviewReload] = useState(0);
  const briefItems = useMemo(() => streamCreativeBrief(plan), [plan]);
  const playbackClock = useMemo(() => {
    if (playbackMode === STREAM_PLAYBACK_MODE.source) return { elapsed: previewSeconds, duration: sourceDuration };
    const activeIndex = streamPlaybackIndex(plan.clips, playbackClipId ?? selectedClipId);
    const active = plan.clips[activeIndex];
    if (!active) return { elapsed: 0, duration: 0 };
    const activeElapsed = Math.max(0, Math.min(previewSeconds, active.end_seconds) - active.start_seconds) / (active.edit?.speed ?? 1);
    if (playbackMode === STREAM_PLAYBACK_MODE.selected) {
      return { elapsed: activeElapsed, duration: clipOutputDuration(active) };
    }
    const prior = plan.clips.slice(0, activeIndex).reduce((sum, clip) => sum + clipOutputDuration(clip), 0);
    return {
      elapsed: prior + activeElapsed,
      duration: plan.clips.reduce((sum, clip) => sum + clipOutputDuration(clip), 0),
    };
  }, [playbackClipId, playbackMode, plan.clips, previewSeconds, selectedClipId, sourceDuration]);
  const previewMusic = useMemo(() => {
    const song = songs?.find((entry) => entry.id === plan.music?.key);
    return song?.previewUrl ? { url: song.previewUrl, volume: plan.music?.volume ?? 0.25 } : null;
  }, [songs, plan.music?.key, plan.music?.volume]);

  useEffect(() => {
    if (stage === 'rendering' || stage === 'rendered') {
      setActiveStep(STREAM_STEP.results);
      setPreviewPlaying(false);
    }
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

  const busy = stage === 'rendering' || saving;

  const setVariant = (variant: StreamVariant) => onPlanChange({ ...plan, variant });
  const faceCrop = resolveFaceCrop(plan.face_crop);
  const setFaceCrop = (rect: NormalizedRect) => onPlanChange({ ...plan, face_crop: rect, face_crop_reviewed: false });
  const confirmFaceCrop = () => {
    onPlanChange({ ...plan, face_crop: faceCrop, face_crop_reviewed: true });
    navigateStep('review');
  };
  function navigateStep(step: StreamStep) {
    setPreviewPlaying(false);
    setActiveStep(step);
    setPlaybackMode(step === 'cuts' ? STREAM_PLAYBACK_MODE.source : STREAM_PLAYBACK_MODE.selected);
    panelRef.current?.scrollTo(0, 0);
  }
  function seek(seconds: number) {
    const bounded = Math.max(0, Math.min(sourceDuration, seconds));
    setPreviewSeconds(bounded);
    setPreviewSeek((previous) => ({ seconds: bounded, revision: previous.revision + 1 }));
  }
  function playSelected() {
    if (!selectedClip) return;
    setPlaybackMode(STREAM_PLAYBACK_MODE.selected);
    setPreviewError(null);
    seek(selectedClip.start_seconds);
    setPreviewPlaying(true);
  }

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
      streamer_banner: {
        ...plan.streamer_banner,
        position_y: clampStreamerBannerPosition(position),
        platform: bannerPlatform,
      },
    });
  const resetStreamerPosition = () => {
    const { position_y: _position, ...banner } = plan.streamer_banner ?? {};
    onPlanChange({ ...plan, streamer_banner: { ...banner, platform: bannerPlatform } });
  };
  const setStreamerSlide = (slideEnabled: boolean) =>
    onPlanChange({
      ...plan,
      streamer_banner: { ...plan.streamer_banner, slide_enabled: slideEnabled, platform: bannerPlatform },
    });

  const longestClipSeconds = Math.max(0, ...plan.clips.map((c) => Math.max(0, c.end_seconds - c.start_seconds)));
  const keyDropStart = plan.keydrop_banner?.start_seconds ?? DEFAULT_KEYDROP_START_SECONDS;
  const keyDropEndRaw = plan.keydrop_banner?.end_seconds;
  const keyDropEnd =
    keyDropEndRaw ??
    (longestClipSeconds > 0 ? Math.min(DEFAULT_KEYDROP_END_SECONDS, longestClipSeconds) : DEFAULT_KEYDROP_END_SECONDS);

  /** Keep the 9:16 monitor inside the plate's on-screen window so code edits are visible. */
  const revealKeyDropOnPreview = (start: number, end: number) => {
    setPreviewPlaying(false);
    seek(keyDropPreviewSourceSeconds(plan.clips, previewSecondsRef.current, start, end));
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
    setCreatingMoment(false);
    setPlaybackMode(STREAM_PLAYBACK_MODE.selected);
    seek(clip.start_seconds);
    panelRef.current?.scrollTo(0, 0);
  };
  const draftClip: StreamClipRange = { id: 'draft', ...draftRange };
  const draftCandidates = insertClipSorted(plan.clips, draftClip);
  const draftIndex = draftCandidates.findIndex((c) => c.id === 'draft');
  const draftIssue =
    streamRangeIssue(draftClip, sourceDuration, draftIndex) ?? streamRangeOverlapIssue(draftCandidates, draftIndex);
  const addMoment = (range = draftRange) => {
    const sourceTitle = job.title?.trim() || 'Short';
    const title = plan.clips.length ? `${sourceTitle} ${plan.clips.length + 1}` : sourceTitle;
    const clip: StreamClipRange = { id: nextClipId(), ...range, title };
    const next = insertClipSorted(plan.clips, clip);
    const index = next.findIndex((c) => c.id === clip.id);
    const issue = streamRangeIssue(clip, sourceDuration, index) ?? streamRangeOverlapIssue(next, index);
    if (issue) {
      toast.error(issue);
      return;
    }
    setClips(next);
    selectClip(clip);
  };
  const removeClip = (clip: StreamClipRange) => {
    const next = plan.clips.filter((c) => c.id !== clip.id);
    setClips(next);
    if (next.length) selectClip(next[0]);
    else {
      setSelectedClipId(null);
      setCreatingMoment(true);
      setPlaybackMode('source');
      setPreviewPlaying(false);
    }
    toast('Momento quitado', {
      position: 'top-right',
      action: {
        label: 'Deshacer',
        onClick: () => {
          const current = latestPlanRef.current;
          if (!current.clips.some((c) => c.id === clip.id))
            onPlanChange({ ...current, clips: insertClipSorted(current.clips, clip) });
          selectClip(clip);
        },
      },
    });
  };

  const setMusicKey = (key: string) => onPlanChange({ ...plan, music: key ? { key, volume: plan.music?.volume } : {} });
  const setMusicVolume = (volume: number) => onPlanChange({ ...plan, music: { key: plan.music?.key, volume } });
  const setGrade = (grade: boolean) => onPlanChange({ ...plan, effects: { grade } });

  const musicKey = plan.music?.key ?? '';
  const musicLabel = songs?.find((song) => song.id === musicKey)?.title ?? musicKey;
  const steps = streamEditorSteps({ plan, musicLabel, renderState, stale, rendering: stage === 'rendering' });
  const activeClip =
    plan.clips.find((c) => previewSeconds >= c.start_seconds && previewSeconds < c.end_seconds) ??
    plan.clips.find((c) => c.id === selectedClipId);
  const clipProgress =
    activeClip && activeClip.end_seconds > activeClip.start_seconds
      ? ((previewSeconds - activeClip.start_seconds) / (activeClip.end_seconds - activeClip.start_seconds)) * 100
      : 0;
  const rangesIssue = streamRangesIssue(plan.clips, sourceDuration);
  const ctaLabel =
    rangesIssue && activeStep === 'review'
      ? 'Corregir momentos →'
      : streamCtaLabel({ plan, rendering: stage === 'rendering', hasRender: hasRenderedVideos, activeStep, stale });
  // While something blocks the render the CTA names it but stays a real link
  // to that step instead of a disabled dead end.
  const blocker = rangesIssue ? STREAM_STEP.cuts : streamPlanBlocker(plan);
  const ctaDisabled = busy || (activeStep === 'cuts' && (!plan.clips.length || rangesIssue !== null));
  const panelTitle = STREAM_STEP_LABEL[activeStep];
  const cropEditor =
    activeStep === STREAM_STEP.layout && variantMeta.needsFaceCrop
      ? { rect: faceCrop, disabled: busy, onChange: setFaceCrop }
      : undefined;
  const sourceMeta = [
    streamSourceLabel(job.source_url) ?? 'Archivo local',
    sourceDuration > 0 ? formatStreamClock(sourceDuration) : null,
  ]
    .filter((part): part is string => part !== null)
    .join(' · ');

  let stepContent: ReactNode;
  if (activeStep === STREAM_STEP.layout) {
    stepContent = (
      <>
        <StreamLayoutBar variant={plan.variant} disabled={busy} onVariantChange={setVariant} />
        <StreamLayoutStep
          needsFaceCrop={variantMeta.needsFaceCrop}
          faceCropReviewed={plan.face_crop_reviewed === true}
          busy={busy}
          onConfirmFaceCrop={confirmFaceCrop}
        />
        <details className="rounded-md border border-border-subtle p-3">
          <summary className="cursor-pointer font-semibold text-body-sm">Banners · opcional</summary>
          <div className="mt-3 flex flex-col gap-3">
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
          </div>
        </details>
        <details className="rounded-md border border-border-subtle p-3">
          <summary className="cursor-pointer font-semibold text-body-sm">Música y efectos · opcional</summary>
          <div className="mt-3 flex flex-col gap-3">
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
          </div>
        </details>
      </>
    );
  } else if (activeStep === STREAM_STEP.cuts) {
    stepContent = (
      <>
        <p className="text-body-sm text-fg-2">
          Reproduce el original, marca el inicio y el final y añade el momento. Cada momento será un Short.
        </p>
        {creatingMoment ? (
          <section aria-label="Nuevo momento" className="flex flex-col gap-3 rounded-md border border-stream/40 p-3">
            <h3 className="font-semibold">Nuevo momento</h3>
            <MomentRange
              id="draft"
              start={draftRange.start_seconds}
              end={draftRange.end_seconds}
              duration={sourceDuration}
              playhead={previewSeconds}
              disabled={busy}
              invalid={draftIssue !== null}
              onChange={(patch) => setDraftRange((current) => ({ ...current, ...patch }))}
            />
            {draftIssue ? (
              <p role="alert" className="text-body-sm text-destructive">
                {draftIssue}
              </p>
            ) : null}
            <Button variant="stream" disabled={busy || draftIssue !== null} onClick={() => addMoment()}>
              Añadir este momento
            </Button>
            {!plan.clips.length && sourceDuration > 0 && sourceDuration <= 60 ? (
              <Button
                variant="outline"
                disabled={busy}
                onClick={() => addMoment({ start_seconds: 0, end_seconds: sourceDuration })}
              >
                Usar vídeo completo
              </Button>
            ) : null}
            {plan.clips.length ? (
              <Button variant="ghost" onClick={() => setCreatingMoment(false)}>
                Cancelar nuevo momento
              </Button>
            ) : null}
          </section>
        ) : (
          <>
            <StreamClipEditor
              clips={plan.clips}
              sourceDuration={sourceDuration}
              selectedClipId={selectedClip?.id ?? null}
              onChange={setClips}
              onSelect={selectClip}
              onRemove={removeClip}
              disabled={busy}
              playheadSeconds={previewSeconds}
              onPlay={playSelected}
            />
            <Button
              variant="outline"
              disabled={busy}
              onClick={() => {
                setDraftRange({
                  start_seconds: Math.min(previewSeconds, Math.max(0, sourceDuration - 1)),
                  end_seconds: Math.min(sourceDuration, previewSeconds + 10),
                });
                setCreatingMoment(true);
                setPlaybackMode('source');
                setPreviewPlaying(false);
                panelRef.current?.scrollTo(0, 0);
              }}
            >
              Nuevo momento
            </Button>
          </>
        )}
      </>
    );
  } else if (activeStep === STREAM_STEP.review) {
    stepContent = (
      <>
        <p className="text-body-sm text-fg-2">Revisa cada Short antes de exportar. Se guardará un vídeo por momento.</p>
        {blocker ? (
          <Button variant="outline" className="h-auto whitespace-normal" onClick={() => navigateStep(blocker)}>
            {rangesIssue || streamBlockerHint(plan)}
          </Button>
        ) : null}
        <ul className="flex flex-col gap-2">
          {plan.clips.map((clip, index) => (
            <li key={clip.id}>
              <Button
                variant={selectedClip?.id === clip.id ? 'secondary' : 'outline'}
                className="h-auto w-full justify-start whitespace-normal text-left"
                onClick={() => selectClip(clip)}
              >
                {index + 1}. {clip.title || `Short ${index + 1}`} ·{' '}
                {formatStreamClock((clip.end_seconds - clip.start_seconds) / (clip.edit?.speed ?? 1))}
              </Button>
            </li>
          ))}
        </ul>
        <Button variant="outline" disabled={!selectedClip} onClick={playSelected}>
          Ver este Short
        </Button>
        <details className="rounded border border-border-subtle p-3">
          <summary className="cursor-pointer text-body-sm">Detalles de la exportación</summary>
          <CreativeBriefList items={briefItems} />
        </details>
      </>
    );
  } else if (stage === 'rendering') {
    stepContent = (
      <StreamRenderStage clips={plan.clips} renderState={renderState} variantLabel={variantMeta.label.toUpperCase()} />
    );
  } else if (renderedPlan && hasRender) {
    stepContent = (
      <StreamRenderResults
        renderState={renderState}
        job={job}
        renderedPlan={renderedPlan}
        stale={stale}
        selectedClipId={resultClip?.clip_id}
        onSelect={setSelectedClipId}
      />
    );
  } else {
    stepContent = <p className="text-body-sm text-fg-3">Todavía no hay un render de este stream.</p>;
  }

  return (
    <StreamFrameSession
      key={previewReload}
      videoSrc={videoSrc}
      seek={previewSeek}
      playing={previewPlaying}
      mode={playbackMode}
      loop={playbackLoop}
      clips={plan.clips}
      selectedClipId={selectedClipId}
      music={previewMusic}
      onPosition={setPreviewSeconds}
      onStatus={setPlaybackStatus}
      onPlayingChange={setPreviewPlaying}
      onClipChange={setPlaybackClipId}
      onMediaError={() => {
        setPreviewPlaying(false);
        setPreviewError(
          'No se pudo decodificar o leer el MP4 de origen. Comprueba que el archivo siga disponible y reintenta la vista previa.',
        );
      }}
    >
      <div className="-mx-(--shell-gutter) -my-10 flex flex-col @[48rem]/content:h-[calc(100dvh-var(--shell-strip-height))]">
        <StreamStepsRail
          steps={steps}
          activeStep={activeStep}
          sourceTitle={job.title?.trim() || 'Clip de stream'}
          sourceMeta={sourceMeta}
          autosave={autosave}
          onSelectStep={navigateStep}
        />

        <div className="grid min-h-0 flex-1 grid-cols-1 @[48rem]/content:grid-cols-[minmax(0,1fr)_340px]">
          <section className="flex min-h-0 min-w-0 flex-col gap-3 overflow-y-auto p-4" aria-label="Monitor">
            {activeStep === 'results' && hasRender && resultClip && renderedPlan ? (
              <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-3">
                <h2 className="font-semibold">{resultTitle}</h2>
                {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
                <video
                  key={`${resultClip.clip_id}:${renderedPlan.updated_at}`}
                  aria-label="Vídeo final"
                  controls
                  playsInline
                  preload="metadata"
                  src={streamsApi.videoUrl(job.id, renderedPlan.variant, resultClip.clip_id, renderedPlan.updated_at)}
                  className="min-h-0 max-h-[60vh] max-w-full rounded-md bg-black"
                />
                <p className="text-label text-fg-3">
                  {formatStreamClock(resultClip.duration_seconds ?? 0)} · 1080 × 1920
                </p>
              </div>
            ) : (
              <>
                <StreamMonitor
                  cropEditor={cropEditor}
                  preview={{
                    variant: plan.variant,
                    faceCrop,
                    gameplayCrop: plan.gameplay_crop,
                    clips: plan.clips,
                    activeClipId: playbackClipId ?? selectedClipId ?? undefined,
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
                    onKeyDropPositionChange: busy || cropEditor ? undefined : setKeyDropPosition,
                    onStreamerPositionChange: cropEditor ? undefined : setStreamerPosition,
                    disabled: busy || cropEditor !== undefined,
                    playheadPercent: clipProgress,
                    className: 'h-full w-auto min-h-[120px]',
                  }}
                  frameSeconds={previewSeconds}
                  sourceDuration={sourceDuration}
                  elapsedSeconds={playbackClock.elapsed}
                  playbackDuration={playbackClock.duration}
                  playing={previewPlaying}
                  status={playbackStatus}
                  canPlay={!busy && sourceDuration > 0 && (playbackMode === STREAM_PLAYBACK_MODE.source || streamPlaybackIndex(plan.clips, selectedClipId) >= 0)}
                  mode={playbackMode}
                  hasSelection={!!selectedClip}
                  clipCount={plan.clips.length}
                  onModeChange={(mode) => {
                    setPreviewPlaying(false);
                    setPlaybackMode(mode);
                    if (mode === STREAM_PLAYBACK_MODE.selected && selectedClip) seek(selectedClip.start_seconds);
                  }}
                  loop={playbackLoop}
                  onLoopChange={setPlaybackLoop}
                  onSeek={seek}
                  previewError={previewError}
                  onTogglePlay={() => {
                    setPreviewError(null);
                    setPreviewPlaying((current) => !current);
                  }}
                  onRetry={() => {
                    setPreviewError(null);
                    setPreviewReload((current) => current + 1);
                  }}
                />
                <div className="pb-3">
                  <StreamSourceTimeline
                    clips={plan.clips}
                    sourceDuration={sourceDuration}
                    selectedClipId={selectedClipId}
                    playheadSeconds={previewSeconds}
                    disabled={busy}
                    onSeek={(seconds) => {
                      setPlaybackMode(STREAM_PLAYBACK_MODE.source);
                      seek(seconds);
                    }}
                    onSelect={selectClip}
                  />
                </div>
              </>
            )}
          </section>

          <div ref={panelRef} className="min-h-0 overflow-y-auto">
            <StreamStepPanel title={panelTitle} key={activeStep}>
              {stepContent}
            </StreamStepPanel>
          </div>
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
          onCreate={() => {
            if (busy) return;
            if (activeStep === 'cuts') {
              navigateStep('layout');
              return;
            }
            if (activeStep === 'layout') {
              if (variantMeta.needsFaceCrop && !plan.face_crop_reviewed) confirmFaceCrop();
              else navigateStep('review');
              return;
            }
            if (hasRenderedVideos && !stale) {
              navigateStep('results');
              return;
            }
            if (blocker) {
              navigateStep(blocker);
              return;
            }
            onCreate();
          }}
          backLabel={activeStep === 'cuts' || activeStep === 'results' ? 'Mis proyectos' : 'Atrás'}
          onBack={() =>
            activeStep === 'cuts' || activeStep === 'results'
              ? onBack()
              : navigateStep(activeStep === 'layout' ? 'cuts' : 'layout')
          }
          action={
            activeStep === 'results' && hasRender && !stale && renderedPlan && resultClip ? (
              <StreamSaveButton
                jobId={job.id}
                variant={renderedPlan.variant}
                revision={renderedPlan.updated_at}
                clipId={resultClip.clip_id}
                title={resultTitle}
                prominent
              />
            ) : undefined
          }
        />
      </div>
    </StreamFrameSession>
  );
}
