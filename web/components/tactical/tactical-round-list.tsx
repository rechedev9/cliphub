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
  // --fg-4 is the hairline/graphics token (3.75:1 on --surface-2); --border is an
  // edge, and an edge token is not a surface fill.
  return 'bg-fg-4';
}

/** The economy line, always in CT-then-T order so rows scan vertically. */
function BuyLine({ round }: { round: TacticalRound }): ReactNode {
  return (
    <span className="block truncate font-mono text-meta uppercase tracking-wider">
      <span className="text-primary">{buyLabel(round.economy.ct_buy)}</span>{' '}
      <span className="text-muted-foreground">vs</span>{' '}
      <span className="text-warning">{buyLabel(round.economy.t_buy)}</span>
    </span>
  );
}

/**
 * One round of the index. The chevron opens the full classification record —
 * the tags the classifier attached, then the rules that produced them: a label
 * an analyst disagrees with has to be traceable, which is the whole point of a
 * deterministic classifier.
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
        // shrink-0 is load-bearing: the list is a flex column with a bounded
        // height, so without it 35 rounds compress to fit instead of
        // scrolling, and overflow-hidden then slices the text of every row.
        // .studio-panel already owns the radius; the selected step keeps the
        // --border-accent edge the raised recipe sets and adds the sanctioned
        // selected-tile glow instead of a stronger hue.
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
            // Two bands, not one row: the meta footline spans the whole button,
            // so the tag run gets ~256px instead of the ~180px the middle
            // column can spare. Both text spans truncate, so nothing grows with
            // content; min-h-20 is the height whenever the winner column shows
            // its gana/pierde line, and the floor rows without it sit on.
            'flex min-h-20 min-w-0 flex-1 flex-col justify-center gap-1.5 px-3 py-2.5 text-left outline-none',
            FOCUS_RING,
          )}
        >
          <span className="flex min-w-0 items-center gap-2">
            <span className="flex w-9 shrink-0 flex-col items-center gap-1">
              <span
                className={cn(
                  'font-mono text-title leading-none tabular-nums',
                  selected ? 'text-primary' : 'text-foreground',
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
              {/* One line, never two: the index column is pinned at 360px
                  (tactical-analysis.tsx) so this measure never grows, and the
                  site moved to the footline so the pair fits. The untruncated
                  string is reprinted in the replay header when the round is
                  selected. */}
              <span className="block truncate font-display text-label font-semibold uppercase leading-tight text-foreground">
                {tPatternLabel(round.class.t_side)}{' '}
                <span className="text-muted-foreground">/</span>{' '}
                {ctPatternLabel(round.class.ct_side)}
              </span>
            </span>

            <span className="flex shrink-0 flex-col items-end gap-1">
              <span
                className={cn(
                  // border-current inherits the side's semantic colour, so the
                  // marker reads as a HUD key without a new token.
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

          {/* Site first, then the tags, as one `·` run — the house idiom
              (tactical-event-list.tsx). tracking-normal rather than the
              mono-meta tracking-wider on purpose: at 0.12em a three-tag round
              loses nine characters of a ~256px band, and this is a dense data
              run, not a HUD label. Overflow ellipsises; the full list is in the
              drawer. */}
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
            'grid w-8 shrink-0 place-items-center border-l border-border-subtle text-muted-foreground outline-none transition-colors hover:bg-primary/10 hover:text-primary',
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
          <p className="font-mono text-body-sm leading-5 text-muted-foreground">
            El clasificador no registró ningún motivo para esta ronda.
          </p>
        ) : (
          <ul className="flex flex-col gap-1.5">
            {reasons.map((reason) => (
              <li
                key={reason}
                className="font-mono text-body-sm leading-5 text-muted-foreground break-words"
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
      // The command strip is a 56px opaque ceiling: sticking at 24px from the
      // viewport parked this header under it, so both the offset and the height
      // budget start below --shell-strip-height.
      className="studio-panel flex flex-col overflow-hidden xl:sticky xl:top-[calc(var(--shell-strip-height)+1.5rem)] xl:max-h-[calc(100vh-var(--shell-strip-height)-3rem)]"
      aria-label="Rondas"
    >
      <header className="flex items-center justify-between gap-3 border-b border-border-subtle px-3 py-3">
        <h2 className="font-mono text-meta uppercase tracking-widest text-fg-3">Rondas</h2>
        <span className="font-mono text-meta tracking-normal tabular-nums text-fg-2">{rounds.length}</span>
      </header>

      {rounds.length === 0 ? (
        <div className="flex flex-col items-center gap-3 px-6 py-12 text-center">
          <ScrollText className="size-5 text-muted-foreground" aria-hidden />
          <p className="text-body-sm leading-5 text-muted-foreground">
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
