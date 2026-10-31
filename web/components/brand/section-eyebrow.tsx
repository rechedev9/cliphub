import { cn } from '@/lib/utils';

export type SectionEyebrowProps = {
  /** Sentence-case section label, e.g. "Partidas", "Biblioteca". */
  label: string;
  /** Optional section number, rendered as `// 0N — LABEL`. */
  number?: number;
  /** Optional count shown as a pill after the label. */
  count?: number;
  /** Signal color: neutral ink (default) everywhere, magenta on the stream route. */
  accent?: 'cyan' | 'magenta';
  className?: string;
};

/**
 * SectionEyebrow: the canonical section head, a small sentence-case label in
 * the body face. Every section head in Studio renders through it. Small and
 * quiet so it frames a section without competing with the screen H1.
 * Without `number` it renders the bare label (e.g. "Layout"). `accent`
 * switches to magenta for the Clips de stream route (magenta = stream).
 */
export function SectionEyebrow({ label, number, count, accent = 'cyan', className }: SectionEyebrowProps) {
  return (
    <div className={cn('flex items-center gap-2', className)}>
      <span
        className={cn(
          'text-meta font-medium',
          accent === 'magenta' ? 'text-stream-text' : 'text-fg-2',
        )}
      >
        {number !== undefined ? `// ${String(number).padStart(2, '0')} — ` : null}
        {label}
      </span>
      {count !== undefined ? (
        <span className="inline-flex min-w-6 items-center justify-center border border-border-strong px-1.5 py-0.5 text-meta tabular-nums text-fg-2">
          {count}
        </span>
      ) : null}
    </div>
  );
}
