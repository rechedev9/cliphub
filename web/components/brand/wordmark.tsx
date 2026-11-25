import { cn } from '@/lib/utils';

export type WordmarkProps = {
  className?: string;
};

export function BrandMark({ className }: { className?: string }) {
  return (
    <svg
      aria-hidden
      viewBox="52 52 896 896"
      className={cn('size-8 shrink-0', className)}
      fill="none"
    >
      <path className="stroke-fg-1" strokeWidth="132" d="M685.8 667.3A250 250 0 1 1 685.8 332.7" />
      <path className="fill-brand-accent" d="M418 352 418 648 700 500Z" />
    </svg>
  );
}

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
