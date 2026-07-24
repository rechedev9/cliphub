'use client';

import { useCallback, useId, useState, type ReactNode } from 'react';
import { ChevronDown, ScrollText } from 'lucide-react';
import { TACTICAL_SIDES } from '@/lib/api/tactical';
import type { TacticalDocument, TacticalFilter, TacticalRound, TacticalSide } from '@/lib/api/tactical';
import { tacticalPerspective } from '@/lib/tactical-filter';
import { buyLabel, ctPatternLabel, roundTagLabel, siteLabel, tPatternLabel } from '@/lib/tactical-labels';
import { cn } from '@/lib/utils';

function sideClass(side: TacticalSide): string {
  return side === TACTICAL_SIDES.ct ? 'text-primary' : 'text-warning';
}

function sideBarClass(winner: TacticalSide | ''): string {
  if (winner === TACTICAL_SIDES.ct) return 'bg-primary';
  if (winner === TACTICAL_SIDES.t) return 'bg-warning';
  return 'bg-border';
}

/** The economy line, always in CT-then-T order so rows scan vertically. */
function BuyLine({ round }: { round: TacticalRound }): ReactNode {
  return (
    <span className="flex min-w-0 flex-wrap items-center gap-x-1.5 font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.1em]">
      <span className="text-primary">{buyLabel(round.economy.ct_buy)}</span>
      <span className="text-muted-foreground">vs</span>
      <span className="text-warning">{buyLabel(round.economy.t_buy)}</span>
    </span>
  );
}

/**
 * One round of the index. The reasons the classifier recorded are one click
 * away on every row: a label an analyst disagrees with has to be traceable to
 * the rule that produced it, which is the whole point of a deterministic
 * classifier.
 */
function RoundRow({
  round,
  perspective,
  selected,
  onSelect,
}: {
  round: TacticalRound;
  perspective: TacticalSide | undefined;
  selected: boolean;
  onSelect: (round: number) => void;
}): ReactNode {
  const [open, setOpen] = useState(false);
  const reasonsId = useId();
  const reasons = round.class.reasons;
  const won = perspective !== undefined && round.winner === perspective;
  const lost = perspective !== undefined && round.winner !== '' && round.winner !== perspective;

  return (
    <li
      className={cn(
        'studio-panel overflow-hidden rounded-lg transition-colors',
        selected ? 'studio-panel-raised border-primary/70' : 'studio-panel-interactive',
      )}
    >
      <div className="flex items-stretch">
        <span className={cn('w-[3px] shrink-0', sideBarClass(round.winner))} aria-hidden />
        <button
          type="button"
          onClick={() => onSelect(round.number)}
          aria-current={selected ? 'true' : undefined}
          className="flex min-h-[68px] min-w-0 flex-1 items-center gap-3 px-3 py-2.5 text-left outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
        >
          <span className="flex w-12 shrink-0 flex-col items-start gap-0.5">
            <span
              className={cn(
                'font-[family-name:var(--font-mono)] text-lg leading-none tabular-nums',
                selected ? 'text-primary' : 'text-foreground',
              )}
            >
              {round.number}
            </span>
            <span className="font-[family-name:var(--font-mono)] text-[11px] leading-none tabular-nums text-muted-foreground">
              {round.score_ct_before}:{round.score_t_before}
            </span>
          </span>

          <span className="flex min-w-0 flex-1 flex-col gap-1">
            <BuyLine round={round} />
            <span className="min-w-0 break-words font-[family-name:var(--font-display)] text-[13px] font-semibold uppercase leading-tight tracking-[0.03em] text-foreground">
              {tPatternLabel(round.class.t_side)}
              <span className="px-1 text-muted-foreground">/</span>
              {ctPatternLabel(round.class.ct_side)}
              <span className="px-1 text-muted-foreground">·</span>
              <span className="text-muted-foreground">{siteLabel(round.class.site)}</span>
            </span>
            {round.class.tags.length > 0 ? (
              <span className="flex flex-wrap gap-1">
                {round.class.tags.map((tag) => (
                  <span
                    key={tag}
                    className="rounded-sm border border-border/80 bg-background/45 px-1.5 py-px font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.1em] text-muted-foreground"
                  >
                    {roundTagLabel(tag)}
                  </span>
                ))}
              </span>
            ) : null}
          </span>

          <span className="flex w-10 shrink-0 flex-col items-end gap-0.5">
            <span
              className={cn(
                'font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.12em]',
                round.winner === '' ? 'text-muted-foreground' : sideClass(round.winner),
              )}
            >
              {round.winner === '' ? '—' : round.winner}
            </span>
            {won ? (
              <span className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.12em] text-success">
                gana
              </span>
            ) : null}
            {lost ? (
              <span className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.12em] text-muted-foreground">
                pierde
              </span>
            ) : null}
          </span>
        </button>

        <button
          type="button"
          onClick={() => setOpen((current) => !current)}
          aria-expanded={open}
          aria-controls={reasonsId}
          aria-label={`Por qué la ronda ${round.number} se clasificó así`}
          className="grid w-10 shrink-0 place-items-center border-l border-border/60 text-muted-foreground outline-none transition-colors hover:bg-primary/10 hover:text-primary focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset"
        >
          <ChevronDown className={cn('size-4 transition-transform', open && 'rotate-180')} aria-hidden />
        </button>
      </div>

      <div id={reasonsId} hidden={!open} className="border-t border-border/60 bg-background/35 px-3 py-3">
        <p className="mb-2 font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.16em] text-muted-foreground">
          Por qué se clasificó así
        </p>
        {reasons.length === 0 ? (
          <p className="font-[family-name:var(--font-mono)] text-[12px] leading-5 text-muted-foreground">
            El clasificador no registró ningún motivo para esta ronda.
          </p>
        ) : (
          <ul className="flex flex-col gap-1.5">
            {reasons.map((reason) => (
              <li
                key={reason}
                className="font-[family-name:var(--font-mono)] text-[12px] leading-5 text-muted-foreground break-words"
              >
                {reason}
              </li>
            ))}
          </ul>
        )}
      </div>
    </li>
  );
}

/**
 * The filtered round index. Selecting a row loads that round into the replay;
 * the list holds exactly the rounds the tendencies were computed from.
 */
export function TacticalRoundList({
  doc,
  rounds,
  filter,
  selected,
  onSelect,
}: {
  doc: TacticalDocument;
  rounds: readonly TacticalRound[];
  filter: TacticalFilter;
  selected: number | null;
  onSelect: (round: number) => void;
}): ReactNode {
  const select = useCallback((round: number) => onSelect(round), [onSelect]);

  return (
    <section
      className="studio-panel flex flex-col rounded-xl xl:sticky xl:top-6 xl:max-h-[calc(100vh-6rem)]"
      aria-label="Rondas"
    >
      <header className="flex items-center justify-between gap-3 border-b border-border/60 px-4 py-3">
        <h2 className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
          Rondas
        </h2>
        <span className="font-[family-name:var(--font-mono)] text-[11px] tabular-nums text-muted-foreground">
          {rounds.length}
        </span>
      </header>

      {rounds.length === 0 ? (
        <div className="flex flex-col items-center gap-3 px-6 py-12 text-center">
          <ScrollText className="size-5 text-muted-foreground" aria-hidden />
          <p className="text-[13px] leading-5 text-muted-foreground">
            Ninguna ronda cumple el filtro actual.
          </p>
        </div>
      ) : (
        <ul className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto overscroll-contain p-3">
          {rounds.map((round) => (
            <RoundRow
              key={round.number}
              round={round}
              perspective={tacticalPerspective(doc.teams, filter, round)}
              selected={round.number === selected}
              onSelect={select}
            />
          ))}
        </ul>
      )}
    </section>
  );
}
