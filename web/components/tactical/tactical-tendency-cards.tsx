import type { ReactNode } from 'react';
import type { TacticalRate, TacticalTendencies } from '@/lib/api/tactical';
import { openingLines, percentText, rateTone, tendencyCards } from '@/lib/tactical-tendency-view';
import type { OutcomeTone, TendencyCard } from '@/lib/tactical-tendency-view';
import { RateBar } from '@/components/tactical/tactical-charts';
import { RoundsLink } from '@/components/tactical/tendency-links';
import { cn } from '@/lib/utils';

const TONE_TEXT: Record<OutcomeTone, string> = {
  win: 'text-outcome-win',
  loss: 'text-outcome-loss',
  even: 'text-fg-2',
};

const WIN_ROW = 'grid grid-cols-[minmax(5.5rem,1fr)_2.75rem_minmax(2.5rem,6rem)_3.5rem] items-center gap-2.5';
const OPENER_ROW = 'grid grid-cols-[minmax(0,1fr)_4rem] gap-2.5';

function CardFrame({ title, hint, children }: { title: string; hint: string; children: ReactNode }): ReactNode {
  return (
    <div className="flex min-w-0 flex-col gap-2.5 rounded-md border border-border-subtle bg-surface-2 p-3.5">
      <div className="flex flex-col gap-0.5">
        <h3 className="text-body-sm font-semibold text-fg-1">{title}</h3>
        <p className="text-meta text-fg-3">{hint}</p>
      </div>
      {children}
    </div>
  );
}

function SmallMark(): ReactNode {
  return <span className="shrink-0 text-meta text-fg-3">pocas</span>;
}

function WinCard({ card }: { card: TendencyCard }): ReactNode {
  return (
    <CardFrame title={card.title} hint={card.hint}>
      <div role="table" aria-label={card.title} className="flex flex-col gap-2">
        <div role="row" className={cn(WIN_ROW, 'text-meta text-fg-3')}>
          <span role="columnheader">{card.column}</span>
          <span role="columnheader" className="text-right">
            Rondas
          </span>
          <span role="columnheader" aria-label="Barra de victorias" />
          <span role="columnheader" className="text-right">
            Gana
          </span>
        </div>
        {card.rows.map((row) => (
          <div key={row.key} role="row" className={cn(WIN_ROW, 'text-body-sm', row.small && 'opacity-70')}>
            <span role="rowheader" className="flex min-w-0 items-center gap-1.5">
              <span className="truncate text-fg-1">{row.label}</span>
              {row.small ? <SmallMark /> : null}
            </span>
            <span role="cell" title={row.shareText} className="text-right tabular-nums text-fg-2">
              <RoundsLink patch={row.patch}>{row.rounds}</RoundsLink>
            </span>
            <span role="cell">
              <RateBar rate={row.win} className={TONE_TEXT[row.tone]} />
            </span>
            <span role="cell" className={cn('text-right font-semibold tabular-nums', TONE_TEXT[row.tone])}>
              {row.winText}
            </span>
          </div>
        ))}
      </div>
    </CardFrame>
  );
}

function DuelStat({
  rate,
  label,
  empty,
  toned = false,
}: {
  rate: TacticalRate;
  label: string;
  empty: string;
  toned?: boolean;
}): ReactNode {
  if (rate.total === 0) {
    return (
      <div className="flex flex-col justify-end">
        <span className="text-body-sm text-fg-3">{empty}</span>
        <span className="text-meta text-fg-3">{label}</span>
      </div>
    );
  }
  return (
    <div className={cn('flex flex-col', !rate.reliable && 'opacity-70')} title={`${rate.count} de ${rate.total}`}>
      <span className={cn('text-stat font-bold tabular-nums', toned ? TONE_TEXT[rateTone(rate)] : 'text-fg-1')}>
        {percentText(rate.pct)}
      </span>
      <span className="flex items-center gap-1.5 text-meta text-fg-3">
        {label}
        {rate.reliable ? null : <SmallMark />}
      </span>
    </div>
  );
}

function OpeningCard({ tendencies }: { tendencies: TacticalTendencies }): ReactNode {
  const lines = openingLines(tendencies.players);
  return (
    <CardFrame title="Duelo de apertura" hint="La primera baja de cada ronda">
      <div className="flex flex-wrap gap-x-6 gap-y-2">
        <DuelStat rate={tendencies.openings.won} label="ganados" empty="Sin duelos" toned />
        <DuelStat rate={tendencies.openings.traded_on_loss} label="vengados al perderlo" empty="No perdió ninguno" />
      </div>
      {lines.length === 0 ? null : (
        <div role="table" aria-label="Duelos de apertura por jugador" className="flex flex-col gap-1">
          <div role="row" className={cn(OPENER_ROW, 'text-meta text-fg-3')}>
            <span role="columnheader">Jugador</span>
            <span role="columnheader" className="text-right">
              Gana-pierde
            </span>
          </div>
          {lines.map((line) => (
            <div key={line.key} role="row" className={cn(OPENER_ROW, 'text-body-sm')}>
              <span role="rowheader" className="truncate text-fg-1">
                <RoundsLink patch={line.patch}>{line.name}</RoundsLink>
              </span>
              <span role="cell" className={cn('text-right font-semibold tabular-nums', TONE_TEXT[line.tone])}>
                {line.text}
              </span>
            </div>
          ))}
        </div>
      )}
    </CardFrame>
  );
}

/** One team's round outcomes by economy, site and round shape, and its opening duels. */
export function TendencyCards({ tendencies }: { tendencies: TacticalTendencies }): ReactNode {
  return (
    <div className="grid grid-cols-[repeat(auto-fit,minmax(min(18.75rem,100%),1fr))] gap-4">
      {tendencyCards(tendencies).map((card) => (
        <WinCard key={card.key} card={card} />
      ))}
      <OpeningCard tendencies={tendencies} />
    </div>
  );
}
