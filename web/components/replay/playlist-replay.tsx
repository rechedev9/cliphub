'use client';

import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { ListVideo } from 'lucide-react';
import { parseReplayRefs } from '@/lib/analysis/replay-link';
import { listLibraryMatches } from '@/lib/api/library';
import type { LibraryMatch } from '@/lib/api/library';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { matchTitle } from '@/lib/library/partidas';
import { teamFocusFromQuery } from '@/lib/replay/player-focus';
import { entryKey, indexOfKey } from '@/lib/replay/playlist';
import { PLAYLISTS_HREF, PLAYLIST_QUERY, isPlaylistId, playlistRefs } from '@/lib/replay/saved-playlist';
import { isWholeRound, tramoFromQuery, writeTramo } from '@/lib/replay/tramo';
import type { Tramo } from '@/lib/replay/tramo';
import { zonesFromQuery } from '@/lib/replay/zone-marks';
import { EmptyState } from '@/components/records/empty-state';
import { useDocumentTitle, useRouteTitle } from '@/components/shell/route-title';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { ReplayWorkspace } from './replay-workspace';
import { PlaylistRoundCard } from './playlist-round-card';
import { TeamReplayHome } from './team-replay-home';
import { useOpenPlaylist } from './use-saved-playlists';

function matchLabel(match: LibraryMatch): string {
  return `${prettyMapName(match.map)} · ${matchTitle(match)}`;
}

/** The library's matches by id, for the names of the rounds; `null` while loading. */
function useLibraryMatches(): ReadonlyMap<string, LibraryMatch> | null {
  const [matches, setMatches] = useState<ReadonlyMap<string, LibraryMatch> | null>(null);
  useEffect(() => {
    let active = true;
    listLibraryMatches()
      .then((list) => {
        if (active) setMatches(new Map(list.map((match) => [match.id, match])));
      })
      .catch(() => {
        // Without the list the strip still plays; it only loses the match names until each loads.
        if (active) setMatches(new Map());
      });
    return () => {
      active = false;
    };
  }, []);
  return matches;
}

/** The day each library match was played, once the list loads. */
function useMatchDate(matches: ReadonlyMap<string, LibraryMatch> | null): (matchId: string) => string | null {
  return useCallback(
    (matchId: string) => {
      const match = matches?.get(matchId);
      return match === undefined ? null : formatShortDate(match.played_at);
    },
    [matches],
  );
}

/**
 * The Replay of a list of rounds from any matches: a saved playlist (`?p=`)
 * with its comment card, or a passing list from `replayHref`. Each match's analysis
 * loads when the strip reaches it.
 */
export function PlaylistReplay(): ReactNode {
  const search = useSearchParams();
  const saved = search.get(PLAYLIST_QUERY);
  if (isPlaylistId(saved)) return <SavedPlaylistReplay key={saved} id={saved} />;
  return <PassingPlaylistReplay />;
}

/** A saved playlist for the team meeting: J and K walk its rounds, each with its comment. */
function SavedPlaylistReplay({ id }: { id: string }): ReactNode {
  const { playlist, replace } = useOpenPlaylist(id);
  const matches = useLibraryMatches();
  const [currentKey, setCurrentKey] = useState<string | null>(null);
  const entries = useMemo(() => (playlist ? playlistRefs(playlist) : []), [playlist]);
  const index = indexOfKey(entries, currentKey);
  const entry = entries[index];
  const name = playlist?.name ?? 'Playlist';
  useRouteTitle(name);
  useDocumentTitle(`${name} · ClipHub`);
  const groupLabel = useCallback(
    (matchId: string) => {
      const match = matches?.get(matchId);
      return match === undefined ? null : `${matchLabel(match)} · ${formatShortDate(match.played_at)}`;
    },
    [matches],
  );
  const matchDate = useMatchDate(matches);

  if (playlist === undefined) return <Skeleton className="h-[60vh] w-full rounded-lg" />;
  if (playlist === null) {
    return (
      <EmptyState
        icon={ListVideo}
        title="Esta playlist ya no está"
        description="Se ha borrado o Studio no ha podido leerla. Tus otras playlists siguen en su lista."
        actions={
          <Button asChild size="sm">
            <Link href={PLAYLISTS_HREF}>Ver las playlists</Link>
          </Button>
        }
      />
    );
  }

  if (entries.length === 0) {
    const empty =
      playlist.rounds.length === 0
        ? 'Esta playlist aún no tiene rondas. Abre una partida en el Replay y pulsa B o el marcador de la ronda que quieras.'
        : 'Ninguna ronda de esta playlist está en tu biblioteca. Importa sus demos en Partidas y las rondas aparecerán solas.';
    return (
      <section className="studio-panel flex flex-col gap-3 p-4 sm:p-5" aria-label="Repetición">
        <h3 className="truncate font-display text-body font-bold text-fg-1" title={playlist.name}>
          {playlist.name}
        </h3>
        <p className="text-body text-fg-2">{empty}</p>
        <Link href={PLAYLISTS_HREF} className="text-body-sm text-fg-2 underline decoration-fg-4 underline-offset-2 hover:text-fg-1">
          Todas las playlists
        </Link>
      </section>
    );
  }
  return (
    <ReplayWorkspace
      source="library"
      entries={entries}
      index={index}
      onIndexChange={(next) => setCurrentKey(entries[next] === undefined ? null : entryKey(entries[next]))}
      groupLabel={groupLabel}
      matchDate={matchDate}
      title={playlist.name}
      back={{ href: PLAYLISTS_HREF, label: 'Volver' }}
      openPlaylistId={playlist.id}
      roundCard={<PlaylistRoundCard playlist={playlist} current={entry} onChange={replace} />}
    />
  );
}

/** A list of rounds that is not saved, such as the rounds behind a figure. */
function PassingPlaylistReplay(): ReactNode {
  const search = useSearchParams();
  const refs = useMemo(() => parseReplayRefs(search), [search]);
  const title = search.get('t') ?? undefined;
  const focusPlayers = useMemo(() => teamFocusFromQuery(search), [search]);
  const router = useRouter();
  const pathname = usePathname();
  // A finding's link names its zones and its tramo (its players come in `jugador`); the tramo stays in the URL as the user moves it.
  const zones = useMemo(() => zonesFromQuery(search), [search]);
  const tramo = useMemo(() => tramoFromQuery(search), [search]);
  // `s=1` opens every round superposed, as a grenade's Reproducir does, and so does a link with a tramo.
  const [tramoLink] = useState(() => !isWholeRound(tramoFromQuery(search)));
  const superpose = search.get('s') === '1' || tramoLink;
  const setTramo = useCallback(
    (next: Tramo) => router.replace(`${pathname}?${writeTramo(search.toString(), next)}`, { scroll: false }),
    [pathname, router, search],
  );
  const matches = useLibraryMatches();
  const [currentKey, setCurrentKey] = useState<string | null>(null);
  // With no rounds the page is the team Replay, which names the tab itself.
  const tabTitle = refs.length === 0 ? undefined : (title ?? 'Replay');
  useRouteTitle(tabTitle);
  useDocumentTitle(tabTitle === undefined ? undefined : `${tabTitle} · ClipHub`);

  const matchIds = useMemo(() => [...new Set(refs.map((ref) => ref.matchId))], [refs]);
  const missing = useMemo(
    () => new Set(matches === null || matches.size === 0 ? [] : matchIds.filter((id) => !matches.has(id))),
    [matchIds, matches],
  );
  const entries = useMemo(() => refs.filter((ref) => !missing.has(ref.matchId)), [missing, refs]);
  const index = indexOfKey(entries, currentKey);
  const groupLabel = useCallback((matchId: string) => {
    const match = matches?.get(matchId);
    return match === undefined ? null : `${matchLabel(match)} · ${formatShortDate(match.played_at)}`;
  }, [matches]);
  const matchDate = useMatchDate(matches);

  if (refs.length === 0) return <TeamReplayHome />;

  return (
    <ReplayWorkspace
      source="library"
      entries={entries}
      index={index}
      onIndexChange={(next) => setCurrentKey(entries[next] === undefined ? null : entryKey(entries[next]))}
      groupLabel={groupLabel}
      matchDate={matchDate}
      title={title}
      focusPlayers={focusPlayers}
      tramo={{ tramo, onChange: setTramo }}
      zones={zones}
      superpose={superpose}
    />
  );
}
