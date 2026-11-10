'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { ListOrdered } from 'lucide-react';
import { toast } from 'sonner';
import { isLibraryOffline } from '@/lib/api/library';
import { matchPatterns, savedPatternRounds, type MatchPatternHit, type MatchPatternsResponse } from '@/lib/api/patterns';
import { patternReplayRefs } from '@/lib/analysis/pattern-finder';
import { replayHref } from '@/lib/analysis/replay-link';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';

type Load = { matchId: string; data: MatchPatternsResponse | null };

/** The saved patterns the current round meets, from the stored detection; each opens all its rounds. */
export function RoundPatterns({ matchId, round }: { matchId: string; round: number }): ReactNode {
  const router = useRouter();
  const [load, setLoad] = useState<Load | null>(null);
  const [opening, setOpening] = useState<string | null>(null);
  useEffect(() => {
    let live = true;
    matchPatterns(matchId)
      .then((data) => {
        if (live) setLoad({ matchId, data });
      })
      .catch(() => {
        if (live) setLoad({ matchId, data: null });
      });
    return () => {
      live = false;
    };
  }, [matchId]);

  const data = load?.matchId === matchId ? load.data : null;
  if (data === null || data.total === 0) return null;
  const hits = data.patterns.filter((hit) => hit.rounds.some((entry) => entry.round === round));

  async function open(hit: MatchPatternHit): Promise<void> {
    setOpening(hit.pattern.id);
    try {
      const run = await savedPatternRounds(hit.pattern.id);
      const refs = patternReplayRefs(run.result);
      if (refs.length === 0) toast.info(`«${hit.pattern.name}» no tiene más rondas.`);
      else router.push(replayHref(refs, hit.pattern.name));
    } catch (error) {
      toast.error(isLibraryOffline(error) ? 'El servicio local no responde.' : `No se pudieron leer las rondas de «${hit.pattern.name}».`);
    } finally {
      setOpening(null);
    }
  }

  return (
    <section aria-label="Patrones de la ronda" className="flex flex-col gap-1.5 border-t border-border-subtle pt-2">
      <h3 className="flex items-baseline justify-between gap-2 px-1 text-meta font-semibold text-fg-3">
        Patrones
        <span className="font-normal tabular-nums">
          {hits.length} de {data.total}
        </span>
      </h3>
      {hits.length === 0 ? <p className="px-1 text-body-sm text-fg-3">Esta ronda no cumple ningún patrón guardado.</p> : null}
      <ul className="flex flex-col gap-1">
        {hits.map((hit) => (
          <li key={`${hit.pattern.id}:${hit.team_key}`} className="flex items-center gap-2 rounded-sm bg-surface-2 px-2 py-1.5">
            <span className="flex min-w-0 flex-1 flex-col">
              <span className="flex min-w-0 items-center gap-1.5">
                <span className="truncate text-body-sm font-medium text-fg-1">{hit.pattern.name}</span>
                {hit.pattern.category === '' ? null : <Badge variant="secondary">{hit.pattern.category}</Badge>}
              </span>
              {hit.team === undefined || hit.team === '' ? null : <span className="truncate text-meta text-fg-3">{hit.team}</span>}
            </span>
            <Button
              type="button"
              variant="ghost"
              size="icon-xs"
              disabled={opening !== null}
              aria-label={`Abrir las rondas de ${hit.pattern.name}`}
              title="Abrir sus rondas"
              onClick={() => void open(hit)}
            >
              <ListOrdered aria-hidden />
            </Button>
          </li>
        ))}
      </ul>
    </section>
  );
}
