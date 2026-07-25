import { browserWindowActivity } from '@/lib/window-activity';

/**
 * One pointer listener for the whole scoreboard, not one per row.
 *
 * `TiltSurface` is the right shape for a handful of large media cards, but the
 * inbox can hold dozens of rows and each instance would attach its own
 * `pointermove`, its own `requestAnimationFrame` loop and its own activity
 * subscription. Here the list owns a single listener, resolves the row under the
 * pointer with `closest()`, and writes the same custom properties the shared
 * `.studio-tilt*` recipe already reads — so the depth gate, reduced motion and
 * forced colours still switch the effect off through `--shell-depth` alone, and
 * React never re-renders while the pointer moves.
 */
const MATCH_ROW_ATTRIBUTE = 'data-match-row';

/** Spread onto a row so the list-level tracker can find and light it. */
export const MATCH_ROW_MARKER: Readonly<Record<string, string>> = { [MATCH_ROW_ATTRIBUTE]: '' };

const MATCH_ROW_SELECTOR = `[${MATCH_ROW_ATTRIBUTE}]`;

/** Coarse pointers have no hover state to parallax against; do not even listen. */
const NO_HOVER_QUERY = '(hover: none)';

/*
 * `.studio-tilt-plane` multiplies --tilt-x/--tilt-y by --tilt-max itself, so
 * these are NORMALISED -1..1 rotations, never degrees; --px/--py are 0..100
 * because the sheen gradient consumes them as `calc(var(--px) * 1%)`.
 */
const TILT_X = '--tilt-x';
const TILT_Y = '--tilt-y';
const POINTER_X = '--px';
const POINTER_Y = '--py';
const SHEEN = '--tilt-sheen';

const TRACKED_PROPERTIES = [TILT_X, TILT_Y, POINTER_X, POINTER_Y, SHEEN] as const;

function clampToUnit(value: number): number {
  return Math.min(1, Math.max(-1, value));
}

/**
 * Attach the cursor-tracked lift/specular sweep to a list of rows. Returns the
 * detach function, so a React 19 ref callback can return it directly.
 */
export function attachMatchRowParallax(list: HTMLElement): () => void {
  if (typeof window === 'undefined') return () => {};
  if (window.matchMedia(NO_HOVER_QUERY).matches) return () => {};

  let frame = 0;
  let pending: { row: HTMLElement; x: number; y: number } | null = null;
  let lit: HTMLElement | null = null;

  const darken = (): void => {
    if (lit === null) return;
    for (const property of TRACKED_PROPERTIES) lit.style.removeProperty(property);
    lit = null;
  };

  const release = (): void => {
    if (frame !== 0) cancelAnimationFrame(frame);
    frame = 0;
    pending = null;
    darken();
  };

  // One getBoundingClientRect per frame rather than per event: the read forces
  // layout and pointermove fires well above the frame rate.
  const apply = (): void => {
    frame = 0;
    if (pending === null) return;
    const { row, x, y } = pending;
    const box = row.getBoundingClientRect();
    if (box.width === 0 || box.height === 0) return;
    if (lit !== null && lit !== row) darken();
    lit = row;
    const px = (x - box.left) / box.width;
    const py = (y - box.top) / box.height;
    row.style.setProperty(TILT_X, clampToUnit((0.5 - py) * 2).toFixed(3));
    row.style.setProperty(TILT_Y, clampToUnit((px - 0.5) * 2).toFixed(3));
    row.style.setProperty(POINTER_X, (px * 100).toFixed(1));
    row.style.setProperty(POINTER_Y, (py * 100).toFixed(1));
    row.style.setProperty(SHEEN, '1');
  };

  const move = (event: PointerEvent): void => {
    // CSS can freeze a transition but cannot stop JS from writing custom
    // properties, so an inactive Studio window is released here.
    if (!browserWindowActivity.isActive()) {
      release();
      return;
    }
    const target = event.target instanceof Element ? event.target.closest(MATCH_ROW_SELECTOR) : null;
    if (!(target instanceof HTMLElement)) {
      release();
      return;
    }
    pending = { row: target, x: event.clientX, y: event.clientY };
    if (frame === 0) frame = requestAnimationFrame(apply);
  };

  const unsubscribe = browserWindowActivity.subscribe(() => {
    if (!browserWindowActivity.isActive()) release();
  });

  list.addEventListener('pointermove', move, { passive: true });
  list.addEventListener('pointerleave', release);
  list.addEventListener('pointercancel', release);

  return () => {
    unsubscribe();
    release();
    list.removeEventListener('pointermove', move);
    list.removeEventListener('pointerleave', release);
    list.removeEventListener('pointercancel', release);
  };
}
