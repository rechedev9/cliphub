'use client';

import type { RefObject, ReactNode } from 'react';
import { Pause, Play } from 'lucide-react';
import type { NormalizedRect, StreamClipRange, StreamVariant } from '@/lib/api/streams';
import { formatStreamTimestamp } from '@/lib/streams/plan';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { StreamPreview } from '@/components/streams/stream-preview';

/**
 * The 9:16 monitor: the vertical frame the render will produce and the montage
 * transport that plays the selected ranges in order.
 *
 * It sticks to the top of the viewport on wide layouts because every control in
 * the left column is judged against it — scrolling the editor away from its own
 * output was the reason the crop pickers felt blind.
 */
export function StreamPreviewColumn({
  variant,
  faceCrop,
  gameplayCrop,
  clips,
  frameSeconds,
  sourceDuration,
  streamerNick,
  streamerPositionY,
  streamerSlideEnabled,
  playing,
  canPlay,
  previewError,
  videoSrc,
  audioRef,
  audioKey,
  busy,
  onStreamerPositionChange,
  onTogglePlay,
  onAudioPause,
  onAudioError,
  onRetry,
}: {
  variant: StreamVariant;
  faceCrop?: NormalizedRect;
  gameplayCrop?: NormalizedRect;
  clips: StreamClipRange[];
  frameSeconds: number;
  sourceDuration: number;
  streamerNick?: string;
  streamerPositionY?: number;
  streamerSlideEnabled?: boolean;
  playing: boolean;
  canPlay: boolean;
  previewError: string | null;
  videoSrc: string;
  audioRef: RefObject<HTMLAudioElement | null>;
  audioKey: number;
  busy: boolean;
  onStreamerPositionChange: (position: number) => void;
  onTogglePlay: () => void;
  onAudioPause: () => void;
  onAudioError: () => void;
  onRetry: () => void;
}): ReactNode {
  return (
    <div className="flex flex-col gap-3 self-start @[64rem]/content:sticky @[64rem]/content:top-[calc(var(--shell-strip-height)+1rem)]">
      <div className="flex items-center justify-between gap-2">
        <span className="font-mono text-meta uppercase tracking-ultra text-fg-3">PREVIEW</span>
        <StatusTag tone={playing ? 'stream' : 'neutral'} dot={playing}>
          9:16
        </StatusTag>
      </div>

      <StreamPreview
        variant={variant}
        faceCrop={faceCrop}
        gameplayCrop={gameplayCrop}
        clips={clips}
        frameSeconds={frameSeconds}
        streamerNick={streamerNick}
        streamerPositionY={streamerPositionY}
        streamerSlideEnabled={streamerSlideEnabled}
        onStreamerPositionChange={onStreamerPositionChange}
        disabled={busy}
      />

      <audio
        key={audioKey}
        ref={audioRef}
        src={videoSrc}
        preload="metadata"
        onPause={onAudioPause}
        onError={onAudioError}
      />

      <div className="flex flex-wrap items-center gap-2">
        <Button type="button" variant="outline" size="sm" disabled={!canPlay} onClick={onTogglePlay}>
          {playing ? <Pause className="size-4" aria-hidden /> : <Play className="size-4" aria-hidden />}
          {playing ? 'PAUSAR PREVIEW' : 'REPRODUCIR MONTAJE'}
        </Button>
        <span className="font-mono text-meta tabular-nums text-fg-3">
          {formatStreamTimestamp(frameSeconds)} / {formatStreamTimestamp(sourceDuration)}
        </span>
      </div>

      {previewError ? (
        <div
          role="alert"
          className="flex flex-col items-start gap-2 border border-destructive/45 bg-destructive/10 p-3 text-body-sm text-destructive"
        >
          <span>{previewError}</span>
          <Button type="button" variant="outline" size="sm" onClick={onRetry}>
            REINTENTAR VISTA PREVIA
          </Button>
        </div>
      ) : null}

      <p className="text-body-sm text-fg-3">
        La preview replica el encuadre vertical del render: las bandas, el recorte de cada fuente y
        el banner en su posición exacta.
      </p>
    </div>
  );
}
