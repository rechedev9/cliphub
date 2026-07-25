'use client';

import { useCallback, type ReactElement } from 'react';
import {
  AMBIENT_FRAME_MS,
  ambientMode,
  createAmbientScene,
  type AmbientMode,
} from '@/lib/studio-ambient';
import { CAPTURE_ACTIVE_ATTRIBUTE } from '@/lib/shell-activity';
import { browserWindowActivity } from '@/lib/window-activity';

const REDUCED_MOTION_QUERY = '(prefers-reduced-motion: reduce)';
const PROFILE_ATTRIBUTE = 'data-performance-profile';
const EFFICIENCY_PROFILE = 'efficiency';
/**
 * How long Studio may sit unfocused before the GL context is handed back. Long
 * enough that alt-tabbing to CS2 and straight back costs nothing, short enough
 * that a window left open behind a capture is not holding VRAM for an hour.
 */
const CONTEXT_RELEASE_MS = 60_000;

/**
 * The volumetric half of the backdrop: a hand-written WebGL depth field drawn
 * behind everything, with the CSS gradients underneath it as both the SSR paint
 * and the permanent fallback when WebGL is unavailable.
 *
 * The lifecycle is a React 19 ref callback with a cleanup return, so mount and
 * unmount are one function and there is no dependency dance. Everything that
 * decides whether to animate lives in `lib/studio-ambient.ts`; this file only
 * reads the four signals and drives the loop.
 */
export function StudioAmbient(): ReactElement {
  const attach = useCallback((canvas: HTMLCanvasElement | null): (() => void) | undefined => {
    if (canvas === null) return undefined;

    const scene = createAmbientScene(canvas);
    if (scene === null) {
      // No WebGL: drop the canvas entirely rather than leaving an empty
      // compositor layer over the gradients that are now doing the work.
      canvas.remove();
      return undefined;
    }

    const root = document.documentElement;
    const reduced = window.matchMedia(REDUCED_MOTION_QUERY);
    let raf = 0;
    let mode: AmbientMode = 'static';
    let lastFrame = 0;
    let releaseTimer: ReturnType<typeof setTimeout> | undefined;

    const currentMode = (): AmbientMode =>
      ambientMode({
        reducedMotion: reduced.matches,
        efficiency: root.getAttribute(PROFILE_ATTRIBUTE) === EFFICIENCY_PROFILE,
        captureActive: root.getAttribute(CAPTURE_ACTIVE_ATTRIBUTE) === 'true',
        windowActive: browserWindowActivity.isActive(),
      });

    const paint = (timeSeconds: number): void => {
      // Cross-fade in only once a real frame exists, so the gradients are never
      // replaced by a flash of empty canvas. `restore()` is asynchronous — the
      // context comes back on `webglcontextrestored`, not on the call — so the
      // first paint after a refocus draws nothing and must not fade the canvas
      // in over the fallback; the restore event repaints and does it then.
      if (scene.draw(timeSeconds, mode)) canvas.dataset.ready = 'true';
    };

    const frame = (now: number): void => {
      raf = requestAnimationFrame(frame);
      if (now - lastFrame < AMBIENT_FRAME_MS) return;
      lastFrame = now;
      paint(now / 1000);
    };

    const sync = (): void => {
      if (raf !== 0) {
        cancelAnimationFrame(raf);
        raf = 0;
      }
      if (releaseTimer !== undefined) {
        clearTimeout(releaseTimer);
        releaseTimer = undefined;
      }

      if (!browserWindowActivity.isActive()) {
        // Still paint one static frame: a window can be unfocused at load (an
        // Electron cold start behind the launcher, a background tab restored on
        // startup) and a backdrop that only appears after the user clicks would
        // read as a rendering glitch. Then hand the context back.
        mode = 'static';
        paint(0);
        releaseTimer = setTimeout(() => scene.release(), CONTEXT_RELEASE_MS);
        return;
      }

      scene.restore();
      mode = currentMode();
      if (mode === 'animated') {
        lastFrame = 0;
        raf = requestAnimationFrame(frame);
      } else {
        paint(0);
      }
    };

    const measure = (): void => {
      const box = canvas.getBoundingClientRect();
      if (scene.resize(box.width, box.height, window.devicePixelRatio)) sync();
    };

    // `data-window-activity` is mirrored onto <html> by WindowActivityPolicy,
    // but the store is the authority and fires first; observing the attribute
    // as well would just draw the same frame twice.
    const attributes = new MutationObserver(sync);
    attributes.observe(root, { attributeFilter: [PROFILE_ATTRIBUTE, CAPTURE_ACTIVE_ATTRIBUTE] });
    const unsubscribe = browserWindowActivity.subscribe(sync);
    reduced.addEventListener('change', sync);
    // The scene re-uploads its GL objects on this event; drawing again is what
    // actually brings the picture back.
    canvas.addEventListener('webglcontextrestored', sync);
    const resize = new ResizeObserver(measure);
    resize.observe(canvas);

    measure();
    sync();

    return () => {
      if (raf !== 0) cancelAnimationFrame(raf);
      if (releaseTimer !== undefined) clearTimeout(releaseTimer);
      attributes.disconnect();
      resize.disconnect();
      unsubscribe();
      reduced.removeEventListener('change', sync);
      canvas.removeEventListener('webglcontextrestored', sync);
      scene.dispose();
    };
  }, []);

  return (
    <div aria-hidden className="ff-ambient-layer ff-ambient-fallback">
      <canvas ref={attach} className="ff-ambient" />
    </div>
  );
}
