'use client';

import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { Gauge, PlugZap } from 'lucide-react';
import { LibraryApiError, getLibraryMatchMetrics, type LibraryMatch } from '@/lib/api/library';
import type { MetricsReport } from '@/lib/api/metrics';
import { playerHref } from '@/lib/clips/routes';
import { ctTeamByRound, topSwings } from '@/lib/metrics/match-view';
import { EmptyState } from '@/components/records/empty-state';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { MatchPlayersTable } from './match-players-table';
import { MatchSwings } from './match-swings';
import { MatchWinChart } from './match-win-chart';
import { MetricsView } from './metrics-view';
import { WinProbability } from './win-probability';

type State = { kind: 'loading' } | { kind: 'ready'; report: MetricsReport } | { kind: 'pending' } | { kind: 'offline' };

const SWING_LIMIT = 5;

/**
 * The read metrics of one match: each round's win chance, the kills that
 * weighed most, and every player in one table before the detailed sections.
 */
function MatchMetricsReport({
  match,
  report,
  onOpenTactical,
}: {
  match: LibraryMatch;
  report: MetricsReport;
  onOpenTactical?: () => void;
}): ReactNode {
  const timelines = [...(report.timelines ?? [])].sort((a, b) => a.round - b.round);
  const players = match.players ?? [];
  const ctTeams = ctTeamByRound(timelines, players);
  const names = new Map(players.map((p) => [p.slot, p.name || p.steamid64]));
  const [round, setRound] = useState(timelines[0]?.round ?? 0);
  const [curveOpen, setCurveOpen] = useState(false);
  const curveId = useId();
  const curveRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!curveOpen) return;
    const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    curveRef.current?.scrollIntoView({ block: 'nearest', behavior: reduceMotion ? 'auto' : 'smooth' });
  }, [curveOpen]);

  const lead =
    timelines.length === 0 ? null : (
      <>
        <MatchWinChart
          match={match}
          timelines={timelines}
          ctTeams={ctTeams}
          names={names}
          round={round}
          onRoundChange={setRound}
          curveOpen={curveOpen}
          curveId={curveId}
          onCurveToggle={() => setCurveOpen((open) => !open)}
        />
        {curveOpen ? (
          <div id={curveId} ref={curveRef} className="scroll-mt-4">
            <WinProbability
              matchId={match.id}
              timelines={timelines}
              players={players}
              round={round}
              onRoundChange={setRound}
              onOpenTactical={onOpenTactical}
            />
          </div>
        ) : null}
        <MatchSwings match={match} swings={topSwings(timelines, SWING_LIMIT)} ctTeams={ctTeams} names={names} onOpenTactical={onOpenTactical} />
      </>
    );
  return (
    <MetricsView
      report={report}
      options={{ playerHref }}
      lead={lead}
      title="Jugadores"
      leading={[{ id: 'metricas-general', label: 'General', content: <MatchPlayersTable match={match} report={report} /> }]}
    />
  );
}

/** The match's advanced metrics: every player's impact, duels, trades and utility. */
export function MatchMetrics({
  match,
  onOpenTactical,
}: {
  match: LibraryMatch;
  /** Switches the record to its Táctica tab when a link here opens the replay. */
  onOpenTactical?: () => void;
}): ReactNode {
  const [state, setState] = useState<State>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    setState({ kind: 'loading' });
    getLibraryMatchMetrics(match.id)
      .then((report) => {
        if (active) setState({ kind: 'ready', report });
      })
      .catch((error: unknown) => {
        if (!active) return;
        setState(error instanceof LibraryApiError && error.status === 409 ? { kind: 'pending' } : { kind: 'offline' });
      });
    return () => {
      active = false;
    };
  }, [match.id, match.updated_at, attempt]);

  if (state.kind === 'loading') {
    return (
      <div role="status" aria-label="Calculando métricas" className="flex flex-col gap-3">
        <Skeleton className="h-48 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  if (state.kind === 'pending') {
    return (
      <EmptyState
        icon={Gauge}
        title="Análisis táctico pendiente"
        description="Las métricas se calculan con el análisis táctico de la demo. Vuelve cuando termine."
      />
    );
  }
  if (state.kind === 'offline') {
    return (
      <EmptyState
        icon={PlugZap}
        title="No se pudieron calcular las métricas"
        description="Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={() => setAttempt((n) => n + 1)}>
            Reintentar
          </Button>
        }
      />
    );
  }
  return <MatchMetricsReport match={match} report={state.report} onOpenTactical={onOpenTactical} />;
}
