'use client';

import { useCallback, useEffect, useState } from 'react';
import type { ReplayRef } from '@/lib/analysis/replay-link';
import { PLAYLIST_KEY, parsePlaylist, togglePlaylistRound } from '@/lib/replay/saved-playlist';

function read(): ReplayRef[] {
  try {
    return parsePlaylist(window.localStorage.getItem(PLAYLIST_KEY));
  } catch {
    return [];
  }
}

/** The playlist the Replay fills with B, shared by every Replay open on this PC. */
export function useSavedPlaylist(): { refs: readonly ReplayRef[]; toggle: (ref: ReplayRef) => void; clear: () => void } {
  const [refs, setRefs] = useState<readonly ReplayRef[]>([]);
  useEffect(() => {
    setRefs(read());
    const sync = (event: StorageEvent): void => {
      if (event.key === PLAYLIST_KEY) setRefs(read());
    };
    window.addEventListener('storage', sync);
    return () => window.removeEventListener('storage', sync);
  }, []);
  const write = useCallback((next: readonly ReplayRef[]) => {
    setRefs(next);
    try {
      window.localStorage.setItem(PLAYLIST_KEY, JSON.stringify(next));
    } catch {
      // Storage is blocked: the playlist lasts for this visit.
    }
  }, []);
  // Reads the store again first, so two Replay windows do not overwrite each other.
  const toggle = useCallback((ref: ReplayRef) => write(togglePlaylistRound(read(), ref)), [write]);
  const clear = useCallback(() => write([]), [write]);
  return { refs, toggle, clear };
}
