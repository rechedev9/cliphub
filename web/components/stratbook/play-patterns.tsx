'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { ListOrdered, Search, Unlink } from 'lucide-react';
import { toast } from 'sonner';
import { isLibraryOffline } from '@/lib/api/library';
import { listSavedPatterns, savedPatternRounds, updateSavedPattern, type SavedPattern } from '@/lib/api/patterns';
import type { LibraryPlay } from '@/lib/api/stratbook';
import { PATTERN_QUERY, patternReplayRefs } from '@/lib/analysis/pattern-finder';
import { replayHref } from '@/lib/analysis/replay-link';
import { SCOPE_PARAMS } from '@/lib/analysis/scope';
import { teamHref } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

const PICK = 'pick';

/** The team's Replay > Patrones with the pattern and its scope, as `studio_path` opens it. */
function finderHref(pattern: SavedPattern, teamId: string): string {
  const params = new URLSearchParams({ tab: 'replay', forma: 'patrones', [SCOPE_PARAMS.map]: pattern.map });
  if (pattern.scope.buy !== undefined) params.set(SCOPE_PARAMS.buy, pattern.scope.buy);
  if (pattern.scope.opponent_buy !== undefined) params.set(SCOPE_PARAMS.opponentBuy, pattern.scope.opponent_buy);
  if (pattern.scope.matches !== undefined) params.set(SCOPE_PARAMS.limit, String(pattern.scope.matches));
  params.set(PATTERN_QUERY, JSON.stringify(pattern.pattern));
  return `${teamHref(teamId)}?${params.toString()}`;
}

/** The saved patterns linked to a play: each opens its rounds or the finder, and one of the map's can be linked. */
export function PlayPatterns({ play }: { play: LibraryPlay }): ReactNode {
  const router = useRouter();
  const [linked, setLinked] = useState<SavedPattern[] | null>(null);
  const [mapPatterns, setMapPatterns] = useState<SavedPattern[]>([]);
  const [version, setVersion] = useState(0);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    let live = true;
    Promise.all([listSavedPatterns({ playId: play.id }), listSavedPatterns({ map: play.map })])
      .then(([own, all]) => {
        if (!live) return;
        setLinked(own);
        setMapPatterns(all);
      })
      .catch(() => {
        if (live) setLinked([]);
      });
    return () => {
      live = false;
    };
  }, [play.id, play.map, version]);

  const link = useCallback(
    async (pattern: SavedPattern, on: boolean) => {
      setBusy(true);
      try {
        await updateSavedPattern(pattern.id, on ? { play_id: play.id } : { clear_play: true });
        setVersion((n) => n + 1);
      } catch (error) {
        toast.error(isLibraryOffline(error) ? 'El servicio local no responde.' : `No se pudo cambiar «${pattern.name}».`);
      } finally {
        setBusy(false);
      }
    },
    [play.id],
  );
  const openRounds = async (pattern: SavedPattern): Promise<void> => {
    try {
      const refs = patternReplayRefs((await savedPatternRounds(pattern.id)).result);
      if (refs.length === 0) toast.info(`«${pattern.name}» no tiene rondas todavía.`);
      else router.push(replayHref(refs, pattern.name));
    } catch (error) {
      toast.error(isLibraryOffline(error) ? 'El servicio local no responde.' : `No se pudieron leer las rondas de «${pattern.name}».`);
    }
  };

  if (linked === null) return null;
  const free = mapPatterns.filter((entry) => entry.play_id !== play.id);
  return (
    <section aria-labelledby="play-patterns" className="flex flex-col gap-2">
      <h2 id="play-patterns" className="text-body font-semibold text-fg-1">
        Patrones
      </h2>
      {linked.length === 0 ? <p className="text-body-sm text-fg-2">Ningún patrón guardado apunta a esta jugada.</p> : null}
      <ul className="flex flex-col gap-1.5">
        {linked.map((pattern) => {
          const teamId = pattern.team_id ?? play.team_id ?? undefined;
          return (
            <li key={pattern.id} className="flex items-center gap-2 rounded-md border border-border bg-surface-1 px-2.5 py-2">
              <span className="flex min-w-0 flex-1 items-center gap-1.5">
                <span className="truncate text-body-sm font-medium text-fg-1">{pattern.name}</span>
                {pattern.category === '' ? null : <Badge variant="secondary">{pattern.category}</Badge>}
              </span>
              <Button type="button" variant="ghost" size="icon-xs" aria-label={`Abrir las rondas de ${pattern.name}`} title="Abrir sus rondas" onClick={() => void openRounds(pattern)}>
                <ListOrdered aria-hidden />
              </Button>
              {teamId === undefined ? null : (
                <Button asChild variant="ghost" size="icon-xs">
                  <Link href={finderHref(pattern, teamId)} aria-label={`Abrir ${pattern.name} en el buscador de patrones`} title="Abrir en el buscador">
                    <Search aria-hidden />
                  </Link>
                </Button>
              )}
              <Button type="button" variant="ghost" size="icon-xs" disabled={busy} aria-label={`Desvincular ${pattern.name}`} title="Desvincular" onClick={() => void link(pattern, false)}>
                <Unlink aria-hidden />
              </Button>
            </li>
          );
        })}
      </ul>
      {free.length > 0 ? (
        <Select
          value={PICK}
          disabled={busy}
          onValueChange={(id) => {
            const pattern = free.find((entry) => entry.id === id);
            if (pattern !== undefined) void link(pattern, true);
          }}
        >
          <SelectTrigger aria-label="Vincular un patrón guardado" className="w-fit">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={PICK} disabled>
              Vincular un patrón de {prettyMapName(play.map)}
            </SelectItem>
            {free.map((pattern) => (
              <SelectItem key={pattern.id} value={pattern.id}>
                {pattern.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ) : null}
    </section>
  );
}
