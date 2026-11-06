'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { getTeamPlaybook } from '@/lib/api/library';
import type { TeamPlaybook as Playbook } from '@/lib/api/playbook';
import { scopeDateRange, type AnalysisScope } from '@/lib/analysis/scope';
import { prettyMapName } from '@/lib/format';
import { poolCategories } from '@/lib/analysis/library-pool';
import { useLibraryPool } from '@/hooks/use-library-pool';
import type { ScopeCounts } from '@/components/analysis/scope-bar';
import { Skeleton } from '@/components/ui/skeleton';
import { CategoryGrid, SiteBars, SkippedNote, TempoChart } from './playbook-summary';
import { ZoneExplorer } from './zone-explorer';

function Section({ title, children, className }: { title: string; children: ReactNode; className?: string }): ReactNode {
  return (
    <section aria-label={title} className={className ?? 'studio-panel flex flex-col gap-3 p-4'}>
      <h2 className="text-body font-semibold text-fg-1">{title}</h2>
      {children}
    </section>
  );
}

function missingText(scope: AnalysisScope): string {
  if (scope.map === '' && scope.side === '') return 'Elige arriba un mapa y un lado, CT o T, para ver su playbook.';
  if (scope.map === '') return 'Elige arriba un mapa: el playbook se lee mapa a mapa.';
  return `Elige arriba el lado, CT o T, para ver cómo juegan ${prettyMapName(scope.map)}.`;
}

/** The team's playbook on one side of one map: round types, tempo, sites and the zones it controls. */
export function TeamPlaybook({
  teamId,
  scope,
  onCounts,
}: {
  teamId: string;
  scope: AnalysisScope;
  onCounts: (counts: ScopeCounts | null) => void;
}): ReactNode {
  const [playbook, setPlaybook] = useState<Playbook | null>(null);
  const [failed, setFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [now] = useState(() => Date.now());
  const { map, side, opponent } = scope;
  const { from, to } = scopeDateRange(scope.period, now);
  const ready = map !== '' && side !== '';
  const readPool = useCallback(
    (id: string) => (ready ? getTeamPlaybook(id, { map, side }) : Promise.reject(new Error('Sin mapa o lado'))),
    [ready, map, side],
  );
  const pool = useLibraryPool(`playbook|${map}|${side}`, readPool);

  useEffect(() => {
    if (map === '' || side === '') return undefined;
    let active = true;
    setPlaybook(null);
    setFailed(false);
    onCounts(null);
    getTeamPlaybook(teamId, { map, side, from, to, opponent_ids: opponent === '' ? [] : [opponent] })
      .then((next) => {
        if (!active) return;
        setPlaybook(next);
        onCounts({ maps: next.matches, rounds: next.round_count });
      })
      .catch(() => {
        if (active) setFailed(true);
      });
    return () => {
      active = false;
    };
  }, [teamId, map, side, opponent, from, to, attempt, onCounts]);

  if (!ready) return <p className="text-body text-fg-2">{missingText(scope)}</p>;
  if (failed) {
    return (
      <p role="alert" className="text-body-sm text-destructive">
        No se pudo leer el playbook. Comprueba que ClipHub Studio sigue abierto y{' '}
        <button type="button" className="underline underline-offset-4" onClick={() => setAttempt((n) => n + 1)}>
          vuelve a intentarlo
        </button>
        .
      </p>
    );
  }
  if (playbook === null) {
    return (
      <div role="status" aria-label="Leyendo el playbook" className="flex flex-col gap-4">
        <Skeleton className="h-36 w-full" />
        <div className="grid gap-4 lg:grid-cols-2">
          <Skeleton className="h-52 w-full" />
          <Skeleton className="h-52 w-full" />
        </div>
        <Skeleton className="h-80 w-full" />
      </div>
    );
  }
  if (playbook.round_count === 0) {
    return (
      <p className="text-body text-fg-2">
        No hay rondas de {side} en {prettyMapName(map)} con este filtro. Prueba con otro periodo o rival.
      </p>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <Section title="Tipos de ronda" className="flex flex-col gap-3">
        <p className="text-body-sm text-fg-2">
          Gana el {Math.round(playbook.win_rate.pct)} % de sus {playbook.round_count} rondas. Pulsa un tipo para ver sus rondas. ▲▼ frente
          a {pool === null ? 'la biblioteca' : `los ${pool.filter((entry) => entry.round_count > 0).length} equipos de la biblioteca`} en el mismo mapa y lado.
        </p>
        <CategoryGrid playbook={playbook} reference={pool === null ? null : poolCategories(pool)} />
      </Section>
      <div className="grid gap-4 lg:grid-cols-2">
        <Section title="Ritmo">
          <TempoChart playbook={playbook} rounds={playbook.rounds} />
        </Section>
        <Section title="Sitios">
          <SiteBars playbook={playbook} />
        </Section>
      </div>
      <Section title="Zonas que controlan">
        <ZoneExplorer key={`${playbook.map}-${playbook.side}`} teamId={teamId} playbook={playbook} />
      </Section>
      <SkippedNote skipped={playbook.skipped} />
    </div>
  );
}
