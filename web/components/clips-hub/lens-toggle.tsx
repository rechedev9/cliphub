import type { ReactNode } from 'react';
import Link from 'next/link';
import { HUB_LENS, hubHref, type HubLens } from '@/lib/clips/routes';
import { cn } from '@/lib/utils';

const LENS_LABEL = {
  partidas: 'Partidas',
  clips: 'Clips',
} as const satisfies Record<HubLens, string>;

export type LensToggleProps = {
  lens: HubLens;
  counts: Record<HubLens, number>;
};

/**
 * Segmented Partidas | Clips switch with a count on each side. Each half is a
 * real link to `?vista=`, so the lens is shareable and middle-clickable.
 */
export function LensToggle({ lens, counts }: LensToggleProps): ReactNode {
  return (
    <nav aria-label="Vista" className="flex font-mono text-label uppercase tracking-wide">
      {Object.values(HUB_LENS).map((key) => {
        const active = lens === key;
        return (
          <Link
            key={key}
            href={hubHref({ lens: key })}
            scroll={false}
            aria-current={active ? 'page' : undefined}
            className={cn(
              'inline-flex min-h-10 items-center gap-2 px-4 transition-colors duration-(--dur-fast) focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring',
              active
                ? 'border border-primary bg-primary text-primary-foreground'
                : 'border border-border-strong text-fg-2 hover:text-fg-1 not-first:border-l-0',
            )}
          >
            {LENS_LABEL[key]}
            <span className={cn('text-meta tabular-nums', active ? 'text-primary-foreground' : 'text-fg-3')}>
              {counts[key]}
            </span>
          </Link>
        );
      })}
    </nav>
  );
}
