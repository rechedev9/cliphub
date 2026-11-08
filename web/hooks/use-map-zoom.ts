'use client';

import { useCallback, useRef, useState, type MouseEvent, type PointerEvent } from 'react';
import { MAX_ZOOM, WHEEL_STEP, panView, sameRect, wheelZooms, zoomLevel, zoomViewAt } from '@/lib/replay/map-view';
import type { RadarRect } from '@/lib/tactical-transform';

/** A press that moves less than this is a click, not a drag. */
const CLICK_SLOP = 4;

type Box = { width: number; height: number };

/** The props a map element takes to zoom on the wheel, pan on a drag and reset on a double click. */
export type MapZoomSurface = {
  ref: (element: HTMLElement | null) => (() => void) | undefined;
  onPointerDown: (event: PointerEvent<HTMLElement>) => void;
  onPointerMove: (event: PointerEvent<HTMLElement>) => void;
  onPointerUp: (event: PointerEvent<HTMLElement>) => void;
  onPointerCancel: () => void;
  onDoubleClick: () => void;
  onClickCapture: (event: MouseEvent<HTMLElement>) => void;
};

export type MapZoom = {
  /** The window onto the map, in the units of `base`. */
  readonly view: RadarRect;
  readonly zoom: number;
  readonly zoomed: boolean;
  readonly grabbing: boolean;
  /** `cursor-grab` while there is something to pan. */
  readonly cursor: string;
  readonly zoomBy: (factor: number) => void;
  readonly reset: () => void;
  /** Spread on every element the map is drawn in; several can share one zoom. */
  readonly surface: MapZoomSurface;
};

function boxOf(element: Element): Box {
  const rect = element.getBoundingClientRect();
  return { width: Math.max(1, rect.width), height: Math.max(1, rect.height) };
}

/**
 * Zoom and pan of a 2D map through `lib/replay/map-view`, the replay's rules:
 * the wheel zooms on the cursor, a drag pans a zoomed map, a double click shows
 * the whole map. `base` is the whole map in any units; a new base resets.
 * With `enabled` false the map ignores the wheel and drags, e.g. while its base is not known yet.
 */
export function useMapZoom(base: RadarRect, options: { enabled?: boolean } = {}): MapZoom {
  const enabled = options.enabled ?? true;
  const [stored, setStored] = useState<{ base: RadarRect; view: RadarRect } | null>(null);
  const [grabbing, setGrabbing] = useState(false);
  const view = stored !== null && sameRect(stored.base, base) ? stored.view : base;
  const state = useRef({ view, base, enabled, box: { width: 1, height: 1 } });
  state.current.view = view;
  state.current.base = base;
  state.current.enabled = enabled;
  const drag = useRef<{ x: number; y: number; moved: boolean; id: number } | null>(null);
  const swallowClick = useRef(false);

  const apply = useCallback((next: RadarRect) => {
    const current = state.current.base;
    state.current.view = next;
    setStored(sameRect(next, current) ? null : { base: current, view: next });
  }, []);

  const ref = useCallback(
    (element: HTMLElement | null) => {
      if (element === null) return undefined;
      state.current.box = boxOf(element);
      const onWheel = (event: WheelEvent): void => {
        const { view: current, base: whole } = state.current;
        if (!state.current.enabled || !wheelZooms(current, whole, event.deltaY)) return;
        event.preventDefault();
        const rect = element.getBoundingClientRect();
        const box = boxOf(element);
        state.current.box = box;
        apply(zoomViewAt({ box, view: current, base: whole, cursor: { x: event.clientX - rect.left, y: event.clientY - rect.top }, factor: WHEEL_STEP ** -event.deltaY }));
      };
      // Not passive, so the page does not scroll under a zooming map.
      element.addEventListener('wheel', onWheel, { passive: false });
      return () => element.removeEventListener('wheel', onWheel);
    },
    [apply],
  );

  const zoomBy = useCallback(
    (factor: number) => {
      const { view: current, base: whole, box, enabled: on } = state.current;
      if (!on) return;
      apply(zoomViewAt({ box, view: current, base: whole, cursor: { x: box.width / 2, y: box.height / 2 }, factor }));
    },
    [apply],
  );
  const reset = useCallback(() => apply(state.current.base), [apply]);
  const zoom = zoomLevel(view, base);

  const surface: MapZoomSurface = {
    ref,
    onPointerDown: (event) => {
      if (event.button !== 0 || !state.current.enabled || zoomLevel(state.current.view, state.current.base) <= 1) return;
      drag.current = { x: event.clientX, y: event.clientY, moved: false, id: event.pointerId };
      event.currentTarget.setPointerCapture(event.pointerId);
    },
    onPointerMove: (event) => {
      const current = drag.current;
      if (current === null || current.id !== event.pointerId) return;
      const dx = event.clientX - current.x;
      const dy = event.clientY - current.y;
      if (!current.moved && Math.hypot(dx, dy) < CLICK_SLOP) return;
      current.moved = true;
      current.x = event.clientX;
      current.y = event.clientY;
      setGrabbing(true);
      const box = boxOf(event.currentTarget);
      state.current.box = box;
      apply(panView({ box, view: state.current.view, base: state.current.base, dx, dy }));
    },
    onPointerUp: () => {
      swallowClick.current = drag.current?.moved ?? false;
      drag.current = null;
      setGrabbing(false);
    },
    onPointerCancel: () => {
      drag.current = null;
      setGrabbing(false);
    },
    onDoubleClick: reset,
    onClickCapture: (event) => {
      if (!swallowClick.current) return;
      swallowClick.current = false;
      event.stopPropagation();
    },
  };

  let cursor = '';
  if (grabbing) cursor = 'cursor-grabbing';
  else if (zoom > 1) cursor = 'cursor-grab';
  return { view, zoom: Math.min(zoom, MAX_ZOOM), zoomed: zoom > 1, grabbing, cursor, zoomBy, reset, surface };
}
