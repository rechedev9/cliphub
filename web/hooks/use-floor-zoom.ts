'use client';

import { useCallback, useRef, useState } from 'react';
import type { MouseEvent, PointerEvent } from 'react';
import { RADAR_LEVELS } from '@/lib/api/tactical';
import { WHEEL_STEP, MAX_ZOOM } from '@/lib/replay/map-view';
import { WHOLE_FLOORS, floorWindow, isWholeView, panFloors, zoomFloors } from '@/lib/replay/floor-split';
import type { FloorPane, FloorView } from '@/lib/replay/floor-split';
import type { RadarRect } from '@/lib/tactical-transform';
import type { MapZoomSurface } from '@/hooks/use-map-zoom';

/** A press that moves less than this is a click, not a drag. */
const CLICK_SLOP = 4;

type Box = { width: number; height: number };

export type FloorZoom = {
  readonly view: FloorView;
  readonly zoom: number;
  readonly zoomed: boolean;
  readonly grabbing: boolean;
  /** `cursor-grab` while there is something to pan. */
  readonly cursor: string;
  readonly zoomBy: (factor: number) => void;
  readonly reset: () => void;
  /** The part of a floor's extent on show, for the plate that draws that floor. */
  readonly windowOf: (rect: RadarRect) => RadarRect;
  /** The props for the plate that shows the floor of `rect`; every plate shares the one zoom. */
  readonly surfaceFor: (rect: RadarRect) => MapZoomSurface;
};

function boxOf(element: Element): Box {
  const rect = element.getBoundingClientRect();
  return { width: Math.max(1, rect.width), height: Math.max(1, rect.height) };
}

/** A plate as the pane of the shared layout: the zoom maths work on a box and the floor's extent. */
function plateAs(box: Box, rect: RadarRect): FloorPane {
  return { level: RADAR_LEVELS.default, x: 0, y: 0, width: box.width, height: box.height, rect };
}

/**
 * Zoom and pan shared by the plates of one map (one per floor), through
 * `lib/replay/floor-split`: the wheel zooms on the cursor, a drag pans a zoomed
 * map, a double click shows the whole map, and every floor moves by the same
 * world distance. With `enabled` false the map ignores the wheel and drags.
 */
export function useFloorZoom(options: { enabled?: boolean } = {}): FloorZoom {
  const enabled = options.enabled ?? true;
  const [view, setView] = useState<FloorView>(WHOLE_FLOORS);
  const [grabbing, setGrabbing] = useState(false);
  const state = useRef({ view, enabled, anchor: null as { box: Box; rect: RadarRect } | null });
  state.current.view = view;
  state.current.enabled = enabled;
  const drag = useRef<{ x: number; y: number; moved: boolean; id: number } | null>(null);
  const swallowClick = useRef(false);

  const apply = useCallback((next: FloorView) => {
    state.current.view = next;
    setView(next);
  }, []);

  const surfaceFor = useCallback(
    (rect: RadarRect): MapZoomSurface => ({
      ref: (element) => {
        if (element === null) return undefined;
        state.current.anchor = { box: boxOf(element), rect };
        const onWheel = (event: WheelEvent): void => {
          const current = state.current;
          if (!current.enabled || !(event.deltaY < 0 || (event.deltaY > 0 && !isWholeView(current.view)))) return;
          event.preventDefault();
          const bounds = element.getBoundingClientRect();
          const box = boxOf(element);
          current.anchor = { box, rect };
          apply(zoomFloors({ pane: plateAs(box, rect), view: current.view, cursor: { x: event.clientX - bounds.left, y: event.clientY - bounds.top }, factor: WHEEL_STEP ** -event.deltaY }));
        };
        // Not passive, so the page does not scroll under a zooming map.
        element.addEventListener('wheel', onWheel, { passive: false });
        return () => element.removeEventListener('wheel', onWheel);
      },
      onPointerDown: (event: PointerEvent<HTMLElement>) => {
        if (event.button !== 0 || !state.current.enabled || isWholeView(state.current.view)) return;
        drag.current = { x: event.clientX, y: event.clientY, moved: false, id: event.pointerId };
        event.currentTarget.setPointerCapture(event.pointerId);
      },
      onPointerMove: (event: PointerEvent<HTMLElement>) => {
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
        state.current.anchor = { box, rect };
        apply(panFloors({ pane: plateAs(box, rect), view: state.current.view, dx, dy }));
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
      onDoubleClick: () => apply(WHOLE_FLOORS),
      onClickCapture: (event: MouseEvent<HTMLElement>) => {
        if (!swallowClick.current) return;
        swallowClick.current = false;
        event.stopPropagation();
      },
    }),
    [apply],
  );

  const zoomBy = useCallback(
    (factor: number) => {
      const { anchor, view: current, enabled: on } = state.current;
      if (!on || anchor === null) return;
      apply(zoomFloors({ pane: plateAs(anchor.box, anchor.rect), view: current, cursor: { x: anchor.box.width / 2, y: anchor.box.height / 2 }, factor }));
    },
    [apply],
  );
  const reset = useCallback(() => apply(WHOLE_FLOORS), [apply]);
  const windowOf = useCallback((rect: RadarRect): RadarRect => floorWindow(rect, view), [view]);

  let cursor = '';
  if (grabbing) cursor = 'cursor-grabbing';
  else if (!isWholeView(view)) cursor = 'cursor-grab';
  return { view, zoom: Math.min(view.zoom, MAX_ZOOM), zoomed: !isWholeView(view), grabbing, cursor, zoomBy, reset, windowOf, surfaceFor };
}
