'use client';

import { useCallback, useId, useState, type ReactNode } from 'react';
import { ChevronDown, ScrollText } from 'lucide-react';
import { TACTICAL_SIDES } from '@/lib/api/tactical';
import type { TacticalDocument, TacticalFilter, TacticalRound, TacticalSide } from '@/lib/api/tactical';
import { tacticalPerspective } from '@/lib/tactical-filter';
import { buyLabel, ctPatternLabel, roundTagLabel, siteLabel, tPatternLabel } from '@/lib/tactical-labels';
import { FOCUS_RING } from '@/components/ui/button';
import { StatusTag } from '@/components/studio/status-tag';
import { cn } from '@/lib/utils';

function sideClass(side: TacticalSide): string {
  return side === TACTICAL_SIDES.ct ? 'text-primary' : 'text-warning';
}

function sideBarClass(winner: TacticalSide | ''): string {
  if (winner === TACTICAL_SIDES.ct) return 'bg-primary';
  if (winner === TACTICAL_SIDES.t) return 'bg-warning';
  return 'bg-fg-4';
}

/** The economy line, always in CT-then-T order so rows scan vertically. */
function BuyLine({ round }: { round: TacticalRound }): ReactNode {
  return (
    <span className="block truncate font-mono text-meta uppercase tracking-wider">
      <span className="text-primary">{buyLabel(round.economy.ct_buy)}</span>{' '}
      <span className="text-fg-3">vs</span>{' '}
      <span className="text-warning">{buyLabel(round.economy.t_buy)}</span>
    </span>
  );
}

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
        'studio-panel shrink-0 overflow-hidden transition-colors',
        selected
          ? 'studio-panel-raised shadow-[var(--elev-3),var(--glow-primary-sm)]'
          : 'studio-panel-interactive',
      )}
    >
      <div className="flex items-stretch">
        <span className={cn('w-1 shrink-0', sideBarClass(round.winner))} aria-hidden />
        <button
          type="button"
          onClick={() => onSelect(round.number)}
          aria-current={selected ? 'true' : undefined}
          className={cn(
            'flex min-h-20 min-w-0 flex-1 flex-col justify-center gap-1.5 px-3 py-2.5 text-left outline-none',
            FOCUS_RING,
          )}
        >
          <span className="flex min-w-0 items-center gap-2">
            <span className="flex w-9 shrink-0 flex-col items-center gap-1">
              <span
                className={cn(
                  'font-mono text-title leading-none tabular-nums',
                  selected ? 'text-primary' : 'text-fg-1',
                )}
              >
                {round.number}
              </span>
              <span className="font-mono text-meta leading-none tracking-normal tabular-nums text-fg-3">
                {round.score_ct_before}:{round.score_t_before}
              </span>
            </span>

            <span className="flex min-w-0 flex-1 flex-col gap-1">
              <BuyLine round={round} />
              <span className="block truncate font-display text-label font-semibold uppercase leading-tight text-fg-1">
                {tPatternLabel(round.class.t_side)}{' '}
                <span className="text-fg-3">/</span>{' '}
                {ctPatternLabel(round.class.ct_side)}
              </span>
            </span>

            <span className="flex shrink-0 flex-col items-end gap-1">
              <span
                className={cn(
                  'inline-flex h-5 items-center justify-center border border-current px-1.5 font-mono text-meta leading-none tracking-wider',
                  round.winner === '' ? 'text-fg-3' : sideClass(round.winner),
                )}
              >
                {round.winner === '' ? '—' : round.winner}
              </span>
              {won ? (
                <span className="font-mono text-meta uppercase tracking-wider text-success">gana</span>
              ) : null}
              {lost ? (
                <span className="font-mono text-meta uppercase tracking-wider text-fg-3">pierde</span>
              ) : null}
            </span>
          </span>

          <span className="block truncate font-mono text-meta uppercase tracking-normal text-fg-3">
            <span className="text-fg-2">{siteLabel(round.class.site)}</span>
            {round.class.tags.map((tag) => (
              <span key={tag}>{` · ${roundTagLabel(tag)}`}</span>
            ))}
          </span>
        </button>

        <button
          type="button"
          onClick={() => setOpen((current) => !current)}
          aria-expanded={open}
          aria-controls={reasonsId}
          aria-label={`Por qué la ronda ${round.number} se clasificó así`}
          className={cn(
            'grid w-8 shrink-0 place-items-center border-l border-border-subtle text-fg-3 outline-none transition-colors hover:bg-primary/10 hover:text-primary',
            FOCUS_RING,
            'focus-visible:outline-offset-[-2px]',
          )}
        >
          <ChevronDown className={cn('size-4 transition-transform', open && 'rotate-180')} aria-hidden />
        </button>
      </div>

      <div id={reasonsId} hidden={!open} className="border-t border-border-subtle bg-surface-1 px-3 py-3">
        {round.class.tags.length > 0 ? (
          <div className="mb-3 flex flex-wrap gap-1.5">
            {round.class.tags.map((tag) => (
              <StatusTag key={tag}>{roundTagLabel(tag)}</StatusTag>
            ))}
          </div>
        ) : null}
        <p className="mb-2 font-mono text-meta uppercase tracking-widest text-fg-3">
          Por qué se clasificó así
        </p>
        {reasons.length === 0 ? (
          <p className="font-mono text-body-sm leading-5 text-fg-2">
            El clasificador no registró ningún motivo para esta ronda.
          </p>
        ) : (
          <ul className="flex flex-col gap-1.5">
            {reasons.map((reason) => (
              <li
                key={reason}
                className="font-mono text-body-sm leading-5 text-fg-2 break-words"
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
      className="studio-panel flex flex-col overflow-hidden xl:sticky xl:top-[calc(var(--shell-strip-height)+1.5rem)] xl:max-h-[calc(100vh-var(--shell-strip-height)-3rem)]"
      aria-label="Rondas"
    >
      <header className="flex items-center justify-between gap-3 border-b border-border-subtle px-3 py-3">
        <h2 className="font-mono text-meta uppercase tracking-widest text-fg-3">Rondas</h2>
        <span className="font-mono text-meta tracking-normal tabular-nums text-fg-2">{rounds.length}</span>
      </header>

      {rounds.length === 0 ? (
        <div className="flex flex-col items-center gap-3 px-6 py-12 text-center">
          <ScrollText className="size-5 text-fg-3" aria-hidden />
          <p className="text-body-sm leading-5 text-fg-2">
            Ninguna ronda cumple el filtro actual.
          </p>
        </div>
      ) : (
        <ul className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto overscroll-contain p-3 scroll-py-3 [scrollbar-gutter:stable]">
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
