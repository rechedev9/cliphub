'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { Clapperboard } from 'lucide-react';
import { fetchTacticalDocument, isServiceUnavailableError, type TacticalDocument } from '@/lib/api/tactical';
import type { LibraryMatch } from '@/lib/api/library';
import { teamByKey, teamLabel } from '@/lib/library/partidas';
import {
  EVERY_ROUND,
  roundHalves,
  roundRows,
  visibleHalves,
  winnerCounts,
  type PerTeam,
  type RoundFilter,
} from '@/lib/library/rounds';
import { AnalysisVideoDialog } from '@/components/analysis-video/analysis-video-dialog';
import { RoundFilters } from '@/components/matches/round-filters';
import { RoundTable, type ChanceStatus, type VisiblePick } from '@/components/matches/round-table';
import { useWinTimelines, type WinTimelines } from '@/components/replay/use-win-timelines';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';

type MatchRoundsProps = { match: LibraryMatch; pathname: string; onOpenRound: () => void };

function chanceStatus(win: WinTimelines | null): ChanceStatus {
  if (win === null || win.kind === 'loading') return 'loading';
  return win.kind === 'ready' && win.timelines.length > 0 ? 'ready' : 'none';
}

function roundsText(count: number): string {
  return count === 1 ? '1 ronda' : `${count} rondas`;
}

/** Every round of a partida by half, with filters and a pick of several rounds for one video. */
export function MatchRounds(props: MatchRoundsProps): ReactNode {
  return <MatchRoundsView key={props.match.id} {...props} />;
}

function MatchRoundsView({ match, pathname, onOpenRound }: MatchRoundsProps): ReactNode {
  const [doc, setDoc] = useState<TacticalDocument | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const [filter, setFilter] = useState<RoundFilter>(EVERY_ROUND);
  const [picked, setPicked] = useState<readonly number[]>([]);
  /** The rounds the video dialog opens with checked; null while it is closed. */
  const [video, setVideo] = useState<readonly number[] | null>(null);
  const win = useWinTimelines(match.id);

  useEffect(() => {
    let active = true;
    setError(null);
    fetchTacticalDocument({ kind: 'library', id: match.id })
      .then((next) => {
        if (active) setDoc(next);
      })
      .catch((err: unknown) => {
        if (!active) return;
        setError(
          isServiceUnavailableError(err)
            ? 'El servicio local no está disponible.'
            : 'No se pudo leer el documento táctico de esta partida.',
        );
      });
    return () => {
      active = false;
    };
  }, [match.id, attempt]);

  if (error !== null) {
    return (
      <section aria-label="Rondas" className="studio-panel flex flex-wrap items-center gap-3 px-4 py-3">
        <p className="text-body-sm text-destructive">{error}</p>
        <Button type="button" size="sm" variant="outline" onClick={() => setAttempt((n) => n + 1)}>
          Reintentar
        </Button>
      </section>
    );
  }
  if (doc === null) {
    return (
      <section role="status" aria-label="Cargando rondas" className="studio-panel flex flex-col gap-2 p-4">
        <Skeleton className="h-8 w-full" />
        {Array.from({ length: 8 }, (_, i) => (
          <Skeleton key={i} className="h-11 w-full" />
        ))}
      </section>
    );
  }

  const chance = chanceStatus(win);
  const rows = roundRows(doc, win?.kind === 'ready' ? win.timelines : []);
  const halves = visibleHalves(roundHalves(rows), filter);
  const visible = halves.flatMap((half) => half.rows.map((row) => row.number));
  const pickedSet = new Set(picked);
  const pickedVisible = visible.filter((round) => pickedSet.has(round)).length;
  let visiblePick: VisiblePick = 'some';
  if (pickedVisible === 0) visiblePick = 'none';
  else if (pickedVisible === visible.length) visiblePick = 'all';
  const teamNames: PerTeam<string> = {
    'ct-start': teamLabel(teamByKey(match, 'ct-start')),
    't-start': teamLabel(teamByKey(match, 't-start')),
  };
  const playerNames = new Map(doc.players.map((player) => [player.slot, player.name || player.steamid64]));

  function togglePick(round: number): void {
    setPicked((current) => (current.includes(round) ? current.filter((n) => n !== round) : [...current, round].sort((a, b) => a - b)));
  }

  function toggleVisible(): void {
    setPicked((current) =>
      visiblePick === 'all'
        ? current.filter((round) => !visible.includes(round))
        : [...new Set([...current, ...visible])].sort((a, b) => a - b),
    );
  }

  return (
    <div className="flex flex-col gap-4">
      {rows.length > 0 ? (
        <RoundFilters value={filter} onChange={setFilter} teamNames={teamNames} counts={winnerCounts(rows)} upsets={chance === 'ready'} />
      ) : null}
      <section aria-label="Rondas" className="studio-panel flex flex-col">
        {rows.length === 0 ? <p className="px-4 py-6 text-body-sm text-fg-2">Esta partida no tiene rondas jugadas.</p> : null}
        {rows.length > 0 && halves.length === 0 ? <NoRoundsMatch filter={filter} onReset={() => setFilter(EVERY_ROUND)} /> : null}
        {halves.length > 0 ? (
          <RoundTable
            halves={halves}
            teamNames={teamNames}
            playerNames={playerNames}
            chance={chance}
            picked={pickedSet}
            visiblePick={visiblePick}
            onTogglePick={togglePick}
            onToggleVisible={toggleVisible}
            pathname={pathname}
            onOpenRound={onOpenRound}
            onVideo={(round) => setVideo([round])}
          />
        ) : null}
        {rows.length > 0 ? (
          <div className="sticky bottom-0 z-10 flex flex-wrap items-center gap-3 rounded-b-lg border-t border-border bg-surface-1 px-4 py-3">
            {picked.length > 0 ? (
              <>
                <span className="text-body-sm font-semibold text-fg-1">
                  {picked.length === 1 ? '1 ronda elegida' : `${picked.length} rondas elegidas`}
                </span>
                <span className="text-meta text-fg-3">
                  {picked.length === 1 ? 'Ronda' : 'Rondas'} {picked.join(', ')}
                </span>
                <span className="grow" />
                <Button type="button" size="sm" variant="ghost" onClick={() => setPicked([])}>
                  Quitar selección
                </Button>
                <Button type="button" size="sm" onClick={() => setVideo(picked)}>
                  <Clapperboard aria-hidden />
                  Crear vídeo con estas rondas
                </Button>
              </>
            ) : (
              <>
                <span className="text-body-sm text-fg-2">Marca varias rondas para hacer un solo vídeo con todas.</span>
                <span className="grow" />
                <span className="text-meta text-fg-3">
                  {visible.length === rows.length ? roundsText(rows.length) : `${visible.length} de ${roundsText(rows.length)}`}
                </span>
              </>
            )}
          </div>
        ) : null}
      </section>
      {chance === 'ready' ? (
        <p className="text-meta text-fg-3">
          El porcentaje en ámbar marca a quien ganó con menos de un 35 % de probabilidad al empezar la ronda. La baja que más pesó es la que más
          movió esa probabilidad.
        </p>
      ) : null}
      {video !== null ? (
        <AnalysisVideoDialog
          sources={[{ matchId: match.id, rounds: doc.rounds.map((round) => round.number) }]}
          selected={video}
          onClose={() => setVideo(null)}
        />
      ) : null}
    </div>
  );
}

function NoRoundsMatch({ filter, onReset }: { filter: RoundFilter; onReset: () => void }): ReactNode {
  let title = 'Ninguna ronda de este tipo';
  let hint = 'Prueba con otro tipo de ronda.';
  if (filter.winner !== 'all' && filter.kind !== 'any') {
    title = 'Ninguna ronda cumple los dos filtros';
    hint = 'Prueba con otro tipo de ronda o quita el filtro de ganador.';
  } else if (filter.winner !== 'all') {
    title = 'Este equipo no ganó ninguna ronda';
    hint = 'Elige Todas para ver las demás.';
  }
  return (
    <div className="flex flex-col items-center gap-2 px-4 py-8 text-center">
      <p className="text-body-sm font-semibold text-fg-1">{title}</p>
      <p className="text-body-sm text-fg-2">{hint}</p>
      <Button type="button" size="sm" variant="outline" onClick={onReset}>
        Quitar filtros
      </Button>
    </div>
  );
}
