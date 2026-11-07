'use client';

import { useId, useState, type ReactNode } from 'react';
import { ChevronRight } from 'lucide-react';
import { TACTICAL_BUY_TYPE_ORDER, TACTICAL_SITE_ORDER } from '@/lib/api/tactical';
import type { TacticalBuyType, TacticalSite, TacticalTendencies } from '@/lib/api/tactical';
import { buyLabel, patternLabel, siteLabel } from '@/lib/tactical-labels';
import { HistogramChart, RateBar, RateRow, RateValue, SmallSampleChip } from '@/components/tactical/tactical-charts';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

const HEAD_CELL = 'px-3 py-2 text-left text-meta font-normal text-fg-3';
const BODY_CELL = 'px-3 py-2 align-middle';
const ROW_HEAD = cn(BODY_CELL, 'text-left text-meta font-normal text-fg-1');
const TABLE = 'w-full min-w-[560px] border-collapse';

function SectionTitle({ children }: { children: ReactNode }): ReactNode {
  return <h3 className="text-meta text-fg-3">{children}</h3>;
}

/** A cross-tab cell: the rate, its denominator, and nothing implied by colour alone. */
function CrossCell({ rounds, pct, reliable }: { rounds: number; pct: number; reliable: boolean }): ReactNode {
  if (rounds === 0) {
    return <span className="text-meta tabular-nums text-fg-4">n=0</span>;
  }
  return (
    <span className="flex flex-col gap-1">
      <span className="flex items-baseline gap-2">
        <span className={cn('text-meta tracking-normal tabular-nums', reliable ? 'text-fg-1' : 'text-fg-3')}>{pct.toFixed(1)} %</span>
        <span className="text-meta tracking-normal tabular-nums text-fg-3">n={rounds}</span>
      </span>
      {reliable ? null : <SmallSampleChip className="self-start" />}
    </span>
  );
}

function EconomyTable({ tendencies }: { tendencies: TacticalTendencies }): ReactNode {
  return (
    <section className="flex flex-col gap-3">
      <SectionTitle>Economía</SectionTitle>
      <div className="overflow-x-auto">
        <table className={TABLE}>
          <thead>
            <tr className="border-b border-border-subtle">
              <th scope="col" className={HEAD_CELL}>
                Compra
              </th>
              <th scope="col" className={cn(HEAD_CELL, 'w-16')}>
                Rondas
              </th>
              <th scope="col" className={cn(HEAD_CELL, 'w-[34%]')}>
                Reparto
              </th>
              <th scope="col" className={cn(HEAD_CELL, 'w-[34%]')}>
                Victorias
              </th>
              <th scope="col" className={HEAD_CELL}>
                Plantada o desactivación
              </th>
            </tr>
          </thead>
          <tbody>
            {tendencies.buys
              .filter((bucket) => bucket.rounds > 0)
              .map((bucket) => (
                <tr key={bucket.buy} className="border-b border-border-subtle last:border-b-0">
                  <th scope="row" className={ROW_HEAD}>
                    {buyLabel(bucket.buy)}
                  </th>
                  <td className={cn(BODY_CELL, 'text-meta tracking-normal tabular-nums text-fg-3')}>{bucket.rounds}</td>
                  <td className={BODY_CELL}>
                    <div className="flex flex-col gap-1.5">
                      <RateBar rate={bucket.share} className="text-fg-2" />
                      <RateValue rate={bucket.share} />
                    </div>
                  </td>
                  <td className={BODY_CELL}>
                    <div className="flex flex-col gap-1.5">
                      <RateBar rate={bucket.win_rate} className="text-success" />
                      <RateValue rate={bucket.win_rate} />
                    </div>
                  </td>
                  <td className={BODY_CELL}>
                    <RateValue rate={bucket.conversion} />
                  </td>
                </tr>
              ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

/** Rows by own buy, columns by `columns`; empty when no row has a round. */
function CrossTable<Column extends string>({
  title,
  corner,
  rows,
  columns,
  columnLabel,
  cell,
}: {
  title: string;
  corner: string;
  rows: readonly TacticalBuyType[];
  columns: readonly Column[];
  columnLabel: (column: Column) => string;
  cell: (buy: TacticalBuyType, column: Column) => { rounds: number; pct: number; reliable: boolean } | undefined;
}): ReactNode {
  if (rows.length === 0) return null;
  return (
    <section className="flex flex-col gap-3">
      <SectionTitle>{title}</SectionTitle>
      <div className="overflow-x-auto">
        <table className={TABLE}>
          <thead>
            <tr className="border-b border-border-subtle">
              <th scope="col" className={HEAD_CELL}>
                {corner}
              </th>
              {columns.map((column) => (
                <th key={column} scope="col" className={HEAD_CELL}>
                  {columnLabel(column)}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((buy) => (
              <tr key={buy} className="border-b border-border-subtle last:border-b-0">
                <th scope="row" className={ROW_HEAD}>
                  {buyLabel(buy)}
                </th>
                {columns.map((column) => {
                  const value = cell(buy, column);
                  return (
                    <td key={column} className={BODY_CELL}>
                      <CrossCell rounds={value?.rounds ?? 0} pct={value?.pct ?? 0} reliable={value?.reliable ?? false} />
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function Timings({ tendencies }: { tendencies: TacticalTendencies }): ReactNode {
  return (
    <section className="flex flex-col gap-3">
      <SectionTitle>Tiempos · segundos desde el fin del congelado</SectionTitle>
      <div className="grid gap-6 lg:grid-cols-3">
        <HistogramChart histogram={tendencies.timings.first_contact} title="Primer contacto" />
        <HistogramChart histogram={tendencies.timings.plant} title="Plantada" />
        <HistogramChart histogram={tendencies.timings.round_duration} title="Duración de ronda" />
      </div>
    </section>
  );
}

function DetailBody({ tendencies }: { tendencies: TacticalTendencies }): ReactNode {
  const { matchups, buy_sites: buySites, openings } = tendencies;
  const ownBuys = TACTICAL_BUY_TYPE_ORDER.filter((buy) => matchups.some((bucket) => bucket.buy === buy && bucket.rounds > 0));
  const opponentBuys = TACTICAL_BUY_TYPE_ORDER.filter((buy) => matchups.some((bucket) => bucket.opponent_buy === buy && bucket.rounds > 0));
  const buySiteBuys = TACTICAL_BUY_TYPE_ORDER.filter((buy) => buySites.some((bucket) => bucket.buy === buy && bucket.rounds > 0));
  const sites = TACTICAL_SITE_ORDER.filter((site) => buySites.some((bucket) => bucket.site === site && bucket.rounds > 0));
  const matchup = (buy: TacticalBuyType, opponent: TacticalBuyType) => {
    const bucket = matchups.find((entry) => entry.buy === buy && entry.opponent_buy === opponent);
    return bucket === undefined ? undefined : { rounds: bucket.rounds, pct: bucket.win_rate.pct, reliable: bucket.win_rate.reliable };
  };
  const buySite = (buy: TacticalBuyType, site: TacticalSite) => {
    const bucket = buySites.find((entry) => entry.buy === buy && entry.site === site);
    return bucket === undefined ? undefined : { rounds: bucket.rounds, pct: bucket.share.pct, reliable: bucket.share.reliable };
  };

  return (
    <div className="flex flex-col gap-8 pt-2">
      <EconomyTable tendencies={tendencies} />
      <CrossTable
        title="Compra propia contra compra rival · victorias"
        corner="Propia \ rival"
        rows={ownBuys}
        columns={opponentBuys}
        columnLabel={buyLabel}
        cell={matchup}
      />
      <CrossTable
        title="Con cada economía, a qué sitio van · reparto"
        corner="Economía \ sitio"
        rows={buySiteBuys}
        columns={sites}
        columnLabel={siteLabel}
        cell={buySite}
      />
      <div className="grid gap-8 lg:grid-cols-2">
        <ShapeShares title="Formas en T · reparto" buckets={tendencies.t_patterns} />
        <ShapeShares title="Formas en CT · reparto" buckets={tendencies.ct_patterns} />
      </div>
      <section className="flex flex-col gap-3">
        <SectionTitle>Duelo de apertura · {openings.rounds} rondas con duelo</SectionTitle>
        <div className="flex flex-col gap-3">
          <RateRow label="Aperturas ganadas" rate={openings.won} />
          <RateRow label="Tradeadas al perderla" rate={openings.traded_on_loss} tone="muted" />
          <RateRow label="Ronda ganada tras ganarla" rate={openings.round_win_after_opening_win} tone="success" />
          <RateRow label="Ronda ganada tras perderla" rate={openings.round_win_after_opening_loss} tone="success" />
        </div>
      </section>
      <Timings tendencies={tendencies} />
    </div>
  );
}

/** The full tables behind the cards: shares, conversions, cross-tabs, shape shares, duel follow-ups and timings. */
export function TendencyDetail({ tendencies }: { tendencies: TacticalTendencies }): ReactNode {
  const [open, setOpen] = useState(false);
  const bodyId = useId();
  return (
    <div className="flex flex-col gap-2 border-t border-border-subtle pt-3">
      <Button
        type="button"
        variant="ghost"
        size="sm"
        aria-expanded={open}
        aria-controls={open ? bodyId : undefined}
        onClick={() => setOpen((value) => !value)}
        className="-ml-3 self-start"
      >
        <ChevronRight className={cn('transition-transform duration-(--dur-fast)', open && 'rotate-90')} aria-hidden />
        Más detalle
      </Button>
      {open ? (
        <div id={bodyId}>
          <DetailBody tendencies={tendencies} />
        </div>
      ) : null}
    </div>
  );
}

function ShapeShares({ title, buckets }: { title: string; buckets: TacticalTendencies['t_patterns'] }): ReactNode {
  return (
    <section className="flex min-w-0 flex-col gap-3">
      <SectionTitle>{title}</SectionTitle>
      <div className="flex flex-col gap-3">
        {buckets
          .filter((bucket) => bucket.rounds > 0)
          .map((bucket) => (
            <RateRow key={bucket.pattern} label={patternLabel(bucket.pattern)} rate={bucket.share} />
          ))}
      </div>
    </section>
  );
}

/** Without a team or side the aggregate cannot own a buy, a win or a duel; shapes and timings still hold. */
export function SideAgnosticTendencies({ tendencies }: { tendencies: TacticalTendencies }): ReactNode {
  return (
    <div className="flex flex-col gap-8">
      <p className="rounded-md border border-primary/45 bg-primary/10 px-3 py-2.5 text-body-sm leading-5 text-fg-1">
        Elige un equipo o un lado en el filtro para ver economía, victorias y duelos de apertura: sin perspectiva el
        agregado no puede atribuir una compra ni una ronda ganada a nadie. El reparto de formas y los tiempos sí son
        válidos tal cual.
      </p>
      <div className="grid gap-8 lg:grid-cols-2">
        <ShapeShares title="Formas en T · reparto" buckets={tendencies.t_patterns} />
        <ShapeShares title="Formas en CT · reparto" buckets={tendencies.ct_patterns} />
      </div>
      <Timings tendencies={tendencies} />
    </div>
  );
}
