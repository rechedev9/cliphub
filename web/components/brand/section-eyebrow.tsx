import { cn } from '@/lib/utils';

export type SectionEyebrowProps = {
  /** Uppercase section label, e.g. "PARTIDAS", "BIBLIOTECA". */
  label: string;
  /** Optional section number, rendered as `// 0N — LABEL`. */
  number?: number;
  /** Optional count shown as a mono pill after the label. */
  count?: number;
  /** Signal color: cyan (default) everywhere, magenta on the stream route. */
  accent?: 'cyan' | 'magenta';
  className?: string;
};

export function SectionEyebrow({ label, number, count, accent = 'cyan', className }: SectionEyebrowProps) {
  return (
    <div className={cn('flex items-center gap-2', className)}>
      <span
        className={cn(
          'font-mono text-meta uppercase tracking-ultra',
          accent === 'magenta' ? 'text-stream-text' : 'text-primary',
        )}
      >
        {number !== undefined ? `// ${String(number).padStart(2, '0')} — ` : null}
        {label}
      </span>
      {count !== undefined ? (
        <span className="inline-flex min-w-6 items-center justify-center border border-border-strong px-1.5 py-0.5 font-mono text-meta tracking-normal tabular-nums text-fg-2">
          {count}
        </span>
      ) : null}
    </div>
  );
}
