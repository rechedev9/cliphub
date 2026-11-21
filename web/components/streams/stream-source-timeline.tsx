'use client';
import { useRef, useState, type KeyboardEvent, type PointerEvent, type ReactNode } from 'react';
import { Crosshair, Mic } from 'lucide-react';
import type { StreamClipRange, StreamPlay } from '@/lib/api/streams';
import type { MomentEdge } from '@/lib/streams/moment-edges';
import { formatStreamClock } from '@/lib/streams/plan';
import { Button } from '@/components/ui/button';
import {
  fullTimelineWindow,
  momentRangeLabel,
  playMarkLabel,
  secondsAtFraction,
  timelineKeySeek,
  timelineMomentRanges,
  timelinePlayhead,
  timelinePlayMarks,
  type PlayMarkState,
  type TimelineWindow,
} from '@/lib/streams/timeline';
import { cn } from '@/lib/utils';

export type TimelineZoom = {
  clipId: string;
  window: TimelineWindow;
  onEdgeDrag: (change: { edge: MomentEdge; seconds: number }) => void;
  onShowAll: () => void;
};

const EDGE_GRIPS: readonly { edge: MomentEdge; label: string }[] = [
  { edge: 'start', label: 'Arrastra para mover el inicio' },
  { edge: 'end', label: 'Arrastra para mover el final' },
];

const MARK_TONE: Readonly<Record<PlayMarkState, string>> = {
  open: 'border-fg-4 bg-surface-0 text-fg-2 hover:border-fg-1 hover:text-fg-1',
  covered: 'border-success bg-surface-0 text-success hover:text-fg-1',
  selected: 'z-10 border-stream bg-stream text-stream-foreground',
};

export function StreamSourceTimeline({
  clips,
  plays,
  activePlayId,
  label,
  sourceDuration,
  selectedClipIds,
  playheadSeconds,
  disabled,
  zoom,
  onSeek,
  onSelect,
  onWatchPlay,
}: {
  clips: StreamClipRange[];
  plays: readonly StreamPlay[];
  activePlayId: string | null;
  label: string;
  sourceDuration: number;
  selectedClipIds: readonly string[];
  playheadSeconds: number;
  disabled: boolean;
  zoom?: TimelineZoom;
  onSeek: (seconds: number) => void;
  onSelect: (clip: StreamClipRange) => void;
  onWatchPlay: (play: StreamPlay) => void;
}): ReactNode {
  const laneRef = useRef<HTMLDivElement>(null);
  const [drag, setDrag] = useState<{ edge: MomentEdge; view: TimelineWindow } | null>(null);
  const view = drag?.view ?? zoom?.window ?? fullTimelineWindow(sourceDuration);
  const marks = timelinePlayMarks({ plays, clips, activePlayId, window: view });
  const ranges = timelineMomentRanges({ clips, selectedClipIds, window: view });
  const seekable = !disabled && sourceDuration > 0;
  const playhead = timelinePlayhead({ seconds: playheadSeconds, window: view });

  const seekAtPointer = (event: PointerEvent<HTMLDivElement>) => {
    const box = event.currentTarget.getBoundingClientRect();
    if (box.width > 0) onSeek(secondsAtFraction((event.clientX - box.left) / box.width, view));
  };
  const editable = zoom && !disabled ? ranges.find((range) => range.clip.id === zoom.clipId) : undefined;
  const dragTo = (event: PointerEvent<HTMLSpanElement>, held: { edge: MomentEdge; view: TimelineWindow }) => {
    const box = laneRef.current?.getBoundingClientRect();
    if (!zoom || !box || box.width <= 0) return;
    zoom.onEdgeDrag({ edge: held.edge, seconds: secondsAtFraction((event.clientX - box.left) / box.width, held.view) });
  };
  const onLaneKey = (event: KeyboardEvent<HTMLDivElement>) => {
    const target = timelineKeySeek({ key: event.key, seconds: playheadSeconds, window: view });
    if (target === null || !seekable) return;
    event.preventDefault();
    event.stopPropagation();
    onSeek(target);
  };

  return (
    <section className="flex flex-col gap-2 rounded-md border border-border-subtle bg-surface-1 p-3" aria-label="Línea de tiempo del vídeo original">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
        <span className="text-label text-fg-2">{label}</span>
        {zoom ? (
          <Button variant="outline" size="sm" onClick={zoom.onShowAll}>
            Ver todo el vídeo
          </Button>
        ) : null}
        <ul aria-label="Leyenda" className="ml-auto flex flex-wrap items-center gap-x-4 gap-y-1 text-meta text-fg-3">
          {plays.length ? (
            <li className="flex items-center gap-1.5">
              <span aria-hidden className="size-2.5 rounded-full border-[1.5px] border-fg-3" />
              Jugada encontrada
            </li>
          ) : null}
          <li className="flex items-center gap-1.5">
            <span aria-hidden className="h-2.5 w-4 rounded-sm bg-stream" />
            Tu momento
          </li>
          {zoom ? <li>Vista ampliada: {formatStreamClock(view.start)} a {formatStreamClock(view.end)}</li> : null}
        </ul>
      </div>

      {plays.length ? (
        <ul aria-label="Jugadas en la línea de tiempo" className="relative h-7">
          {marks.map((mark) => {
            const Icon = mark.voice ? Mic : Crosshair;
            return (
              <li key={mark.play.id} className="absolute top-0.5 -translate-x-1/2 hover:z-20 focus-within:z-20" style={{ left: `${mark.leftPercent}%` }}>
                <button
                  type="button"
                  disabled={disabled}
                  aria-pressed={mark.state === 'selected'}
                  aria-label={playMarkLabel(mark)}
                  title={playMarkLabel(mark)}
                  onClick={() => onWatchPlay(mark.play)}
                  className={cn(
                    'flex h-6 items-center gap-0.5 rounded-full border-[1.5px] px-1.5 text-meta font-semibold tabular-nums focus-visible:outline-2 focus-visible:outline-ring',
                    MARK_TONE[mark.state],
                  )}
                >
                  <Icon aria-hidden className="size-3" />
                  {mark.number}
                </button>
              </li>
            );
          })}
        </ul>
      ) : null}

      <div ref={laneRef} className="relative h-11 rounded border border-border-subtle bg-surface-0">
        <div
          role="slider"
          tabIndex={seekable ? 0 : -1}
          aria-label="Posición en el vídeo original"
          aria-disabled={!seekable}
          aria-valuemin={playhead.valueMin}
          aria-valuemax={playhead.valueMax}
          aria-valuenow={playhead.valueNow}
          aria-valuetext={formatStreamClock(playheadSeconds)}
          onKeyDown={onLaneKey}
          onPointerDown={(event) => {
            if (!seekable || event.button !== 0) return;
            event.currentTarget.setPointerCapture(event.pointerId);
            seekAtPointer(event);
          }}
          onPointerMove={(event) => {
            if (seekable && event.currentTarget.hasPointerCapture(event.pointerId)) seekAtPointer(event);
          }}
          className={cn('absolute inset-0 touch-none focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring', seekable && 'cursor-pointer')}
        />
        {ranges.map((range) => (
          <button
            key={range.clip.id}
            type="button"
            disabled={disabled}
            aria-pressed={range.selected}
            aria-label={momentRangeLabel(range)}
            title={momentRangeLabel(range)}
            onClick={() => onSelect(range.clip)}
            style={{ left: `${range.leftPercent}%`, width: `${range.widthPercent}%` }}
            className={cn(
              'absolute inset-y-1 grid min-w-2 place-items-center overflow-hidden rounded-sm border border-stream text-meta font-semibold tabular-nums text-stream-foreground focus-visible:z-10 focus-visible:outline-2 focus-visible:outline-ring',
              range.selected ? 'z-[1] bg-stream' : 'bg-stream/55 hover:bg-stream/75',
            )}
          >
            {range.number}
          </button>
        ))}
        {editable
          ? EDGE_GRIPS.map(({ edge, label }) => (
              <span
                key={edge}
                aria-hidden
                title={label}
                onPointerDown={(event) => {
                  if (event.button !== 0) return;
                  event.preventDefault();
                  event.currentTarget.setPointerCapture(event.pointerId);
                  setDrag({ edge, view });
                }}
                onPointerMove={(event) => {
                  if (drag && event.currentTarget.hasPointerCapture(event.pointerId)) dragTo(event, drag);
                }}
                onPointerUp={() => setDrag(null)}
                onPointerCancel={() => setDrag(null)}
                style={{ left: `${edge === 'start' ? editable.leftPercent : editable.leftPercent + editable.widthPercent}%` }}
                className="absolute -inset-y-1.5 z-[3] grid w-4 -translate-x-1/2 cursor-ew-resize touch-none place-items-center rounded-sm bg-fg-1 after:h-3.5 after:w-0.5 after:rounded-full after:bg-stream"
              />
            ))
          : null}
        {sourceDuration > 0 && playhead.leftPercent !== null ? (
          <span
            aria-hidden
            className="pointer-events-none absolute inset-y-0 z-[2] w-0.5 -translate-x-px bg-fg-1"
            style={{ left: `${playhead.leftPercent}%` }}
          />
        ) : null}
      </div>
      <div className="flex justify-between text-label tabular-nums text-fg-3">
        <span>{formatStreamClock(view.start)}</span>
        <span>{formatStreamClock(view.end)}</span>
      </div>
    </section>
  );
}
