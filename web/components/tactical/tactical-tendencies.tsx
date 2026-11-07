'use client';

import { useId, type ReactNode } from 'react';
import { MIN_RELIABLE_SAMPLE } from '@/lib/api/tactical';
import type { TacticalTendencies } from '@/lib/api/tactical';
import { teamSummaryLine } from '@/lib/tactical-tendency-view';
import { TendencyCards } from '@/components/tactical/tactical-tendency-cards';
import { SideAgnosticTendencies, TendencyDetail } from '@/components/tactical/tactical-tendency-detail';
import { Skeleton } from '@/components/ui/skeleton';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { cn } from '@/lib/utils';

/** The panel's team toggle: which team it reads, and whether the replay filter already fixed it. */
export type TendencyTeamChoice = {
  teams: readonly { key: string; name: string }[];
  selected: string;
  locked: boolean;
  onSelect: (key: string) => void;
};

function TeamToggle({ team }: { team: TendencyTeamChoice }): ReactNode {
  return (
    <div title={team.locked ? 'El filtro del replay ya elige el equipo' : undefined}>
      <ToggleGroup
        type="single"
        variant="filter"
        spacing={2}
        aria-label="Equipo"
        value={team.selected}
        disabled={team.locked}
        onValueChange={(next: string) => {
          if (next !== '') team.onSelect(next);
        }}
      >
        {team.teams.map((option) => (
          <ToggleGroupItem key={option.key} value={option.key} className="max-w-56">
            <span className="min-w-0 truncate">{option.name}</span>
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
    </div>
  );
}

function Subline({
  tendencies,
  teamName,
  filtered,
  roundCount,
  stale,
}: {
  tendencies: TacticalTendencies | null;
  teamName: string | undefined;
  filtered: boolean;
  roundCount: number;
  stale: boolean;
}): ReactNode {
  if (teamName === undefined) {
    return <p className="text-body-sm text-fg-2">Calculadas sobre las {roundCount} rondas filtradas.</p>;
  }
  if (tendencies === null || tendencies.round_count === 0) return null;
  return (
    <p className={cn('text-body-sm text-fg-2 transition-opacity duration-(--dur-fast)', stale && 'opacity-60')}>
      {teamSummaryLine({ rounds: tendencies.round_count, wins: tendencies.wins, filtered })}
      {tendencies.round_count < MIN_RELIABLE_SAMPLE ? <span className="ml-1.5 text-meta text-fg-3">pocas</span> : null}
    </p>
  );
}

function Body({ tendencies, teamName }: { tendencies: TacticalTendencies; teamName: string | undefined }): ReactNode {
  if (tendencies.round_count === 0) {
    return (
      <p className="text-body leading-6 text-fg-2">
        {teamName === undefined
          ? 'El filtro actual no selecciona ninguna ronda, así que no hay nada que agregar.'
          : `Con los filtros del replay, ${teamName} no tiene ninguna ronda en esta partida.`}
      </p>
    );
  }
  if (tendencies.perspective === undefined) return <SideAgnosticTendencies tendencies={tendencies} />;
  return (
    <>
      <TendencyCards tendencies={tendencies} />
      <p className="text-meta text-fg-3">
        «Pocas» marca una cifra con menos de {MIN_RELIABLE_SAMPLE} rondas: es un dato, todavía no una tendencia.
      </p>
      <TendencyDetail tendencies={tendencies} />
    </>
  );
}

/**
 * What one team did in the rounds the replay filter selected. Without two teams
 * in the demo there is no toggle and the panel reads the filter as it is.
 */
export function TacticalTendenciesPanel({
  tendencies,
  error,
  roundCount,
  stale = false,
  team,
  filtered = true,
}: {
  tendencies: TacticalTendencies | null;
  error: string | null;
  roundCount: number;
  /** The numbers on screen belong to an earlier filter or team while the next answer loads. */
  stale?: boolean;
  team?: TendencyTeamChoice;
  /** The replay filter narrows the rounds. */
  filtered?: boolean;
}): ReactNode {
  const titleId = useId();
  const teamName = team === undefined ? undefined : (team.teams.find((option) => option.key === team.selected)?.name ?? team.selected);

  return (
    <section className="studio-panel flex flex-col gap-4.5 p-5" aria-labelledby={titleId}>
      <header className="flex flex-wrap items-center gap-3">
        <div className="flex min-w-0 flex-[1_1_22.5rem] flex-col gap-0.5">
          <h2 id={titleId} className="font-display text-title font-semibold text-fg-1">
            {teamName === undefined ? 'Tendencias' : `Qué hizo ${teamName}`}
          </h2>
          <Subline tendencies={tendencies} teamName={teamName} filtered={filtered} roundCount={roundCount} stale={stale} />
        </div>
        {team === undefined ? null : <TeamToggle team={team} />}
      </header>

      {error === null ? null : (
        <p className="rounded-md border border-destructive/45 bg-destructive/10 px-3 py-2 text-body-sm leading-5 break-words text-destructive">
          No se pudieron calcular las tendencias: {error}
        </p>
      )}

      {error === null && tendencies === null ? (
        <div className="grid grid-cols-[repeat(auto-fit,minmax(min(18.75rem,100%),1fr))] gap-4" aria-hidden>
          <Skeleton className="h-44 w-full" />
          <Skeleton className="h-44 w-full" />
          <Skeleton className="h-44 w-full" />
        </div>
      ) : null}

      {tendencies === null ? null : (
        <div aria-busy={stale} className={cn('flex flex-col gap-4.5 transition-opacity duration-(--dur-fast)', stale && 'opacity-60')}>
          <Body tendencies={tendencies} teamName={teamName} />
        </div>
      )}
    </section>
  );
}
