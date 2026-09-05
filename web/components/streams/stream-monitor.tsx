'use client';

import type { ComponentProps, ReactNode } from 'react';
import { Pause, Play, Repeat } from 'lucide-react';
import type { NormalizedRect } from '@/lib/api/streams';
import { PLAYBACK_STATUS, type PlaybackStatus } from '@/lib/playback-session';
import { STREAM_PLAYBACK_MODE, type StreamPlaybackMode } from '@/lib/stream-playback';
import { formatStreamClock } from '@/lib/streams/plan';
import { Button } from '@/components/ui/button';
import { CropPicker } from '@/components/streams/crop-picker';
import { StreamPreview } from '@/components/streams/stream-preview';
import { StreamFrameCanvas } from '@/components/streams/stream-frame-session';

export type StreamCropEditor = { rect: NormalizedRect; disabled: boolean; onChange: (rect: NormalizedRect) => void };

/** Guided source/output monitor backed by the editor's one shared decoder. */
export function StreamMonitor({
  preview,
  cropEditor,
  frameSeconds,
  sourceDuration,
  elapsedSeconds,
  playbackDuration,
  playing,
  status,
  canPlay,
  previewError,
  mode,
  loop,
  onModeChange,
  onLoopChange,
  onSeek,
  onTogglePlay,
  onRetry,
  hasSelection,
  clipCount,
}: {
  preview: ComponentProps<typeof StreamPreview>;
  cropEditor?: StreamCropEditor;
  frameSeconds: number;
  sourceDuration: number;
  elapsedSeconds: number;
  playbackDuration: number;
  playing: boolean;
  status: PlaybackStatus;
  canPlay: boolean;
  previewError: string | null;
  mode: StreamPlaybackMode;
  loop: boolean;
  onModeChange: (mode: StreamPlaybackMode) => void;
  onLoopChange: (loop: boolean) => void;
  onSeek: (seconds: number) => void;
  onTogglePlay: () => void;
  onRetry: () => void;
  hasSelection: boolean;
  clipCount: number;
}): ReactNode {
  const playLabel = {
    [STREAM_PLAYBACK_MODE.source]: 'Reproducir vídeo original',
    [STREAM_PLAYBACK_MODE.selected]: 'Reproducir este Short',
    [STREAM_PLAYBACK_MODE.sequence]: 'Reproducir todos los Shorts',
  }[mode];
  const statusLabels: Partial<Record<PlaybackStatus, string>> = {
    [PLAYBACK_STATUS.loading]: 'Cargando vídeo…',
    [PLAYBACK_STATUS.buffering]: 'Esperando vídeo…',
    [PLAYBACK_STATUS.seeking]: 'Buscando posición…',
  };
  const statusLabel = statusLabels[status];

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <div className="grid h-[min(24vh,240px)] min-h-[150px] shrink-0 grid-cols-[minmax(0,1fr)_minmax(100px,0.6fr)] items-center gap-4 overflow-hidden">
        <div className="flex min-h-0 min-w-0 flex-col self-stretch">
          <p className="mb-2 text-label font-semibold text-fg-2">
            {cropEditor ? 'Selecciona la cámara en el original' : 'Vídeo original'}
          </p>
          {cropEditor ? (
            <CropPicker {...cropEditor} />
          ) : (
            <div className="relative min-h-0 flex-1 overflow-hidden rounded-md bg-black">
              <StreamFrameCanvas mode="contain" className="size-full" />
            </div>
          )}
          {cropEditor ? (
            <p className="mt-2 text-label text-fg-3">Arrastra el marco y su esquina. También puedes usar las flechas.</p>
          ) : null}
        </div>
        <div className="flex min-h-0 self-stretch flex-col items-center">
          <p className="mb-2 text-label font-semibold text-fg-2">Vista del Short</p>
          <StreamPreview {...preview} className="min-h-0 flex-1 w-auto" />
        </div>
      </div>
      <div className="flex shrink-0 flex-wrap items-center gap-2" role="group" aria-label="Qué reproducir">
        <Button variant={mode === STREAM_PLAYBACK_MODE.source ? 'secondary' : 'ghost'} size="sm" aria-pressed={mode === STREAM_PLAYBACK_MODE.source} onClick={() => onModeChange(STREAM_PLAYBACK_MODE.source)}>
          Vídeo original
        </Button>
        <Button variant={mode === STREAM_PLAYBACK_MODE.selected ? 'secondary' : 'ghost'} size="sm" disabled={!hasSelection} aria-pressed={mode === STREAM_PLAYBACK_MODE.selected} onClick={() => onModeChange(STREAM_PLAYBACK_MODE.selected)}>
          Short seleccionado
        </Button>
        {clipCount > 1 ? (
          <Button variant={mode === STREAM_PLAYBACK_MODE.sequence ? 'secondary' : 'ghost'} size="sm" aria-pressed={mode === STREAM_PLAYBACK_MODE.sequence} onClick={() => onModeChange(STREAM_PLAYBACK_MODE.sequence)}>
            Todos los Shorts
          </Button>
        ) : null}
      </div>
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <Button variant="outline" disabled={!canPlay} onClick={onTogglePlay}>
            {playing ? <Pause aria-hidden /> : <Play aria-hidden />}
            {playing ? 'Pausar' : playLabel}
          </Button>
          <Button variant={loop ? 'secondary' : 'ghost'} size="icon" aria-label="Repetir reproducción" aria-pressed={loop} onClick={() => onLoopChange(!loop)}>
            <Repeat aria-hidden />
          </Button>
        </div>
        <output aria-label="Tiempo de reproducción" className="font-mono text-label tabular-nums text-fg-2">
          {formatStreamClock(elapsedSeconds)} / {formatStreamClock(playbackDuration)}
        </output>
      </div>
      <input type="range" aria-label="Posición del vídeo original" min={0} max={sourceDuration || 1} step={0.01} value={Math.min(sourceDuration, Math.max(0, frameSeconds))} disabled={sourceDuration <= 0} onChange={(event) => onSeek(Number(event.target.value))} className="w-full shrink-0 accent-stream" />
      {statusLabel ? <p role="status" className="text-label text-fg-3">{statusLabel}</p> : null}
      {previewError ? (
        <div role="alert" className="text-body-sm text-destructive">
          <p>{previewError}</p>
          <Button variant="outline" size="sm" onClick={onRetry}>Reintentar</Button>
        </div>
      ) : null}
    </div>
  );
}
