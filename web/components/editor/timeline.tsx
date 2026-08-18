'use client';

import { useEffect, useRef, useState, type PointerEvent as ReactPointerEvent, type ReactElement } from 'react';
import { Minus, Plus, Trash2 } from 'lucide-react';
import { StatusTag } from '@/components/studio/status-tag';
import { Button, FOCUS_RING } from '@/components/ui/button';
import {
  addTrack,
  deleteItem,
  deleteTrack,
  moveItem,
  splitItemAt,
  trimItem,
} from '@/lib/editor/document';
import {
  documentDuration,
  EDITOR_TRACK_KINDS,
  itemOutputDuration,
  itemSpeed,
  itemTimelineEnd,
  type EditorDocument,
  type EditorItem,
  type EditorTrack,
  type EditorTrackKind,
} from '@/lib/editor/evaluate';
import { EDITOR_LIMITS } from '@/lib/editor/validate';
import { cn } from '@/lib/utils';

const DEFAULT_PX_PER_SECOND = 80;
const MIN_PX_PER_SECOND = 24;
const MAX_PX_PER_SECOND = 240;
const ZOOM_STEP = 1.25;
const SNAP_SECONDS = 0.05;
const MIN_OUTPUT_SECONDS = 0.05;
const MIN_TIMELINE_SECONDS = 8;
const TRACK_LABEL_WIDTH = 136;
const HANDLE_WIDTH = 8;

export type EditorTimelineProps = {
  doc: EditorDocument;
  time: number;
  selectedId: string | null;
  locked: boolean;
  onSeek: (time: number) => void;
  onSelect: (id: string | null) => void;
  onChange: (next: EditorDocument) => void;
};

type MoveDrag = {
  kind: 'move';
  itemId: string;
  originX: number;
  originStart: number;
  doc: EditorDocument;
};

type TrimDrag = {
  kind: 'trim-in' | 'trim-out';
  itemId: string;
  originX: number;
  sourceIn: number;
  sourceOut: number;
  timelineStart: number;
  speed: number;
  doc: EditorDocument;
};

type SeekDrag = {
  kind: 'seek';
};

type DragSession = MoveDrag | TrimDrag | SeekDrag;

export function EditorTimeline({
  doc,
  time,
  selectedId,
  locked,
  onSeek,
  onSelect,
  onChange,
}: EditorTimelineProps): ReactElement {
  const [pxPerSecond, setPxPerSecond] = useState(DEFAULT_PX_PER_SECOND);
  const scrollerRef = useRef<HTMLDivElement>(null);
  const dragRef = useRef<DragSession | null>(null);
  const propsRef = useRef({ doc, time, selectedId, locked, onSeek, onSelect, onChange, pxPerSecond });
  propsRef.current = { doc, time, selectedId, locked, onSeek, onSelect, onChange, pxPerSecond };

  const duration = Math.max(documentDuration(doc), time + 2, MIN_TIMELINE_SECONDS);
  const timelineWidth = Math.max(duration * pxPerSecond, 1);
  const atTrackLimit = doc.tracks.length >= EDITOR_LIMITS.maxTracks;
  const videoTrackCount = doc.tracks.filter((track) => track.kind === EDITOR_TRACK_KINDS.video).length;

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent): void => {
      const current = propsRef.current;
      if (current.locked || current.selectedId === null) return;
      if (isTextEntry(event.target)) return;
      if (event.key === 'Delete' || event.key === 'Backspace') {
        event.preventDefault();
        current.onChange(deleteItem(current.doc, current.selectedId));
        current.onSelect(null);
        return;
      }
      if (event.code !== 'KeyS' || event.ctrlKey || event.metaKey || event.altKey) return;
      event.preventDefault();
      current.onChange(splitItemAt(current.doc, current.selectedId, current.time));
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);

  useEffect(() => {
    const onMove = (event: PointerEvent): void => {
      const drag = dragRef.current;
      if (drag === null) return;
      const current = propsRef.current;
      if (drag.kind === 'seek') {
        current.onSeek(timeFromPointer(event.clientX, scrollerRef.current, current.pxPerSecond));
        return;
      }
      if (current.locked) return;
      if (drag.kind === 'move') {
        const dt = (event.clientX - drag.originX) / current.pxPerSecond;
        const nextStart = snapStart(Math.max(0, drag.originStart + dt), drag.doc, drag.itemId);
        current.onChange(moveItem(drag.doc, drag.itemId, nextStart));
        return;
      }
      applyTrim(drag, event.clientX, current.pxPerSecond, current.onChange);
    };
    const onUp = (): void => {
      dragRef.current = null;
    };
    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp);
    window.addEventListener('pointercancel', onUp);
    return () => {
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerup', onUp);
      window.removeEventListener('pointercancel', onUp);
    };
  }, []);

  function beginSeek(event: ReactPointerEvent<HTMLElement>): void {
    event.preventDefault();
    dragRef.current = { kind: 'seek' };
    onSeek(timeFromPointer(event.clientX, scrollerRef.current, pxPerSecond));
  }

  function beginMove(event: ReactPointerEvent<HTMLElement>, item: EditorItem): void {
    event.preventDefault();
    event.stopPropagation();
    onSelect(item.id);
    if (locked) return;
    dragRef.current = {
      kind: 'move',
      itemId: item.id,
      originX: event.clientX,
      originStart: item.timeline_start,
      doc,
    };
  }

  function beginTrim(event: ReactPointerEvent<HTMLElement>, item: EditorItem, kind: TrimDrag['kind']): void {
    event.preventDefault();
    event.stopPropagation();
    onSelect(item.id);
    if (locked) return;
    dragRef.current = {
      kind,
      itemId: item.id,
      originX: event.clientX,
      sourceIn: item.source_in,
      sourceOut: item.source_out,
      timelineStart: item.timeline_start,
      speed: itemSpeed(item),
      doc,
    };
  }

  function zoomBy(factor: number): void {
    setPxPerSecond((prev) => clamp(prev * factor, MIN_PX_PER_SECOND, MAX_PX_PER_SECOND));
  }

  function handleAddTrack(kind: EditorTrackKind): void {
    if (locked || atTrackLimit) return;
    onChange(addTrack(doc, kind));
  }

  function handleDeleteTrack(track: EditorTrack): void {
    if (locked || !canDeleteTrack(track, videoTrackCount, doc.tracks.length)) return;
    onChange(deleteTrack(doc, track.id));
    if (selectedId !== null && track.items.some((item) => item.id === selectedId)) {
      onSelect(null);
    }
  }

  const ticks = rulerTicks(duration, pxPerSecond);
  const playheadX = time * pxPerSecond;

  return (
    <section className="studio-panel flex min-h-0 min-w-0 flex-col overflow-hidden p-4">
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={locked || atTrackLimit}
          onClick={() => handleAddTrack(EDITOR_TRACK_KINDS.video)}
        >
          + pista de vídeo
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={locked || atTrackLimit}
          onClick={() => handleAddTrack(EDITOR_TRACK_KINDS.audio)}
        >
          + pista de audio
        </Button>
        <div className="ml-auto flex items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="icon-sm"
            aria-label="Alejar"
            disabled={pxPerSecond <= MIN_PX_PER_SECOND}
            onClick={() => zoomBy(1 / ZOOM_STEP)}
          >
            <Minus />
          </Button>
          <Button
            type="button"
            variant="outline"
            size="icon-sm"
            aria-label="Acercar"
            disabled={pxPerSecond >= MAX_PX_PER_SECOND}
            onClick={() => zoomBy(ZOOM_STEP)}
          >
            <Plus />
          </Button>
        </div>
      </div>

      <div className="flex min-h-0 min-w-0 flex-1">
        <div className="flex shrink-0 flex-col border-r border-border" style={{ width: TRACK_LABEL_WIDTH }}>
          <div className="flex h-10 items-end px-2 pb-1">
            <span className="font-mono text-meta uppercase tracking-wider text-fg-3">Pistas</span>
          </div>
          {doc.tracks.map((track) => (
            <div key={track.id} className="flex h-12 items-center gap-2 border-t border-border px-2">
              <div className="min-w-0 flex-1">
                <p className="truncate font-mono text-meta text-fg-1">{track.id}</p>
                <StatusTag size="sm" tone={track.kind === EDITOR_TRACK_KINDS.video ? 'primary' : 'neutral'}>
                  {track.kind}
                </StatusTag>
              </div>
              <Button
                type="button"
                variant="destructive"
                size="icon-sm"
                aria-label={`Eliminar pista ${track.id}`}
                disabled={locked || !canDeleteTrack(track, videoTrackCount, doc.tracks.length)}
                onClick={() => handleDeleteTrack(track)}
              >
                <Trash2 />
              </Button>
            </div>
          ))}
        </div>

        <div ref={scrollerRef} className="min-w-0 flex-1 overflow-x-auto overflow-y-hidden">
          <div className="relative select-none" style={{ width: timelineWidth }}>
            <div
              role="slider"
              aria-label="Regla de tiempo"
              aria-valuemin={0}
              aria-valuenow={Number(time.toFixed(2))}
              aria-valuetext={formatTimecode(time)}
              tabIndex={0}
              className={cn('relative h-10 cursor-ew-resize border-b border-border bg-surface-0', FOCUS_RING)}
              onPointerDown={beginSeek}
              onKeyDown={(event) => {
                if (event.key === 'ArrowLeft') {
                  event.preventDefault();
                  onSeek(Math.max(0, time - 0.05));
                  return;
                }
                if (event.key === 'ArrowRight') {
                  event.preventDefault();
                  onSeek(time + 0.05);
                }
              }}
            >
              {ticks.map((tick) => (
                <span
                  key={tick}
                  className="absolute top-2 font-mono text-meta tabular-nums text-fg-3"
                  style={{ left: tick * pxPerSecond }}
                >
                  {formatTick(tick)}
                </span>
              ))}
            </div>

            {doc.tracks.map((track) => (
              <div
                key={track.id}
                className="relative h-12 border-t border-border bg-surface-0"
                onPointerDown={(event) => {
                  beginSeek(event);
                  onSelect(null);
                }}
              >
                {track.items.map((item) => {
                  const selected = item.id === selectedId;
                  const left = item.timeline_start * pxPerSecond;
                  const width = Math.max(itemOutputDuration(item) * pxPerSecond, HANDLE_WIDTH * 2);
                  return (
                    <div
                      key={item.id}
                      className="absolute top-1 h-10"
                      style={{ left, width }}
                    >
                      <button
                        type="button"
                        aria-pressed={selected}
                        disabled={locked}
                        className={cn(
                          FOCUS_RING,
                          'h-full w-full truncate border px-2 text-left font-mono text-meta',
                          selected
                            ? 'border-accent bg-surface-3 text-primary shadow-[var(--elev-1),var(--glow-primary-sm)]'
                            : 'border-border-strong bg-surface-3 text-fg-1',
                          locked ? 'pointer-events-none' : 'cursor-grab active:cursor-grabbing',
                        )}
                        onPointerDown={(event) => beginMove(event, item)}
                      >
                        {item.id}
                      </button>
                      {locked ? null : (
                        <>
                          <button
                            type="button"
                            aria-label={`Recortar inicio de ${item.id}`}
                            className={cn(
                              FOCUS_RING,
                              'absolute top-0 left-0 h-full cursor-ew-resize border-0',
                              selected ? 'bg-primary' : 'bg-fg-3',
                            )}
                            style={{ width: HANDLE_WIDTH }}
                            onPointerDown={(event) => beginTrim(event, item, 'trim-in')}
                          />
                          <button
                            type="button"
                            aria-label={`Recortar final de ${item.id}`}
                            className={cn(
                              FOCUS_RING,
                              'absolute top-0 right-0 h-full cursor-ew-resize border-0',
                              selected ? 'bg-primary' : 'bg-fg-3',
                            )}
                            style={{ width: HANDLE_WIDTH }}
                            onPointerDown={(event) => beginTrim(event, item, 'trim-out')}
                          />
                        </>
                      )}
                    </div>
                  );
                })}
              </div>
            ))}

            <div
              className="pointer-events-none absolute top-0 bottom-0 z-20"
              style={{ left: playheadX }}
            >
              <button
                type="button"
                aria-label="Cabezal de reproducción"
                className={cn(
                  FOCUS_RING,
                  'pointer-events-auto absolute top-0 bottom-0 w-4 -translate-x-1/2 cursor-ew-resize border-0 bg-transparent',
                )}
                onPointerDown={beginSeek}
              />
              <div className="absolute top-0 bottom-0 w-0.5 -translate-x-1/2 bg-primary shadow-[var(--glow-primary-sm)]" />
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

function applyTrim(
  drag: TrimDrag,
  clientX: number,
  pxPerSecond: number,
  onChange: (next: EditorDocument) => void,
): void {
  const dt = (clientX - drag.originX) / pxPerSecond;
  if (drag.kind === 'trim-out') {
    const minOut = drag.sourceIn + MIN_OUTPUT_SECONDS * drag.speed;
    const sourceOut = Math.max(minOut, drag.sourceOut + dt * drag.speed);
    onChange(trimItem(drag.doc, drag.itemId, drag.sourceIn, sourceOut));
    return;
  }
  const minIn = 0;
  const maxIn = drag.sourceOut - MIN_OUTPUT_SECONDS * drag.speed;
  let timelineStart = Math.max(0, drag.timelineStart + dt);
  let sourceIn = drag.sourceIn + (timelineStart - drag.timelineStart) * drag.speed;
  if (sourceIn < minIn) {
    timelineStart += (minIn - sourceIn) / drag.speed;
    sourceIn = minIn;
  }
  if (sourceIn > maxIn) {
    timelineStart -= (sourceIn - maxIn) / drag.speed;
    sourceIn = maxIn;
  }
  if (timelineStart < 0) {
    sourceIn -= (0 - timelineStart) * drag.speed;
    timelineStart = 0;
  }
  onChange(moveItem(trimItem(drag.doc, drag.itemId, sourceIn, drag.sourceOut), drag.itemId, timelineStart));
}

function snapStart(start: number, doc: EditorDocument, movingId: string): number {
  const edges = [0];
  for (const track of doc.tracks) {
    for (const item of track.items) {
      if (item.id === movingId) continue;
      edges.push(item.timeline_start, itemTimelineEnd(item));
    }
  }
  let best = start;
  let bestDist = SNAP_SECONDS;
  for (const edge of edges) {
    const dist = Math.abs(start - edge);
    if (dist <= bestDist) {
      best = edge;
      bestDist = dist;
    }
  }
  return Math.max(0, best);
}

function timeFromPointer(clientX: number, scroller: HTMLDivElement | null, pxPerSecond: number): number {
  if (scroller === null || pxPerSecond <= 0) return 0;
  const x = clientX - scroller.getBoundingClientRect().left + scroller.scrollLeft;
  return Math.max(0, x / pxPerSecond);
}

function rulerTicks(duration: number, pxPerSecond: number): number[] {
  let step = 1;
  if (pxPerSecond < 36) step = 5;
  else if (pxPerSecond < 60) step = 2;
  const ticks: number[] = [];
  for (let t = 0; t <= duration + 0.0001; t += step) {
    ticks.push(Number(t.toFixed(2)));
  }
  return ticks;
}

function formatTick(seconds: number): string {
  const total = Math.max(0, Math.floor(seconds));
  const minutes = Math.floor(total / 60);
  const rest = total % 60;
  return `${minutes}:${String(rest).padStart(2, '0')}`;
}

function formatTimecode(seconds: number): string {
  const clamped = Math.max(0, seconds);
  const minutes = Math.floor(clamped / 60);
  const rest = clamped - minutes * 60;
  return `${minutes}:${rest.toFixed(2).padStart(5, '0')}`;
}

function canDeleteTrack(track: EditorTrack, videoTrackCount: number, trackCount: number): boolean {
  if (trackCount <= 1) return false;
  if (track.kind === EDITOR_TRACK_KINDS.video && videoTrackCount <= 1) return false;
  return true;
}

function isTextEntry(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  const tag = target.tagName;
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true;
  return target.isContentEditable;
}

function clamp(value: number, min: number, max: number): number {
  if (value < min) return min;
  if (value > max) return max;
  return value;
}
