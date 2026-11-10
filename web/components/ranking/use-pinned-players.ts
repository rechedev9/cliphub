'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { getPinnedPlayers, savablePins, setPinnedPlayers } from '@/lib/api/pinned-players';
import { PINS_STORAGE_KEY, parsePins, togglePin, type PinnedPlayer } from '@/lib/ranking/ranking';

function readLocal(): PinnedPlayer[] {
  try {
    return parsePins(window.localStorage.getItem(PINS_STORAGE_KEY));
  } catch {
    return [];
  }
}

function writeLocal(pins: readonly PinnedPlayer[]): void {
  try {
    window.localStorage.setItem(PINS_STORAGE_KEY, JSON.stringify(pins));
  } catch {
    // The browser copy is only a fallback when Studio does not answer.
  }
}

/**
 * The players kept on top of the ranking. Studio keeps the list, so an assistant reads the
 * same one; the browser copy covers a Studio that does not answer and moves old pins up once.
 */
export function usePinnedPlayers(): { pins: readonly PinnedPlayer[]; toggle: (pin: PinnedPlayer) => void } {
  const [pins, setPins] = useState<readonly PinnedPlayer[]>([]);
  const current = useRef<readonly PinnedPlayer[]>([]);

  const apply = useCallback((next: readonly PinnedPlayer[]) => {
    current.current = next;
    setPins(next);
    writeLocal(next);
  }, []);

  // Read after mount: the server render has no storage.
  useEffect(() => {
    let cancelled = false;
    getPinnedPlayers()
      .then(async (saved) => {
        if (cancelled) return;
        const local = savablePins(readLocal());
        if (saved.length === 0 && local.length > 0) {
          apply(local);
          await setPinnedPlayers(local);
        } else {
          apply(saved);
        }
      })
      .catch(() => {
        if (!cancelled && current.current.length === 0) apply(readLocal());
      });
    return () => {
      cancelled = true;
    };
  }, [apply]);

  const toggle = useCallback(
    (pin: PinnedPlayer) => {
      const next = togglePin(current.current, pin);
      apply(next);
      setPinnedPlayers(next).catch(() => {
        // The pins stay on this screen and in the browser copy until Studio answers again.
      });
    },
    [apply],
  );

  return { pins, toggle };
}
