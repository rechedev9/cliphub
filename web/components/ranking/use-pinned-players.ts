'use client';

import { useCallback, useEffect, useState } from 'react';
import { PINS_STORAGE_KEY, parsePins, togglePin, type PinnedPlayer } from '@/lib/ranking/ranking';

/** The players kept on top of the ranking. Pins are a per-profile convenience: storage may be missing and the page works without it. */
export function usePinnedPlayers(): { pins: readonly PinnedPlayer[]; toggle: (pin: PinnedPlayer) => void } {
  const [pins, setPins] = useState<readonly PinnedPlayer[]>([]);

  // Read after mount: the server render has no storage.
  useEffect(() => {
    try {
      setPins(parsePins(window.localStorage.getItem(PINS_STORAGE_KEY)));
    } catch {
      setPins([]);
    }
  }, []);

  const toggle = useCallback((pin: PinnedPlayer) => {
    setPins((current) => {
      const next = togglePin(current, pin);
      try {
        window.localStorage.setItem(PINS_STORAGE_KEY, JSON.stringify(next));
      } catch {
        // The pins stay in memory for this session.
      }
      return next;
    });
  }, []);

  return { pins, toggle };
}
