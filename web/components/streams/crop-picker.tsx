'use client';

import { useCallback, useId, useRef, type ReactNode } from 'react';
import type { NormalizedRect } from '@/lib/api/streams';
import { StreamFrameCanvas, useStreamFrame } from '@/components/streams/stream-frame-session';

const MIN_SIZE = 0.08;
const KEYBOARD_STEP = 0.005;
const KEYBOARD_LARGE_STEP = 0.02;

type Drag = { kind: 'move' | 'resize'; startClientX: number; startClientY: number; startRect: NormalizedRect };

const MOVE_LABEL = 'Mover región de recorte del facecam';
const RESIZE_LABEL = 'Redimensionar región de recorte del facecam';

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

/** Keeps the rect inside 0..1 on both axes and respects independent minimum dimensions. */
function clampRect(rect: NormalizedRect, minWidth: number, minHeight: number): NormalizedRect {
  const width = clamp(rect.width, minWidth, 1);
  const height = clamp(rect.height, minHeight, 1);
  const x = clamp(rect.x, 0, 1 - width);
  const y = clamp(rect.y, 0, 1 - height);
  return { x, y, width, height };
}

/**
 * The facecam source crop picker. Pointer and keyboard controls both emit a
 * normalized rectangle against the shared session frame.
 */
export function CropPicker({
  rect,
  onChange,
  disabled = false,
}: {
  rect: NormalizedRect;
  onChange: (rect: NormalizedRect) => void;
  disabled?: boolean;
}): ReactNode {
  const containerRef = useRef<HTMLDivElement>(null);
  const dragRef = useRef<Drag | null>(null);
  const frame = useStreamFrame();
  const sourceAspectRatio = frame.sourceWidth > 0 && frame.sourceHeight > 0
    ? `${frame.sourceWidth} / ${frame.sourceHeight}`
    : null;
  const instructionsId = useId();
  const safeRect = clampRect(rect, MIN_SIZE, MIN_SIZE);

  const normalizedDelta = useCallback((clientX: number, clientY: number, drag: Drag) => {
    const container = containerRef.current;
    if (!container) return { dx: 0, dy: 0 };
    const box = container.getBoundingClientRect();
    return {
      dx: box.width > 0 ? (clientX - drag.startClientX) / box.width : 0,
      dy: box.height > 0 ? (clientY - drag.startClientY) / box.height : 0,
    };
  }, []);

  const beginDrag = useCallback(
    (dragKind: Drag['kind']) => (event: React.PointerEvent<HTMLButtonElement>) => {
      if (disabled) return;
      event.preventDefault();
      event.stopPropagation();
      event.currentTarget.setPointerCapture(event.pointerId);
      dragRef.current = {
        kind: dragKind,
        startClientX: event.clientX,
        startClientY: event.clientY,
        startRect: safeRect,
      };
    },
    [disabled, safeRect],
  );

  const onPointerMove = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      const drag = dragRef.current;
      if (!drag) return;
      const { dx, dy } = normalizedDelta(event.clientX, event.clientY, drag);
      if (drag.kind === 'move') {
        onChange(clampRect(
          { ...drag.startRect, x: drag.startRect.x + dx, y: drag.startRect.y + dy },
          MIN_SIZE,
          MIN_SIZE,
        ));
        return;
      }
      onChange(clampRect(
        { ...drag.startRect, width: drag.startRect.width + dx, height: drag.startRect.height + dy },
        MIN_SIZE,
        MIN_SIZE,
      ));
    },
    [normalizedDelta, onChange],
  );

  const endDrag = useCallback(() => {
    dragRef.current = null;
  }, []);

  const moveWithKeyboard = useCallback(
    (event: React.KeyboardEvent<HTMLButtonElement>) => {
      const step = event.shiftKey ? KEYBOARD_LARGE_STEP : KEYBOARD_STEP;
      let next: NormalizedRect | null = null;
      if (event.key === 'ArrowLeft') next = { ...safeRect, x: safeRect.x - step };
      if (event.key === 'ArrowRight') next = { ...safeRect, x: safeRect.x + step };
      if (event.key === 'ArrowUp') next = { ...safeRect, y: safeRect.y - step };
      if (event.key === 'ArrowDown') next = { ...safeRect, y: safeRect.y + step };
      if (!next) return;
      event.preventDefault();
      onChange(clampRect(next, MIN_SIZE, MIN_SIZE));
    },
    [onChange, safeRect],
  );

  const resizeWithKeyboard = useCallback(
    (event: React.KeyboardEvent<HTMLButtonElement>) => {
      const step = event.shiftKey ? KEYBOARD_LARGE_STEP : KEYBOARD_STEP;
      let next: NormalizedRect | null = null;
      if (event.key === 'ArrowLeft') next = { ...safeRect, width: safeRect.width - step };
      if (event.key === 'ArrowRight') next = { ...safeRect, width: safeRect.width + step };
      if (event.key === 'ArrowUp') next = { ...safeRect, height: safeRect.height - step };
      if (event.key === 'ArrowDown') next = { ...safeRect, height: safeRect.height + step };
      if (!next) return;
      event.preventDefault();
      onChange(clampRect(next, MIN_SIZE, MIN_SIZE));
    },
    [onChange, safeRect],
  );

  return (
    <div className="flex flex-col gap-3" data-stream-crop-picker="facecam">
      <p id={instructionsId} className="sr-only">
        Usa las flechas para ajustar el recorte. Mantén Mayús para mover o redimensionar más rápido.
      </p>
      <div
        ref={containerRef}
        className="relative aspect-video w-full touch-none overflow-hidden rounded-lg border border-border bg-background select-none"
        style={sourceAspectRatio ? { aspectRatio: sourceAspectRatio } : undefined}
        onPointerMove={onPointerMove}
        onPointerUp={endDrag}
        onPointerCancel={endDrag}
      >
        <StreamFrameCanvas
          mode="contain"
          className="pointer-events-none absolute inset-0 h-full w-full object-contain"
        />
        <button
          type="button"
          disabled={disabled}
          aria-label={MOVE_LABEL}
          aria-describedby={instructionsId}
          onPointerDown={beginDrag('move')}
          onKeyDown={moveWithKeyboard}
          className={'absolute cursor-move rounded-sm border-2 border-primary bg-primary/10 shadow-[0_0_0_9999px_color-mix(in_oklch,var(--background)_70%,transparent)] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:cursor-default disabled:opacity-40'}
          style={{
            left: `${safeRect.x * 100}%`,
            top: `${safeRect.y * 100}%`,
            width: `${safeRect.width * 100}%`,
            height: `${safeRect.height * 100}%`,
          }}
        />
        <button
          type="button"
          disabled={disabled}
          aria-label={RESIZE_LABEL}
          aria-describedby={instructionsId}
          onPointerDown={beginDrag('resize')}
          onKeyDown={resizeWithKeyboard}
          className={'absolute size-4 -translate-x-1/2 -translate-y-1/2 cursor-nwse-resize rounded-sm border-2 border-background bg-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:cursor-default disabled:opacity-40'}
          style={{
            left: `${(safeRect.x + safeRect.width) * 100}%`,
            top: `${(safeRect.y + safeRect.height) * 100}%`,
          }}
        />
      </div>
    </div>
  );
}
