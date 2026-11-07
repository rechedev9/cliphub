import type { ReactNode } from 'react';
import { compareDelta, deltaArrow, type DeltaTone } from '@/lib/analysis/compare';
import { cn } from '@/lib/utils';

export const DELTA_TONE_CLASS: Record<DeltaTone, string> = {
  better: 'text-outcome-win',
  worse: 'text-outcome-loss',
  even: 'text-fg-3',
};

const TONE_WORD: Record<DeltaTone, string> = { better: 'mejor', worse: 'peor', even: 'igual' };

/**
 * A number read against a reference the Skybox way, "52 % ▲ (24)": the arrow
 * gives the direction, its colour whether that is good, and n the sample.
 */
export function DeltaValue({
  value,
  format,
  reference,
  higherIsBetter = true,
  epsilon,
  n,
  referenceLabel,
  className,
}: {
  value: number | null;
  format: (value: number) => string;
  reference?: number | null;
  higherIsBetter?: boolean;
  epsilon?: number;
  /** Sample behind the value, shown in parentheses. */
  n?: number;
  /** What the reference is, read in the tooltip and by screen readers. */
  referenceLabel?: string;
  className?: string;
}): ReactNode {
  if (value === null) return <span className={cn('text-fg-3', className)}>Sin dato</span>;
  const result = reference === undefined || reference === null ? null : compareDelta(value, reference, { higherIsBetter, epsilon });
  const arrow = result === null || result.tone === 'even' ? '' : deltaArrow(result.delta, epsilon);
  const title =
    result !== null && reference !== undefined && reference !== null
      ? `${referenceLabel ?? 'Referencia'}: ${format(reference)}`
      : undefined;
  // An even result keeps the arrow's width, so values in a compared column stay aligned.
  let arrowMark: ReactNode = null;
  if (arrow !== '') {
    arrowMark = (
      <span aria-hidden className={cn('text-[0.7em]', result ? DELTA_TONE_CLASS[result.tone] : '')}>
        {arrow}
      </span>
    );
  } else if (result !== null) {
    arrowMark = <span aria-hidden className="invisible text-[0.7em]">▲</span>;
  }
  return (
    <span className={cn('inline-flex items-baseline gap-1 whitespace-nowrap tabular-nums', className)} title={title}>
      <span className="font-medium text-fg-1">{format(value)}</span>
      {arrowMark}
      {n === undefined ? null : <span className="text-meta text-fg-3">({n})</span>}
      {title && result ? <span className="sr-only">, {TONE_WORD[result.tone]} que {title}</span> : null}
    </span>
  );
}
