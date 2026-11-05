'use client';

import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { ListVideo } from 'lucide-react';
import { parseReplayRefs } from '@/lib/analysis/replay-link';
import { listLibraryMatches } from '@/lib/api/library';
import type { LibraryMatch } from '@/lib/api/library';
import { COLLECTIONS_HREF } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import { matchTitle } from '@/lib/library/partidas';
import { entryKey, indexOfKey } from '@/lib/replay/playlist';
import { EmptyState } from '@/components/records/empty-state';
import { useDocumentTitle, useRouteTitle } from '@/components/shell/route-title';
import { Button } from '@/components/ui/button';
import { ReplayWorkspace } from './replay-workspace';

function matchLabel(match: LibraryMatch): string {
  return `${prettyMapName(match.map)} · ${matchTitle(match)}`;
}

/**
 * A playlist of rounds from any matches, from `replayHref`. Each match's
 * analysis loads when the strip reaches it; the left panel narrows the list to
 * some of its matches.
 */
export function PlaylistReplay(): ReactNode {
  const search = useSearchParams();
  const refs = useMemo(() => parseReplayRefs(search), [search]);
  const title = search.get('t') ?? undefined;
  const [matches, setMatches] = useState<ReadonlyMap<string, LibraryMatch> | null>(null);
  const [hidden, setHidden] = useState<ReadonlySet<string>>(new Set());
  const [currentKey, setCurrentKey] = useState<string | null>(null);
  useRouteTitle(title ?? 'Repetición');
  useDocumentTitle(`${title ?? 'Repetición'} · ClipHub`);

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

  const matchIds = useMemo(() => [...new Set(refs.map((ref) => ref.matchId))], [refs]);
  const missing = useMemo(
    () => new Set(matches === null || matches.size === 0 ? [] : matchIds.filter((id) => !matches.has(id))),
    [matchIds, matches],
  );
  const entries = useMemo(
    () => refs.filter((ref) => !hidden.has(ref.matchId) && !missing.has(ref.matchId)),
    [hidden, missing, refs],
  );
  const index = indexOfKey(entries, currentKey);
  const groupLabel = useCallback((matchId: string) => {
    const match = matches?.get(matchId);
    return match === undefined ? null : matchLabel(match);
  }, [matches]);

  if (refs.length === 0) {
    return (
      <EmptyState
        icon={ListVideo}
        title="Esta lista no tiene rondas"
        description="Abre una colección o un informe y pulsa Reproducir para ver sus rondas una tras otra."
        actions={
          <Button asChild size="sm">
            <Link href={COLLECTIONS_HREF}>Ver colecciones</Link>
          </Button>
        }
      />
    );
  }

  const filters = matchIds.length < 2 && missing.size === 0 ? undefined : (
    <section className="flex flex-col gap-2" aria-label="Partidas de la lista">
      <h3 className="text-meta text-fg-3">Partidas</h3>
      <ul className="flex flex-col gap-1">
        {matchIds.map((id) => {
          const match = matches?.get(id);
          const count = refs.filter((ref) => ref.matchId === id).length;
          const gone = missing.has(id);
          return (
            <li key={id}>
              <label className="flex items-start gap-2 py-1 text-body-sm text-fg-1">
                <input
                  type="checkbox"
                  className="mt-1 accent-primary"
                  checked={!gone && !hidden.has(id)}
                  disabled={gone}
                  onChange={() =>
                    setHidden((current) => {
                      const next = new Set(current);
                      if (next.has(id)) next.delete(id);
                      else next.add(id);
                      return next;
                    })
                  }
                />
                <span className="flex min-w-0 flex-col">
                  <span className="truncate">{match === undefined ? 'Partida' : matchLabel(match)}</span>
                  <span className="text-meta text-fg-3">
                    {gone ? 'Ya no está en la biblioteca' : `${count} ${count === 1 ? 'ronda' : 'rondas'}`}
                  </span>
                </span>
              </label>
            </li>
          );
        })}
      </ul>
    </section>
  );

  return (
    <ReplayWorkspace
      source="library"
      entries={entries}
      index={index}
      onIndexChange={(next) => setCurrentKey(entries[next] === undefined ? null : entryKey(entries[next]))}
      groupLabel={groupLabel}
      filters={filters}
      title={title}
    />
  );
}
