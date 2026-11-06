'use client';

import { useCallback, useEffect, useRef, useState, type RefObject } from 'react';

/**
 * A full-screen mode for `ref`: the element fills the window from the top
 * layer as a manual popover, so no ancestor's container clips it, and the
 * window goes full screen where the Fullscreen API allows it.
 */
export function useFullscreen(ref: RefObject<HTMLElement | null>): { active: boolean; toggle: () => void } {
  const [active, setActive] = useState(false);
  const on = useRef(false);
  const screen = useRef(false);

  const leave = useCallback(() => {
    const element = ref.current;
    if (screen.current && document.fullscreenElement !== null) void document.exitFullscreen();
    if (on.current && element !== null && element.matches(':popover-open')) element.hidePopover();
    element?.removeAttribute('popover');
    on.current = false;
    screen.current = false;
    setActive(false);
  }, [ref]);

  useEffect(() => {
    // Esc or F11 leaving full screen ends the mode too.
    const sync = (): void => {
      if (document.fullscreenElement !== null) screen.current = true;
      else if (screen.current && on.current) leave();
    };
    document.addEventListener('fullscreenchange', sync);
    return () => {
      document.removeEventListener('fullscreenchange', sync);
      if (on.current) leave();
    };
  }, [leave]);

  const toggle = useCallback(() => {
    const element = ref.current;
    if (element === null) return;
    if (on.current) {
      leave();
      return;
    }
    on.current = true;
    element.setAttribute('popover', 'manual');
    element.showPopover();
    setActive(true);
    document.documentElement.requestFullscreen().catch(() => undefined);
  }, [ref, leave]);

  return { active, toggle };
}
