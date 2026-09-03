'use client';

import type { ComponentProps, RefObject, ReactNode } from 'react';
import { Pause, Play } from 'lucide-react';
import { formatStreamClock } from '@/lib/streams/plan';
import { Button } from '@/components/ui/button';
import { StreamPreview } from '@/components/streams/stream-preview';

/** The 9:16 monitor plus its transport: play the montage, read the source clock. */
export function StreamMonitor({
  preview,
  frameSeconds,
  sourceDuration,
  playing,
  canPlay,
  previewError,
  videoSrc,
  audioRef,
  audioKey,
  onTogglePlay,
  onAudioPause,
  onAudioError,
  onRetry,
}: {
  preview: ComponentProps<typeof StreamPreview>;
  frameSeconds: number;
  sourceDuration: number;
  playing: boolean;
  canPlay: boolean;
  previewError: string | null;
  videoSrc: string;
  audioRef: RefObject<HTMLAudioElement | null>;
  audioKey: number;
  onTogglePlay: () => void;
  onAudioPause: () => void;
  onAudioError: () => void;
  onRetry: () => void;
}): ReactNode {
  return (
    <div className="flex min-h-0 flex-1 items-center justify-center gap-5">
      <div className="flex h-full max-h-[400px] min-h-[120px] flex-none">
        <StreamPreview {...preview} />
      </div>

      <audio
        key={audioKey}
        ref={audioRef}
        src={videoSrc}
        preload="metadata"
        onPause={onAudioPause}
        onError={onAudioError}
      />

      <div className="flex max-w-[8rem] flex-col gap-2 self-end pb-1.5">
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={!canPlay}
          onClick={onTogglePlay}
          className="justify-start border-stream/45 font-mono uppercase tracking-wider text-stream-text hover:border-stream hover:bg-stream/10"
        >
          {playing ? <Pause aria-hidden /> : <Play aria-hidden />}
          {playing ? 'Pausar' : 'Reproducir montaje'}
        </Button>
        <span className="inline-flex h-8 items-center border border-border-strong px-2 font-mono text-meta tabular-nums text-fg-2">
          {formatStreamClock(frameSeconds)} / {formatStreamClock(sourceDuration)}
        </span>
        {previewError ? (
          <div role="alert" className="flex flex-col items-start gap-2 border border-destructive/45 bg-destructive/10 p-2 text-body-sm text-destructive">
            <span>{previewError}</span>
            <Button type="button" variant="outline" size="sm" onClick={onRetry}>
              Reintentar
            </Button>
          </div>
        ) : (
          <p className="font-mono text-meta uppercase tracking-wider text-fg-3">
            La preview replica bandas, recortes y banners del render.
          </p>
        )}
      </div>
    </div>
  );
}
