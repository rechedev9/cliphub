'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { getLibraryTeamScout, getTeamPlaybook, getTeamPrep } from '@/lib/api/library';
import type { ScoutReport } from '@/lib/api/scout';
import { minRoundsNote } from '@/lib/analysis/reference';
import { libraryMatchesText, normsCategories, poolMatchups, significantStats } from '@/lib/analysis/library-pool';
import { prepStatInputs } from '@/lib/analysis/prep-stats';
import type { TeamPlaybook, TeamPrep } from '@/lib/api/playbook';
import { prettyMapName } from '@/lib/format';
import type { ReplayForm } from '@/lib/replay/forms';
import { REPLAY_FORM_LABEL } from '@/lib/replay/forms';
import { EmptySample } from '@/components/analysis/empty-sample';
import { SkippedNote } from '@/components/playbook/playbook-summary';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useLibraryNorms } from '@/hooks/use-library-norms';
import { useReferences } from '@/hooks/use-references';
import { CtPositions, MatchupTables, PlaybookBrief, PrepSection, TempoByBuy } from './prep-sections';
import { SignificantStats } from './significant-stats';
import { AttackPrep } from './attack-sections';

type Loaded = { prep: TeamPrep; t: TeamPlaybook; ct: TeamPlaybook; scout: ScoutReport | null };

/** What a rival's preparation on one map reads: the team, the map, the days and the lineup, as the API takes them. */
export type PrepQuery = { teamId: string; map: string; from?: string; to?: string; lineup?: readonly string[]; opponentIds?: readonly string[] };

/**
 * The preparation of a rival on one map: buys against buys, tempo and attack,
 * what stands out, CT positions and both playbooks. `onOpenReplay` gives each section the
 * door to the Replay form that shows it; the printed sheet has none.
 */
export function PrepBody({
  query,
  header,
  onOpenReplay,
}: {
  query: PrepQuery;
  /** Drawn over the sections once the prep is read, such as the printed sheet's title. */
  header?: (prep: TeamPrep) => ReactNode;
  onOpenReplay?: (form: ReplayForm) => void;
}): ReactNode {
  const { teamId, map, from, to, lineup, opponentIds } = query;
  const [data, setData] = useState<Loaded | null>(null);
  const [failed, setFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const loadedNorms = useLibraryNorms(map);
  const norms = loadedNorms === 'failed' ? null : loadedNorms;
  const references = useReferences(map);

  useEffect(() => {
    let active = true;
    setData(null);
    setFailed(false);
    const base = { map, from, to, lineup, opponent_ids: opponentIds };
    Promise.all([
      getTeamPrep(teamId, base),
      getTeamPlaybook(teamId, { ...base, side: 'T' }),
      getTeamPlaybook(teamId, { ...base, side: 'CT' }),
      getLibraryTeamScout(teamId, { maps: [map], from, to, lineup, opponent_ids: opponentIds }).catch(() => null),
    ])
      .then(([prep, t, ct, scout]) => {
        if (active) setData({ prep, t, ct, scout });
      })
      .catch(() => {
        if (active) setFailed(true);
      });
    return () => {
      active = false;
    };
  }, [teamId, map, from, to, lineup, opponentIds, attempt]);

  if (failed) {
    return (
      <div className="flex flex-col items-start gap-3">
        <p role="alert" className="text-body-sm text-destructive">
          No se pudo preparar este rival. Comprueba que ClipHub Studio sigue abierto y que el equipo sigue en la biblioteca.
        </p>
        <Button type="button" variant="outline" size="sm" onClick={() => setAttempt((n) => n + 1)}>
          Reintentar
        </Button>
      </div>
    );
  }
  if (data === null) {
    return (
      <div role="status" aria-label="Preparando el rival" className="flex flex-col gap-5">
        <Skeleton className="h-56 w-full" />
        <Skeleton className="h-40 w-full" />
        <Skeleton className="h-72 w-full" />
      </div>
    );
  }

  const { prep, t, ct, scout } = data;
  const matchupPool = norms === null ? null : poolMatchups([norms]);
  const baseline = references !== null && references !== 'failed' ? references.library : null;
  const stats = significantStats(prepStatInputs({ prep, scout, library: matchupPool, baseline }));
  const statsNote = `Frente a ${norms === null ? 'la biblioteca' : `los dos equipos de ${libraryMatchesText(norms.matches)}`} en ${prettyMapName(prep.map)}: la apertura, frente al 50 % que suman todos los equipos; ADR y bajas, frente a la media de sus jugadores; victorias, frente a todos esos equipos. Solo sale lo que se aleja de verdad: 5 puntos en aperturas, 6 en victorias.${baseline ? minRoundsNote(baseline.min_rounds) : ''}`;
  const door = (form: ReplayForm): ReactNode =>
    onOpenReplay === undefined ? undefined : (
      <Button type="button" variant="ghost" size="xs" onClick={() => onOpenReplay(form)} className="print:hidden">
        Ver en {REPLAY_FORM_LABEL[form]}
      </Button>
    );
  return (
    <>
      {header?.(prep)}
      {prep.round_count === 0 ? (
        <EmptySample>
          <p className="text-body text-fg-2">
            No hay rondas de {prep.team_name} en {prettyMapName(prep.map)} en este periodo
            {lineup !== undefined && lineup.length > 0 ? ' con esta alineación. Prueba con todos los jugadores o con todas las fechas.' : '. Prueba con todas las fechas.'}
          </p>
        </EmptySample>
      ) : (
        <>
          <PrepSection title="Compras contra compras" hint="Cada cifra abre sus rondas en Repetición.">
            <MatchupTables prep={prep} library={matchupPool} />
          </PrepSection>
          <PrepSection title="Ritmo por compra">
            <TempoByBuy prep={prep} />
          </PrepSection>
          <AttackPrep query={query} />
          <PrepSection title="Lo que destaca">
            <SignificantStats stats={stats} loading={loadedNorms === null || references === null} note={statsNote} />
          </PrepSection>
          <PrepSection title="Posiciones CT por tramo" action={door('calor')}>
            <CtPositions prep={prep} />
          </PrepSection>
          <PrepSection title="Playbook en T" action={door('recorridos')}>
            <PlaybookBrief playbook={t} library={norms === null ? null : normsCategories(norms, 'T')} />
          </PrepSection>
          <PrepSection title="Playbook en CT" action={door('recorridos')}>
            <PlaybookBrief playbook={ct} library={norms === null ? null : normsCategories(norms, 'CT')} />
          </PrepSection>
        </>
      )}
      <SkippedNote skipped={prep.skipped} />
    </>
  );
}
