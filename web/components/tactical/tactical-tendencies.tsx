'use client';

import type { ReactNode } from 'react';
import { MIN_RELIABLE_SAMPLE, TACTICAL_BUY_TYPE_ORDER, TACTICAL_SITE_ORDER } from '@/lib/api/tactical';
import type { TacticalBuyType, TacticalSite, TacticalTendencies } from '@/lib/api/tactical';
import { buyLabel, patternLabel, siteLabel } from '@/lib/tactical-labels';
import {
  HistogramChart,
  RateBar,
  RateRow,
  RateValue,
  SmallSampleChip,
} from '@/components/tactical/tactical-charts';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';

const HEAD_CELL =
  'px-3 py-2 text-left font-mono text-meta font-normal uppercase tracking-wider text-fg-3';
const BODY_CELL = 'px-3 py-2 align-middle';

function SectionTitle({ children }: { children: ReactNode }): ReactNode {
  return (
    <h3 className="font-mono text-meta uppercase tracking-widest text-fg-3">
      {children}
    </h3>
  );
}

/** A cross-tab cell: the rate, its denominator, and nothing implied by colour alone. */
function CrossCell({
  rounds,
  pct,
  reliable,
}: {
  rounds: number;
  pct: number;
  reliable: boolean;
}): ReactNode {
  if (rounds === 0) {
    return <span className="font-mono text-meta text-fg-3">—</span>;
  }
  return (
    <span className="flex flex-col gap-1">
      <span className="flex items-baseline gap-2">
        <span
          className={cn(
            'font-mono text-meta tracking-normal tabular-nums',
            reliable ? 'text-foreground' : 'text-muted-foreground',
          )}
        >
          {pct.toFixed(1)} %
        </span>
        <span className="font-mono text-meta tracking-normal tabular-nums text-fg-3">
          n={rounds}
        </span>
      </span>
      {reliable ? null : <SmallSampleChip className="self-start" />}
    </span>
  );
}

/**
 * The aggregate answer to "what does this team do", over exactly the rounds the
 * filter selected.
 *
 * Every rate on this panel carries its denominator, and anything the aggregate
 * flagged below `MIN_RELIABLE_SAMPLE` rounds is marked as a small sample instead
 * of being presented as a tendency.
 */
export function TacticalTendenciesPanel({
  tendencies,
  error,
  roundCount,
}: {
  tendencies: TacticalTendencies | null;
  error: string | null;
  roundCount: number;
}): ReactNode {
  return (
    <section className="studio-panel px-5 py-5 sm:px-6 sm:py-6" aria-label="Tendencias">
      <header className="flex flex-wrap items-baseline justify-between gap-x-6 gap-y-2 border-b border-border-subtle pb-4">
        <h2 className="font-display text-title font-bold uppercase tracking-tight text-foreground">
          Tendencias
        </h2>
        <p className="font-mono text-meta uppercase tracking-wider text-fg-3">
          calculadas sobre las {roundCount} rondas filtradas · tasas con menos de {MIN_RELIABLE_SAMPLE} rondas
          marcadas
        </p>
      </header>

      {error !== null ? (
        <p className="mt-5 rounded-md border border-destructive/45 bg-destructive/10 px-3 py-2 font-mono text-body-sm leading-5 break-words text-destructive">
          No se pudieron calcular las tendencias: {error}
        </p>
      ) : null}

      {error === null && tendencies === null ? (
        <div className="mt-5 grid gap-4 lg:grid-cols-2" aria-hidden>
          <Skeleton className="h-56 w-full rounded-lg" />
          <Skeleton className="h-56 w-full rounded-lg" />
        </div>
      ) : null}

      {tendencies !== null && tendencies.round_count === 0 ? (
        <p className="mt-5 text-body leading-6 text-muted-foreground">
          El filtro actual no selecciona ninguna ronda, así que no hay nada que agregar.
        </p>
      ) : null}

      {tendencies !== null && tendencies.round_count > 0 ? (
        <TendenciesBody tendencies={tendencies} />
      ) : null}
    </section>
  );
}

function TendenciesBody({ tendencies }: { tendencies: TacticalTendencies }): ReactNode {
  const buys = tendencies.buys.filter((bucket) => bucket.rounds > 0);
  const ownBuys = TACTICAL_BUY_TYPE_ORDER.filter((buy) =>
    tendencies.matchups.some((bucket) => bucket.buy === buy && bucket.rounds > 0),
  );
  const opponentBuys = TACTICAL_BUY_TYPE_ORDER.filter((buy) =>
    tendencies.matchups.some((bucket) => bucket.opponent_buy === buy && bucket.rounds > 0),
  );
  const buySiteBuys = TACTICAL_BUY_TYPE_ORDER.filter((buy) =>
    tendencies.buy_sites.some((bucket) => bucket.buy === buy && bucket.rounds > 0),
  );
  const sites = TACTICAL_SITE_ORDER.filter((site) =>
    tendencies.buy_sites.some((bucket) => bucket.site === site && bucket.rounds > 0),
  );
  const winPct = tendencies.round_count > 0 ? (tendencies.wins / tendencies.round_count) * 100 : 0;
  // Without a perspective the aggregate cannot attribute a buy, a win or a duel
  // to anybody: every economy bucket collapses into "unknown" and every win rate
  // is zero by construction. Showing those numbers would be showing zeroes as
  // findings, so the panel asks for a side instead and keeps only what a
  // side-agnostic pass can honestly answer.
  const perspective = tendencies.perspective;

  const matchup = (buy: TacticalBuyType, opponent: TacticalBuyType) =>
    tendencies.matchups.find((bucket) => bucket.buy === buy && bucket.opponent_buy === opponent);
  const buySite = (buy: TacticalBuyType, site: TacticalSite) =>
    tendencies.buy_sites.find((bucket) => bucket.buy === buy && bucket.site === site);

  return (
    <div className="mt-5 flex flex-col gap-8">
      <div className="flex flex-wrap items-center gap-x-8 gap-y-3">
        <Headline label="Rondas" value={String(tendencies.round_count)} />
        <Headline
          label="Ganadas"
          value={
            perspective === undefined
              ? '—'
              : `${tendencies.wins}/${tendencies.round_count} (${winPct.toFixed(1)} %)`
          }
          warn={perspective !== undefined && tendencies.round_count < MIN_RELIABLE_SAMPLE}
        />
        <Headline label="Perspectiva" value={perspective ?? 'sin fijar'} />
      </div>

      {perspective === undefined ? (
        <p className="rounded-md border border-primary/45 bg-primary/10 px-3 py-2.5 text-body-sm leading-5 text-foreground">
          Elige un equipo o un lado en el filtro para ver economía, victorias y duelos de apertura: sin
          perspectiva el agregado no puede atribuir una compra ni una ronda ganada a nadie. El reparto de
          formas y los tiempos sí son válidos tal cual.
        </p>
      ) : null}

      {perspective === undefined ? null : (
      <section className="flex flex-col gap-3">
        <SectionTitle>Economía</SectionTitle>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[560px] border-collapse">
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
              {buys.map((bucket) => (
                <tr key={bucket.buy} className="border-b border-border-subtle last:border-b-0">
                  <th
                    scope="row"
                    className={cn(
                      BODY_CELL,
                      'text-left font-mono text-meta font-normal uppercase tracking-wider text-foreground',
                    )}
                  >
                    {buyLabel(bucket.buy)}
                  </th>
                  <td className={cn(BODY_CELL, 'font-mono text-meta tracking-normal tabular-nums text-muted-foreground')}>
                    {bucket.rounds}
                  </td>
                  <td className={BODY_CELL}>
                    <div className="flex flex-col gap-1.5">
                      <RateBar rate={bucket.share} className="text-primary" />
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
      )}

      {perspective !== undefined && ownBuys.length > 0 ? (
        <section className="flex flex-col gap-3">
          <SectionTitle>Compra propia contra compra rival · victorias</SectionTitle>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[560px] border-collapse">
              <thead>
                <tr className="border-b border-border-subtle">
                  <th scope="col" className={HEAD_CELL}>
                    Propia \ rival
                  </th>
                  {opponentBuys.map((buy) => (
                    <th key={buy} scope="col" className={HEAD_CELL}>
                      {buyLabel(buy)}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {ownBuys.map((buy) => (
                  <tr key={buy} className="border-b border-border-subtle last:border-b-0">
                    <th
                      scope="row"
                      className={cn(
                        BODY_CELL,
                        'text-left font-mono text-meta font-normal uppercase tracking-wider text-foreground',
                      )}
                    >
                      {buyLabel(buy)}
                    </th>
                    {opponentBuys.map((opponent) => {
                      const bucket = matchup(buy, opponent);
                      return (
                        <td key={opponent} className={BODY_CELL}>
                          <CrossCell
                            rounds={bucket?.rounds ?? 0}
                            pct={bucket?.win_rate.pct ?? 0}
                            reliable={bucket?.win_rate.reliable ?? false}
                          />
                        </td>
                      );
                    })}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      ) : null}

      {perspective !== undefined && buySiteBuys.length > 0 ? (
        <section className="flex flex-col gap-3">
          <SectionTitle>Con cada economía, a qué sitio van · reparto</SectionTitle>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[560px] border-collapse">
              <thead>
                <tr className="border-b border-border-subtle">
                  <th scope="col" className={HEAD_CELL}>
                    Economía \ sitio
                  </th>
                  {sites.map((site) => (
                    <th key={site} scope="col" className={HEAD_CELL}>
                      {siteLabel(site)}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {buySiteBuys.map((buy) => (
                  <tr key={buy} className="border-b border-border-subtle last:border-b-0">
                    <th
                      scope="row"
                      className={cn(
                        BODY_CELL,
                        'text-left font-mono text-meta font-normal uppercase tracking-wider text-foreground',
                      )}
                    >
                      {buyLabel(buy)}
                    </th>
                    {sites.map((site) => {
                      const bucket = buySite(buy, site);
                      return (
                        <td key={site} className={BODY_CELL}>
                          <CrossCell
                            rounds={bucket?.rounds ?? 0}
                            pct={bucket?.share.pct ?? 0}
                            reliable={bucket?.share.reliable ?? false}
                          />
                        </td>
                      );
                    })}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      ) : null}

      <div className="grid gap-8 lg:grid-cols-2">
        <section className="flex min-w-0 flex-col gap-3">
          <SectionTitle>
            Formas en T · reparto{perspective === undefined ? '' : ' y victorias'}
          </SectionTitle>
          <div className="flex flex-col gap-3">
            {tendencies.t_patterns
              .filter((bucket) => bucket.rounds > 0)
              .map((bucket) => (
                <RateRow
                  key={bucket.pattern}
                  label={patternLabel(bucket.pattern)}
                  rate={bucket.share}
                  meta={
                    perspective === undefined ? undefined : (
                      <WinRateNote pct={bucket.win_rate.pct} total={bucket.win_rate.total} />
                    )
                  }
                />
              ))}
          </div>
        </section>

        <section className="flex min-w-0 flex-col gap-3">
          <SectionTitle>
            Formas en CT · reparto{perspective === undefined ? '' : ' y victorias'}
          </SectionTitle>
          <div className="flex flex-col gap-3">
            {tendencies.ct_patterns
              .filter((bucket) => bucket.rounds > 0)
              .map((bucket) => (
                <RateRow
                  key={bucket.pattern}
                  label={patternLabel(bucket.pattern)}
                  rate={bucket.share}
                  meta={
                    perspective === undefined ? undefined : (
                      <WinRateNote pct={bucket.win_rate.pct} total={bucket.win_rate.total} />
                    )
                  }
                />
              ))}
          </div>
        </section>
      </div>

      {perspective === undefined ? null : (
        <section className="flex flex-col gap-3">
          <SectionTitle>Duelo de apertura · {tendencies.openings.rounds} rondas con duelo</SectionTitle>
          <div className="flex flex-col gap-3">
            <RateRow label="Aperturas ganadas" rate={tendencies.openings.won} />
            <RateRow label="Tradeadas al perderla" rate={tendencies.openings.traded_on_loss} tone="muted" />
            <RateRow
              label="Ronda ganada tras ganarla"
              rate={tendencies.openings.round_win_after_opening_win}
              tone="success"
            />
            <RateRow
              label="Ronda ganada tras perderla"
              rate={tendencies.openings.round_win_after_opening_loss}
              tone="success"
            />
          </div>
        </section>
      )}

      <section className="flex flex-col gap-3">
        <SectionTitle>Tiempos · segundos desde el fin del congelado</SectionTitle>
        <div className="grid gap-6 lg:grid-cols-3">
          <HistogramChart histogram={tendencies.timings.first_contact} title="Primer contacto" />
          <HistogramChart histogram={tendencies.timings.plant} title="Plantada" />
          <HistogramChart histogram={tendencies.timings.round_duration} title="Duración de ronda" />
        </div>
      </section>
    </div>
  );
}

function Headline({
  label,
  value,
  warn = false,
}: {
  label: string;
  value: string;
  warn?: boolean;
}): ReactNode {
  return (
    <div className="flex flex-col gap-1">
      <span className="font-mono text-meta uppercase tracking-widest text-fg-3">
        {label}
      </span>
      <span className="flex items-center gap-2">
        <span className="font-mono text-body-sm tabular-nums text-foreground">{value}</span>
        {warn ? <SmallSampleChip /> : null}
      </span>
    </div>
  );
}

/** The win rate that belongs next to a share, with its own denominator. */
function WinRateNote({ pct, total }: { pct: number; total: number }): ReactNode {
  return (
    <span className="font-mono text-meta uppercase tracking-wider tabular-nums text-fg-3">
      gana {total > 0 ? `${pct.toFixed(1)} %` : '—'} (n={total})
    </span>
  );
}
