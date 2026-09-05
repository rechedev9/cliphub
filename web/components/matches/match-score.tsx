import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

/**
 * Parse a "rounds-rounds" score string (e.g. "13-7") into its two halves.
 * Returns null for either side that is not a number so callers can fall back.
 *
 * This is the ONLY score parser in the app. `matches/[id]/page.tsx` used to ship
 * a second, regex-based copy whose edge cases diverged from this one (it
 * rejected "13 - 7", which `parseInt` accepts here) for the same domain concept,
 * two files apart in the import graph.
 */
export function parseScore(score: string): { ours: number | null; theirs: number | null } {
  const [left, right] = score.split('-', 2);
  const ours = Number.parseInt(left ?? '', 10);
  const theirs = Number.parseInt(right ?? '', 10);
  return {
    ours: Number.isNaN(ours) ? null : ours,
    theirs: Number.isNaN(theirs) ? null : theirs,
  };
}

/** A match is a win when our round count is strictly higher than theirs. */
export function isWin(score: string): boolean {
  const { ours, theirs } = parseScore(score);
  if (ours === null || theirs === null) return false;
  return ours > theirs;
}

type MatchOutcome = 'win' | 'loss' | 'draw';

type MatchScoreSize = 'md' | 'lg';

/**
 * 30px in a row, 40px on the detail header. Both are mono steps of the v4 scale
 * with the display tracking neutralised: a scoreboard figure is not a headline,
 * and `--text-display*`'s negative tracking closes mono counters that are meant
 * to stay on the tabular grid.
 */
const VALUE_SIZE_CLASS = {
  md: 'text-display-sm',
  lg: 'text-display',
} as const satisfies Record<MatchScoreSize, string>;

const RULE_SIZE_CLASS = {
  md: 'h-[3px]',
  lg: 'h-1',
} as const satisfies Record<MatchScoreSize, string>;

/** The under-rule is the win/loss cue that survives forced colours as geometry. */
const RULE_TONE_CLASS = {
  win: 'bg-primary',
  loss: 'bg-fg-4',
  draw: 'bg-border-strong',
} as const satisfies Record<MatchOutcome, string>;

const OURS_TONE_CLASS = {
  win: 'text-primary',
  loss: 'text-fg-1',
  draw: 'text-fg-1',
} as const satisfies Record<MatchOutcome, string>;

const THEIRS_TONE_CLASS = {
  win: 'text-fg-3',
  loss: 'text-fg-1',
  draw: 'text-fg-1',
} as const satisfies Record<MatchOutcome, string>;

const OUTCOME_LABEL = {
  win: 'victoria',
  loss: 'derrota',
  draw: 'empate',
} as const satisfies Record<MatchOutcome, string>;

function scoreOutcome(ours: number, theirs: number): MatchOutcome {
  if (ours > theirs) return 'win';
  if (ours < theirs) return 'loss';
  return 'draw';
}

export type MatchScoreProps = {
  /** Raw API score string; anything unparseable renders nothing. */
  score: string;
  size?: MatchScoreSize;
  className?: string;
};

/**
 * The round score, typeset as a scoreboard rather than as body text: mono
 * tabular figures at the largest step in the row, a hairline rule between the
 * two halves instead of the literal `" : "` spaces (which render as digit-width
 * figure spaces inside a `tabular-nums` box and jitter against the counters),
 * and a 3px under-rule in the outcome tone.
 *
 * `role="img"` + `aria-label` because the outcome is otherwise carried by colour
 * and geometry alone: the label spells out "13 a 7 ·
 * victoria" for assistive tech while the visual stays two numbers and a rule.
 */
export function MatchScore({ score, size = 'md', className }: MatchScoreProps): ReactNode {
  const { ours, theirs } = parseScore(score);
  if (ours === null || theirs === null) return null;
  const outcome = scoreOutcome(ours, theirs);

  return (
    <div
      role="img"
      aria-label={`Marcador ${ours} a ${theirs} · ${OUTCOME_LABEL[outcome]}`}
      className={cn('flex shrink-0 flex-col gap-1.5', className)}
    >
      <span
        className={cn(
          'flex items-center gap-2.5 font-mono leading-none tabular-nums tracking-normal',
          VALUE_SIZE_CLASS[size],
        )}
      >
        <span className={OURS_TONE_CLASS[outcome]}>{ours}</span>
        <span aria-hidden className="h-[0.72em] w-px shrink-0 bg-border-strong" />
        <span className={THEIRS_TONE_CLASS[outcome]}>{theirs}</span>
      </span>
      <span aria-hidden className={cn('w-full', RULE_SIZE_CLASS[size], RULE_TONE_CLASS[outcome])} />
    </div>
  );
}
