'use client';
import type { ReactNode } from 'react';
import type { StreamClipRange } from '@/lib/api/streams';
import { clipTimelineGeometry, formatStreamClock } from '@/lib/streams/plan';
import { cn } from '@/lib/utils';
export function StreamSourceTimeline({
  clips,
  sourceDuration,
  selectedClipId,
  playheadSeconds,
  disabled,
  onSeek,
  onSelect,
}: {
  clips: StreamClipRange[];
  sourceDuration: number;
  selectedClipId: string | null;
  playheadSeconds: number;
  disabled: boolean;
  onSeek: (seconds: number) => void;
  onSelect: (clip: StreamClipRange) => void;
}): ReactNode {
  return (
    <section className="rounded-md border border-border-subtle bg-surface-1 p-3" aria-label="Timeline de la fuente">
      <label className="flex flex-col gap-2 text-label text-fg-2">
        Recorrer el vídeo original
        <input
          aria-label="Posición en el vídeo original"
          type="range"
          min={0}
          max={sourceDuration || 1}
          step={0.1}
          value={playheadSeconds}
          disabled={disabled || sourceDuration <= 0}
          onChange={(e) => onSeek(Number(e.target.value))}
          className="h-5 w-full cursor-pointer accent-stream"
        />
      </label>
      <div className="relative mt-2 h-8 rounded bg-surface-3" aria-label="Momentos seleccionados">
        {clips.map((clip, index) => {
          const geometry = clipTimelineGeometry(clip, sourceDuration);
          if (!geometry) return null;
          return (
            <button
              key={clip.id}
              type="button"
              disabled={disabled}
              aria-pressed={clip.id === selectedClipId}
              aria-label={`Seleccionar el corte ${index + 1}: ${formatStreamClock(clip.start_seconds)} a ${formatStreamClock(clip.end_seconds)}`}
              onClick={() => onSelect(clip)}
              style={{ left: `${geometry.startPercent}%`, width: `${geometry.widthPercent}%` }}
              className={cn(
                'absolute inset-y-0 min-w-1 overflow-hidden rounded border text-label focus-visible:z-10 focus-visible:outline-2 focus-visible:outline-ring',
                clip.id === selectedClipId ? 'border-stream bg-stream/30' : 'border-stream/40 bg-stream/10',
              )}
            >
              {index + 1}
            </button>
          );
        })}
      </div>
      <div className="mt-1 flex justify-between font-mono text-label text-fg-3">
        <span>0:00</span>
        <span>{formatStreamClock(sourceDuration)}</span>
      </div>
    </section>
  );
}
