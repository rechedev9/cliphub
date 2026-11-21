'use client';

import { streamTitle } from '@/lib/streams/title';

import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import {
  STREAM_VARIANTS,
  streamsApi,
  type NormalizedRect,
  type StreamClipRange,
  type StreamEditPlan,
  type StreamJob,
  type StreamPlay,
  type StreamRenderState,
} from '@/lib/api/streams';
import { api } from '@/lib/api';
import { isKeyDropBannerStyle, type Song } from '@/lib/api/types';
import { keyDropPreviewSourceSeconds } from '@/lib/stream-preview';
import {
  formatStreamClock,
  clipOutputDuration,
  planFingerprint,
  resolveFaceCrop,
  streamSourceLabel,
} from '@/lib/streams/plan';
import { PLAYBACK_STATUS, type PlaybackStatus } from '@/lib/playback-session';
import { STREAM_PLAYBACK_MODE, streamPlaybackIndex, type StreamPlaybackMode } from '@/lib/stream-playback';
import {
  STREAM_STEP,
  STREAM_STEP_LABEL,
  streamCtaLabel,
  streamEditorSteps,
  streamFooterLabels,
  streamPlanBlocker,
  streamRenderQueued,
  streamTimelineLabel,
  type StreamStep,
} from '@/lib/streams/editor';
import { StreamFrameSession } from '@/components/streams/stream-frame-session';
import { StreamStepsRail, type StreamAutosaveState } from '@/components/streams/stream-steps-rail';
import { StreamMonitor } from '@/components/streams/stream-monitor';
import { StreamSourceTimeline } from '@/components/streams/stream-source-timeline';
import { StreamStepPanel } from '@/components/streams/stream-step-panel';
import { StreamFooter } from '@/components/streams/stream-footer';
import { streamRangesIssue } from '@/lib/clip-edit';
import { StreamSaveButton } from '@/components/streams/stream-save-button';
import { streamMusicOffsets, streamPlanShorts, type StreamShort } from '@/lib/streams/groups';
import { recordRendererError } from '@/lib/desktop-telemetry';
import { streamBannerActions } from '@/components/streams/stream-banner-actions';
import { StreamLookStep } from '@/components/streams/stream-look-step';
import { StreamMomentsStep } from '@/components/streams/stream-moments-step';
import { StreamResultsStep, StreamResultVideo } from '@/components/streams/stream-results-step';
import { StreamReviewStep } from '@/components/streams/stream-review-step';
import { useStreamMoments } from '@/components/streams/use-stream-moments';
import { momentZoomWindow } from '@/lib/streams/moment-edges';
import { MOMENTS_KEY } from '@/lib/streams/moments-keys';
import { timelineSeekKeepsPlayback } from '@/lib/streams/timeline';

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

  const [requestedStep, setActiveStep] = useState<StreamStep>(hasRender ? STREAM_STEP.results : STREAM_STEP.cuts);
  // A rejected or failed export leaves nothing to save, so that step falls back to the one that can retry.
  const activeStep =
    requestedStep === STREAM_STEP.results && stage === 'editing' && !hasRender ? STREAM_STEP.review : requestedStep;
  const [selectedClipId, setSelectedClipId] = useState<string | null>(plan.clips[0]?.id ?? null);
  const resultVideos = renderState?.videos ?? [];
  const shorts = useMemo(() => streamPlanShorts(plan), [plan]);
  // A moment inside a joined Short selects that Short: its video and, in Revisar, its preview.
  const selectedShort = shorts.find((s) => s.clips.some((c) => c.id === selectedClipId));
  const selectedVideoId = selectedShort?.id ?? selectedClipId;
  const resultIndex = Math.max(
    0,
    resultVideos.findIndex((v) => v.clip_id === selectedVideoId),
  );
  const resultClip = resultVideos[resultIndex];
  const resultTitle = resultClip?.title || `Short ${resultIndex + 1}`;
  const hasRenderedVideos = hasRender && resultVideos.length > 0;
  const [playbackMode, setPlaybackMode] = useState<StreamPlaybackMode>(STREAM_PLAYBACK_MODE.source);
  const panelRef = useRef<HTMLDivElement>(null);
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
  // In Revisar the selected Short is the unit: a joined one plays its moments in the group's order.
  const previewGroup = activeStep === STREAM_STEP.review && selectedShort?.groupId ? selectedShort : undefined;
  const groupPlaying = previewGroup && playbackMode === STREAM_PLAYBACK_MODE.selected ? previewGroup : undefined;
  const [previewPlay, setPreviewPlay] = useState<{ play: StreamPlay; range: StreamClipRange } | null>(null);
  const [foundPlays, setFoundPlays] = useState<readonly StreamPlay[]>([]);
  const [zoomed, setZoomed] = useState(false);
  const cutsStep = activeStep === STREAM_STEP.cuts;
  const previewing = cutsStep && playbackMode === STREAM_PLAYBACK_MODE.selected ? previewPlay : null;
  const groupClips = groupPlaying?.clips;
  const playbackClips = useMemo(
    () => (previewing ? [previewing.range] : (groupClips ?? plan.clips)),
    [previewing, groupClips, plan.clips],
  );
  const sessionMode = groupPlaying ? STREAM_PLAYBACK_MODE.sequence : playbackMode;
  const sessionClipId = previewing?.range.id ?? selectedClipId;
  const playbackClock = useMemo(() => {
    if (sessionMode === STREAM_PLAYBACK_MODE.source) return { elapsed: previewSeconds, duration: sourceDuration };
    const activeIndex = streamPlaybackIndex(playbackClips, playbackClipId ?? sessionClipId);
    const active = playbackClips[activeIndex];
    if (!active) return { elapsed: 0, duration: 0 };
    const activeElapsed = Math.max(0, Math.min(previewSeconds, active.end_seconds) - active.start_seconds) / (active.edit?.speed ?? 1);
    if (sessionMode === STREAM_PLAYBACK_MODE.selected) {
      return { elapsed: activeElapsed, duration: clipOutputDuration(active) };
    }
    const prior = playbackClips.slice(0, activeIndex).reduce((sum, clip) => sum + clipOutputDuration(clip), 0);
    return {
      elapsed: prior + activeElapsed,
      duration: playbackClips.reduce((sum, clip) => sum + clipOutputDuration(clip), 0),
    };
  }, [playbackClipId, sessionMode, playbackClips, previewSeconds, sessionClipId, sourceDuration]);
  const musicOffsets = useMemo(() => streamMusicOffsets(shorts), [shorts]);
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

  const faceCrop = resolveFaceCrop(plan.face_crop);
  const setFaceCrop = (rect: NormalizedRect) => onPlanChange({ ...plan, face_crop: rect, face_crop_reviewed: false });
  const confirmFaceCrop = () => {
    onPlanChange({ ...plan, face_crop: faceCrop, face_crop_reviewed: true });
    navigateStep('review');
  };
  function navigateStep(step: StreamStep) {
    setPreviewPlaying(false);
    setPreviewPlay(null);
    setActiveStep(step);
    setPlaybackMode(step === 'cuts' ? STREAM_PLAYBACK_MODE.source : STREAM_PLAYBACK_MODE.selected);
    const first = step === STREAM_STEP.review && selectedShort?.groupId ? selectedShort.clips[0] : undefined;
    if (first) seek(first.start_seconds);
    panelRef.current?.scrollTo(0, 0);
  }
  function seek(seconds: number) {
    const bounded = Math.max(0, Math.min(sourceDuration, seconds));
    setPreviewSeconds(bounded);
    setPreviewSeek((previous) => ({ seconds: bounded, revision: previous.revision + 1 }));
  }
  function playSelected() {
    const first = previewGroup?.clips[0] ?? selectedClip;
    if (!first) return;
    setPreviewPlay(null);
    setPlaybackMode(STREAM_PLAYBACK_MODE.selected);
    setPreviewError(null);
    seek(first.start_seconds);
    setPreviewPlaying(true);
  }
  function watchPlay(play: StreamPlay) {
    setZoomed(false);
    setPreviewPlay({ play, range: { id: `play:${play.id}`, start_seconds: play.start_seconds, end_seconds: play.end_seconds } });
    setPlaybackMode(STREAM_PLAYBACK_MODE.selected);
    setPreviewError(null);
    seek(play.start_seconds);
    setPreviewPlaying(true);
  }
  function playAllMoments() {
    const [first] = plan.clips;
    if (!first) return;
    setZoomed(false);
    setPreviewPlay(null);
    setSelectedClipId(first.id);
    setPlaybackMode(STREAM_PLAYBACK_MODE.sequence);
    setPreviewError(null);
    seek(first.start_seconds);
    setPreviewPlaying(true);
  }

  /** Keep the 9:16 monitor inside the plate's on-screen window so code edits are visible. */
  const revealKeyDropOnPreview = (start: number, end: number) => {
    setPreviewPlaying(false);
    seek(keyDropPreviewSourceSeconds(plan.clips, previewSecondsRef.current, start, end));
  };
  const banners = streamBannerActions({ job, plan, onPlanChange, revealKeyDrop: revealKeyDropOnPreview });
  const { bannerPlatform, keyDropStart, keyDropEnd, setKeyDropPosition, setStreamerPosition } = banners;

  const moments = useStreamMoments({
    job,
    plan,
    sourceDuration,
    onPlanChange,
    focusClip: (clip: StreamClipRange) => {
      setPreviewPlay(null);
      setZoomed(true);
      setSelectedClipId(clip.id);
      setPlaybackMode(STREAM_PLAYBACK_MODE.selected);
      seek(clip.start_seconds);
      panelRef.current?.scrollTo(0, 0);
    },
    clearSelection: () => {
      setPreviewPlay(null);
      setSelectedClipId(null);
      setPlaybackMode('source');
      setPreviewPlaying(false);
    },
    showEdge: (seconds: number) => {
      setPreviewPlay(null);
      setPreviewPlaying(false);
      setPlaybackMode(STREAM_PLAYBACK_MODE.source);
      seek(seconds);
    },
    showWholeVideo: () => setZoomed(false),
  });
  const zoomClip = cutsStep && zoomed ? selectedClip : undefined;
  const selectClip = moments.selectClip;
  const playMoment = (clip: StreamClipRange) => {
    selectClip(clip);
    setPreviewError(null);
    setPreviewPlaying(true);
  };
  const selectShort = (short: StreamShort) => {
    const [first] = short.clips;
    if (!first) return;
    setPreviewPlaying(false);
    selectClip(first);
  };

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
  // An empty plan is a missing step, not a broken range: the blocker below names it.
  const rangesIssue = plan.clips.length > 0 ? streamRangesIssue(plan.clips, sourceDuration) : null;
  const ctaLabel =
    rangesIssue && activeStep === 'review'
      ? 'Corregir momentos →'
      : streamCtaLabel({
          plan,
          rendering: stage === 'rendering',
          queued: streamRenderQueued(renderState),
          hasRender: hasRenderedVideos,
          activeStep,
          stale,
        });
  // While something blocks the render the CTA names it but stays a real link
  // to that step instead of a disabled dead end.
  const blocker = rangesIssue ? STREAM_STEP.cuts : streamPlanBlocker(plan);
  const ctaDisabled = busy || (activeStep === 'cuts' && (!plan.clips.length || rangesIssue !== null));
  const panelTitle = STREAM_STEP_LABEL[activeStep];
  const footer = streamFooterLabels({ step: activeStep, plan, stale, sourceDuration });
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

  let timelineClipId = playbackMode === STREAM_PLAYBACK_MODE.sequence ? (playbackClipId ?? selectedClipId) : selectedClipId;
  if (previewing) timelineClipId = null;
  let watchingLabel = 'Vídeo original';
  if (previewing) watchingLabel = `Jugada · ${previewing.play.label}`;
  else if (sessionMode === STREAM_PLAYBACK_MODE.sequence) watchingLabel = 'Todos tus momentos seguidos';
  else if (sessionMode === STREAM_PLAYBACK_MODE.selected && selectedClip) watchingLabel = selectedClip.title?.trim() || 'Tu momento';
  const canPlay = !busy && sourceDuration > 0 && (playbackMode === STREAM_PLAYBACK_MODE.source || streamPlaybackIndex(playbackClips, sessionClipId) >= 0);
  const togglePlay = () => {
    if (!canPlay) return;
    setPreviewError(null);
    setPreviewPlaying((current) => !current);
  };
  const monitor = (
    <StreamMonitor
      cropEditor={cropEditor}
      preview={{
        variant: plan.variant,
        faceCrop,
        gameplayCrop: plan.gameplay_crop,
        clips: playbackClips,
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
      elapsedSeconds={playbackClock.elapsed}
      playbackDuration={playbackClock.duration}
      playing={previewPlaying}
      status={playbackStatus}
      canPlay={canPlay}
      mode={playbackMode}
      hasSelection={!!selectedClip}
      clipCount={plan.clips.length}
      onModeChange={(mode) => {
        setPreviewPlaying(false);
        setPlaybackMode(mode);
        const first = previewGroup?.clips[0] ?? selectedClip;
        if (mode === STREAM_PLAYBACK_MODE.selected && first) seek(first.start_seconds);
      }}
      loop={playbackLoop}
      onLoopChange={setPlaybackLoop}
      previewError={previewError}
      onTogglePlay={togglePlay}
      onRetry={() => {
        setPreviewError(null);
        setPreviewReload((current) => current + 1);
      }}
      watching={cutsStep ? watchingLabel : undefined}
      playShortcut={cutsStep ? MOMENTS_KEY.play : undefined}
    />
  );
  const timeline = (
    <StreamSourceTimeline
      clips={plan.clips}
      plays={cutsStep && job.source_kind === 'vod' ? foundPlays : []}
      activePlayId={previewing?.play.id ?? null}
      onWatchPlay={watchPlay}
      label={streamTimelineLabel({
        mode: playbackMode,
        shorts,
        clipId: timelineClipId,
      })}
      sourceDuration={sourceDuration}
      selectedClipIds={selectedShort?.clips.map((clip) => clip.id) ?? []}
      playheadSeconds={previewSeconds}
      disabled={busy}
      zoom={
        zoomClip
          ? {
              clipId: zoomClip.id,
              window: momentZoomWindow(zoomClip, sourceDuration),
              onEdgeDrag: ({ edge, seconds }) => moments.moveEdge({ clipId: zoomClip.id, edge, seconds }),
              onShowAll: () => setZoomed(false),
            }
          : undefined
      }
      onSeek={(seconds) => {
        const keeps = timelineSeekKeepsPlayback({
          mode: sessionMode,
          seconds,
          preview: previewing?.range ?? null,
          selected: selectedClip,
          clips: playbackClips,
        });
        if (!keeps) {
          setPreviewPlay(null);
          setPlaybackMode(STREAM_PLAYBACK_MODE.source);
        }
        seek(seconds);
      }}
      onSelect={selectClip}
    />
  );

  let stepContent: ReactNode;
  if (activeStep === STREAM_STEP.layout) {
    stepContent = (
      <StreamLookStep job={job} plan={plan} banners={banners} songs={songs} busy={busy} onPlanChange={onPlanChange} />
    );
  } else if (activeStep === STREAM_STEP.cuts) {
    stepContent = (
      <StreamMomentsStep
        job={job}
        plan={plan}
        moments={moments}
        sourceDuration={sourceDuration}
        playheadSeconds={previewSeconds}
        selectedClipId={selectedClipId}
        cardClipId={selectedClip?.id ?? null}
        plays={foundPlays}
        activePlayId={previewing?.play.id ?? null}
        busy={busy}
        player={monitor}
        timeline={timeline}
        onWatchPlay={watchPlay}
        onPlayMoment={playMoment}
        onPlayAll={playAllMoments}
        onPlaySelected={playSelected}
        onTogglePlay={togglePlay}
        onPlaysShown={setFoundPlays}
      />
    );
  } else if (activeStep === STREAM_STEP.review) {
    stepContent = (
      <StreamReviewStep
        plan={plan}
        shorts={shorts}
        selectedShortId={selectedShort?.id ?? null}
        musicLabel={musicLabel}
        blocker={blocker}
        rangesIssue={rangesIssue}
        canPlay={!!selectedClip}
        busy={busy}
        onNavigate={navigateStep}
        onSelectShort={selectShort}
        onPlaySelected={playSelected}
        onPlanChange={onPlanChange}
      />
    );
  } else {
    stepContent = (
      <StreamResultsStep
        job={job}
        stage={stage}
        shorts={shorts}
        renderState={renderState}
        renderedPlan={renderedPlan}
        hasRender={hasRender}
        stale={stale}
        variantLabel={variantMeta.label}
        selectedClipId={resultClip?.clip_id}
        onSelect={setSelectedClipId}
      />
    );
  }

  return (
    <StreamFrameSession
      key={previewReload}
      videoSrc={videoSrc}
      seek={previewSeek}
      playing={previewPlaying}
      mode={sessionMode}
      loop={cutsStep ? false : playbackLoop}
      clips={playbackClips}
      selectedClipId={sessionClipId}
      music={previewMusic}
      musicOffsets={musicOffsets}
      onPosition={setPreviewSeconds}
      onStatus={setPlaybackStatus}
      onPlayingChange={setPreviewPlaying}
      onClipChange={setPlaybackClipId}
      onMediaError={(detail) => {
        recordRendererError('stream_preview.error', new Error(`Vista previa de stream: ${detail}`));
        setPreviewPlaying(false);
        setPreviewError(
          'No se pudo decodificar o leer el MP4 de origen. Comprueba que el archivo siga disponible y reintenta la vista previa.',
        );
      }}
    >
      <div className="-mx-(--shell-gutter) -my-4 flex flex-col @[48rem]/content:h-[calc(100dvh-var(--shell-strip-height))]">
        <StreamStepsRail
          steps={steps}
          activeStep={activeStep}
          sourceTitle={streamTitle(job)}
          sourceMeta={sourceMeta}
          autosave={autosave}
          onSelectStep={navigateStep}
        />

        {cutsStep ? (
          <div ref={panelRef} className="min-h-0 min-w-0 flex-1 overflow-y-auto">
            <h2 className="sr-only">{panelTitle}</h2>
            {stepContent}
          </div>
        ) : (
          <div className="grid min-h-0 flex-1 grid-cols-1 @[48rem]/content:grid-cols-[minmax(0,1fr)_340px]">
            <section className="flex min-h-0 min-w-0 flex-col gap-3 overflow-y-auto p-4" aria-label="Monitor">
              {activeStep === 'results' && hasRender && resultClip && renderedPlan ? (
                <StreamResultVideo job={job} renderedPlan={renderedPlan} video={resultClip} title={resultTitle} />
              ) : (
                <>
                  {monitor}
                  <div className="shrink-0 pb-3">{timeline}</div>
                </>
              )}
            </section>

            <div ref={panelRef} className="min-h-0 overflow-y-auto bg-surface-1 shadow-[inset_1px_0_0_0_var(--border-subtle)]">
              <StreamStepPanel title={panelTitle} key={activeStep}>
                {stepContent}
              </StreamStepPanel>
            </div>
          </div>
        )}

        <StreamFooter
          error={error}
          countLabel={footer.count}
          summary={footer.summary}
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
          backLabel={activeStep === 'cuts' || activeStep === 'results' ? 'Tus proyectos' : 'Atrás'}
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
