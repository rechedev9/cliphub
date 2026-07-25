'use client';

import { useCallback, useRef, type ReactNode } from 'react';
import { Twitch } from 'lucide-react';
import type { NormalizedRect, StreamClipRange, StreamVariant } from '@/lib/api/streams';
import { DEFAULT_OVERLAY_FONT_SIZE } from '@/lib/clip-edit';
import { StreamFrameCanvas } from '@/components/streams/stream-frame-session';
import {
  activeTextOverlays,
  clampStreamerBannerPosition,
  resolveStreamerBannerPosition,
  STREAMER_BANNER_MAX_POSITION,
  STREAMER_BANNER_MIN_POSITION,
  type FrameSize,
} from '@/lib/stream-preview';

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

/**
 * Renders one output band with the same geometry as FFmpeg: crop the source
 * rect, scale it proportionally until it covers the band, then center-crop the
 * excess. The video element itself always keeps the source aspect ratio.
 */
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

/**
 * Live 9:16 preview: facecam over gameplay for stack variants, or gameplay
 * only for the no-facecam variant. Band sizes and crop geometry mirror the
 * render variant registry in internal/streamclips.
 */
export function StreamPreview({
  variant,
  faceCrop,
  gameplayCrop,
  clips = EMPTY_CLIPS,
  frameSeconds,
  streamerNick,
  streamerPositionY,
  streamerSlideEnabled = false,
  onStreamerPositionChange,
  disabled = false,
}: {
  variant: StreamVariant;
  faceCrop?: NormalizedRect;
  gameplayCrop?: NormalizedRect;
  clips?: StreamClipRange[];
  frameSeconds: number;
  streamerNick?: string;
  streamerPositionY?: number;
  streamerSlideEnabled?: boolean;
  onStreamerPositionChange?: (position: number) => void;
  disabled?: boolean;
}): ReactNode {
  const containerRef = useRef<HTMLDivElement>(null);
  const dragRef = useRef<{ startClientY: number; startPosition: number } | null>(null);
  const gameplay = gameplayCrop ?? FULL_FRAME;
  const layout = PREVIEW_LAYOUTS[variant];
  const faceLayout = layout.face;
  const facePct = faceLayout
    ? (faceLayout.height * 100) / (faceLayout.height + layout.gameplay.height)
    : 0;
  const bannerPosition = resolveStreamerBannerPosition(variant, streamerPositionY);
  const activeOverlays = activeTextOverlays(clips, frameSeconds);

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

  return (
    <div
      ref={containerRef}
      className="relative mx-auto aspect-[9/16] w-full max-w-[220px] overflow-hidden rounded-xl border border-border bg-background shadow-lg"
      style={{ containerType: 'size' }}
    >
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
          <div className={`flex h-full w-full items-center bg-[#9146ff] text-white shadow-sm ${streamerSlideEnabled ? 'streamer-banner-slide-preview' : ''}`}>
            <span className="flex h-full w-[11%] shrink-0 items-center justify-center bg-[#5b1ba9]">
              <Twitch className="h-[62%] w-[62%]" strokeWidth={2.6} aria-hidden />
            </span>
            <span className="truncate px-[3%] font-[family-name:var(--font-display)] text-[clamp(7px,3.2vw,12px)] font-black leading-none tracking-[0.02em]">
              {streamerNick}
            </span>
          </div>
        </div>
      ) : null}
      <style>{`
        .streamer-banner-slide-preview {
          animation: streamer-banner-slide-preview 2.8s ease-in-out 1;
        }

        @keyframes streamer-banner-slide-preview {
          0%, 10% { transform: translateX(-105%); }
          24%, 76% { transform: translateX(0); }
          90%, 100% { transform: translateX(-105%); }
        }

        @media (prefers-reduced-motion: reduce) {
          .streamer-banner-slide-preview { animation: none; }
        }
      `}</style>
    </div>
  );
}
