'use client';

import type { KeyboardEvent, MouseEvent, ReactNode } from 'react';
import { Scissors } from 'lucide-react';
import type { StreamClipRange } from '@/lib/api/streams';
import { clipTimelineGeometry, formatStreamClock } from '@/lib/streams/plan';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/**
 * The whole source as one strip: a band per cut (fades as edge gradients), the
 * playhead, and a ruler. Clicking a gap asks for a new cut at that second; the
 * caller decides whether it fits. The same request is reachable from the
 * keyboard through the header button and Enter/Space on the track, which both
 * cut at the playhead. Clicking a band only selects it; cuts are removed from
 * their card. The scale is the full probed duration.
 */
const ADD_CUT_HINT = 'clic en hueco: nuevo corte · clic en corte: seleccionar';
const NO_DURATION_HINT = 'Sin duración de la fuente';

export function StreamSourceTimeline({
  clips,
  sourceDuration,
  selectedClipId,
  playheadSeconds,
  disabled,
  onAddAt,
  onSelect,
}: {
  clips: StreamClipRange[];
  sourceDuration: number;
  selectedClipId: string | null;
  playheadSeconds: number;
  disabled: boolean;
  onAddAt: (seconds: number) => void;
  onSelect: (clip: StreamClipRange) => void;
}): ReactNode {
  const hasScale = Number.isFinite(sourceDuration) && sourceDuration > 0;
  const playheadPercent = hasScale ? Math.min(100, Math.max(0, (playheadSeconds / sourceDuration) * 100)) : 0;

  const canAdd = !disabled && hasScale;

  const handleTrackClick = (event: MouseEvent<HTMLDivElement>): void => {
    if (!canAdd) return;
    const box = event.currentTarget.getBoundingClientRect();
    if (box.width <= 0) return;
    onAddAt(((event.clientX - box.left) / box.width) * sourceDuration);
  };

  const handleTrackKeyDown = (event: KeyboardEvent<HTMLDivElement>): void => {
    if (!canAdd) return;
    if (event.key !== 'Enter' && event.key !== ' ') return;
    event.preventDefault();
    onAddAt(playheadSeconds);
  };

  return (
    <section className="studio-panel flex flex-col gap-2 p-3.5" aria-label="Timeline de la fuente">
      <div className="flex min-w-0 items-center justify-between gap-3 font-mono text-meta uppercase tracking-widest text-fg-3">
        <span className="flex min-w-0 flex-col gap-0.5">
          <span className="shrink-0">Timeline de la fuente · {formatStreamClock(sourceDuration)}</span>
          {/* An instruction, not a state: the cuts on the track below are what
              the accent is spent on. */}
          <span className="normal-case tracking-wider">{hasScale ? ADD_CUT_HINT : NO_DURATION_HINT}</span>
        </span>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={!canAdd}
          onClick={() => onAddAt(playheadSeconds)}
          className="shrink-0 font-mono uppercase tracking-wider"
        >
          <Scissors aria-hidden />
          Añadir corte aquí
        </Button>
      </div>

      <div
        role="button"
        tabIndex={0}
        aria-disabled={!canAdd}
        aria-label={`Añadir un corte en ${formatStreamClock(playheadSeconds)}`}
        onClick={handleTrackClick}
        onKeyDown={handleTrackKeyDown}
        className={cn(
          'studio-rim relative h-[52px] border border-border-subtle bg-surface-0 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring',
          canAdd ? 'cursor-crosshair' : 'cursor-default',
        )}
      >
        {clips.map((clip, index) => {
          const geometry = clipTimelineGeometry(clip, sourceDuration);
          if (geometry === null) return null;
          const selected = clip.id === selectedClipId;
          return (
            <button
              key={clip.id}
              type="button"
              disabled={disabled}
              aria-label={`Seleccionar el corte ${index + 1}: ${formatStreamClock(clip.start_seconds)} a ${formatStreamClock(clip.end_seconds)}`}
              onClick={(event) => {
                event.stopPropagation();
                onSelect(clip);
              }}
              className={cn(
                'studio-enter absolute inset-y-1 flex items-center justify-center overflow-hidden border font-mono text-meta whitespace-nowrap transition-colors duration-(--dur-fast) ease-standard focus-visible:z-10 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring',
                selected ? 'border-stream bg-stream/30 text-fg-1' : 'border-stream/45 bg-stream/15 text-fg-2 hover:bg-stream/25',
              )}
              style={{ left: `${geometry.startPercent}%`, width: `${geometry.widthPercent}%` }}
            >
              {geometry.fadeInPercent > 0 ? (
                <span
                  aria-hidden
                  className="absolute inset-y-0 left-0 bg-gradient-to-r from-surface-0/80 to-transparent"
                  style={{ width: `${geometry.fadeInPercent}%` }}
                />
              ) : null}
              <span className="relative truncate px-1">
                {String(index + 1).padStart(2, '0')} · {formatStreamClock(clip.start_seconds)}–{formatStreamClock(clip.end_seconds)}
              </span>
              {geometry.fadeOutPercent > 0 ? (
                <span
                  aria-hidden
                  className="absolute inset-y-0 right-0 bg-gradient-to-l from-surface-0/80 to-transparent"
                  style={{ width: `${geometry.fadeOutPercent}%` }}
                />
              ) : null}
            </button>
          );
        })}
        {hasScale ? (
          <span
            aria-hidden
            className="pointer-events-none absolute inset-y-0 w-0.5 bg-fg-1 transition-[left] duration-(--dur-base) linear"
            style={{ left: `${playheadPercent}%` }}
          />
        ) : null}
      </div>

      <div aria-hidden className="flex justify-between font-mono text-meta tabular-nums text-fg-3">
        <span>0:00</span>
        <span>{formatStreamClock(sourceDuration * 0.25)}</span>
        <span>{formatStreamClock(sourceDuration * 0.5)}</span>
        <span>{formatStreamClock(sourceDuration * 0.75)}</span>
        <span>{formatStreamClock(sourceDuration)}</span>
      </div>
    </section>
  );
}
