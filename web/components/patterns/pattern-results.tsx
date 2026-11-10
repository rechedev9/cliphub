'use client';

import Link from 'next/link';
import type { ReactNode } from 'react';
import { Play } from 'lucide-react';
import type { PatternBlockKind, PatternResult } from '@/lib/api/patterns';
import { TREND_LABEL, entryLabel, monthLabel, patternReplayRefs, percentLabel, roundCountLabel } from '@/lib/analysis/pattern-finder';
import { replayHref } from '@/lib/analysis/replay-link';
import { formatShortDate } from '@/lib/format';
import { cn } from '@/lib/utils';
import { EmptySample } from '@/components/analysis/empty-sample';
import { Button } from '@/components/ui/button';
import { Sparkline } from '@/components/ui/sparkline';

type Props = {
  result: PatternResult;
  /** The kind of the block whose moment each row shows. */
  timed: PatternBlockKind;
  title: string;
  /** Shown beside the open button, such as the save button. */
  actions?: ReactNode;
};

function Figure({ label, value, children }: { label: string; value: string; children?: ReactNode }): ReactNode {
  return (
    <div className="flex flex-col gap-0.5 rounded-md border border-border bg-surface-1 px-2.5 py-2">
      <span className="text-meta text-fg-2">{label}</span>
      <span className="flex items-center gap-2 text-body font-semibold tabular-nums text-fg-1">
        {value}
        {children}
      </span>
    </div>
  );
}

/** What the pattern found: how many rounds, how they ended, the trend and every round by match. */
export function PatternResults({ result, timed, title, actions }: Props): ReactNode {
  const refs = patternReplayRefs(result);
  const shares = result.months.map((month) => (month.rounds === 0 ? 0 : month.matched / month.rounds));
  return (
    <div className="flex flex-col gap-3">
      <h2 className="text-body font-semibold tabular-nums text-fg-1">
        {result.matched} de {roundCountLabel(result.rounds)}
      </h2>
      <div className="grid grid-cols-2 gap-2">
        <Figure label="Ganan" value={result.matched === 0 ? 'Sin rondas' : percentLabel(result.win_rate)} />
        <Figure label="Últimos 30 días" value={roundCountLabel(result.recent_matched)} />
        <Figure label="Tendencia" value={TREND_LABEL[result.trend]}>
          <Sparkline values={shares} label="Parte de las rondas por mes" format={percentLabel} width={64} height={20} />
        </Figure>
        <Figure label="Desde" value={result.months[0] === undefined ? 'Sin fecha' : monthLabel(result.months[0].month)} />
      </div>
      <div className="flex flex-wrap gap-2">
        {refs.length === 0 ? (
          <Button type="button" size="sm" disabled>
            <Play aria-hidden />
            Ver en Repetición
          </Button>
        ) : (
          <Button asChild size="sm">
            <Link href={replayHref(refs, title)}>
              <Play aria-hidden />
              Ver {refs.length === 1 ? 'la ronda' : `las ${refs.length} rondas`} en Repetición
            </Link>
          </Button>
        )}
        {actions}
      </div>
      {result.missing_positions > 0 ? (
        <p className="text-body-sm text-fg-2">
          {result.missing_positions} {result.missing_positions === 1 ? 'ronda no tiene' : 'rondas no tienen'} posiciones guardadas y no cuentan para los bloques de jugador.
        </p>
      ) : null}
      {result.matched === 0 ? (
        <EmptySample>
          <p className="text-body-sm text-fg-2">Ninguna ronda cumple el patrón. Cambia el tramo, la zona o los bloques.</p>
        </EmptySample>
      ) : (
        <ol className="flex flex-col gap-3">
          {result.matches
            .filter((match) => match.matched.length > 0)
            .map((match) => (
              <li key={`${match.match_id}:${match.team_key ?? ''}`} className="flex flex-col gap-1">
                <h3 className="flex items-baseline justify-between gap-2 text-body-sm font-medium text-fg-1">
                  <span className="truncate">
                    {match.team ?? 'Equipo'} vs {match.opponent ?? 'Rival'}
                  </span>
                  <span className="shrink-0 text-meta tabular-nums text-fg-3">
                    {formatShortDate(match.played_at)} · {match.matched.length} de {match.rounds}
                  </span>
                </h3>
                <ul className="flex flex-col">
                  {match.matched.map((row) => (
                    <li key={row.round}>
                      <Link
                        href={replayHref([row.tick === undefined ? { matchId: match.match_id, round: row.round } : { matchId: match.match_id, round: row.round, tick: row.tick }], title)}
                        className="grid grid-cols-[2.5rem_minmax(0,1fr)_auto] items-center gap-2 rounded-sm px-1.5 py-1 text-body-sm hover:bg-surface-2 focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
                      >
                        <span className="font-mono tabular-nums text-fg-2">R{row.round}</span>
                        <span className="truncate text-fg-1">{row.seconds === undefined ? `Lado ${row.side}` : entryLabel(timed, row.seconds)}</span>
                        <span className={cn('font-medium', row.won ? 'text-outcome-win' : 'text-outcome-loss')}>{row.won ? 'Gana' : 'Pierde'}</span>
                      </Link>
                    </li>
                  ))}
                </ul>
              </li>
            ))}
        </ol>
      )}
    </div>
  );
}
