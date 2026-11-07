import type { ReactNode } from 'react';
import { ChevronRight } from 'lucide-react';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/** A `<details>` summary with the Studio chevron instead of the browser's triangle; it turns when open. */
export function DisclosureSummary({ children, className }: { children: ReactNode; className?: string }): ReactNode {
  return (
    <summary className={cn('flex cursor-pointer list-none items-center gap-1.5 [&::-webkit-details-marker]:hidden', FOCUS_RING, className)}>
      <ChevronRight
        aria-hidden
        className="size-4 shrink-0 text-fg-3 transition-transform duration-(--dur-fast) [details[open]>summary>&]:rotate-90"
      />
      {children}
    </summary>
  );
}
