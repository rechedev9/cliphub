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

function dropLocal(): void {
  try {
    window.localStorage.removeItem(PINS_STORAGE_KEY);
  } catch {
    // Nothing to clean.
  }
}

type Sync = {
  /** loading: the first read has not answered; ready: Studio's list is on screen; offline: Studio did not answer. */
  status: 'loading' | 'ready' | 'offline';
  pins: readonly PinnedPlayer[];
  /** Toggles made before the first read answered, applied on top of Studio's list. */
  queued: PinnedPlayer[];
  /** Counts toggles, so a read that started before one never overwrites it. */
  edits: number;
  saving: boolean;
  latest: readonly PinnedPlayer[] | null;
};

/**
 * The players kept on top of the ranking. Studio keeps the list, so an assistant reads the same
 * one: it is read again when the window comes back, saves go one at a time, and pins in the
 * browser from before are moved up once and then removed.
 */
export function usePinnedPlayers(): { pins: readonly PinnedPlayer[]; toggle: (pin: PinnedPlayer) => void } {
  const [pins, setPins] = useState<readonly PinnedPlayer[]>([]);
  const sync = useRef<Sync>({ status: 'loading', pins: [], queued: [], edits: 0, saving: false, latest: null });

  const show = useCallback((next: readonly PinnedPlayer[]) => {
    sync.current.pins = next;
    setPins(next);
  }, []);

  const save = useCallback(async (next: readonly PinnedPlayer[]) => {
    const state = sync.current;
    state.latest = next;
    if (state.saving) return;
    state.saving = true;
    try {
      while (state.latest !== null) {
        const batch = state.latest;
        state.latest = null;
        await setPinnedPlayers(batch);
        if (state.latest === null) dropLocal();
      }
    } catch {
      // The pins stay on this screen and in the browser copy until the next read.
      state.latest = null;
    } finally {
      state.saving = false;
    }
  }, []);

  const read = useCallback(
    async (first: boolean) => {
      const state = sync.current;
      const edits = state.edits;
      try {
        const saved = await getPinnedPlayers();
        if (state.saving || state.latest !== null) return;
        const local = first ? savablePins(readLocal()) : [];
        if (state.edits === edits || first) {
          const legacy = saved.length === 0 && local.length > 0;
          const base = legacy ? local : saved;
          const next = state.queued.reduce((list, pin) => togglePin(list, pin), base);
          state.queued = [];
          state.status = 'ready';
          show(next);
          if (legacy || next.length !== base.length || next.some((pin, i) => base[i]?.steamid64 !== pin.steamid64)) void save(next);
          else dropLocal();
        }
      } catch {
        if (first && state.status === 'loading') {
          state.status = 'offline';
          show(state.queued.reduce((list, pin) => togglePin(list, pin), readLocal()));
          state.queued = [];
        }
      }
    },
    [save, show],
  );

  useEffect(() => {
    void read(true);
    const again = (): void => {
      if (document.visibilityState === 'visible' && sync.current.status !== 'loading') void read(false);
    };
    window.addEventListener('focus', again);
    document.addEventListener('visibilitychange', again);
    return () => {
      window.removeEventListener('focus', again);
      document.removeEventListener('visibilitychange', again);
    };
  }, [read]);

  const toggle = useCallback(
    (pin: PinnedPlayer) => {
      const state = sync.current;
      state.edits += 1;
      if (state.status === 'loading') {
        state.queued = [...state.queued, pin];
        show(togglePin(state.pins, pin));
        return;
      }
      const next = togglePin(state.pins, pin);
      show(next);
      if (state.status === 'offline') writeLocal(next);
      void save(next);
    },
    [save, show],
  );

  return { pins, toggle };
}
