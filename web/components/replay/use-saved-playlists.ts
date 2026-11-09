'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import type { ReplayRef } from '@/lib/analysis/replay-link';
import { isLibraryNotFound, listLibraryMatches } from '@/lib/api/library';
import {
  MAX_PLAYLIST_ROUND_NOTES,
  addPlaylistRounds,
  createPlaylist,
  getPlaylist,
  listPlaylists,
  patchPlaylistRound,
  removePlaylistRound,
  type PlaylistNote,
  type SavedPlaylist,
} from '@/lib/api/playlists';
import {
  DEFAULT_PLAYLIST_NAME,
  LEGACY_PLAYLIST_KEY,
  TARGET_PLAYLIST_KEY,
  editRoundNotes,
  holdsRound,
  legacyRoundsToSave,
  parseLegacyPlaylist,
  playlistRound,
  roundAnnotations,
  targetPlaylist,
} from '@/lib/replay/saved-playlist';
import type { AnnotationStore } from '@/components/tactical/spatial/annotation-panel';

/** Every part of the window that shows a playlist reads it again on this event. */
const PLAYLISTS_CHANGED = 'cliphub:playlists-changed';

export function announcePlaylistChange(): void {
  window.dispatchEvent(new Event(PLAYLISTS_CHANGED));
}

/** Runs `reload` whenever a playlist changes in this window. */
export function usePlaylistChanges(reload: () => void): void {
  useEffect(() => {
    window.addEventListener(PLAYLISTS_CHANGED, reload);
    return () => window.removeEventListener(PLAYLISTS_CHANGED, reload);
  }, [reload]);
}

function readStored(key: string): string | null {
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}

function writeStored(key: string, value: string | null): void {
  try {
    if (value === null) window.localStorage.removeItem(key);
    else window.localStorage.setItem(key, value);
  } catch {
    // Storage is blocked: B keeps adding to the newest playlist.
  }
}

/** Saves the playlist the Replay used to keep in the browser as a library one, once; answers its id. */
async function saveLegacyPlaylist(): Promise<string | null> {
  const raw = readStored(LEGACY_PLAYLIST_KEY);
  if (raw === null) return null;
  // Taken first, so a second Replay window does not save it again.
  writeStored(LEGACY_PLAYLIST_KEY, null);
  const refs = parseLegacyPlaylist(raw);
  if (refs.length === 0) return null;
  try {
    const matches = await listLibraryMatches();
    const rounds = legacyRoundsToSave(refs, new Map(matches.map((m) => [m.id, m.round_count])));
    if (rounds.length === 0) return null;
    return (await createPlaylist({ name: DEFAULT_PLAYLIST_NAME, rounds })).id;
  } catch {
    writeStored(LEGACY_PLAYLIST_KEY, raw);
    return null;
  }
}

export type SavedPlaylistsControl = {
  /** `null` while loading. */
  playlists: SavedPlaylist[] | null;
  /** The playlist B adds to. */
  target: SavedPlaylist | undefined;
  busy: boolean;
  error: string | null;
  /** B: puts the round in the target playlist or takes it out; with none, starts one. */
  toggle: (ref: ReplayRef) => void;
  /** Puts the round in one playlist or takes it out; that playlist becomes the target. */
  toggleIn: (id: string, ref: ReplayRef) => void;
  /** Starts a playlist with this round, which becomes the target; answers whether it was saved. */
  create: (input: NewPlaylist) => Promise<boolean>;
};

type NewPlaylist = { name: string; folder: string; author: string; ref: ReplayRef };

/** The saved playlists the Replay's round panel adds to; `enabled` is false where rounds are not library rounds. */
export function useSavedPlaylists(enabled: boolean): SavedPlaylistsControl {
  const [playlists, setPlaylists] = useState<SavedPlaylist[] | null>(null);
  const [picked, setPicked] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const reload = useCallback(() => setAttempt((n) => n + 1), []);
  usePlaylistChanges(reload);

  useEffect(() => {
    if (!enabled) return;
    let active = true;
    setPicked(readStored(TARGET_PLAYLIST_KEY));
    void (async () => {
      const saved = await saveLegacyPlaylist();
      if (saved !== null) {
        writeStored(TARGET_PLAYLIST_KEY, saved);
        if (active) setPicked(saved);
      }
      try {
        const list = await listPlaylists();
        if (active) setPlaylists(list);
      } catch {
        if (active) setPlaylists((current) => current ?? []);
      }
    })();
    return () => {
      active = false;
    };
  }, [enabled, attempt]);

  const save = useCallback(async (work: () => Promise<SavedPlaylist>): Promise<boolean> => {
    setBusy(true);
    setError(null);
    try {
      const saved = await work();
      setPlaylists((list) => [saved, ...(list ?? []).filter((p) => p.id !== saved.id)]);
      setPicked(saved.id);
      writeStored(TARGET_PLAYLIST_KEY, saved.id);
      announcePlaylistChange();
      return true;
    } catch {
      setError('No se pudo guardar la playlist. Comprueba que Studio sigue abierto.');
      return false;
    } finally {
      setBusy(false);
    }
  }, []);

  const toggleIn = useCallback(
    (id: string, ref: ReplayRef) => {
      const round = { match_id: ref.matchId, round: ref.round };
      void save(async () => {
        // The list may be a moment old: read the playlist itself before choosing add or remove.
        const current = await getPlaylist(id);
        return holdsRound(current, ref)
          ? removePlaylistRound({ playlistId: id, matchId: ref.matchId, round: ref.round })
          : addPlaylistRounds(id, { rounds: [round] });
      });
    },
    [save],
  );
  const create = useCallback(
    (input: NewPlaylist) =>
      save(() =>
        createPlaylist({ name: input.name, folder: input.folder, author: input.author, rounds: [{ match_id: input.ref.matchId, round: input.ref.round }] }),
      ),
    [save],
  );
  const target = playlists === null ? undefined : targetPlaylist(playlists, picked);
  const targetId = target?.id;
  const toggle = useCallback(
    (ref: ReplayRef) => {
      if (playlists === null || busy) return;
      if (targetId !== undefined) toggleIn(targetId, ref);
      else void create({ name: DEFAULT_PLAYLIST_NAME, folder: '', author: '', ref });
    },
    [busy, create, playlists, targetId, toggleIn],
  );
  return { playlists, target, busy, error, toggle, toggleIn, create };
}

/** One saved playlist, read again whenever a playlist changes in this window. `undefined` while loading, `null` when it is gone. */
export function useOpenPlaylist(id: string | null): { playlist: SavedPlaylist | null | undefined; replace: (next: SavedPlaylist) => void } {
  const [playlist, setPlaylist] = useState<SavedPlaylist | null | undefined>(undefined);
  const [attempt, setAttempt] = useState(0);
  const reload = useCallback(() => setAttempt((n) => n + 1), []);
  usePlaylistChanges(reload);
  useEffect(() => {
    if (id === null) return;
    let active = true;
    getPlaylist(id)
      .then((next) => {
        if (active) setPlaylist(next);
      })
      .catch((error: unknown) => {
        if (!active) return;
        // A playlist deleted elsewhere closes; a failed read keeps what is on screen.
        if (isLibraryNotFound(error)) setPlaylist(null);
        else setPlaylist((current) => (current === undefined ? null : current));
      });
    return () => {
      active = false;
    };
  }, [id, attempt]);
  return { playlist: id === null ? null : playlist, replace: setPlaylist };
}

/**
 * The notes and drawings of the open playlist's round `at`, kept with that
 * round instead of its match, as the Replay's notes panel reads them.
 */
export function usePlaylistRoundNotes(input: {
  playlist: SavedPlaylist | null | undefined;
  at: ReplayRef | undefined;
  onSaved: (next: SavedPlaylist) => void;
}): AnnotationStore {
  const { playlist, at, onSaved } = input;
  const [error, setError] = useState<string | null>(null);
  const round = playlist && at ? playlistRound(playlist, at) : undefined;
  const notes = useMemo(() => (round === undefined ? null : roundAnnotations(round)), [round]);

  const write = useCallback(
    async (edit: (held: PlaylistNote[]) => PlaylistNote[] | string, failure: string): Promise<boolean> => {
      if (!playlist || round === undefined) return false;
      const next = edit(round.notes ?? []);
      if (typeof next === 'string') {
        setError(next);
        return false;
      }
      try {
        onSaved(await patchPlaylistRound({ playlistId: playlist.id, matchId: round.match_id, round: round.round }, { notes: next }));
        announcePlaylistChange();
        setError(null);
        return true;
      } catch {
        setError(failure);
        return false;
      }
    },
    [onSaved, playlist, round],
  );

  return {
    notes,
    error,
    create: (note) =>
      write(
        (held) =>
          held.length >= MAX_PLAYLIST_ROUND_NOTES
            ? `Una ronda de la playlist guarda hasta ${MAX_PLAYLIST_ROUND_NOTES} notas. Borra una para guardar otra.`
            : editRoundNotes(held, { add: { tick: note.tick, text: note.text, strokes: note.strokes } }),
        'No se pudo guardar la nota en la playlist.',
      ),
    update: (id, text) => write((held) => editRoundNotes(held, { id, text }), 'No se pudo editar la nota.'),
    remove: async (id) => {
      await write((held) => editRoundNotes(held, { remove: id }), 'No se pudo borrar la nota.');
    },
  };
}
