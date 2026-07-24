'use client';

import { useCallback, type ReactNode } from 'react';
import { browserWindowActivity } from '@/lib/window-activity';
import { cn } from '@/lib/utils';

/** Coarse pointers have no hover state to parallax against; do not even listen. */
const NO_HOVER_QUERY = '(hover: none)';

/*
 * `.studio-tilt-plane` multiplies --tilt-x/--tilt-y by --tilt-max itself, so the
 * host writes NORMALISED -1..1 rotations, never degrees. --px/--py are 0..100
 * because the sheen gradient consumes them as `calc(var(--px) * 1%)`. Writing
 * degrees here would produce a 24deg tilt against a 6deg design ceiling and
 * would bypass the --shell-depth gate entirely.
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

export type TiltSurfaceProps = {
  children: ReactNode;
  /** Class on the perspective container. */
  className?: string;
  /** Class on the tilting plane — the element the sheen and children ride on. */
  planeClassName?: string;
  /** Drop the specular sweep on surfaces where it would fight the content. */
  sheen?: boolean;
};

/**
 * Pointer-parallax wrapper for media surfaces. Writes CSS custom properties on a
 * ref: zero React re-renders, one requestAnimationFrame coalesced per pointer
 * burst, and one getBoundingClientRect per frame rather than per event (the
 * rect read forces layout, and pointermove fires far above the frame rate).
 *
 * Degradation is the CSS layer's job — --shell-depth flattens the transform
 * under the efficiency profile, reduced motion and forced colours — with one
 * exception: CSS can freeze a transition but cannot stop JS from writing custom
 * properties, so an inactive Studio window is released here.
 */
export function TiltSurface({ children, className, planeClassName, sheen = true }: TiltSurfaceProps): ReactNode {
  const attach = useCallback((node: HTMLDivElement | null): (() => void) | undefined => {
    if (node === null || typeof window === 'undefined') return undefined;
    if (window.matchMedia(NO_HOVER_QUERY).matches) return undefined;

    let frame = 0;
    let pending: { x: number; y: number } | null = null;

    const release = (): void => {
      if (frame !== 0) cancelAnimationFrame(frame);
      frame = 0;
      pending = null;
      for (const property of TRACKED_PROPERTIES) node.style.removeProperty(property);
    };

    const apply = (): void => {
      frame = 0;
      if (pending === null) return;
      const box = node.getBoundingClientRect();
      if (box.width === 0 || box.height === 0) return;
      const px = (pending.x - box.left) / box.width;
      const py = (pending.y - box.top) / box.height;
      node.style.setProperty(TILT_X, clampToUnit((0.5 - py) * 2).toFixed(3));
      node.style.setProperty(TILT_Y, clampToUnit((px - 0.5) * 2).toFixed(3));
      node.style.setProperty(POINTER_X, (px * 100).toFixed(1));
      node.style.setProperty(POINTER_Y, (py * 100).toFixed(1));
      node.style.setProperty(SHEEN, '1');
    };

    const move = (event: PointerEvent): void => {
      if (!browserWindowActivity.isActive()) return;
      pending = { x: event.clientX, y: event.clientY };
      if (frame === 0) frame = requestAnimationFrame(apply);
    };

    const unsubscribe = browserWindowActivity.subscribe(() => {
      if (!browserWindowActivity.isActive()) release();
    });

    node.addEventListener('pointermove', move, { passive: true });
    node.addEventListener('pointerleave', release);
    node.addEventListener('pointercancel', release);
    // Capture phase: the blur that ends a tilt comes from a focused descendant.
    node.addEventListener('blur', release, true);

    return () => {
      unsubscribe();
      release();
      node.removeEventListener('pointermove', move);
      node.removeEventListener('pointerleave', release);
      node.removeEventListener('pointercancel', release);
      node.removeEventListener('blur', release, true);
    };
  }, []);

  return (
    <div ref={attach} className={cn('studio-tilt', className)}>
      <div className={cn('studio-tilt-plane', sheen && 'studio-tilt-sheen', planeClassName)}>{children}</div>
    </div>
  );
}
