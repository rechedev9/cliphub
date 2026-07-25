import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

export type LongOperationTone = 'primary' | 'stream';

const TONE_TEXT_CLASS = {
  primary: 'text-primary',
  stream: 'text-stream-text',
} as const satisfies Record<LongOperationTone, string>;

const TONE_FILL_CLASS = {
  primary: 'bg-primary',
  stream: 'bg-stream',
} as const satisfies Record<LongOperationTone, string>;

/**
 * Stopwatch shape, not the remaining-availability shape. `formatCountdown` in
 * lib/format renders "14h 3m", which reads as a deadline; an operation that has
 * been running for ninety seconds has to read as 01:30.
 */
function formatElapsed(seconds: number): string {
  const total = Math.max(0, Math.floor(seconds));
  const hours = Math.floor(total / 3600);
  const minutes = String(Math.floor((total % 3600) / 60)).padStart(2, '0');
  const secs = String(total % 60).padStart(2, '0');
  return hours > 0 ? `${hours}:${minutes}:${secs}` : `${minutes}:${secs}`;
}

export type LongOperationProps = {
  /** What the pipeline is doing right now, e.g. "CAPTURANDO". */
  stage: string;
  /** Secondary fact, e.g. "SEGMENTOS 2/4" or "CORTES + RITMO". */
  detail?: ReactNode;
  /**
   * Real progress, 0-100. Leave it out when the stage reports none: the bar goes
   * indeterminate rather than showing a number the pipeline never produced.
   */
  percent?: number;
  /** Seconds the operation has been running. The caller owns the clock. */
  elapsedSec?: number;
  tone?: LongOperationTone;
  className?: string;
};

/**
 * The shared "something slow is happening" surface for capture, render and
 * stream jobs: stage, determinate or indeterminate progress, and elapsed time in
 * tabular figures. Announced through `aria-live="polite"` because a local render
 * can run for minutes with no other feedback.
 */
export function LongOperation({
  stage,
  detail,
  percent,
  elapsedSec,
  tone = 'primary',
  className,
}: LongOperationProps): ReactNode {
  const pct = percent === undefined ? undefined : Math.min(100, Math.max(0, Math.round(percent)));
  const determinate = pct !== undefined;

  return (
    <div role="status" aria-live="polite" className={cn('flex flex-col gap-2', className)}>
      <div className="flex items-baseline justify-between gap-3">
        {/* Template literal, not cn(): tailwind-merge reads a custom `--text-*`
            key as a colour and would drop `text-meta` behind the tone class. */}
        <span className={`min-w-0 truncate font-mono text-meta uppercase ${TONE_TEXT_CLASS[tone]}`}>{stage}</span>
        <span className="flex shrink-0 items-baseline gap-2.5 font-mono text-meta tabular-nums">
          {determinate ? <span className={TONE_TEXT_CLASS[tone]}>{pct}%</span> : null}
          {elapsedSec !== undefined ? <span className="text-fg-3">{formatElapsed(elapsedSec)}</span> : null}
        </span>
      </div>

      <div
        role="progressbar"
        aria-label={stage}
        aria-valuemin={determinate ? 0 : undefined}
        aria-valuemax={determinate ? 100 : undefined}
        aria-valuenow={pct}
        className="h-1 w-full overflow-hidden bg-surface-0"
      >
        {determinate ? (
          <span
            className={cn('block h-full transition-[width] duration-(--dur-data) ease-standard', TONE_FILL_CLASS[tone])}
            style={{ width: `${pct}%` }}
          />
        ) : (
          // animate-pulse is the only zero-CSS indeterminate available today; it
          // is not covered by the efficiency profile. See the CSS request in the
          // kit report for the `.studio-indeterminate` sweep that replaces it.
          <span className={cn('block h-full w-2/5 animate-pulse', TONE_FILL_CLASS[tone])} />
        )}
      </div>

      {detail ? <p className="font-mono text-meta uppercase text-fg-3">{detail}</p> : null}
    </div>
  );
}
