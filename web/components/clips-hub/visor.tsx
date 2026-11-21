'use client';

import { useCallback, useEffect, useRef, useState, type KeyboardEvent, type PointerEvent, type ReactNode, type WheelEvent } from 'react';
import { Pause, Play, Volume2, VolumeX, X } from 'lucide-react';
import { Dialog as DialogPrimitive } from 'radix-ui';
import { visorAfterDelete, type VisorEntry } from '@/lib/clips/visor-queue';
import { claimMediaPlayback } from '@/lib/media-playback-owner';
import { cn } from '@/lib/utils';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { VisorActions } from '@/components/clips-hub/visor-actions';
import { PlatformLayerPicker, PlatformLayerShapes, usePlatformLayer } from '@/components/clips-hub/visor-platform-layer';

const MUTED_STORAGE_KEY = 'cliphub-visor-muted';
const WHEEL_STEP_PX = 40;
const WHEEL_GESTURE_GAP_MS = 200;
const DRAG_SHARE = 0.18;
const FLICK_PX_PER_MS = 0.5;
const TAP_PX = 6;
const EDGE_DAMPING = 0.3;

const CONTROL_CLASS = 'size-11 rounded-full motion-safe:active:scale-[0.97] [&_svg:not([class*=size-])]:size-5';

export type VisorProps = {
  entries: readonly VisorEntry[];
  currentId: string;
  onMove: (entryId: string) => void;
  onClose: () => void;
  onChange: () => void;
};

type Drag = { pointerId: number; startY: number; startAt: number; offset: number };
type WheelGesture = { sum: number; lastAt: number; moved: boolean };

function readMuted(): boolean {
  try {
    return localStorage.getItem(MUTED_STORAGE_KEY) === '1';
  } catch {
    return false;
  }
}

function writeMuted(muted: boolean): boolean {
  try {
    localStorage.setItem(MUTED_STORAGE_KEY, muted ? '1' : '0');
    return true;
  } catch {
    return false;
  }
}

export function Visor({ entries: queued, currentId, onMove, onClose, onChange }: VisorProps): ReactNode {
  const [unplayable, setUnplayable] = useState<ReadonlySet<string>>(() => new Set());
  const entries = queued.filter((entry) => !unplayable.has(entry.id));
  const index = Math.max(0, entries.findIndex((entry) => entry.id === currentId));
  const current = entries[index];
  const stageRef = useRef<HTMLDivElement>(null);
  const wheel = useRef<WheelGesture>({ sum: 0, lastAt: 0, moved: false });
  const drag = useRef<Drag | null>(null);
  const [dragOffset, setDragOffset] = useState<number | null>(null);
  const [muted, setMuted] = useState(readMuted);
  const shownId = current?.id ?? null;
  const [pausedId, setPausedId] = useState<string | null>(null);
  const paused = shownId !== null && pausedId === shownId;
  const setPaused = useCallback((next: boolean): void => setPausedId(next ? shownId : null), [shownId]);
  const togglePaused = (): void => setPausedId((was) => (was === shownId ? null : shownId));
  const [layer, setLayer] = usePlatformLayer();

  const step = useCallback((delta: 1 | -1): void => {
    const next = entries[index + delta];
    if (next === undefined) return;
    onMove(next.id);
  }, [entries, index, onMove]);

  useEffect(() => {
    if (shownId !== null) stageRef.current?.focus({ preventScroll: true });
  }, [shownId]);

  const toggleMuted = (): void => {
    setMuted((was) => {
      writeMuted(!was);
      return !was;
    });
  };

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>): void => {
    if (event.altKey || event.ctrlKey || event.metaKey || !ownsEvent(event)) return;
    if (event.key === ' ' && event.target instanceof Element && event.target.closest('button') !== null) return;
    const actions: Record<string, () => void> = {
      ArrowDown: () => step(1),
      ArrowUp: () => step(-1),
      ' ': togglePaused,
      m: toggleMuted,
      M: toggleMuted,
    };
    const action = actions[event.key];
    if (action === undefined) return;
    event.preventDefault();
    action();
  };

  const onWheel = (event: WheelEvent<HTMLDivElement>): void => {
    if (!ownsEvent(event)) return;
    const gesture = wheel.current;
    const now = performance.now();
    if (now - gesture.lastAt > WHEEL_GESTURE_GAP_MS) Object.assign(gesture, { sum: 0, moved: false });
    gesture.lastAt = now;
    if (gesture.moved) return;
    gesture.sum += event.deltaY;
    if (Math.abs(gesture.sum) < WHEEL_STEP_PX) return;
    gesture.moved = true;
    step(gesture.sum > 0 ? 1 : -1);
  };

  const onPointerDown = (event: PointerEvent<HTMLDivElement>): void => {
    if (event.button !== 0 || drag.current !== null) return;
    event.currentTarget.setPointerCapture(event.pointerId);
    drag.current = { pointerId: event.pointerId, startY: event.clientY, startAt: performance.now(), offset: 0 };
  };

  const onPointerMove = (event: PointerEvent<HTMLDivElement>): void => {
    const active = drag.current;
    if (active === null || active.pointerId !== event.pointerId) return;
    const raw = event.clientY - active.startY;
    const pastEdge = (raw < 0 && index === entries.length - 1) || (raw > 0 && index === 0);
    active.offset = pastEdge ? raw * EDGE_DAMPING : raw;
    if (Math.abs(raw) > TAP_PX) setDragOffset(active.offset);
  };

  const onPointerUp = (event: PointerEvent<HTMLDivElement>): void => {
    const active = drag.current;
    if (active === null || active.pointerId !== event.pointerId) return;
    drag.current = null;
    setDragOffset(null);
    const moved = event.clientY - active.startY;
    if (Math.abs(moved) <= TAP_PX) {
      togglePaused();
      return;
    }
    const height = stageRef.current?.clientHeight ?? 1;
    const speed = Math.abs(moved) / Math.max(1, performance.now() - active.startAt);
    if (Math.abs(moved) > height * DRAG_SHARE || speed > FLICK_PX_PER_MS) step(moved < 0 ? 1 : -1);
  };

  const onPointerCancel = (): void => {
    drag.current = null;
    setDragOffset(null);
  };

  if (current === undefined) return null;
  const leave = (): void => {
    const next = visorAfterDelete(entries, current.id);
    if (next === null) onClose();
    else onMove(next);
  };
  const leaveUnplayable = (): void => {
    leave();
    setUnplayable((was) => new Set([...was, current.id]));
  };
  const neighbours = [index - 1, index, index + 1].flatMap((at) => {
    const entry = entries[at];
    return entry === undefined ? [] : [{ entry, offset: at - index }];
  });

  return (
    <DialogPrimitive.Root open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Content
          aria-describedby={undefined}
          onOpenAutoFocus={(event) => {
            event.preventDefault();
            stageRef.current?.focus({ preventScroll: true });
          }}
          onKeyDown={onKeyDown}
          onWheel={onWheel}
          className="fixed inset-0 z-50 grid grid-cols-[1fr_auto_1fr] content-start justify-items-center gap-x-4 gap-y-3 bg-background px-4 pt-3 pb-4 outline-none"
        >
          <div className="col-span-3 flex w-full max-w-3xl items-center gap-3">
            <p className="text-body-sm font-semibold text-fg-1 tabular-nums" aria-live="polite">
              {index + 1} de {entries.length}
            </p>
            {current.mark === null ? null : <StatusTag className="shrink-0">{current.mark}</StatusTag>}
            <DialogPrimitive.Title className="min-w-0 flex-1 truncate text-body-sm text-fg-2">{current.title}</DialogPrimitive.Title>
            <DialogPrimitive.Close asChild>
              <Button type="button" variant="ghost" size="icon" aria-label="Cerrar el visor" className={CONTROL_CLASS}>
                <X aria-hidden />
              </Button>
            </DialogPrimitive.Close>
          </div>

          <div
            ref={stageRef}
            tabIndex={-1}
            role="group"
            aria-roledescription="Short"
            aria-label={current.title}
            onPointerDown={onPointerDown}
            onPointerMove={onPointerMove}
            onPointerUp={onPointerUp}
            onPointerCancel={onPointerCancel}
            className="relative col-start-2 aspect-[9/16] h-[min(calc(100dvh_-_9rem),calc((100vw_-_28rem)_*_16_/_9))] touch-none overflow-hidden rounded-[2rem] border border-border-strong bg-black outline-none select-none"
          >
            {neighbours.map(({ entry, offset }) => (
              <div
                key={entry.id}
                aria-hidden={offset !== 0 || undefined}
                style={{ transform: `translateY(calc(${offset * 100}% + ${dragOffset ?? 0}px))` }}
                className={cn(
                  'absolute inset-0 motion-reduce:transition-none',
                  dragOffset === null && 'transition-transform duration-(--dur-slow) ease-standard',
                )}
              >
                {offset === 0 ? (
                  <ActiveShort key={entry.item.playbackUrl} entry={entry} muted={muted} paused={paused} onPausedChange={setPaused} onUnplayable={leaveUnplayable} />
                ) : (
                  <Poster url={entry.item.posterUrl} />
                )}
              </div>
            ))}
            <PlatformLayerShapes layer={layer} />
          </div>

          <VisorActions key={current.id} entry={current} onChange={onChange} onDeleted={leave} className="col-start-3 self-end justify-self-start" />

          <div className="col-start-2 flex items-center gap-2">
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label={paused ? 'Reproducir' : 'Pausar'}
              aria-keyshortcuts="Space"
              onClick={togglePaused}
              className={CONTROL_CLASS}
            >
              {paused ? <Play aria-hidden /> : <Pause aria-hidden />}
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label={muted ? 'Activar el sonido' : 'Silenciar'}
              aria-pressed={muted}
              aria-keyshortcuts="M"
              onClick={toggleMuted}
              className={CONTROL_CLASS}
            >
              {muted ? <VolumeX aria-hidden /> : <Volume2 aria-hidden />}
            </Button>
            <PlatformLayerPicker value={layer} onChange={setLayer} />
          </div>
        </DialogPrimitive.Content>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  );
}

function ownsEvent(event: { target: EventTarget; currentTarget: Element }): boolean {
  return event.target instanceof Node && event.currentTarget.contains(event.target);
}

function Poster({ url }: { url: string | undefined }): ReactNode {
  if (url === undefined) return null;
  return <img src={url} alt="" draggable={false} className="size-full object-cover" />;
}

type ActiveShortProps = {
  entry: VisorEntry;
  muted: boolean;
  paused: boolean;
  onPausedChange: (paused: boolean) => void;
  onUnplayable: () => void;
};

function ActiveShort({ entry, muted, paused, onPausedChange, onUnplayable }: ActiveShortProps): ReactNode {
  const videoRef = useRef<HTMLVideoElement>(null);
  const barRef = useRef<HTMLDivElement>(null);
  const owner = useRef({});

  useEffect(() => {
    const video = videoRef.current;
    if (video !== null) video.muted = muted;
  }, [muted]);

  useEffect(() => {
    const video = videoRef.current;
    if (video === null) return undefined;
    if (paused) {
      video.pause();
      return undefined;
    }
    const release = claimMediaPlayback(owner.current, () => video.pause());
    let live = true;
    video.play().catch(() => {
      if (live) onPausedChange(true);
    });
    return () => {
      live = false;
      release();
    };
  }, [paused, onPausedChange]);

  useEffect(() => {
    let frame = 0;
    const draw = (): void => {
      const video = videoRef.current;
      const bar = barRef.current;
      if (video !== null && bar !== null && video.duration > 0) {
        bar.style.transform = `scaleX(${video.currentTime / video.duration})`;
      }
      frame = requestAnimationFrame(draw);
    };
    frame = requestAnimationFrame(draw);
    return () => cancelAnimationFrame(frame);
  }, []);

  return (
    <>
      <video
        ref={videoRef}
        src={entry.item.playbackUrl}
        poster={entry.item.posterUrl}
        loop
        playsInline
        preload="auto"
        onPlay={() => onPausedChange(false)}
        onPause={() => onPausedChange(true)}
        onError={() => { if (entry.renderFailed) onUnplayable(); }}
        className="size-full object-cover"
      />
      <div aria-hidden className="absolute inset-x-0 bottom-0 h-1 bg-white/20">
        <div ref={barRef} className="h-full origin-left scale-x-0 bg-white" />
      </div>
    </>
  );
}
