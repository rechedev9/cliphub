import { cn } from '@/lib/utils';

export type WordmarkProps = {
  className?: string;
};

/**
 * ClipHub mark: a CH monogram inside a hexagon badge, with a play triangle
 * breaking out of the H. Uses semantic brand tokens so it stays legible in
 * high-contrast themes; the orange-to-purple lockup gradient lives in the
 * static asset at web/public/brand/cliphub-mark.svg.
 */
export function BrandMark({ className }: { className?: string }) {
  return (
    <svg
      aria-hidden
      viewBox="0 0 64 64"
      className={cn('size-8 shrink-0', className)}
      fill="none"
    >
      <path className="fill-surface-0 stroke-border" strokeWidth="2" strokeLinejoin="round" d="M20 5h24l16 27-16 27H20L4 32 20 5Z" />
      <path className="stroke-fg-1" strokeWidth="5.5" d="M31.4 23.9A8.5 8.5 0 1 0 31.4 36.1" />
      <path className="stroke-fg-1" strokeWidth="5.5" d="M39 21v15M48.5 21v15M39 28.5h9.5" />
      <path className="fill-primary" d="M34 37.5 48.5 45 34 52.5V37.5Z" />
    </svg>
  );
}

/** The full horizontal lockup used by the sidebar and standalone headers. */
export function Wordmark({ className }: WordmarkProps) {
  return (
    <span className={cn('inline-flex items-center gap-2.5', className)}>
      <BrandMark className="size-9" />
      <span className="font-display text-title font-bold leading-none tracking-wider text-fg-1">
        ClipHub
      </span>
    </span>
  );
}
