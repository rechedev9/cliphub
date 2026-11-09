'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { ListVideo, Radar } from 'lucide-react';
import { listLibraryMatches } from '@/lib/api/library';
import type { LibraryMatch } from '@/lib/api/library';
import { MATCHES_HREF, matchHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { matchScoreLabel, matchTitle } from '@/lib/library/partidas';
import { MATCH_REPLAY_TAB, MATCH_TAB_QUERY } from '@/lib/library/rounds';
import { PLAYLISTS_HREF } from '@/lib/replay/saved-playlist';
import { EmptyState } from '@/components/records/empty-state';
import { Button } from '@/components/ui/button';

/** How many recent partidas the Replay offers to open before sending the user to the library. */
const RECENT_LIMIT = 12;

function replayHref(match: LibraryMatch): string {
  return `${matchHref(match.id)}?${MATCH_TAB_QUERY}=${MATCH_REPLAY_TAB}`;
}

function byNewest(a: LibraryMatch, b: LibraryMatch): number {
  return b.played_at.localeCompare(a.played_at);
}

/** The Replay with nothing chosen yet: the latest partidas, each one click from its rounds. */
export function ReplayHome(): ReactNode {
  // `undefined` while loading, `null` when the library could not be read.
  const [matches, setMatches] = useState<LibraryMatch[] | null | undefined>(undefined);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    listLibraryMatches()
      .then((list) => {
        if (active) setMatches([...list].sort(byNewest));
      })
      .catch(() => {
        if (active) setMatches(null);
      });
    return () => {
      active = false;
    };
  }, [attempt]);

  if (matches === undefined) return <p className="text-body-sm text-fg-3">Leyendo tus partidas…</p>;
  if (matches === null) {
    return (
      <EmptyState
        icon={Radar}
        title="No se pudo leer la biblioteca"
        description="Studio no ha podido leer tus partidas. Vuelve a intentarlo."
        actions={
          <Button size="sm" onClick={() => setAttempt((n) => n + 1)}>
            Reintentar
          </Button>
        }
      />
    );
  }
  if (matches.length === 0) {
    return (
      <EmptyState
        icon={Radar}
        title="Aún no hay partidas que reproducir"
        description="Importa una demo en Partidas y sus rondas aparecerán aquí."
        actions={
          <Button asChild size="sm">
            <Link href={MATCHES_HREF}>Ir a Partidas</Link>
          </Button>
        }
      />
    );
  }

  const recent = matches.slice(0, RECENT_LIMIT);
  return (
    <section className="flex flex-col gap-3" aria-labelledby="replay-home-title">
      <div className="flex items-baseline justify-between gap-3">
        <h2 id="replay-home-title" className="text-title text-fg-1">
          Elige una partida
        </h2>
        <span className="flex items-baseline gap-3">
          {matches.length > recent.length ? (
            <Link href={MATCHES_HREF} className="text-body-sm text-fg-2 underline decoration-fg-4 underline-offset-2 hover:text-fg-1">
              Ver las {matches.length} partidas
            </Link>
          ) : null}
          <Button asChild variant="outline" size="sm">
            <Link href={PLAYLISTS_HREF}>
              <ListVideo aria-hidden />
              Playlists
            </Link>
          </Button>
        </span>
      </div>
      <ul className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
        {recent.map((match) => (
          <li key={match.id}>
            <Link
              href={replayHref(match)}
              className="flex flex-col gap-1 rounded-md border border-border bg-surface-2 px-3 py-2.5 hover:border-border-strong hover:bg-surface-3"
            >
              <span className="flex items-baseline justify-between gap-2">
                <span className="truncate text-body font-medium text-fg-1">{prettyMapName(match.map)}</span>
                <span className="font-mono text-body-sm text-fg-2">{matchScoreLabel(match)}</span>
              </span>
              <span className="truncate text-body-sm text-fg-2">{matchTitle(match)}</span>
              <span className="text-meta text-fg-3">
                {formatShortDate(match.played_at)} · {match.round_count} {match.round_count === 1 ? 'ronda' : 'rondas'}
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}
