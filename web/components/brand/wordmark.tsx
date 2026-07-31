import { cn } from '@/lib/utils';

export type WordmarkProps = {
  className?: string;
};

/**
 * TickCut mark: demo timeline ticks cut by a vertical blade, with a rec LED.
 * Uses semantic brand tokens so it stays legible in high-contrast themes.
 */
export function BrandMark({ className }: { className?: string }) {
  return (
    <svg
      aria-hidden
      viewBox="0 0 64 64"
      className={cn('size-8 shrink-0', className)}
      fill="none"
    >
      <rect width="64" height="64" rx="14" className="fill-surface-0" />
      <path className="stroke-fg-1" strokeWidth="2.5" strokeLinecap="round" d="M10 34h44" />
      <path className="stroke-fg-1" strokeWidth="2" strokeLinecap="round" d="M16 34v-5M28 34v-9M40 34v-5M52 34v-7" />
      <path className="stroke-primary" strokeWidth="3" strokeLinecap="round" d="M22 34v-12M34 34v-16M46 34v-12" />
      <path className="fill-fg-1" d="M38.5 12.5 20 48.5l4.2 2.2 18.5-36-4.2-2.2Z" />
      <rect x="48" y="10" width="6" height="6" rx="1.2" className="fill-brand-accent" />
    </svg>
  );
}

/** The full horizontal lockup used by the sidebar and standalone headers. */
export function Wordmark({ className }: WordmarkProps) {
  return (
    <span className={cn('inline-flex items-center gap-2.5', className)}>
      <BrandMark className="size-9" />
      <span className="font-display text-title font-bold leading-none tracking-wider text-fg-1">
        TickCut
      </span>
    </span>
  );
}
