'use client';
import type { ComponentProps, RefObject, ReactNode } from 'react';
import { Pause, Play } from 'lucide-react';
import type { NormalizedRect } from '@/lib/api/streams';
import { formatStreamClock } from '@/lib/streams/plan';
import { Button } from '@/components/ui/button';
import { CropPicker } from '@/components/streams/crop-picker';
import { StreamPreview } from '@/components/streams/stream-preview';
import { StreamFrameCanvas } from '@/components/streams/stream-frame-session';
export type StreamCropEditor = { rect: NormalizedRect; disabled: boolean; onChange: (rect: NormalizedRect) => void };
export type StreamPlaybackMode = 'source' | 'clip' | 'montage';
export function StreamMonitor({
  preview,
  cropEditor,
  elapsedSeconds,
  playbackDuration,
  playing,
  canPlay,
  previewError,
  videoSrc,
  audioRef,
  audioKey,
  onTogglePlay,
  onAudioError,
  onRetry,
  mode,
  onModeChange,
  hasSelection,
  clipCount,
}: {
  preview: ComponentProps<typeof StreamPreview>;
  cropEditor?: StreamCropEditor;
  elapsedSeconds: number;
  playbackDuration: number;
  playing: boolean;
  canPlay: boolean;
  previewError: string | null;
  videoSrc: string;
  audioRef: RefObject<HTMLAudioElement | null>;
  audioKey: number;
  onTogglePlay: () => void;
  onAudioError: () => void;
  onRetry: () => void;
  mode: StreamPlaybackMode;
  onModeChange: (mode: StreamPlaybackMode) => void;
  hasSelection: boolean;
  clipCount: number;
}): ReactNode {
  const playLabel = {
    source: 'Reproducir vídeo original',
    clip: 'Reproducir este Short',
    montage: 'Reproducir todos los Shorts',
  }[mode];
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <div className="grid min-h-0 flex-1 grid-cols-[minmax(0,1fr)_minmax(100px,0.6fr)] items-center gap-4">
        <div className="min-w-0">
          <p className="mb-2 text-label font-semibold text-fg-2">
            {cropEditor ? 'Selecciona la cámara en el original' : 'Vídeo original'}
          </p>
          {cropEditor ? (
            <CropPicker {...cropEditor} />
          ) : (
            <div className="relative aspect-video overflow-hidden rounded-md bg-black">
              <StreamFrameCanvas mode="contain" className="size-full" />
            </div>
          )}
          {cropEditor ? (
            <p className="mt-2 text-label text-fg-3">
              Arrastra el marco y su esquina. También puedes usar las flechas.
            </p>
          ) : null}
        </div>
        <div className="flex min-h-0 flex-col items-center">
          <p className="mb-2 text-label font-semibold text-fg-2">Vista del Short</p>
          <StreamPreview {...preview} className="h-[min(36vh,360px)] w-auto min-h-[150px]" />
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-2" role="group" aria-label="Qué reproducir">
        <Button
          variant={mode === 'source' ? 'secondary' : 'ghost'}
          size="sm"
          aria-pressed={mode === 'source'}
          onClick={() => onModeChange('source')}
        >
          Vídeo original
        </Button>
        <Button
          variant={mode === 'clip' ? 'secondary' : 'ghost'}
          size="sm"
          disabled={!hasSelection}
          aria-pressed={mode === 'clip'}
          onClick={() => onModeChange('clip')}
        >
          Short seleccionado
        </Button>
        {clipCount > 1 ? (
          <Button variant="ghost" size="sm" aria-pressed={mode === 'montage'} onClick={() => onModeChange('montage')}>
            Todos los Shorts
          </Button>
        ) : null}
      </div>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <Button variant="outline" disabled={!canPlay} onClick={onTogglePlay}>
          {playing ? <Pause aria-hidden /> : <Play aria-hidden />}
          {playing ? 'Pausar' : playLabel}
        </Button>
        <output aria-label="Tiempo de reproducción" className="font-mono text-label tabular-nums text-fg-2">
          {formatStreamClock(elapsedSeconds)} / {formatStreamClock(playbackDuration)}
        </output>
      </div>
      <audio key={audioKey} ref={audioRef} src={videoSrc} preload="metadata" onError={onAudioError} />
      {previewError ? (
        <div role="alert" className="text-body-sm text-destructive">
          <p>{previewError}</p>
          <Button variant="outline" size="sm" onClick={onRetry}>
            Reintentar
          </Button>
        </div>
      ) : null}
    </div>
  );
}
