'use client';

import type { ComponentProps, RefObject, ReactNode } from 'react';
import { Pause, Play } from 'lucide-react';
import type { NormalizedRect } from '@/lib/api/streams';
import { formatStreamClock } from '@/lib/streams/plan';
import { Button } from '@/components/ui/button';
import { CropPicker } from '@/components/streams/crop-picker';
import { StreamPreview } from '@/components/streams/stream-preview';
import { cn } from '@/lib/utils';

export type StreamCropEditor = {
  rect: NormalizedRect;
  disabled: boolean;
  onChange: (rect: NormalizedRect) => void;
};

function Transport({
  frameSeconds,
  sourceDuration,
  playing,
  canPlay,
  onTogglePlay,
}: {
  frameSeconds: number;
  sourceDuration: number;
  playing: boolean;
  canPlay: boolean;
  onTogglePlay: () => void;
}): ReactNode {
  return (
    <>
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={!canPlay}
        onClick={onTogglePlay}
        className="justify-start font-mono uppercase tracking-wider"
      >
        {playing ? <Pause aria-hidden /> : <Play aria-hidden />}
        {playing ? 'Pausar' : 'Reproducir montaje'}
      </Button>
      <span className="inline-flex h-8 items-center border border-border-strong px-2 font-mono text-meta tabular-nums text-fg-2">
        {formatStreamClock(frameSeconds)} / {formatStreamClock(sourceDuration)}
      </span>
    </>
  );
}

function PreviewStatus({ previewError, onRetry }: { previewError: string | null; onRetry: () => void }): ReactNode {
  if (previewError) {
    return (
      <div role="alert" className="flex flex-col items-start gap-2 border border-destructive/45 bg-destructive/10 p-2 text-body-sm text-destructive">
        <span>{previewError}</span>
        <Button type="button" variant="outline" size="sm" onClick={onRetry}>
          Reintentar
        </Button>
      </div>
    );
  }
  return (
    <p className="font-mono text-meta uppercase tracking-wider text-fg-3">
      La preview replica bandas, recortes y banners del render.
    </p>
  );
}

/**
 * The 9:16 monitor plus its transport: play the montage, read the source
 * clock. With `cropEditor` the facecam crop is edited on the source frame
 * beside the 9:16 result, and the transport moves under that frame so the
 * timeline below never leaves the screen. The crop column is width-capped from
 * the viewport height because an aspect-ratio box cannot shrink to its row.
 */
export function StreamMonitor({
  preview,
  cropEditor,
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
  cropEditor?: StreamCropEditor;
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
  const transport = (
    <Transport
      frameSeconds={frameSeconds}
      sourceDuration={sourceDuration}
      playing={playing}
      canPlay={canPlay}
      onTogglePlay={onTogglePlay}
    />
  );

  return (
    <div className="flex min-h-0 flex-1 items-center justify-center gap-5">
      {cropEditor ? (
        <div className="flex min-h-0 max-h-full min-w-0 flex-1 max-w-[min(720px,calc((100vh-560px)*16/9))] flex-col gap-2">
          <span className="font-mono text-meta uppercase tracking-widest text-fg-3">Recorte de facecam · fuente 16:9</span>
          <CropPicker rect={cropEditor.rect} onChange={cropEditor.onChange} disabled={cropEditor.disabled} />
          <div className="flex flex-wrap items-center gap-2">
            {transport}
            <span className="font-mono text-meta uppercase tracking-wider text-fg-3">
              Arrastra el marco · esquina para redimensionar · flechas en teclado
            </span>
          </div>
          {previewError ? <PreviewStatus previewError={previewError} onRetry={onRetry} /> : null}
        </div>
      ) : null}

      <div className={cn('h-full max-h-full min-h-[120px] flex-none', cropEditor ? 'hidden @[64rem]/content:flex' : 'flex')}>
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

      {cropEditor ? null : (
        <div className="flex w-max min-w-[10rem] flex-col gap-2 self-end pb-1.5">
          {transport}
          <PreviewStatus previewError={previewError} onRetry={onRetry} />
        </div>
      )}
    </div>
  );
}
