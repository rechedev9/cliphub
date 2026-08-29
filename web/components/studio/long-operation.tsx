import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

type LongOperationTone = 'primary' | 'stream';

const TONE_TEXT_CLASS = {
  primary: 'text-primary',
  stream: 'text-stream-text',
} as const satisfies Record<LongOperationTone, string>;

const TONE_FILL_CLASS = {
  primary: 'bg-primary',
  stream: 'bg-stream',
} as const satisfies Record<LongOperationTone, string>;

/** Elapsed clock (01:30), not `formatCountdown`'s remaining-availability shape. */
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
  /** 0-100 from the pipeline. Omit when the stage reports none. */
  percent?: number;
  /** Seconds the operation has been running. The caller owns the clock. */
  elapsedSec?: number;
  tone?: LongOperationTone;
  className?: string;
};

/** Shared capture/render/stream progress: stage, bar, elapsed. `aria-live="polite"`. */
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
  const complete = pct === 100;

  return (
    <div
      role="status"
      aria-live="polite"
      data-complete={complete ? 'true' : undefined}
      className={cn('flex flex-col gap-2', className)}
    >
      <div className="flex items-baseline justify-between gap-3">
        {/* Template literal, not cn(): tailwind-merge reads a custom `--text-*`
            key as a colour and would drop `text-meta` behind the tone class. */}
        <span className={`flex min-w-0 items-center gap-2 font-mono text-meta uppercase ${TONE_TEXT_CLASS[tone]}`}>
          {complete ? (
            <svg viewBox="0 0 16 16" className="size-3 shrink-0" aria-hidden>
              <path className="studio-op-check-path" d="M3.5 8.5 6.5 11.5 12.5 4.5" />
            </svg>
          ) : null}
          <span className="min-w-0 truncate">{stage}</span>
        </span>
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
          <span className={cn('studio-indeterminate block h-full w-2/5', TONE_FILL_CLASS[tone])} />
        )}
      </div>

      {detail ? <p className="font-mono text-meta uppercase text-fg-3">{detail}</p> : null}
    </div>
  );
}
