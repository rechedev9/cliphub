'use client';

import { useCallback, useEffect, useRef, type ReactNode } from 'react';
import { Twitch } from 'lucide-react';
import type { NormalizedRect, StreamClipRange, StreamerBannerPlatform, StreamVariant } from '@/lib/api/streams';
import { DEFAULT_OVERLAY_FONT_SIZE } from '@/lib/clip-edit';
import { StreamFrameCanvas, useStreamFrame } from '@/components/streams/stream-frame-session';
import { streamAffiliateSlide, streamAffiliateWindow, streamBannerSlide, streamPreviewFade } from '@/lib/stream-playback';
import {
  clampKeyDropBannerPosition,
  clampStreamerBannerPosition,
  KEYDROP_BANNER_MAX_POSITION,
  KEYDROP_BANNER_MIN_POSITION,
  resolveKeyDropBannerPosition,
  resolveStreamerBannerPosition,
  STREAMER_BANNER_MAX_POSITION,
  STREAMER_BANNER_MIN_POSITION,
  type FrameSize,
} from '@/lib/stream-preview';
import type { KeyDropBannerStyle } from '@/lib/api/streams';
import { affiliateDisplayLabel, stylesForFamily, type AffiliateFamily } from '@/lib/api/types';
import { cn } from '@/lib/utils';

const FULL_FRAME: NormalizedRect = { x: 0, y: 0, width: 1, height: 1 };
const EMPTY_CLIPS: StreamClipRange[] = [];
const PREVIEW_HEIGHT = 1920;

const PREVIEW_LAYOUTS: Record<
  StreamVariant,
  { face?: FrameSize; gameplay: FrameSize }
> = {
  'streamer-vertical-stack-40-60': {
    face: { width: 1080, height: 768 },
    gameplay: { width: 1080, height: 1152 },
  },
  'streamer-vertical-stack': {
    face: { width: 1080, height: 520 },
    gameplay: { width: 1080, height: 1400 },
  },
  'streamer-fullframe-nocam': {
    gameplay: { width: 1080, height: 1920 },
  },
};

/** One preview band using the same cover-crop geometry as the FFmpeg render. */
function CroppedFrame({
  rect,
  output,
  band,
  className,
}: {
  rect: NormalizedRect;
  output: FrameSize;
  band: 'facecam' | 'gameplay';
  className?: string;
}) {
  return (
    <div className={className} style={{ overflow: 'hidden', position: 'relative' }} data-preview-band={band}>
      <StreamFrameCanvas
        mode="cover"
        rect={rect}
        outputWidth={output.width}
        outputHeight={output.height}
        className="absolute inset-0 h-full w-full"
      />
    </div>
  );
}

/** Live 9:16 preview; band sizes mirror internal/streamclips variants. */
export function StreamPreview({
  variant,
  faceCrop,
  gameplayCrop,
  clips = EMPTY_CLIPS,
  activeClipId,
  grade = false,
  frameSeconds,
  streamerNick,
  streamerPlatform = 'twitch',
  streamerPositionY,
  streamerSlideEnabled = false,
  onStreamerPositionChange,
  keyDropFamily,
  keyDropStyle,
  keyDropCode,
  keyDropPositionY,
  keyDropSlideEnabled = false,
  keyDropStartSeconds = 0,
  keyDropEndSeconds,
  onKeyDropPositionChange,
  disabled = false,
  className,
  playheadPercent,
}: {
  variant: StreamVariant;
  faceCrop?: NormalizedRect;
  gameplayCrop?: NormalizedRect;
  clips?: StreamClipRange[];
  activeClipId?: string;
  grade?: boolean;
  frameSeconds: number;
  streamerNick?: string;
  streamerPlatform?: StreamerBannerPlatform;
  streamerPositionY?: number;
  streamerSlideEnabled?: boolean;
  onStreamerPositionChange?: (position: number) => void;
  keyDropFamily?: AffiliateFamily | '';
  keyDropStyle?: KeyDropBannerStyle | '';
  keyDropCode?: string;
  keyDropPositionY?: number;
  keyDropSlideEnabled?: boolean;
  /** Plate visibility window relative to each clip start (seconds). */
  keyDropStartSeconds?: number;
  keyDropEndSeconds?: number;
  onKeyDropPositionChange?: (position: number) => void;
  disabled?: boolean;
  /** Sizing override; the default is a 220px-wide column frame. */
  className?: string;
  /** Progress inside the current cut, drawn as the 3px magenta head at the foot. */
  playheadPercent?: number;
}): ReactNode {
  const containerRef = useRef<HTMLDivElement>(null);
  const compositionRef = useRef<HTMLDivElement>(null);
  const streamerSlideRef = useRef<HTMLDivElement>(null);
  const affiliateRef = useRef<HTMLDivElement>(null);
  const affiliateSlideRef = useRef<HTMLDivElement>(null);
  const progressRef = useRef<HTMLSpanElement>(null);
  const overlayRefs = useRef<(HTMLSpanElement | null)[]>([]);
  const { session } = useStreamFrame();
  const dragRef = useRef<{ startClientY: number; startPosition: number } | null>(null);
  const keyDropDragRef = useRef<{ startClientY: number; startPosition: number } | null>(null);
  const gameplay = gameplayCrop ?? FULL_FRAME;
  const layout = PREVIEW_LAYOUTS[variant];
  const faceLayout = layout.face;
  const facePct = faceLayout
    ? (faceLayout.height * 100) / (faceLayout.height + layout.gameplay.height)
    : 0;
  const bannerPosition = resolveStreamerBannerPosition(variant, streamerPositionY);
  const keyDropPosition = resolveKeyDropBannerPosition(keyDropPositionY);
  const keyDropPlate = stylesForFamily(keyDropFamily ?? '').find((entry) => entry.id === keyDropStyle);
  const keyDropLabel = affiliateDisplayLabel(keyDropFamily ?? '', keyDropStyle ?? '', keyDropCode ?? '');
  // Clip identity is explicit: overlapping or reordered cuts are distinct Shorts.
  const activeClip = clips.find((clip) => clip.id === activeClipId);
  const activeOverlays = activeClip?.edit?.text_overlays ?? [];
  const clipLen = activeClip ? activeClip.end_seconds - activeClip.start_seconds : 0;
  const kdWindow = streamAffiliateWindow(keyDropStartSeconds, keyDropEndSeconds ?? 0, clipLen);
  const updateLayers = useRef<(seconds: number) => void>(() => {});
  updateLayers.current = (seconds) => {
    const local = seconds - (activeClip?.start_seconds ?? 0);
    const inClip = activeClip !== undefined && local >= 0 && local < clipLen;
    if (compositionRef.current) compositionRef.current.style.opacity = String(activeClip ? streamPreviewFade(activeClip, seconds) : 1);
    if (streamerSlideRef.current) streamerSlideRef.current.style.transform = 'translateX(' + (streamerSlideEnabled ? streamBannerSlide(local, clipLen) : 0) + '%)';
    if (affiliateRef.current) affiliateRef.current.style.visibility = inClip && local >= kdWindow.start && local <= kdWindow.end ? 'visible' : 'hidden';
    if (affiliateSlideRef.current) affiliateSlideRef.current.style.transform = 'translateX(' + (keyDropSlideEnabled ? streamAffiliateSlide(local, keyDropStartSeconds, keyDropEndSeconds ?? 0, clipLen) : 0) + '%)';
    activeOverlays.forEach((overlay, index) => {
      const element = overlayRefs.current[index];
      if (element) element.style.visibility = inClip && local >= (overlay.start_seconds ?? 0) && local <= (overlay.end_seconds ?? clipLen) ? 'visible' : 'hidden';
    });
    if (progressRef.current) progressRef.current.style.width = String(clipLen > 0 ? Math.min(100, Math.max(0, local / clipLen * 100)) : 0) + '%';
  };
  useEffect(() => {
    if (!session) return;
    const update = (): void => updateLayers.current(session.frame.seconds);
    update();
    return session.subscribeFrames(update);
  }, [session]);
  useEffect(() => { updateLayers.current(session?.frame.seconds ?? frameSeconds); });
  useEffect(() => {
    const plate = affiliateRef.current;
    const container = containerRef.current;
    if (!plate || !container) return;
    const resize = (): void => {
      // FFmpeg keeps the full plate inside the output even at edge positions.
      container.style.setProperty('--affiliate-half-height', String(plate.offsetHeight / 2) + 'px');
    };
    const observer = new ResizeObserver(resize);
    observer.observe(plate);
    resize();
    return () => observer.disconnect();
  }, [keyDropStyle]);

  const beginBannerDrag = useCallback((event: React.PointerEvent<HTMLDivElement>) => {
    if (disabled || !onStreamerPositionChange) return;
    event.preventDefault();
    event.currentTarget.setPointerCapture(event.pointerId);
    dragRef.current = { startClientY: event.clientY, startPosition: bannerPosition };
  }, [bannerPosition, disabled, onStreamerPositionChange]);

  const moveBanner = useCallback((event: React.PointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current;
    const container = containerRef.current;
    if (!drag || !container || !onStreamerPositionChange) return;
    const height = container.clientHeight;
    if (height <= 0) return;
    onStreamerPositionChange(clampStreamerBannerPosition(drag.startPosition + (event.clientY - drag.startClientY) / height));
  }, [onStreamerPositionChange]);

  const endBannerDrag = useCallback(() => {
    dragRef.current = null;
  }, []);

  const moveBannerWithKeyboard = useCallback((event: React.KeyboardEvent<HTMLDivElement>) => {
    if (disabled || !onStreamerPositionChange) return;
    let next: number | undefined;
    if (event.key === 'ArrowUp') next = bannerPosition - 0.01;
    if (event.key === 'ArrowDown') next = bannerPosition + 0.01;
    if (event.key === 'Home') next = STREAMER_BANNER_MIN_POSITION;
    if (event.key === 'End') next = STREAMER_BANNER_MAX_POSITION;
    if (next === undefined) return;
    event.preventDefault();
    onStreamerPositionChange(clampStreamerBannerPosition(next));
  }, [bannerPosition, disabled, onStreamerPositionChange]);

  const beginKeyDropDrag = useCallback((event: React.PointerEvent<HTMLDivElement>) => {
    if (disabled || !onKeyDropPositionChange) return;
    event.preventDefault();
    event.currentTarget.setPointerCapture(event.pointerId);
    keyDropDragRef.current = { startClientY: event.clientY, startPosition: keyDropPosition };
  }, [disabled, keyDropPosition, onKeyDropPositionChange]);

  const moveKeyDrop = useCallback((event: React.PointerEvent<HTMLDivElement>) => {
    const drag = keyDropDragRef.current;
    const container = containerRef.current;
    if (!drag || !container || !onKeyDropPositionChange) return;
    const height = container.clientHeight;
    if (height <= 0) return;
    onKeyDropPositionChange(clampKeyDropBannerPosition(drag.startPosition + (event.clientY - drag.startClientY) / height));
  }, [onKeyDropPositionChange]);

  const endKeyDropDrag = useCallback(() => {
    keyDropDragRef.current = null;
  }, []);

  const moveKeyDropWithKeyboard = useCallback((event: React.KeyboardEvent<HTMLDivElement>) => {
    if (disabled || !onKeyDropPositionChange) return;
    let next: number | undefined;
    if (event.key === 'ArrowUp') next = keyDropPosition - 0.01;
    if (event.key === 'ArrowDown') next = keyDropPosition + 0.01;
    if (event.key === 'Home') next = KEYDROP_BANNER_MIN_POSITION;
    if (event.key === 'End') next = KEYDROP_BANNER_MAX_POSITION;
    if (next === undefined) return;
    event.preventDefault();
    onKeyDropPositionChange(clampKeyDropBannerPosition(next));
  }, [disabled, keyDropPosition, onKeyDropPositionChange]);

  return (
    <div
      ref={containerRef}
      className={cn(
        'relative aspect-[9/16] overflow-hidden border border-stream/45 bg-black shadow-[var(--elev-2)]',
        className ?? 'mx-auto w-full max-w-[220px]',
      )}
      style={{ containerType: 'size' }}
    >
      <div ref={compositionRef} className="absolute inset-0" style={{ filter: grade ? 'contrast(1.05) saturate(1.15)' : undefined }}>
      <div className="flex h-full w-full flex-col">
        {faceLayout ? (
          <div style={{ height: `${facePct}%` }} className="w-full">
            <CroppedFrame
              rect={faceCrop ?? FULL_FRAME}
              output={faceLayout}
              band="facecam"
              className="h-full w-full"
            />
          </div>
        ) : null}
        <div style={{ height: faceLayout ? `${100 - facePct}%` : '100%' }} className="w-full">
          <CroppedFrame
            rect={gameplay}
            output={layout.gameplay}
            band="gameplay"
            className="h-full w-full"
          />
        </div>
      </div>
      {activeOverlays.map((overlay, i) => (
        <span
          key={i}
          ref={(element) => { overlayRefs.current[i] = element; }}
          className="pointer-events-none absolute left-0 w-full -translate-y-1/2 px-[4%] text-center font-[family-name:var(--font-display)] font-black leading-tight text-white"
          style={{
            top: `${overlay.position_y * 100}%`,
            // Match the render: font_size output pixels on the 1920px-tall canvas.
            fontSize: `${((overlay.font_size ?? DEFAULT_OVERLAY_FONT_SIZE) / PREVIEW_HEIGHT) * 100}cqh`,
            textShadow: '0 0 2px rgba(0,0,0,0.9), 0 1px 2px rgba(0,0,0,0.55)',
          }}
        >
          {overlay.text}
        </span>
      ))}
      {streamerNick ? (
        <div
          role="slider"
          tabIndex={disabled ? -1 : 0}
          aria-label="Posición del banner en la vista previa"
          aria-orientation="vertical"
          aria-valuemin={STREAMER_BANNER_MIN_POSITION * 100}
          aria-valuemax={STREAMER_BANNER_MAX_POSITION * 100}
          aria-valuenow={Math.round(bannerPosition * 1000) / 10}
          aria-disabled={disabled}
          data-streamer-banner
          onPointerDown={beginBannerDrag}
          onPointerMove={moveBanner}
          onPointerUp={endBannerDrag}
          onPointerCancel={endBannerDrag}
          onKeyDown={moveBannerWithKeyboard}
          className={`absolute left-0 h-[5%] w-full -translate-y-1/2 touch-none select-none ${disabled ? 'cursor-default opacity-60' : 'cursor-ns-resize'}`}
          style={{ top: `${bannerPosition * 100}%` }}
        >
          <div
            ref={streamerSlideRef}
            className={`flex h-full w-full items-center shadow-sm ${
              streamerPlatform === 'kick' ? 'bg-[#53fc18] text-black' : 'bg-[#9146ff] text-white'
            }`}
          >
            <span
              className={`flex h-full w-[11%] shrink-0 items-center justify-center ${
                streamerPlatform === 'kick' ? 'bg-[#0d0d0d] text-[#53fc18]' : 'bg-[#5b1ba9]'
              }`}
            >
              {streamerPlatform === 'kick' ? (
                <KickMark />
              ) : (
                <Twitch className="h-[62%] w-[62%]" strokeWidth={2.6} aria-hidden />
              )}
            </span>
            <span className="truncate px-[3%] font-[family-name:var(--font-display)] text-[clamp(7px,3.2vw,12px)] font-black leading-none tracking-[0.02em]">
              {streamerNick}
            </span>
          </div>
        </div>
      ) : null}
      {keyDropStyle ? (
        <div
          ref={affiliateRef}
          role="slider"
          tabIndex={disabled ? -1 : 0}
          aria-label="Posición del banner afiliado en la vista previa"
          aria-orientation="vertical"
          aria-valuemin={KEYDROP_BANNER_MIN_POSITION * 100}
          aria-valuemax={KEYDROP_BANNER_MAX_POSITION * 100}
          aria-valuenow={Math.round(keyDropPosition * 1000) / 10}
          aria-disabled={disabled}
          data-keydrop-banner
          onPointerDown={beginKeyDropDrag}
          onPointerMove={moveKeyDrop}
          onPointerUp={endKeyDropDrag}
          onPointerCancel={endKeyDropDrag}
          onKeyDown={moveKeyDropWithKeyboard}
          className={`absolute left-1/2 w-[55%] -translate-x-1/2 -translate-y-1/2 touch-none select-none ${disabled ? 'cursor-default opacity-90' : 'cursor-ns-resize'}`}
          style={{ top: `clamp(var(--affiliate-half-height, 0px), ${keyDropPosition * 100}%, calc(100% - var(--affiliate-half-height, 0px)))` }}
        >
          {/* Same plates the Go renderer embeds; live code is drawn on top. */}
          <div ref={affiliateSlideRef} className="relative w-full">
            <img
              src={keyDropPlate?.preview ?? ''}
              alt=""
              draggable={false}
              className="pointer-events-none block h-auto w-full select-none drop-shadow-[0_4px_12px_rgba(0,0,0,0.55)]"
            />
            {keyDropPlate?.coverClass ? (
              <span aria-hidden className={`pointer-events-none absolute ${keyDropPlate.textClass} ${keyDropPlate.coverClass}`} />
            ) : null}
            <span
              className={`pointer-events-none absolute flex items-center justify-center truncate text-center font-[family-name:var(--font-display)] text-[clamp(6px,2.5vw,11px)] font-black leading-none tracking-[0.03em] text-white drop-shadow-[0_1px_2px_rgba(0,0,0,0.95)] ${keyDropPlate?.textClass ?? 'left-[28%] right-[10%] top-[44%] h-[15%]'}`}
              aria-hidden
            >
              {keyDropLabel}
            </span>
          </div>
        </div>
      ) : null}
      </div>
      {playheadPercent !== undefined ? (
        <span
          ref={progressRef}
          aria-hidden
          className="pointer-events-none absolute bottom-0 left-0 h-[3px] bg-stream"
          style={{ width: `${Math.min(100, Math.max(0, playheadPercent))}%` }}
        />
      ) : null}
    </div>
  );
}

function KickMark(): ReactNode {
  return (
    <svg viewBox="0 0 8 9" className="h-[75%] w-auto" shapeRendering="crispEdges" aria-hidden>
      <path fill="currentColor" d="M0 0h3v2h1V1h1V0h3v3H7v1H6v1h1v1h1v3H5V8H4V7H3v2H0z" />
    </svg>
  );
}
